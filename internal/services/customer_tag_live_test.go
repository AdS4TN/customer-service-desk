package services

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Credentials are read-only; synthetic conversations stay in the isolated test DB.
func TestCustomerTagLiveExtraction(t *testing.T) {
	path := os.Getenv("AGENT_DESK_RECEPTION_LIVE_DB")
	if path == "" {
		t.Skip("opt in with AGENT_DESK_RECEPTION_LIVE_DB and AGENT_DESK_RECEPTION_LIVE_CONFIG")
	}
	id, err := strconv.ParseInt(os.Getenv("AGENT_DESK_RECEPTION_LIVE_CONFIG"), 10, 64)
	if err != nil {
		t.Fatal("valid model ID required")
	}
	source, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatal("cannot read configured database")
	}
	conn, _ := source.DB()
	defer conn.Close()
	var config models.AIConfig
	if source.First(&config, id).Error != nil {
		t.Fatal("model configuration unavailable")
	}
	for _, tc := range []struct {
		name, text string
		categories []string
		leadCount  int
	}{
		{"greeting", "你好，666，谢谢！", nil, 0},
		{"product_question", "Model P 这款的防水等级是什么？我只是先了解性能。", []string{"product", "inquiry_type"}, 0},
		{"after_sales", "上个月买的产品坏了，我要申请退款，不需要购买新产品。", []string{"service_need"}, 0},
		{"profile_and_quote", "我是 Alex，来自 Example Trading 公司，我在加拿大。我的邮箱是 buyer@example.com，需要采购200件 Model P，请报价。", []string{"product", "inquiry_type"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c, m := setupMemoryTest(t)
			createProfileCustomer(t, db, c)
			a := AIAgentService.Get(c.AIAgentID)
			configured := config
			configured.ID = a.AIConfigID
			if err := db.Save(&configured).Error; err != nil {
				t.Fatal("test model copy failed")
			}
			if err := db.Model(&m).Update("content", tc.text).Error; err != nil {
				t.Fatal(err)
			}
			s := &conversationMemoryService{complete: func(ctx context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
				result, err := ai.LLM.ChatWithConfig(ctx, cfg, system, input)
				if err == nil && result != nil {
					var output struct {
						Entries []memoryCandidate `json:"entries"`
					}
					if json.Unmarshal([]byte(result.Content), &output) == nil {
						for _, e := range output.Entries {
							if e.Kind == string(enums.MemoryKindCustomerTag) && (e.Evidence == "" || e.FieldKey != "") {
								t.Logf("tag contract: evidence_present=%t field_key_present=%t confirmed=%t deleted=%t", e.Evidence != "", e.FieldKey != "", e.Confirmed, e.Deleted)
							}
						}
					}
				}
				return result, err
			}}
			started := time.Now()
			if err := s.process(claimMemory(t, c)); err != nil {
				failLiveExtraction(t, err)
			}
			v, err := s.View(c.ID)
			if err != nil {
				t.Fatal(err)
			}
			categories := map[string]bool{}
			count := 0
			for _, entry := range v.Entries {
				if entry.Entry.Kind == string(enums.MemoryKindCustomerTag) {
					count++
					categories[entry.Entry.Topic] = true
				}
			}
			if tc.categories == nil && count != 0 {
				t.Fatal("greeting incorrectly tagged")
			}
			for _, category := range tc.categories {
				if !categories[category] {
					t.Errorf("missing category: %s", category)
				}
			}
			leads, err := repositories.SalesLeadRepository.ForConversation(db, c.ID)
			if err != nil || len(leads) != tc.leadCount {
				t.Fatalf("unexpected lead count: %d", len(leads))
			}
			if tc.name == "profile_and_quote" {
				customer := CustomerService.Get(c.CustomerID)
				if customer.Name != "Alex" || customer.PrimaryEmail != "buyer@example.com" {
					t.Fatal("customer profile not automatically filled")
				}
				fields := map[string]string{}
				for _, entry := range v.Dossier {
					if entry.Entry.Kind == string(enums.MemoryKindCustomerProfile) {
						fields[entry.Entry.Topic] = entry.Entry.Value
					}
				}
				if !strings.Contains(fields["company"], "Example Trading") || !strings.Contains(fields["region"], "加拿大") {
					t.Fatalf("synthetic company or region missing: %v", fields)
				}
				correction := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, ClientMsgID: "profile-live-correction", Content: "这笔订单数量改为300件。邮箱也改为 new@example.com，不要再使用之前的邮箱。"}
				if err := db.Create(&correction).Error; err != nil {
					t.Fatal(err)
				}
				if err := s.process(claimMemory(t, c)); err != nil {
					failLiveExtraction(t, err)
				}
				updated, err := repositories.SalesLeadRepository.ForConversation(db, c.ID)
				if err != nil || len(updated) != 1 || updated[0].ID != leads[0].ID {
					t.Fatal("follow-up duplicated inquiry")
				}
				var data request.LeadData
				_ = json.Unmarshal([]byte(updated[0].Data), &data)
				if !strings.Contains(data.Quantity, "300") || CustomerService.Get(c.CustomerID).PrimaryEmail != "new@example.com" {
					t.Fatal("correction not reflected in lead and profile")
				}
			}
			var sent int64
			db.Model(&models.Message{}).Where("conversation_id = ? AND sender_type <> ?", c.ID, enums.IMSenderTypeCustomer).Count(&sent)
			if sent != 0 {
				t.Fatal("extraction sent a reply")
			}
			t.Logf("%s: %d tags, %d leads, %s", tc.name, count, tc.leadCount, time.Since(started).Round(time.Millisecond))
		})
	}
}

func failLiveExtraction(t *testing.T, err error) {
	t.Helper()
	var stage *memoryStageError
	if errors.As(err, &stage) {
		switch stage.code {
		case "memory_validation_failed", "inquiry_validation_failed", "lead_validation_failed":
			// Only local validators for synthetic input, never provider/SQL errors.
			t.Fatalf("live extraction: %s: %v", stage.code, stage.cause)
		}
	}
	t.Fatalf("live extraction: %s; provider details omitted", memoryFailureCode(err))
}

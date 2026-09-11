package services

import (
	"encoding/json"
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

// Only read model settings from the live DB. All messages and writes are isolated;
// no channel sender is invoked by this test.
func TestLeadLiveExtraction(t *testing.T) {
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
		count      int
		correction string
	}{
		{"quote", "我是 Alex，来自 Example Trading，需要采购200件Model P发往加拿大，请报价，也想先拿样品。比较着急。请通过buyer@example.com联系我。", 1, "数量改为300件，不要样品了。不要通过邮件联系我，先在这里报价。"},
		{"greeting", "你好，谢谢！", 0, ""},
		{"support", "上个月买的产品坏了，我要投诉，请帮我处理退款。不要推销新产品。", 0, ""},
		{"contact_only", "我的邮箱是buyer@example.com。", 0, ""},
		{"separate_orders", "请分别报价两个独立订单：订单A采购200件Model P发加拿大，订单B采购50件Model Q发德国。", 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c, m := setupMemoryTest(t)
			a := AIAgentService.Get(c.AIAgentID)
			configured := config
			configured.ID = a.AIConfigID
			if err := db.Save(&configured).Error; err != nil {
				t.Fatal("test model copy failed")
			}
			if err := db.Model(&m).Update("content", tc.text).Error; err != nil {
				t.Fatal(err)
			}
			s := &conversationMemoryService{complete: ai.LLM.ChatWithConfig}
			started := time.Now()
			if err := s.process(claimMemory(t, c)); err != nil {
				t.Fatal("live lead extraction failed; provider details omitted")
			}
			rows, err := repositories.SalesLeadRepository.ForConversation(db, c.ID)
			if err != nil || len(rows) != tc.count {
				t.Fatalf("got %d leads, want %d", len(rows), tc.count)
			}
			if tc.name == "quote" {
				var data request.LeadData
				json.Unmarshal([]byte(rows[0].Data), &data)
				if data.Email != "buyer@example.com" || !strings.Contains(data.Quantity, "200") {
					t.Fatal("explicit contact/quantity missing")
				}
				tags := map[enums.LeadTag]bool{}
				for _, tag := range data.AutoTags {
					tags[tag] = true
				}
				for _, tag := range []enums.LeadTag{enums.LeadTagQuote, enums.LeadTagSample, enums.LeadTagBulk, enums.LeadTagUrgent, enums.LeadTagContact} {
					if !tags[tag] {
						t.Errorf("missing tag %s", tag)
					}
				}
				m2 := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: tc.correction, ClientMsgID: "lead-live-correction"}
				if err := db.Create(&m2).Error; err != nil {
					t.Fatal(err)
				}
				if err := s.process(claimMemory(t, c)); err != nil {
					t.Fatal("live correction extraction failed")
				}
				updated, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
				if len(updated) != 1 || updated[0].ID != rows[0].ID {
					t.Fatal("correction duplicated purchase")
				}
				json.Unmarshal([]byte(updated[0].Data), &data)
				if !strings.Contains(data.Quantity, "300") || data.Email != "" {
					t.Fatal("correction not reflected")
				}
				for _, tag := range data.AutoTags {
					if tag == enums.LeadTagSample || tag == enums.LeadTagContact {
						t.Errorf("obsolete tag %s retained", tag)
					}
				}
			}
			var count int64
			db.Model(&models.Message{}).Where("conversation_id = ? AND sender_type <> ?", c.ID, enums.IMSenderTypeCustomer).Count(&count)
			if count != 0 {
				t.Fatal("extraction sent a message")
			}
			t.Logf("%s: %d leads, %s", tc.name, tc.count, time.Since(started).Round(time.Millisecond))
		})
	}
}

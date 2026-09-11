package services

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"agent-desk/internal/ai"
	"agent-desk/internal/ai/rag"
	"agent-desk/internal/ai/runtime/retrievers"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/enums"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Live generation uses synthetic conversations and synthetic retrieval evidence.
// Only the model configuration is read from the user's database; no channel runs.
func TestCopilotLiveReception(t *testing.T) {
	path := os.Getenv("AGENT_DESK_RECEPTION_LIVE_DB")
	if path == "" {
		t.Skip("opt in with AGENT_DESK_RECEPTION_LIVE_DB and AGENT_DESK_RECEPTION_LIVE_CONFIG")
	}
	id, err := strconv.ParseInt(os.Getenv("AGENT_DESK_RECEPTION_LIVE_CONFIG"), 10, 64)
	if err != nil {
		t.Fatal("valid config ID required")
	}
	source, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatal("cannot read configured database")
	}
	conn, _ := source.DB()
	defer conn.Close()
	var live models.AIConfig
	if source.First(&live, id).Error != nil {
		t.Fatal("model configuration unavailable")
	}
	for _, tc := range []struct {
		name     string
		messages []string
		must     []string
	}{
		{"product", []string{"What material is Model P made of? Please answer in English."}, []string{"304"}},
		{"ambiguous_inquiry", []string{"I want to buy your products. Can you quote me? Please answer in English."}, nil},
		{"two_purchases", []string{"Please answer in English. Order A is 200 units of Model P for Canada. Order B is 50 units of Model Q for Germany. Summarize them separately."}, []string{"200", "50", "Canada", "Germany"}},
		{"correction", []string{"Please reply in English. I need 200 units of Model P for Canada.", "Correction: make that 300 units, not 200. Confirm the updated quantity."}, []string{"300"}},
		{"declined_question", []string{"Please answer in English. I do not want to share a budget or phone number. What is the warranty for Model P?"}, []string{"two"}},
		{"human_request", []string{"Please connect me to a human, not another AI reply. Answer in English."}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, c, m := setupCopilotTest(t)
			agent := AIAgentService.Get(c.AIAgentID)
			configured := live
			configured.ID = agent.AIConfigID
			if err := db.Save(&configured).Error; err != nil {
				t.Fatal("cannot copy test model")
			}
			for index, content := range tc.messages {
				if index == 0 {
					db.Model(&m).Update("content", content)
				} else {
					m = models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: content, ClientMsgID: "copilot-live-" + strconv.Itoa(index)}
					if err := db.Create(&m).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			db.Model(&c).Update("last_message_id", m.ID)
			if _, err := AIAgentService.PublishAIAgent(agent.ID, &dto.AuthPrincipal{UserID: 1, Username: "test"}); err != nil {
				t.Fatal(err)
			}
			agent = AIAgentService.Get(agent.ID)
			revision := models.AgentRevision{}
			db.First(&revision, agent.PublishedRevisionID)
			var definition map[string]any
			json.Unmarshal([]byte(revision.Definition), &definition)
			definition["agent"].(map[string]any)["knowledgeIds"] = "1"
			raw, _ := json.Marshal(definition)
			db.Model(&revision).Update("definition", string(raw))
			s := &conversationCopilotService{complete: ai.LLM.ChatWithConfig, retrieve: func(context.Context, models.AIAgent, string) (*retrievers.KnowledgeRetrieveResult, error) {
				evidence := "Model P is made of 304 stainless steel and has a two-year warranty. No price, inventory, quotation, or delivery time has been verified."
				return &retrievers.KnowledgeRetrieveResult{ContextText: evidence, ContextResults: []rag.RetrieveResult{{DocumentID: 1, ChunkID: 1, Content: evidence}}}, nil
			}}
			r, err := s.Suggest(context.Background(), c.ID)
			if err != nil {
				t.Fatal("live suggestion failed; provider details omitted")
			}
			for _, word := range tc.must {
				if !strings.Contains(strings.ToLower(r.Content), strings.ToLower(word)) {
					t.Errorf("required fact %q missing", word)
				}
			}
			if strings.Count(r.Content, "?") > 1 {
				t.Error("more than one question")
			}
			for _, forbidden := range []string{"being passed", "come back to you", "has been transferred", "**"} {
				if strings.Contains(strings.ToLower(r.Content), forbidden) {
					t.Errorf("unsupported commitment or format: %s", forbidden)
				}
			}
			t.Logf("%s | %d ms | %s", tc.name, r.DurationMs, r.Content)
		})
	}
}

package runtime

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	svc "agent-desk/internal/services"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Opt-in: real streaming model, synthetic messages/context, read-only config,
// no channel dispatch, conversation writes, tools or audit persistence.
func TestReplyLanguageLive(t *testing.T) {
	path := os.Getenv("AGENT_DESK_REPLY_LANGUAGE_LIVE_DB")
	if path == "" {
		t.Skip("set AGENT_DESK_REPLY_LANGUAGE_LIVE_DB and AGENT_DESK_REPLY_LANGUAGE_LIVE_CONFIG")
	}
	id, err := strconv.ParseInt(os.Getenv("AGENT_DESK_REPLY_LANGUAGE_LIVE_CONFIG"), 10, 64)
	if err != nil || id <= 0 {
		t.Fatal("valid model config ID required")
	}
	db, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	if err != nil {
		t.Fatal("cannot read model configuration")
	}
	conn, _ := db.DB()
	defer conn.Close()
	sqls.SetDB(db)
	var cfg models.AIConfig
	if db.First(&cfg, id).Error != nil {
		t.Fatal("model configuration missing")
	}
	history := []models.Message{}
	engine := &AgentLoopEngine{
		history: func(_ int64, before int64, _ int) []models.Message { return history },
		retrieve: func(context.Context, models.AIAgent, string) (string, int, error) {
			return "中文文档：AX-204 产品需要根据尺寸确认价格。", 1, nil
		},
		memory: func(models.Conversation) string {
			return "Untrusted old customer tags: 产品 AX-204；1200×1200、一扇开启扇；询问价格；旧沟通语言中文。These are old unconfirmed notes, not current requests."
		},
	}
	for i, tc := range []struct {
		name, text string
		script     *unicode.RangeTable
	}{
		{"english_greeting", "hello", unicode.Latin},
		{"english_smalltalk", "how are you", unicode.Latin},
		{"keep_english_question_marks", "？？", unicode.Latin},
		{"keep_english_emoji", "🙂", unicode.Latin},
		{"explicit_english", "speak English", unicode.Latin},
		{"switch_to_chinese", "你好，请用中文回答", unicode.Han},
		{"switch_to_arabic", "مرحبا، تحدث معي بالعربية", unicode.Arabic},
		{"keep_arabic_question_marks", "？？", unicode.Arabic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := languageMessage(int64(i*2+1), tc.text)
			req := RunInput{Conversation: models.Conversation{ID: 15, CustomerName: "访客测试"}, UserMessage: message, AIAgent: models.AIAgent{KnowledgeIDs: "1"}}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			turn := engine.prepareTurn(ctx, req, &svc.AgentRevisionSnapshot{})
			definitions := append(agentLoopToolDefinitions(turn), agentLoopDecisionTool)
			var decision *ConversationDecision
			execute := func(_ context.Context, call ai.ToolCall) (string, error) {
				if call.Name == "conversation_decision" {
					var err error
					decision, err = parseConversationDecision(call.Arguments)
					return `{"status":"recorded in synthetic test only"}`, err
				}
				return `{"status":"unavailable","reason":"Synthetic test; no external tools execute"}`, nil
			}
			started := time.Now()
			var first time.Duration
			result, err := einoAgentLoopStream(ctx, cfg, turn.SystemPrompt, turn.UserPrompt, definitions, 4, execute, func(delta string) {
				if first == 0 && delta != "" {
					first = time.Since(started)
				}
			})
			if err != nil {
				t.Fatal("live model failed; provider details omitted")
			}
			replyText, handoff, _, err := resolveAgentLoopReply(result.Content, decision)
			if err != nil || handoff {
				t.Fatal("synthetic greeting must produce an ordinary reply")
			}
			result.Content = replyText
			if !strings.ContainsFunc(result.Content, func(r rune) bool { return unicode.Is(tc.script, r) }) {
				t.Error("expected language script missing")
			}
			if tc.script != unicode.Han && strings.ContainsFunc(result.Content, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Error("unexpected Chinese in non-Chinese reply")
			}
			for _, term := range []string{"AX-204", "1200"} {
				if strings.Contains(result.Content, term) {
					t.Errorf("unrelated old product surfaced: %s", term)
				}
			}
			t.Logf("first=%s total=%s reply=%s", first.Round(time.Millisecond), time.Since(started).Round(time.Millisecond), result.Content)
			history = append(history, message)
			reply := languageMessage(message.ID+1, result.Content)
			reply.SenderType = enums.IMSenderTypeAI
			history = append(history, reply)
		})
	}
}

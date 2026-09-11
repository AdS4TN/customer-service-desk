package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
)

func languageMessage(id int64, text string) models.Message {
	return models.Message{ID: id, ConversationID: 15, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: text, SendStatus: enums.IMMessageStatusSent}
}

func TestReplyLanguageContextUsesOnlyCustomerTextBeforeTrigger(t *testing.T) {
	now := time.Now()
	staff := languageMessage(2, "STAFF_CHINESE")
	staff.SenderType = enums.IMSenderTypeAI
	recalled := languageMessage(3, "RECALLED")
	recalled.RecalledAt = &now
	failed := languageMessage(4, "FAILED")
	failed.SendStatus = enums.IMMessageStatusFailed
	media := languageMessage(5, "MEDIA_CHINESE")
	media.MessageType = enums.IMMessageTypeImage
	other := languageMessage(6, "OTHER_CUSTOMER")
	other.ConversationID = 16
	req := RunInput{Conversation: models.Conversation{ID: 15}, UserMessage: languageMessage(10, "？？")}
	context := agentLoopLanguageContext(req, []models.Message{languageMessage(1, "hello"), staff, recalled, failed, media, other, languageMessage(7, "how are you"), languageMessage(8, "？"), languageMessage(9, "12345"), languageMessage(11, "FUTURE_MESSAGE")})
	for _, excluded := range []string{"STAFF_CHINESE", "RECALLED", "FAILED", "MEDIA_CHINESE", "OTHER_CUSTOMER", "FUTURE_MESSAGE", "12345", "？？"} {
		if strings.Contains(context, excluded) {
			t.Fatalf("language context contains excluded sample %s", excluded)
		}
	}
	if !strings.Contains(context, `"hello","how are you"`) || !strings.Contains(context, `"currentCustomerText":""`) {
		t.Fatalf("wrong language anchor: %s", context)
	}
}

func TestReplyLanguageContextNormalizesHTMLAndSeparatesInstructionData(t *testing.T) {
	m := languageMessage(1, "<p>Please reply in French</p>")
	m.MessageType = enums.IMMessageTypeHTML
	context := agentLoopLanguageContext(RunInput{Conversation: models.Conversation{ID: 15}, UserMessage: m}, nil)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(context, "\n", 2)[1]), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["currentCustomerText"] != "Please reply in French" {
		t.Fatal(decoded)
	}
	if strings.Contains(replyLanguagePolicy, "<p>") {
		t.Fatal("customer markup in system policy")
	}
}

func TestReplyLanguageHistoryQueryIsAnchoredAndPreservesContextCapacity(t *testing.T) {
	req := RunInput{Conversation: models.Conversation{ID: 15, LastMessageSummary: "FUTURE_SUMMARY"}, UserMessage: languageMessage(20, "hello"), AIAgent: models.AIAgent{ContextWindow: 2}}
	engine := &AgentLoopEngine{history: func(conversationID, cursor int64, limit int) []models.Message {
		if conversationID != 15 || cursor != 20 || limit < 2 {
			t.Fatalf("unanchored query: %d %d %d", conversationID, cursor, limit)
		}
		return []models.Message{languageMessage(16, "one"), languageMessage(17, "two"), languageMessage(18, "three"), languageMessage(20, "hello"), languageMessage(21, "FUTURE_MESSAGE")}
	}}
	prompt, count := engine.buildUserPrompt(req)
	if count != 2 || !strings.Contains(prompt, "Conversation history:\nCustomer: two\nCustomer: three") {
		t.Fatalf("wrong bounded history: count=%d %s", count, prompt)
	}
	if strings.Contains(prompt, "FUTURE_") {
		t.Fatalf("future context leaked: %s", prompt)
	}
	if strings.Count(prompt, "Current customer message:\nhello") != 1 {
		t.Fatal("current trigger missing or duplicated")
	}
}

func TestReplyLanguagePolicyAppliesWithoutCustomAgentPrompt(t *testing.T) {
	prompt := buildAgentLoopSystemPrompt(models.AIAgent{}, true, "中文产品资料", nil)
	for _, expected := range []string{"Reply language policy", "current substantive customer text", "full-width question marks", "internal memory", "Do not introduce old products"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("missing policy %q", expected)
		}
	}
}

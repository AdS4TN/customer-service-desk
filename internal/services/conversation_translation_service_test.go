package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
)

const translationTestJSON = `{"sourceLanguage":"en","targetLanguage":"zh-CN","content":"需要 200 件。","messageId":999,"cached":true}`

func TestTranslationHTMLPreservesTextAndRejectsMediaOnly(t *testing.T) {
	m := models.Message{ConversationID: 1, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeHTML, Content: `<p>AX-204 &amp; BX-12</p><p>Not 50<br>100</p><a href="https://example.com/catalog">Catalog</a><img src="private">`}
	if got := translationText(&m, 1); got != "AX-204 & BX-12\nNot 50\n100\nCatalog (https://example.com/catalog)" {
		t.Fatalf("incorrect text: %s", got)
	}
	m.Content = `<img src="private"><script>do not translate this</script>`
	if translationText(&m, 1) != "" {
		t.Fatal("media caption invented")
	}
	if translationText(&m, 2) != "" {
		t.Fatal("cross-conversation source")
	}
}

func TestTranslationAutoRejectsRecalledLanguageSample(t *testing.T) {
	db, c, m := setupCopilotTest(t)
	s := &conversationTranslationService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		db.Model(&m).Update("recalled_at", time.Now())
		return &ai.ChatCompletionResult{Content: translationTestJSON}, nil
	}}
	if _, err := s.Text(context.Background(), c.ID, request.TranslateConversationText{Text: "Hello", TargetLanguage: enums.TranslationLanguageAuto}); err == nil {
		t.Fatal("recalled language sample accepted")
	}
}

func TestTranslationMessageCacheAndIsolation(t *testing.T) {
	db, c, m := setupCopilotTest(t)
	if err := db.AutoMigrate(&models.MessageTranslation{}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s := &conversationTranslationService{complete: func(_ context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		calls++
		if system != translationPrompt || strings.Contains(system, "PUBLISHED_POLICY") || cfg.MaxRetryCount != 0 {
			t.Fatal("translation used business prompt or retry")
		}
		var data map[string]string
		if json.Unmarshal([]byte(input), &data) != nil || data["text"] != translationText(&m, c.ID) {
			t.Fatal("source not authoritative")
		}
		return &ai.ChatCompletionResult{Content: translationTestJSON}, nil
	}}
	req := request.TranslateConversationMessage{MessageID: m.ID, TargetLanguage: enums.TranslationLanguageChinese}
	for i := 0; i < 2; i++ {
		r, err := s.Message(context.Background(), c.ID, req)
		if err != nil || r.Cached != (i == 1) || r.MessageID != m.ID {
			t.Fatalf("cache result: %+v %v", r, err)
		}
	}
	if calls != 1 {
		t.Fatal("cache miss")
	}
	if _, err := s.Message(context.Background(), c.ID+100, req); err == nil {
		t.Fatal("cross conversation accepted")
	}
	var count int64
	db.Model(&models.Message{}).Count(&count)
	if count != 1 || MessageService.Get(m.ID).Content != m.Content {
		t.Fatal("translation mutated messages")
	}
	db.Model(&m).Update("content", "A changed source")
	m.Content = "A changed source"
	if _, err := s.Message(context.Background(), c.ID, req); err != nil || calls != 2 {
		t.Fatal("edited source cache reused")
	}
	db.Model(&m).Update("recalled_at", time.Now())
	if _, err := s.Message(context.Background(), c.ID, req); err == nil {
		t.Fatal("recalled cache returned")
	}
}

func TestTranslationRejectsConcurrentChanges(t *testing.T) {
	for _, kind := range []string{"message", "assignment", "closed", "customer", "agent_disabled", "recall"} {
		t.Run(kind, func(t *testing.T) {
			db, c, m := setupCopilotTest(t)
			db.AutoMigrate(&models.MessageTranslation{})
			s := &conversationTranslationService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
				switch kind {
				case "message":
					db.Model(&c).Update("last_message_id", m.ID+1)
				case "assignment":
					db.Model(&c).Update("current_assignee_id", 99)
				case "closed":
					db.Model(&c).Update("status", enums.IMConversationStatusClosed)
				case "customer":
					db.Model(&c).Update("customer_id", 99)
				case "agent_disabled":
					db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("status", enums.StatusDisabled)
				case "recall":
					db.Model(&m).Update("recalled_at", time.Now())
				}
				return &ai.ChatCompletionResult{Content: translationTestJSON}, nil
			}}
			var err error
			if kind == "recall" {
				_, err = s.Message(context.Background(), c.ID, request.TranslateConversationMessage{MessageID: m.ID, TargetLanguage: enums.TranslationLanguageChinese})
			} else {
				_, err = s.Text(context.Background(), c.ID, request.TranslateConversationText{Text: "Hello", TargetLanguage: enums.TranslationLanguageChinese})
			}
			if err == nil {
				t.Fatal("stale translation accepted")
			}
		})
	}
}

func TestTranslationAutoDetectUsesOnlyCustomerAndNoMetadata(t *testing.T) {
	db, c, m := setupCopilotTest(t)
	staff := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeAgent, MessageType: enums.IMMessageTypeText, Content: "Operator language must not be a sample"}
	db.Create(&staff)
	s := &conversationTranslationService{complete: func(_ context.Context, _ models.AIConfig, _, input string) (*ai.ChatCompletionResult, error) {
		var data map[string]string
		json.Unmarshal([]byte(input), &data)
		if data["customerLanguageSample"] != translationText(&m, c.ID) || data["text"] != "Chinese operator draft" {
			t.Fatal("wrong language sample")
		}
		return &ai.ChatCompletionResult{Content: translationTestJSON}, nil
	}}
	r, err := s.Text(context.Background(), c.ID, request.TranslateConversationText{Text: "Chinese operator draft", TargetLanguage: enums.TranslationLanguageAuto})
	if err != nil || r.MessageID != 0 || r.Cached || r.ConversationID != c.ID {
		t.Fatalf("untrusted metadata: %+v %v", r, err)
	}
}

func TestTranslationValidationAndProviderFailure(t *testing.T) {
	for _, raw := range []string{"", "not json", `{"sourceLanguage":"en","targetLanguage":"fr","content":"wrong"}`, `{"sourceLanguage":"en","targetLanguage":"zh-CN","content":""}`, `{"sourceLanguage":"auto","targetLanguage":"zh-CN","content":"wrong"}`} {
		s := &conversationTranslationService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
			return &ai.ChatCompletionResult{Content: raw}, nil
		}}
		if _, err := s.translate(context.Background(), models.AIConfig{}, "Hello", enums.TranslationLanguageChinese, ""); err == nil {
			t.Fatal("invalid output accepted")
		}
	}
	s := &conversationTranslationService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return nil, errors.New("private provider token")
	}}
	if _, err := s.translate(context.Background(), models.AIConfig{}, "Hello", enums.TranslationLanguageChinese, ""); err == nil || strings.Contains(err.Error(), "private provider") {
		t.Fatal("provider error leaked")
	}
	for _, source := range []string{"", strings.Repeat("a", 6001)} {
		if _, err := s.translate(context.Background(), models.AIConfig{}, source, enums.TranslationLanguageChinese, ""); err == nil {
			t.Fatal("invalid length accepted")
		}
	}
	s.complete = func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: translationTestJSON, FinishReason: "length"}, nil
	}
	if _, err := s.translate(context.Background(), models.AIConfig{}, "Hello", enums.TranslationLanguageChinese, ""); err == nil {
		t.Fatal("truncated result accepted")
	}
	s.complete = func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: `{"sourceLanguage":"en","targetLanguage":"und","content":""}`}, nil
	}
	if _, err := s.translate(context.Background(), models.AIConfig{}, "Hello", enums.TranslationLanguageAuto, "123"); err == nil {
		t.Fatal("ambiguous customer language accepted")
	}
}

package builders

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/i18nx"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestLocalizeConversationSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		locale  string
		summary string
		want    string
	}{
		{
			name:    "image summary in english",
			locale:  i18nx.LocaleEnUS,
			summary: "[图片]",
			want:    "[Image]",
		},
		{
			name:    "attachment summary in english",
			locale:  i18nx.LocaleEnUS,
			summary: "[附件] spec.pdf",
			want:    "[Attachment] spec.pdf",
		},
		{
			name:    "recalled message in english",
			locale:  i18nx.LocaleEnUS,
			summary: "该消息已撤回",
			want:    "This message was recalled.",
		},
		{
			name:    "business text is not translated",
			locale:  i18nx.LocaleEnUS,
			summary: "客户反馈无法登录",
			want:    "客户反馈无法登录",
		},
		{
			name:    "chinese locale keeps existing summary",
			locale:  i18nx.LocaleZhCN,
			summary: "[图片]",
			want:    "[图片]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := localizeConversationSummary(tt.locale, tt.summary); got != tt.want {
				t.Fatalf("localizeConversationSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocalizeRenderableMessageContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		locale  string
		content string
		want    string
	}{
		{
			name:    "recalled message in english",
			locale:  i18nx.LocaleEnUS,
			content: "该消息已撤回",
			want:    "This message was recalled.",
		},
		{
			name:    "normal customer message is not translated",
			locale:  i18nx.LocaleEnUS,
			content: "客户反馈无法登录",
			want:    "客户反馈无法登录",
		},
		{
			name:    "chinese locale keeps content",
			locale:  i18nx.LocaleZhCN,
			content: "该消息已撤回",
			want:    "该消息已撤回",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := localizeRenderableMessageContent(tt.locale, tt.content); got != tt.want {
				t.Fatalf("localizeRenderableMessageContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMessageIncludesWorkflowRunID(t *testing.T) {
	resp := BuildMessageWithReadStatesAndLocale(&models.Message{
		ID:             1,
		ConversationID: 2,
		SenderType:     enums.IMSenderTypeAI,
		MessageType:    enums.IMMessageTypeText,
		Content:        "AI reply",
		WorkflowRunID:  9988,
	}, nil, nil, nil, nil, nil, i18nx.DefaultLocale)

	if resp.WorkflowRunID != 9988 {
		t.Fatalf("resp.WorkflowRunID=%d want 9988", resp.WorkflowRunID)
	}
}

func TestBuildMessageJSONDoesNotExposeSeqNo(t *testing.T) {
	resp := BuildMessageWithReadStatesAndLocale(&models.Message{
		ID:             1,
		ConversationID: 2,
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "hello",
	}, nil, nil, nil, nil, nil, i18nx.DefaultLocale)

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal message response: %v", err)
	}
	if strings.Contains(string(raw), "seqNo") {
		t.Fatalf("message response should not expose seqNo, got %s", raw)
	}
}

func TestBuildAIMessageUsesPublishedPublicIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AIAgent{}, &models.AgentRevision{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	agent := &models.AIAgent{Name: "draft", DisplayName: "Draft name", Avatar: "/draft.png", Status: enums.StatusOk}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	revision := &models.AgentRevision{
		AgentID: agent.ID, Revision: 1, Status: enums.StatusOk,
		Definition: `{"agent":{"name":"published","displayName":"Published Support","avatar":"/published.png","statusText":"Online"}}`,
	}
	if err := db.Create(revision).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if err := db.Model(agent).Update("published_revision_id", revision.ID).Error; err != nil {
		t.Fatalf("publish agent: %v", err)
	}

	resp := buildMessageWithReadStatesAndLocale(&models.Message{
		ID: 1, ConversationID: 2, SenderType: enums.IMSenderTypeAI, SenderID: agent.ID,
		MessageType: enums.IMMessageTypeText, Content: "Hello",
	}, nil, nil, nil, nil, nil, nil, i18nx.DefaultLocale)
	if resp.SenderName != "Published Support" || resp.SenderAvatar != "/published.png" {
		t.Fatalf("message identity = name %q avatar %q", resp.SenderName, resp.SenderAvatar)
	}
}

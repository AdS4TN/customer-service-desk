package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

func TestUnifiedInboxFiltersAndSafeChannels(t *testing.T) {
	db := setupTelegramTestDB(t)
	channels := []models.Channel{
		{Name: "Website", ChannelID: "web-inbox", ChannelType: enums.ChannelTypeWeb, ConfigJSON: `{"secret":"private-value"}`},
		{Name: "WhatsApp", ChannelID: "wa-inbox", ChannelType: enums.ChannelTypeWhatsApp},
	}
	if err := db.Create(&channels).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	items := []models.Conversation{
		{ChannelID: channels[0].ID, CustomerName: "Web buyer", Status: enums.IMConversationStatusAIServing, LastActiveAt: now},
		{ChannelID: channels[1].ID, CustomerName: "WA buyer", Status: enums.IMConversationStatusPending, AgentUnreadCount: 2, LastActiveAt: now.Add(time.Second)},
		{ChannelID: channels[1].ID, CustomerName: "Assigned buyer", Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7, LastActiveAt: now.Add(2 * time.Second)},
		{ChannelID: channels[0].ID, CustomerName: "Closed buyer", Status: enums.IMConversationStatusClosed, CurrentAssigneeID: 8, LastActiveAt: now},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		filter  request.AgentConversationFilter
		keyword string
		option  request.InboxFilter
		count   int
	}{
		{"all", request.AgentConversationFilterAll, "", request.InboxFilter{}, 4},
		{"unread", request.AgentConversationFilterUnread, "", request.InboxFilter{}, 1},
		{"mine", request.AgentConversationFilterMine, "", request.InboxFilter{}, 1},
		{"whatsapp", request.AgentConversationFilterAll, "", request.InboxFilter{ChannelType: enums.ChannelTypeWhatsApp}, 2},
		{"account", request.AgentConversationFilterAll, "", request.InboxFilter{ChannelID: channels[0].ID}, 2},
		{"combined", request.AgentConversationFilterAll, "Assigned", request.InboxFilter{ChannelType: enums.ChannelTypeWhatsApp}, 1},
		{"closed", request.AgentConversationFilterClosed, "", request.InboxFilter{}, 1},
		{"mismatch", request.AgentConversationFilterAll, "", request.InboxFilter{ChannelID: channels[0].ID, ChannelType: enums.ChannelTypeWhatsApp}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			list, _, err := ConversationService.ListConversations(7, tc.filter, tc.keyword, &sqls.Paging{Page: 1, Limit: 50}, tc.option)
			if err != nil || len(list) != tc.count {
				t.Fatalf("count=%d want=%d error=%v", len(list), tc.count, err)
			}
		})
	}
	if _, _, err := ConversationService.ListConversations(7, request.AgentConversationFilterAll, "", &sqls.Paging{Page: 1, Limit: 50}, request.InboxFilter{ChannelType: "invalid"}); err == nil {
		t.Fatal("invalid channel filter accepted")
	}
	encoded, err := json.Marshal(ConversationService.InboxChannels())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-value") || strings.Contains(string(encoded), "configJson") {
		t.Fatal("inbox metadata exposed channel configuration")
	}
}

func TestUnifiedInboxTakeover(t *testing.T) {
	db := setupTelegramTestDB(t)
	if err := db.AutoMigrate(&models.AgentProfile{}, &models.ConversationAssignment{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentProfile{UserID: 7, AgentCode: "INBOX7", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentProfile{UserID: 8, AgentCode: "INBOX8", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	conversation := models.Conversation{Status: enums.IMConversationStatusAIServing}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{UserID: 7, Username: "operator"}
	if err := ConversationService.AssignConversation(request.AssignConversationRequest{ConversationID: conversation.ID, AssigneeID: 8}, operator); err == nil {
		t.Fatal("AI takeover assigned to another operator")
	}
	if err := ConversationService.AssignConversation(request.AssignConversationRequest{ConversationID: conversation.ID, AssigneeID: 7}, operator); err != nil {
		t.Fatal(err)
	}
	current := ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusActive || current.CurrentAssigneeID != 7 {
		t.Fatal("takeover did not activate human reception")
	}
	if err := repositories.AssignInboxConversation(db, &conversation, map[string]any{"current_assignee_id": 8}); err == nil {
		t.Fatal("stale takeover overwrote assignment")
	}
	if err := ConversationService.AssignConversation(request.AssignConversationRequest{ConversationID: conversation.ID, AssigneeID: 8}, &dto.AuthPrincipal{UserID: 8}); err == nil {
		t.Fatal("second operator stole active conversation")
	}
}

func TestUnifiedInboxRetry(t *testing.T) {
	db := setupTelegramTestDB(t)
	channel := models.Channel{Name: "WA", ChannelID: "retry-channel", ChannelType: enums.ChannelTypeWhatsApp}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	conversation := models.Conversation{ChannelID: channel.ID, Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{UserID: 7}
	for _, tc := range []struct {
		name    string
		sender  enums.IMSenderType
		user    int64
		reason  string
		closed  bool
		success bool
	}{
		{"own failed", enums.IMSenderTypeAgent, 7, "send_failed", false, true},
		{"other operator", enums.IMSenderTypeAgent, 8, "send_failed", false, false},
		{"stale AI", enums.IMSenderTypeAI, 7, "send_failed", false, false},
		{"cancelled", enums.IMSenderTypeAgent, 7, "reply_cancelled", false, false},
		{"account changed", enums.IMSenderTypeAgent, 7, "account_changed", false, false},
		{"closed", enums.IMSenderTypeAgent, 7, "send_failed", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.closed {
				if err := ConversationService.Updates(conversation.ID, map[string]any{"status": enums.IMConversationStatusClosed}); err != nil {
					t.Fatal(err)
				}
			}
			message := models.Message{ConversationID: conversation.ID, ClientMsgID: tc.name, SenderType: tc.sender, SenderID: tc.user, SendStatus: enums.IMMessageStatusFailed}
			if err := db.Create(&message).Error; err != nil {
				t.Fatal(err)
			}
			outbox := models.ChannelMessageOutbox{ChannelType: enums.ChannelTypeWhatsApp, ConversationID: conversation.ID, MessageID: message.ID, SendStatus: "ignored", LastError: tc.reason, Payload: `{"whatsappMessageId":"stable"}`}
			if err := db.Create(&outbox).Error; err != nil {
				t.Fatal(err)
			}
			result, err := ConversationService.RetryInboxMessage(message.ID, operator)
			if (err == nil) != tc.success {
				t.Fatalf("success=%t err=%v", tc.success, err)
			}
			if tc.success {
				if result.ID != message.ID || result.SendStatus != enums.IMMessageStatusSending {
					t.Fatal("retry created a different message")
				}
				updated := ChannelMessageOutboxService.Get(outbox.ID)
				if updated.Payload != outbox.Payload || updated.SendStatus != "pending" {
					t.Fatal("retry changed protocol id or failed to requeue")
				}
				if _, err := ConversationService.RetryInboxMessage(message.ID, operator); err == nil {
					t.Fatal("duplicate retry accepted")
				}
			}
		})
	}
}

func TestUnifiedInboxRealtimeZeroValues(t *testing.T) {
	payload, err := json.Marshal(RealtimeConversationChangedPayload{})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"currentAssigneeId":0`, `"agentUnreadCount":0`, `"customerUnreadCount":0`} {
		if !strings.Contains(string(payload), field) {
			t.Fatalf("zero transition omitted: %s", field)
		}
	}
}

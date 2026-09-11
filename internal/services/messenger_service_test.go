package services

import (
	"errors"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/linkedchat"
	"agent-desk/internal/repositories"
)

func TestMessengerFlow(t *testing.T) {
	db := setupTelegramTestDB(t)
	agent := &models.AIAgent{Name: "Messenger test", ServiceMode: enums.IMConversationServiceModeAIFirst, PublishedRevisionID: 1, Status: enums.StatusOk}
	if err := db.Create(agent).Error; err != nil {
		t.Fatal(err)
	}
	op := &dto.AuthPrincipal{UserID: 1, Username: "admin"}
	channel, err := ChannelService.CreateChannel(request.CreateChannelRequest{Name: "Messenger", ChannelType: enums.ChannelTypeMessenger, AIAgentID: agent.ID, ConfigJSON: `{"cookie":"must-not-save","autoConnect":true}`}, op)
	if err != nil {
		t.Fatal(err)
	}
	if channel.ConfigJSON != "{}" {
		t.Fatal("accepted credentials in ordinary channel configuration")
	}
	fake := &fakeWhatsApp{state: linkedchat.Status{State: "connected", Account: "123"}}
	s := &linkedChatService{kind: "messenger", sessions: map[int64]whatsAppSession{channel.ID: fake}}
	oldHook, calls := TriggerAIReplyAsyncHook, 0
	TriggerAIReplyAsyncHook = func(models.Conversation, models.Message) { calls++ }
	t.Cleanup(func() { TriggerAIReplyAsyncHook = oldHook })
	old := linkedchat.Incoming{ID: "old", Account: "123", Chat: "e2ee:456", Text: "old question", History: true, SentAt: time.Now().Add(-time.Hour)}
	for range 2 {
		if err = s.Receive(channel.ID, old); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 0 {
		t.Fatal("historical import triggered AI")
	}
	m := MessageService.Take("client_msg_id = ?", s.messageID(channel.ID, old))
	if m == nil {
		t.Fatal("history not imported")
	}
	conv := ConversationService.Get(m.ConversationID)
	if conv.AgentUnreadCount != 0 {
		t.Fatal("history marked unread")
	}
	identity := ConversationService.GetConversationExternalIdentity(conv)
	if identity == nil || identity.ExternalSource != enums.ExternalSourceMessenger {
		t.Fatal("wrong customer channel identity")
	}
	live := old
	live.ID = "live"
	live.History = false
	live.SentAt = time.Now()
	for range 2 {
		if err = s.Receive(channel.ID, live); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("live message not deduplicated")
	}
	reply, err := MessageService.SendAIMessage(conv.ID, agent.ID, "reply", enums.IMMessageTypeText, "Hello", "", op)
	if err != nil {
		t.Fatal(err)
	}
	item := ChannelMessageOutboxService.GetByMessageID(enums.ChannelTypeMessenger, reply.ID)
	if item == nil || reply.SendStatus != enums.IMMessageStatusSending {
		t.Fatal("no Messenger outbox")
	}
	if len(repositories.ListWhatsAppDueOutbox(db, 20)) != 0 {
		t.Fatal("Messenger outbox entered WhatsApp dispatcher")
	}
	fake.err = errors.New("test network failure")
	if err = s.sendOutbox(*item); err != nil {
		t.Fatal(err)
	}
	fake.err = nil
	item = ChannelMessageOutboxService.Get(item.ID)
	if err = s.sendOutbox(*item); err != nil {
		t.Fatal(err)
	}
	if len(fake.ids) != 2 || fake.ids[0] != fake.ids[1] {
		t.Fatal("retry ID was not stable")
	}
	if fake.sent[1] != "123|e2ee:456|Hello" {
		t.Fatal("wrong outbound recipient")
	}
	echo := live
	echo.ID = "remote-id"
	echo.EchoID = fake.ids[1]
	echo.FromMe = true
	if err = s.Receive(channel.ID, echo); err != nil {
		t.Fatal(err)
	}
	if MessageService.Take("client_msg_id = ?", s.messageID(channel.ID, echo)) != nil {
		t.Fatal("own outbound echo duplicated")
	}
	phone := echo
	phone.ID = "phone"
	phone.EchoID = "phone"
	if err = s.Receive(channel.ID, phone); err != nil {
		t.Fatal(err)
	}
	if ConversationService.Get(conv.ID).Status != enums.IMConversationStatusPending {
		t.Fatal("phone reply did not pause AI")
	}
	if calls != 1 {
		t.Fatal("own reply triggered AI")
	}
	list, _, _ := MessageService.FindByConversationIDCursor(conv.ID, 0, 100, "", "")
	if len(list) != 4 {
		t.Fatalf("unexpected stored message count %d", len(list))
	}
	if err := s.SetMessengerCredentials(channel.ID, "not-a-cookie"); err == nil {
		t.Fatal("accepted malformed login")
	}
}

package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"agent-desk/internal/whatsapp"
)

type fakeWhatsApp struct {
	state whatsapp.Status
	sent  []string
	ids   []string
	err   error
}

func (f *fakeWhatsApp) Status() whatsapp.Status      { return f.state }
func (f *fakeWhatsApp) Connect() error               { return nil }
func (f *fakeWhatsApp) Disconnect()                  { f.state.State = "disconnected" }
func (f *fakeWhatsApp) Logout(context.Context) error { return nil }
func (f *fakeWhatsApp) Send(_ context.Context, account, chat, id, text string) error {
	f.sent = append(f.sent, account+"|"+chat+"|"+text)
	f.ids = append(f.ids, id)
	return f.err
}

func TestWhatsAppFlow(t *testing.T) {
	db := setupTelegramTestDB(t)
	agent := &models.AIAgent{Name: "WA test", ServiceMode: enums.IMConversationServiceModeAIFirst, PublishedRevisionID: 1, Status: enums.StatusOk}
	if err := db.Create(agent).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{UserID: 1, Username: "admin"}
	channel, err := ChannelService.CreateChannel(request.CreateChannelRequest{Name: "WA", ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID, ConfigJSON: `{"session":"bad","autoConnect":true}`}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if channel.ConfigJSON != "{}" {
		t.Fatal("accepted client session configuration")
	}
	fake := &fakeWhatsApp{state: whatsapp.Status{State: "connected", Account: "111@s.whatsapp.net"}}
	s := &whatsAppService{sessions: map[int64]whatsAppSession{channel.ID: fake}}
	hook := TriggerAIReplyAsyncHook
	calls := 0
	TriggerAIReplyAsyncHook = func(models.Conversation, models.Message) { calls++ }
	t.Cleanup(func() { TriggerAIReplyAsyncHook = hook })
	incoming := whatsapp.Incoming{ID: "id1", Account: fake.state.Account, Chat: "222@lid", Name: "Customer", Text: "What are your opening hours?", SentAt: time.Now()}
	if err := s.Receive(channel.ID, incoming); err != nil {
		t.Fatal(err)
	}
	if err := s.Receive(channel.ID, incoming); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("AI triggered %d times for duplicate message", calls)
	}
	m := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, incoming))
	if m == nil {
		t.Fatal("incoming missing")
	}
	conv := ConversationService.Get(m.ConversationID)
	identity := ConversationService.GetConversationExternalIdentity(conv)
	if identity.ExternalID != whatsAppIdentity(channel.ID, incoming.Account, incoming.Chat) {
		t.Fatal("wrong identity")
	}
	reply, err := MessageService.SendAIMessage(conv.ID, agent.ID, "reply", enums.IMMessageTypeText, "We open at 9.", "", operator)
	if err != nil {
		t.Fatal(err)
	}
	if reply.SendStatus != enums.IMMessageStatusSending {
		t.Fatal("reply falsely marked sent before delivery")
	}
	item := ChannelMessageOutboxService.GetByMessageID(enums.ChannelTypeWhatsApp, reply.ID)
	if item == nil {
		t.Fatal("missing durable outbox")
	}
	fake.err = errors.New("network unavailable")
	if err := s.sendOutbox(*item); err != nil {
		t.Fatal(err)
	}
	item = ChannelMessageOutboxService.Get(item.ID)
	if item.SendStatus != "failed" || item.RetryCount != 1 {
		t.Fatalf("no retry: %+v", item)
	}
	fake.err = nil
	if err := s.sendOutbox(*item); err != nil {
		t.Fatal(err)
	}
	if len(fake.ids) != 2 || fake.ids[0] != fake.ids[1] {
		t.Fatal("retry changed protocol message id")
	}
	if MessageService.Get(reply.ID).SendStatus != enums.IMMessageStatusSent {
		t.Fatal("sent state missing")
	}
	if fake.sent[1] != "111@s.whatsapp.net|222@lid|We open at 9." {
		t.Fatalf("wrong target %q", fake.sent[1])
	}
	if err := s.sendOutbox(*item); err != nil {
		t.Fatal(err)
	}
	if len(fake.ids) != 2 {
		t.Fatal("already-sent outbox replayed")
	}
	// A duplicate after closure must not start a new conversation or re-trigger AI.
	if err := ConversationService.Updates(conv.ID, map[string]any{"status": enums.IMConversationStatusClosed}); err != nil {
		t.Fatal(err)
	}
	if err := s.Receive(channel.ID, incoming); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("closed conversation replay triggered AI")
	}
	// The same person contacting a different channel is independently scoped.
	channel2, err := ChannelService.CreateChannel(request.CreateChannelRequest{Name: "WA2", ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Receive(channel2.ID, incoming); err != nil {
		t.Fatal(err)
	}
	m2 := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel2.ID, incoming))
	if m2 == nil || m2.ConversationID == m.ConversationID {
		t.Fatal("channels share a conversation")
	}
	// A queued AI answer must be cancelled after human takeover.
	conv2 := ConversationService.Get(m2.ConversationID)
	if conv2.CustomerID == conv.CustomerID {
		t.Fatal("channels share a customer identity")
	}
	reply2, err := MessageService.SendAIMessage(conv2.ID, agent.ID, "reply2", enums.IMMessageTypeText, "An old AI reply", "", operator)
	if err != nil {
		t.Fatal(err)
	}
	if err := ConversationService.Updates(conv2.ID, map[string]any{"status": enums.IMConversationStatusPending}); err != nil {
		t.Fatal(err)
	}
	item2 := ChannelMessageOutboxService.GetByMessageID(enums.ChannelTypeWhatsApp, reply2.ID)
	if err := s.sendOutbox(*item2); err != nil {
		t.Fatal(err)
	}
	if ChannelMessageOutboxService.Get(item2.ID).LastError != "reply_cancelled" {
		t.Fatal("AI reply not cancelled")
	}
	// Claim recovery and due-time filtering do not starve later channels.
	if err := ChannelMessageOutboxService.Updates(item.ID, map[string]any{"send_status": "sending"}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.RecoverWhatsAppOutbox(db); err != nil {
		t.Fatal(err)
	}
	if ChannelMessageOutboxService.Get(item.ID).SendStatus != "pending" {
		t.Fatal("claim was not recovered")
	}
}

func TestWhatsAppHistoryImport(t *testing.T) {
	db := setupTelegramTestDB(t)
	agent := &models.AIAgent{Name: "History test", ServiceMode: enums.IMConversationServiceModeAIFirst, PublishedRevisionID: 1, Status: enums.StatusOk}
	if err := db.Create(agent).Error; err != nil {
		t.Fatal(err)
	}
	channel, err := ChannelService.CreateChannel(request.CreateChannelRequest{Name: "History", ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID}, &dto.AuthPrincipal{UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	s := &whatsAppService{sessions: make(map[int64]whatsAppSession)}
	hook := TriggerAIReplyAsyncHook
	calls := 0
	TriggerAIReplyAsyncHook = func(models.Conversation, models.Message) { calls++ }
	t.Cleanup(func() { TriggerAIReplyAsyncHook = hook })
	base := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	latest := whatsapp.Incoming{ID: "latest", Account: "111@s.whatsapp.net", Chat: "222@lid", Name: "Existing customer", Text: "latest text", SentAt: base.Add(time.Hour), History: true}
	old := latest
	old.ID, old.Text, old.SentAt, old.FromMe = "old", "phone reply", base, true
	for _, msg := range []whatsapp.Incoming{latest, old, latest, old} {
		if err := s.Receive(channel.ID, msg); err != nil {
			t.Fatal(err)
		}
	}
	m := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, latest))
	if m == nil {
		t.Fatal("history missing")
	}
	conv := ConversationService.Get(m.ConversationID)
	if !conv.LastMessageAt.Equal(latest.SentAt) || conv.LastMessageID != m.ID || conv.AgentUnreadCount != 0 {
		t.Fatalf("history changed newest summary or unread state: %+v", conv)
	}
	var count int64
	db.Model(&models.Message{}).Where("conversation_id = ?", conv.ID).Count(&count)
	if count != 2 {
		t.Fatalf("history duplicated or welcome injected: %d", count)
	}
	phone := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, old))
	if phone.SenderType != enums.IMSenderTypeAgent || phone.SentAt == nil || !phone.SentAt.Equal(base) {
		t.Fatal("lost phone direction or original timestamp")
	}
	if calls != 0 {
		t.Fatal("history triggered AI")
	}
	db.Model(&models.ChannelMessageOutbox{}).Count(&count)
	if count != 0 {
		t.Fatal("history was sent to WhatsApp")
	}
	// Backfilled messages have larger IDs; pagination must still follow send time.
	list, cursor, _ := MessageService.FindByConversationIDCursor(conv.ID, 0, 1, "", "")
	if len(list) != 1 || list[0].ID != m.ID {
		t.Fatal("latest page follows import ID")
	}
	list, _, _ = MessageService.FindByConversationIDCursor(conv.ID, cursor, 1, "", "")
	if len(list) != 1 || list[0].ID != phone.ID {
		t.Fatal("older page skipped backfill")
	}
	// Preserve the preexisting LID identity when history supplies a phone-number JID.
	live := latest
	live.ID, live.Chat, live.ChatAliases, live.History = "new", "222@s.whatsapp.net", []string{"222@lid"}, false
	if err := s.Receive(channel.ID, live); err != nil {
		t.Fatal(err)
	}
	live.Chat = "222@lid"
	newMessage := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, live))
	if calls != 1 || newMessage == nil || newMessage.ConversationID != conv.ID {
		t.Fatal("live message lost or customer split")
	}
	if ConversationService.Get(conv.ID).AgentUnreadCount != 1 {
		t.Fatal("history became unread when a live message arrived")
	}
	// Platform echoes have the same protocol ID as our outbox and must not be imported.
	reply, err := MessageService.SendAIMessage(conv.ID, agent.ID, "reply", enums.IMMessageTypeText, "platform reply", "", &dto.AuthPrincipal{UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	echo := old
	echo.ID = whatsAppProtocolID(channel.ChannelID, reply.ID)
	if err := s.Receive(channel.ID, echo); err != nil {
		t.Fatal(err)
	}
	if MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, echo)) != nil {
		t.Fatal("platform echo duplicated")
	}
	// A live phone response pauses AI; historical phone replies did not.
	phoneLive := old
	phoneLive.ID, phoneLive.History, phoneLive.SentAt = "phone-live", false, time.Now()
	if err := s.Receive(channel.ID, phoneLive); err != nil {
		t.Fatal(err)
	}
	if ConversationService.Get(conv.ID).Status != enums.IMConversationStatusPending {
		t.Fatal("phone reply did not pause AI")
	}
	if calls != 1 {
		t.Fatal("outgoing message triggered AI")
	}
	// Late history attaches without reopening a closed conversation.
	if err := ConversationService.Updates(conv.ID, map[string]any{"status": enums.IMConversationStatusClosed}); err != nil {
		t.Fatal(err)
	}
	old.ID = "late-history"
	if err := s.Receive(channel.ID, old); err != nil {
		t.Fatal(err)
	}
	if ConversationService.Get(conv.ID).Status != enums.IMConversationStatusClosed {
		t.Fatal("history reopened conversation")
	}
	late := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, old))
	if late.ConversationID != conv.ID {
		t.Fatal("late history created another conversation")
	}
}

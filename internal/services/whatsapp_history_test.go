package services

import (
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/linkedchat"
	"agent-desk/internal/whatsapp"
)

func TestWhatsAppAutomaticHistoryTargets(t *testing.T) {
	db := setupTelegramTestDB(t)
	channel := models.Channel{ChannelID: "wa-history", ChannelType: enums.ChannelTypeWhatsApp, Status: enums.StatusOk}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	for i, external := range []string{
		whatsAppIdentity(channel.ID, "111@s.whatsapp.net", "222@lid"),
		whatsAppIdentity(channel.ID, "999@s.whatsapp.net", "222@lid"),
		whatsAppIdentity(channel.ID+1, "111@s.whatsapp.net", "222@lid"),
		whatsAppIdentity(channel.ID, "111@s.whatsapp.net", "group@g.us"),
	} {
		identity := models.CustomerIdentity{CustomerID: int64(i + 1), ExternalSource: enums.ExternalSourceWhatsApp, ExternalID: external}
		if err := db.Create(&identity).Error; err != nil {
			t.Fatal(err)
		}
		for range 2 {
			conv := models.Conversation{ChannelID: channel.ID, CustomerID: identity.CustomerID}
			if err := db.Create(&conv).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	s := &whatsAppService{sessions: map[int64]whatsAppSession{}}
	targets, err := s.historyTargets(channel.ID, "111@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].ConversationID != 2 || targets[0].Chat != "222@lid" || targets[0].MessageID != "" {
		t.Fatalf("history targets not scoped/deduplicated: %+v", targets)
	}
	if err := db.Model(&channel).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	targets, err = s.historyTargets(channel.ID, "111@s.whatsapp.net")
	if err != nil || len(targets) != 0 {
		t.Fatal("disabled channel requested history")
	}
}

func TestWhatsAppBackfilledStickerNoReplies(t *testing.T) {
	db := setupTelegramTestDB(t)
	agent := models.AIAgent{Name: "History", PublishedRevisionID: 1, Status: enums.StatusOk}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatal(err)
	}
	channel := models.Channel{ChannelID: "wa-sticker-history", ChannelType: enums.ChannelTypeWhatsApp, Status: enums.StatusOk, AIAgentID: agent.ID}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	s := &whatsAppService{sessions: map[int64]whatsAppSession{}}
	previousHook := TriggerAIReplyAsyncHook
	calls := 0
	TriggerAIReplyAsyncHook = func(models.Conversation, models.Message) { calls++ }
	t.Cleanup(func() { TriggerAIReplyAsyncHook = previousHook })
	msg := whatsapp.Incoming{ID: "old-sticker", Account: "111@s.whatsapp.net", Chat: "222@lid", History: true,
		SentAt: time.Now().Add(-time.Hour), Message: &linkedchat.Message{Kind: "sticker", State: "unavailable"}}
	for range 2 {
		if err := s.Receive(channel.ID, msg); err != nil {
			t.Fatal(err)
		}
	}
	saved := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, msg))
	if saved == nil || !saved.IsHistorical {
		t.Fatal("historical sticker missing")
	}
	var count int64
	db.Model(&models.Message{}).Count(&count)
	if count != 1 || calls != 0 || ConversationService.Get(saved.ConversationID).AgentUnreadCount != 0 {
		t.Fatal("history duplicated or triggered live behavior")
	}
	db.Model(&models.ChannelMessageOutbox{}).Count(&count)
	if count != 0 {
		t.Fatal("history queued outgoing messages")
	}
}

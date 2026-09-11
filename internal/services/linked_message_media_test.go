package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/linkedchat"
	"agent-desk/internal/pkg/utils"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestWhatsAppMediaImportAndUpdates(t *testing.T) {
	db := setupTelegramTestDB(t)
	agent := &models.AIAgent{Name: "Media test", ServiceMode: enums.IMConversationServiceModeAIFirst, PublishedRevisionID: 1, Status: enums.StatusOk}
	if err := db.Create(agent).Error; err != nil {
		t.Fatal(err)
	}
	channel, err := ChannelService.CreateChannel(request.CreateChannelRequest{Name: "WA media", ChannelType: enums.ChannelTypeWhatsApp, AIAgentID: agent.ID}, &dto.AuthPrincipal{UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	s := &whatsAppService{sessions: map[int64]whatsAppSession{}}
	hook := TriggerAIReplyAsyncHook
	calls := 0
	TriggerAIReplyAsyncHook = func(models.Conversation, models.Message) { calls++ }
	t.Cleanup(func() { TriggerAIReplyAsyncHook = hook })
	for _, kind := range []string{"image", "sticker", "audio", "video", "document", "reaction", "contact", "location", "unsupported", "view_once"} {
		msg := linkedchat.Incoming{ID: kind, Account: "owner", Chat: "customer", SentAt: time.Now(), Message: &linkedchat.Message{Kind: kind, State: "pending"}}
		if err := s.Receive(channel.ID, msg); err != nil {
			t.Fatal(err)
		}
		if err := s.Receive(channel.ID, msg); err != nil {
			t.Fatal(err)
		}
		saved := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, msg))
		if saved == nil || saved.IsHistorical {
			t.Fatal("live media not persisted correctly")
		}
		_, payload := utils.BuildRenderableMessage(saved)
		var rendered map[string]any
		if json.Unmarshal([]byte(payload), &rendered) != nil || rendered["linkedMessage"] == nil {
			t.Fatal("response lost media metadata")
		}
		msg.Message.State = "unavailable"
		if err := s.Receive(channel.ID, msg); err != nil {
			t.Fatal(err)
		}
		saved = MessageService.Get(saved.ID)
		if json.Unmarshal([]byte(saved.Payload), &rendered) != nil || rendered["linkedMessage"].(map[string]any)["state"] != "unavailable" {
			t.Fatal("failed download not persisted")
		}
		conv := ConversationService.Get(saved.ConversationID)
		if conv.AgentUnreadCount == 0 || conv.Status != enums.IMConversationStatusPending {
			t.Fatal("media did not enter human inbox")
		}
	}
	var count int64
	db.Model(&models.Message{}).Count(&count)
	if count != 10 || calls != 0 {
		t.Fatalf("duplicates or unwanted AI: count=%d calls=%d", count, calls)
	}
	historical := linkedchat.Incoming{ID: "old-media", Account: "owner", Chat: "older-customer", History: true, SentAt: time.Now().Add(-time.Hour), Message: &linkedchat.Message{Kind: "sticker", State: "unavailable"}}
	if err := s.Receive(channel.ID, historical); err != nil {
		t.Fatal(err)
	}
	m := MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, historical))
	if m == nil || !m.IsHistorical || ConversationService.Get(m.ConversationID).AgentUnreadCount != 0 || calls != 0 {
		t.Fatal("history caused live side effects")
	}
	var oldConfig *config.Config
	func() {
		defer func() { _ = recover() }() // Other unit tests may leave config uninitialized.
		value := config.Current()
		oldConfig = &value
	}()
	t.Cleanup(func() { config.SetCurrent(oldConfig) })
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{Default: enums.AssetProviderLocal, Local: config.LocalStorageConfig{Root: t.TempDir(), BaseURL: "/storage"}}})
	if err := db.AutoMigrate(&models.Asset{}); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "download")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	historical.Message.State = "ready"
	historical.MediaPath = file.Name()
	if err := s.Receive(channel.ID, historical); err != nil {
		t.Fatal(err)
	}
	if err := s.Receive(channel.ID, historical); err != nil {
		t.Fatal(err)
	}
	m = MessageService.Get(m.ID)
	_, hydrated := utils.BuildRenderableMessage(m)
	var result map[string]any
	if json.Unmarshal([]byte(hydrated), &result) != nil || result["url"] == nil || result["linkedMessage"].(map[string]any)["state"] != "ready" {
		t.Fatal("ready attachment lost URL or metadata")
	}
	db.Model(&models.Asset{}).Count(&count)
	if count != 1 {
		t.Fatal("attachment replay duplicated upload")
	}
	historical.ID, historical.MediaPath, historical.Message.State = "interrupted", "", "pending"
	if err := s.Receive(channel.ID, historical); err != nil {
		t.Fatal(err)
	}
	if err := s.recoverPendingMedia(); err != nil {
		t.Fatal(err)
	}
	m = MessageService.Take("client_msg_id = ?", whatsAppMessageID(channel.ID, historical))
	if json.Unmarshal([]byte(m.Payload), &result) != nil || result["linkedMessage"].(map[string]any)["state"] != "unavailable" {
		t.Fatal("interrupted download stayed pending")
	}
}

func TestIncomingMediaSafeFilename(t *testing.T) {
	for _, mime := range []string{"text/html", "image/svg+xml", "application/javascript"} {
		if safeIncomingFilename(mime) != "attachment.bin" {
			t.Fatal("active content extension allowed")
		}
	}
}

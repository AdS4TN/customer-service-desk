package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/linkedchat"
	"agent-desk/internal/pkg/openidentity"
	"agent-desk/internal/services/storage"
	"bytes"
	"context"
	"encoding/json"
	"github.com/mlogclub/simple/sqls"
	"image"
	"image/png"
	"testing"
	"time"
)

func TestReceptionWorkTransitions(t *testing.T) {
	db := setupTelegramTestDB(t)
	item := models.Conversation{Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{UserID: 7}
	apply := func(message models.Message) {
		t.Helper()
		message.ConversationID = item.ID
		if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error { return ConversationWorkService.onMessage(ctx, &message) }); err != nil {
			t.Fatal(err)
		}
	}
	apply(models.Message{ID: 10, SenderType: enums.IMSenderTypeCustomer})
	first := ConversationService.Get(item.ID)
	if first.WorkStatus != enums.ConversationWorkStatusNeedsReply || first.PendingSince == nil || first.ReplyDueAt == nil {
		t.Fatal("customer did not enter reply queue")
	}
	apply(models.Message{ID: 11, SenderType: enums.IMSenderTypeCustomer})
	current := ConversationService.Get(item.ID)
	if !current.PendingSince.Equal(*first.PendingSince) {
		t.Fatal("repeat message reset deadline")
	}
	apply(models.Message{ID: 12, SenderType: enums.IMSenderTypeAgent, ReplyToCustomerMessageID: 10, SendStatus: enums.IMMessageStatusSent})
	if ConversationService.Get(item.ID).WorkStatus != enums.ConversationWorkStatusNeedsReply {
		t.Fatal("old delayed reply cleared new request")
	}
	apply(models.Message{ID: 13, SenderType: enums.IMSenderTypeAgent, ReplyToCustomerMessageID: 11, SendStatus: enums.IMMessageStatusSending})
	if ConversationService.Get(item.ID).WorkStatus != enums.ConversationWorkStatusNeedsReply {
		t.Fatal("queued reply marked complete")
	}
	apply(models.Message{ID: 13, SenderType: enums.IMSenderTypeAgent, ReplyToCustomerMessageID: 11, SendStatus: enums.IMMessageStatusSent})
	if ConversationService.Get(item.ID).WorkStatus != enums.ConversationWorkStatusWaitingCustomer {
		t.Fatal("sent reply did not wait for customer")
	}
	current = ConversationService.Get(item.ID)
	req := request.UpdateConversationWork{Revision: current.WorkRevision, Status: enums.ConversationWorkStatusSnoozed, SnoozeMinutes: 1, ReplyTargetMinutes: 5}
	if err := ConversationWorkService.Update(item.ID, req, &dto.AuthPrincipal{UserID: 8}); err == nil {
		t.Fatal("another operator snoozed conversation")
	}
	if err := ConversationWorkService.Update(item.ID, req, operator); err != nil {
		t.Fatal(err)
	}
	apply(models.Message{ID: 14, SenderType: enums.IMSenderTypeCustomer, IsHistorical: true})
	if ConversationService.Get(item.ID).WorkStatus != enums.ConversationWorkStatusSnoozed {
		t.Fatal("history woke conversation")
	}
	if err := ConversationWorkService.Update(item.ID, req, operator); err == nil {
		t.Fatal("stale revision accepted")
	}
	apply(models.Message{ID: 15, SenderType: enums.IMSenderTypeCustomer})
	if c := ConversationService.Get(item.ID); c.WorkStatus != enums.ConversationWorkStatusNeedsReply || c.SnoozedUntil != nil {
		t.Fatal("new customer message did not wake snooze")
	}
	current = ConversationService.Get(item.ID)
	req.Revision = current.WorkRevision
	if err := ConversationWorkService.Update(item.ID, req, operator); err != nil {
		t.Fatal(err)
	}
	if err := ConversationWorkService.processDue(item.ID, time.Now().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if ConversationService.Get(item.ID).WorkStatus != enums.ConversationWorkStatusNeedsReply {
		t.Fatal("expired snooze did not wake")
	}
	list, _, err := ConversationService.ListConversations(7, "needs_reply", "", &sqls.Paging{Page: 1, Limit: 50})
	if err != nil || len(list) != 1 {
		t.Fatalf("reply queue: %v %d", err, len(list))
	}
}

func TestReceptionWorkReadDoesNotResolveAndReminderDeduplicates(t *testing.T) {
	db := setupTelegramTestDB(t)
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		t.Fatal(err)
	}
	item := models.Conversation{Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceGuest, ExternalID: "synthetic", ExternalName: "Synthetic"}
	linkReceptionTestCustomer(t, item.ID, external)
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return ConversationParticipantService.CreateCustomerParticipant(ctx, item.ID, external)
	}); err != nil {
		t.Fatal(err)
	}
	hook := TriggerAIReplyAsyncHook
	TriggerAIReplyAsyncHook = nil
	t.Cleanup(func() { TriggerAIReplyAsyncHook = hook })
	message, err := MessageService.SendCustomerMessage(item.ID, "read-state", enums.IMMessageTypeText, "Synthetic customer question", "", external)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		_, err := ConversationReadStateService.MarkAgentRead(ctx, ConversationService.Get(item.ID), &dto.AuthPrincipal{UserID: 7}, message)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	current := ConversationService.Get(item.ID)
	if current.WorkStatus != enums.ConversationWorkStatusNeedsReply {
		t.Fatal("read resolved pending reply")
	}
	future := time.Now().Add(time.Hour)
	for range 2 {
		if err := ConversationWorkService.processDue(item.ID, future); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	db.Model(&models.Notification{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate reminder: %d", count)
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return ConversationWorkService.onMessage(ctx, &models.Message{ID: message.ID + 1, ConversationID: item.ID, SenderType: enums.IMSenderTypeCustomer})
	}); err != nil {
		t.Fatal(err)
	}
	if err := ConversationWorkService.processDue(item.ID, future); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.Notification{}).Count(&count)
	if count != 1 {
		t.Fatal("repeat message caused duplicate overdue alert")
	}
}

func TestReceptionPrivateNotesIsolationAndMentions(t *testing.T) {
	db := setupTelegramTestDB(t)
	if err := db.AutoMigrate(&models.ConversationNote{}, &models.Notification{}, &models.User{}, &models.UserRole{}, &models.RolePermission{}, &models.Permission{}, &models.UserPermission{}, &models.ConversationAssignment{}); err != nil {
		t.Fatal(err)
	}
	item := models.Conversation{Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	permission := models.Permission{Code: constants.PermissionConversationView.Code, Status: enums.StatusOk}
	if err := db.Create(&permission).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.User{ID: 8, Username: "colleague", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.UserPermission{UserID: 8, PermissionID: permission.ID, Effect: 1}).Error; err != nil {
		t.Fatal(err)
	}
	op := &dto.AuthPrincipal{UserID: 7}
	req := request.CreateConversationNote{ClientID: "stable-note", Content: "Internal synthetic note", MentionIDs: []int64{8, 8}}
	hook := TriggerAIReplyAsyncHook
	calls := 0
	TriggerAIReplyAsyncHook = func(models.Conversation, models.Message) { calls++ }
	t.Cleanup(func() { TriggerAIReplyAsyncHook = hook })
	for range 2 {
		if err := ConversationWorkService.AddNote(item.ID, req, op); err != nil {
			t.Fatal(err)
		}
	}
	view, err := ConversationWorkService.Collaboration(item.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Notes) != 1 || len(view.Notes[0].Mentions) != 1 {
		t.Fatal("note or mentions not idempotent")
	}
	var count int64
	db.Model(&models.Notification{}).Count(&count)
	if count != 1 {
		t.Fatal("duplicate mention notification")
	}
	db.Model(&models.Message{}).Count(&count)
	if count != 0 || calls != 0 {
		t.Fatal("private note entered message or AI pipeline")
	}
	db.Model(&models.ChannelMessageOutbox{}).Count(&count)
	if count != 0 {
		t.Fatal("private note queued externally")
	}
	req.ClientID = "invalid-mention"
	req.MentionIDs = []int64{99}
	if err := ConversationWorkService.AddNote(item.ID, req, op); err == nil {
		t.Fatal("unauthorized mention accepted")
	}
}

type fakeMediaChannel struct {
	fakeWhatsApp
	media []linkedchat.OutboundMedia
}

func (f *fakeMediaChannel) SendMedia(_ context.Context, _, _, id string, media linkedchat.OutboundMedia) error {
	f.ids = append(f.ids, id)
	f.media = append(f.media, media)
	return f.err
}

func TestReceptionAcceptedHumanSendSurvivesClose(t *testing.T) {
	db := setupTelegramTestDB(t)
	channel := models.Channel{Name: "Synthetic", ChannelID: "synthetic", ChannelType: enums.ChannelTypeWhatsApp, Status: enums.StatusOk}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	item := models.Conversation{ChannelID: channel.ID, Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	external := openidentity.ExternalUser{ExternalSource: enums.ExternalSourceWhatsApp, ExternalID: whatsAppIdentity(channel.ID, "111@s.whatsapp.net", "222@s.whatsapp.net")}
	linkReceptionTestCustomer(t, item.ID, external)
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return ConversationParticipantService.CreateCustomerParticipant(ctx, item.ID, external)
	}); err != nil {
		t.Fatal(err)
	}
	fake := &fakeMediaChannel{fakeWhatsApp: fakeWhatsApp{state: linkedchat.Status{State: "connected", Account: "111@s.whatsapp.net"}}}
	s := &linkedChatService{sessions: map[int64]whatsAppSession{channel.ID: fake}}
	message, err := MessageService.SendAgentMessage(item.ID, 7, "before-close", enums.IMMessageTypeText, "Synthetic reply", "", &dto.AuthPrincipal{UserID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := ConversationService.Updates(item.ID, map[string]any{"status": enums.IMConversationStatusClosed}); err != nil {
		t.Fatal(err)
	}
	outbox := ChannelMessageOutboxService.GetByMessageID(enums.ChannelTypeWhatsApp, message.ID)
	if err := s.sendOutbox(*outbox); err != nil {
		t.Fatal(err)
	}
	if len(fake.sent) != 1 || MessageService.Get(message.ID).SendStatus != enums.IMMessageStatusSent {
		t.Fatal("accepted human message cancelled by close")
	}
}

func linkReceptionTestCustomer(t *testing.T, id int64, external openidentity.ExternalUser) {
	t.Helper()
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		customerID, err := CustomerService.EnsureExternalCustomer(ctx, external)
		if err != nil {
			return err
		}
		return ctx.Tx.Model(&models.Conversation{}).Where("id = ?", id).Update("customer_id", customerID).Error
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReceptionLinkedMediaUsesStoredAsset(t *testing.T) {
	for _, kind := range []string{enums.ChannelTypeWhatsApp, enums.ChannelTypeMessenger} {
		t.Run(kind, func(t *testing.T) {
			db := setupTelegramTestDB(t)
			if err := db.AutoMigrate(&models.Asset{}); err != nil {
				t.Fatal(err)
			}
			var oldConfig *config.Config
			func() { defer func() { _ = recover() }(); value := config.Current(); oldConfig = &value }()
			t.Cleanup(func() { config.SetCurrent(oldConfig) })
			config.SetCurrent(&config.Config{Storage: config.StorageConfig{Default: enums.AssetProviderLocal, Local: config.LocalStorageConfig{Root: t.TempDir(), BaseURL: "/storage"}}})
			var content bytes.Buffer
			if err := png.Encode(&content, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
				t.Fatal(err)
			}
			provider, err := storage.NewProvider(enums.AssetProviderLocal)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Upload(bytes.NewReader(content.Bytes()), "synthetic.png", storage.UploadInfo{}); err != nil {
				t.Fatal(err)
			}
			asset := models.Asset{AssetID: "synthetic-image", StorageKey: "synthetic.png", Provider: enums.AssetProviderLocal, Filename: "synthetic.png", MimeType: "image/png", FileSize: int64(content.Len()), Status: enums.AssetStatusSuccess}
			if err := db.Create(&asset).Error; err != nil {
				t.Fatal(err)
			}
			channel := models.Channel{Name: "Synthetic", ChannelID: "media", ChannelType: kind, Status: enums.StatusOk}
			if err := db.Create(&channel).Error; err != nil {
				t.Fatal(err)
			}
			item := models.Conversation{ChannelID: channel.ID, Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7}
			if err := db.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			s := &linkedChatService{kind: kind, sessions: map[int64]whatsAppSession{}}
			external := openidentity.ExternalUser{ExternalSource: s.externalSource(), ExternalID: whatsAppIdentity(channel.ID, "owner", "recipient")}
			linkReceptionTestCustomer(t, item.ID, external)
			fake := &fakeMediaChannel{fakeWhatsApp: fakeWhatsApp{state: linkedchat.Status{State: "connected", Account: "owner"}}}
			s.sessions[channel.ID] = fake
			payload, _ := json.Marshal(map[string]string{"assetId": asset.AssetID, "storageKey": "ignored-client-key", "provider": "ignored-client-provider"})
			message, err := MessageService.SendAgentMessage(item.ID, 7, "image", enums.IMMessageTypeImage, "", string(payload), &dto.AuthPrincipal{UserID: 7})
			if err != nil {
				t.Fatal(err)
			}
			outbox := ChannelMessageOutboxService.GetByMessageID(kind, message.ID)
			if outbox == nil {
				t.Fatal("media was not durably queued")
			}
			if err := s.sendOutbox(*outbox); err != nil {
				t.Fatal(err)
			}
			if len(fake.media) != 1 || !fake.media[0].Image || !bytes.Equal(fake.media[0].Data, content.Bytes()) || len(fake.sent) != 0 {
				t.Fatal("media not sent from canonical asset")
			}
			if err := s.sendOutbox(*outbox); err != nil {
				t.Fatal(err)
			}
			if len(fake.media) != 1 {
				t.Fatal("media replayed")
			}
		})
	}
}

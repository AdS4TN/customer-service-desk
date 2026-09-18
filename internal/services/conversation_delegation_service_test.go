package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"agent-desk/internal/whatsapp"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupDelegationTest(t *testing.T) (*gorm.DB, models.Conversation, models.Message, *conversationDelegationService) {
	t.Helper()
	db, c, m := setupCopilotTest(t)
	if err := db.AutoMigrate(&models.ConversationDelegation{}, &models.ConversationDelegationEvent{}, &models.Notification{}, &models.AgentProfile{}, &models.ConversationAssignment{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Customer{ID: c.CustomerID, Name: "Synthetic customer"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AgentProfile{UserID: 1, Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	channel := models.Channel{ChannelType: enums.ChannelTypeWeb, Status: enums.StatusOk, AIAgentID: c.AIAgentID}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&c).Updates(map[string]any{"channel_id": channel.ID, "service_mode": enums.IMConversationServiceModeHumanOnly, "last_customer_message_id": m.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&m).Update("send_status", enums.IMMessageStatusSent).Error; err != nil {
		t.Fatal(err)
	}
	c = *ConversationService.Get(c.ID)
	old := ConversationCopilotService
	ConversationCopilotService = &conversationCopilotService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: `{"action":"reply","reply":"What delivery region should we use?","reason":""}`}, nil
	}}
	t.Cleanup(func() { ConversationCopilotService = old })
	return db, c, m, &conversationDelegationService{}
}

func startTrial(t *testing.T, s *conversationDelegationService, c models.Conversation) *models.ConversationDelegation {
	t.Helper()
	d, err := repositories.GetConversationDelegation(sqls.DB(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(c.ID, request.StartConversationDelegation{Revision: d.Revision, AIAgentID: c.AIAgentID, PreviewOnly: true, DurationMinutes: 60}, &dto.AuthPrincipal{UserID: 1}); err != nil {
		t.Fatal(err)
	}
	d, err = repositories.GetConversationDelegation(sqls.DB(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func assertNoDelegationSend(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	var messages, outbox int64
	db.Model(&models.Message{}).Where("conversation_id = ? AND sender_type = ?", id, enums.IMSenderTypeAI).Count(&messages)
	db.Model(&models.ChannelMessageOutbox{}).Where("conversation_id = ?", id).Count(&outbox)
	if messages != 0 || outbox != 0 {
		t.Fatalf("trial dispatched: messages=%d outbox=%d", messages, outbox)
	}
}

func TestDelegationTrialPreservesOwnershipAndNeverSends(t *testing.T) {
	db, c, _, s := setupDelegationTest(t)
	d := startTrial(t, s, c)
	if err := s.Preview(context.Background(), c.ID, d.Revision, &dto.AuthPrincipal{UserID: 1}); err != nil {
		t.Fatal(err)
	}
	current := ConversationService.Get(c.ID)
	customer := repositories.CustomerRepository.Get(db, c.CustomerID)
	if current.CurrentAssigneeID != 1 || current.ServiceMode != enums.IMConversationServiceModeHumanOnly || customer.OwnerUserID != 1 || !current.DelegationManaged {
		t.Fatal("ownership or global mode changed")
	}
	events, err := repositories.DelegationEvents(db, c.ID)
	if err != nil || len(events) != 2 || events[0].Kind != "preview" || events[0].Content == "" {
		t.Fatalf("missing private result: %v", err)
	}
	assertNoDelegationSend(t, db, c.ID)
}

func TestDelegationRejectsLegacyAIAtFinalCommit(t *testing.T) {
	db, c, _, s := setupDelegationTest(t)
	d := startTrial(t, s, c)
	if err := s.Stop(c.ID, d.Revision, &dto.AuthPrincipal{UserID: 1}); err != nil {
		t.Fatal(err)
	}
	_, err := MessageService.sendValidatedMessage(ConversationService.Get(c.ID), enums.IMSenderTypeAI, c.AIAgentID, "legacy-in-flight", enums.IMMessageTypeText, "Must not send", "", &dto.AuthPrincipal{}, nil, "", 0, nil)
	if !errors.Is(err, errDelegationStale) {
		t.Fatalf("legacy AI response survived reclaim: %v", err)
	}
	assertNoDelegationSend(t, db, c.ID)
}

func TestDelegationCancellationUpdatesPendingMessageStatus(t *testing.T) {
	db, c, _, s := setupDelegationTest(t)
	d := startTrial(t, s, c)
	queued := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeAI, DelegationRevision: d.Revision, SendStatus: enums.IMMessageStatusSending}
	if err := db.Create(&queued).Error; err != nil {
		t.Fatal(err)
	}
	job := models.ChannelMessageOutbox{ConversationID: c.ID, MessageID: queued.ID, SendStatus: "pending"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(c.ID, d.Revision, &dto.AuthPrincipal{UserID: 1}); err != nil {
		t.Fatal(err)
	}
	db.First(&job, job.ID)
	if job.SendStatus != "ignored" || MessageService.Get(queued.ID).SendStatus != enums.IMMessageStatusFailed {
		t.Fatal("cancelled reply still appears to be sending")
	}
}

func TestDelegationPermanentDeliveryFailureReturnsToOwner(t *testing.T) {
	db, c, _, s := setupDelegationTest(t)
	d := startTrial(t, s, c)
	message := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeAI, DelegationRevision: d.Revision, SendStatus: enums.IMMessageStatusSending}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	job := models.ChannelMessageOutbox{ConversationID: c.ID, MessageID: message.ID, SendStatus: "pending"}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	linked := &linkedChatService{}
	if err := linked.finishOutbox(job, "ignored", "send_failed"); err != nil {
		t.Fatal(err)
	}
	after, _ := repositories.GetConversationDelegation(db, c.ID)
	if after.Active || after.EndReason != "sending_failed" || ConversationService.Get(c.ID).CurrentAssigneeID != 1 {
		t.Fatal("failed delivery did not return control to owner")
	}
	var notes int64
	db.Model(&models.Notification{}).Where("recipient_user_id = ?", 1).Count(&notes)
	if notes != 1 {
		t.Fatal("missing delivery failure notification")
	}
}

func TestDelegationLinkedPhoneHistoryAndEcho(t *testing.T) {
	db, c, _, s := setupDelegationTest(t)
	if err := db.Model(&models.Channel{}).Where("id = ?", c.ChannelID).Updates(map[string]any{"channel_type": enums.ChannelTypeWhatsApp, "channel_id": "synthetic"}).Error; err != nil {
		t.Fatal(err)
	}
	identity := models.CustomerIdentity{CustomerID: c.CustomerID, ExternalSource: enums.ExternalSourceWhatsApp, ExternalID: whatsAppIdentity(c.ChannelID, "synthetic-owner", "synthetic-customer")}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatal(err)
	}
	d := startTrial(t, s, c)
	linked := &linkedChatService{sessions: make(map[int64]whatsAppSession)}
	incoming := whatsapp.Incoming{ID: "old-phone", Account: "synthetic-owner", Chat: "synthetic-customer", Text: "Historical phone reply", FromMe: true, History: true, SentAt: time.Now().Add(-time.Hour)}
	if err := linked.Receive(c.ChannelID, incoming); err != nil {
		t.Fatal(err)
	}
	after, _ := repositories.GetConversationDelegation(db, c.ID)
	if !after.Active {
		t.Fatal("phone history stopped delegation")
	}
	// A saved platform reply is echoed by the linked phone; it is not a new human reply.
	reply := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeAI, DelegationRevision: d.Revision, SendStatus: enums.IMMessageStatusSent}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatal(err)
	}
	incoming.ID, incoming.History, incoming.SentAt = whatsAppProtocolID("synthetic", reply.ID), false, time.Now()
	job := models.ChannelMessageOutbox{ChannelType: enums.ChannelTypeWhatsApp, ConversationID: c.ID, MessageID: reply.ID, SendStatus: "sent", Payload: linked.outboxPayload(incoming.ID)}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	if err := linked.Receive(c.ChannelID, incoming); err != nil {
		t.Fatal(err)
	}
	after, _ = repositories.GetConversationDelegation(db, c.ID)
	if !after.Active || MessageService.Take("client_msg_id = ?", linked.messageID(c.ChannelID, incoming)) != nil {
		t.Fatal("platform echo treated as human takeover")
	}
	incoming.ID = "new-phone-reply"
	if err := linked.Receive(c.ChannelID, incoming); err != nil {
		t.Fatal(err)
	}
	after, _ = repositories.GetConversationDelegation(db, c.ID)
	if after.Active || after.EndReason != "phone_reply" {
		t.Fatal("new phone reply did not take over")
	}
	var jobs int64
	db.Model(&models.ChannelMessageOutbox{}).Count(&jobs)
	if jobs != 1 {
		t.Fatal("receive created a send job")
	}
}

func TestDelegationPermissionsHoldAndStaleRevision(t *testing.T) {
	db, c, _, s := setupDelegationTest(t)
	req := request.StartConversationDelegation{AIAgentID: c.AIAgentID, PreviewOnly: true}
	if err := s.Start(c.ID, req, &dto.AuthPrincipal{UserID: 2}); err == nil {
		t.Fatal("other salesperson delegated customer")
	}
	req.PreviewOnly = false
	if err := s.Start(c.ID, req, &dto.AuthPrincipal{UserID: 1}); err == nil {
		t.Fatal("human-only setting bypassed")
	}
	db.Model(&c).Update("service_mode", enums.IMConversationServiceModeAIFirst)
	db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("service_mode", enums.IMConversationServiceModeAIFirst)
	db.Model(&models.Channel{}).Where("id = ?", c.ChannelID).Update("channel_type", enums.ChannelTypeWhatsApp)
	if err := s.Start(c.ID, req, &dto.AuthPrincipal{UserID: 1}); err == nil {
		t.Fatal("WhatsApp production hold bypassed")
	}
	d := startTrial(t, s, c)
	if err := s.Stop(c.ID, d.Revision-1, &dto.AuthPrincipal{UserID: 1}); err == nil {
		t.Fatal("stale stop accepted")
	}
	if err := s.Stop(c.ID, d.Revision, &dto.AuthPrincipal{UserID: 2}); err == nil {
		t.Fatal("other salesperson took over")
	}
	assertNoDelegationSend(t, db, c.ID)
}

func TestDelegationReclaimDiscardsInFlightGeneration(t *testing.T) {
	db, c, _, s := setupDelegationTest(t)
	d := startTrial(t, s, c)
	entered, release := make(chan struct{}), make(chan struct{})
	ConversationCopilotService.complete = func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		close(entered)
		<-release
		return &ai.ChatCompletionResult{Content: `{"action":"reply","reply":"Old response"}`}, nil
	}
	done := make(chan error, 1)
	go func() { done <- s.Preview(context.Background(), c.ID, d.Revision, &dto.AuthPrincipal{UserID: 1}) }()
	<-entered
	if err := s.Stop(c.ID, d.Revision, &dto.AuthPrincipal{UserID: 1}); err != nil {
		t.Fatal(err)
	}
	startTrial(t, s, c)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	events, _ := repositories.DelegationEvents(db, c.ID)
	for _, e := range events {
		if e.Kind == "preview" {
			t.Fatal("old generation survived reclaim and restart")
		}
	}
	assertNoDelegationSend(t, db, c.ID)
}

func TestDelegationHandoffFailureAndExpiryNotifyOriginalOwner(t *testing.T) {
	for _, kind := range []string{"handoff", "generation_failed", "expired", "unsupported_message"} {
		t.Run(kind, func(t *testing.T) {
			db, c, m, s := setupDelegationTest(t)
			d := startTrial(t, s, c)
			switch kind {
			case "handoff":
				ConversationCopilotService.complete = func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
					return &ai.ChatCompletionResult{Content: `{"action":"handoff","reply":"","reason":"Customer requests a negotiated discount; original salesperson approval is needed."}`}, nil
				}
			case "generation_failed":
				ConversationCopilotService.complete = func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
					return nil, errors.New("synthetic provider failure")
				}
			case "expired":
				db.Model(d).Update("expires_at", time.Now().Add(-time.Minute))
			case "unsupported_message":
				db.Model(&m).Update("message_type", enums.IMMessageTypeImage)
			}
			err := s.Preview(context.Background(), c.ID, d.Revision, &dto.AuthPrincipal{UserID: 1})
			if kind != "generation_failed" && err != nil {
				t.Fatal(err)
			}
			after, _ := repositories.GetConversationDelegation(db, c.ID)
			if after.Active || after.EndReason != kind {
				t.Fatalf("not handed back: %+v", after)
			}
			var notes []models.Notification
			db.Find(&notes)
			if len(notes) != 1 || notes[0].RecipientUserID != 1 || notes[0].BizID != c.ID {
				t.Fatal("original owner was not notified")
			}
			if ConversationService.Get(c.ID).CurrentAssigneeID != 1 {
				t.Fatal("handoff moved to public pool")
			}
			assertNoDelegationSend(t, db, c.ID)
		})
	}
}

func TestDelegationPhoneAndWorkbenchRepliesStopButHistoryDoesNot(t *testing.T) {
	for _, phone := range []bool{false, true} {
		t.Run(map[bool]string{false: "workbench", true: "phone"}[phone], func(t *testing.T) {
			db, c, _, s := setupDelegationTest(t)
			startTrial(t, s, c)
			m := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeAgent, SenderID: 1, SendStatus: enums.IMMessageStatusSent, IsHistorical: true}
			if phone {
				m.SenderID = 0
			}
			if err := sqls.WithTransaction(func(tx *sqls.TxContext) error { return ConversationWorkService.onMessage(tx, &m) }); err != nil {
				t.Fatal(err)
			}
			d, _ := repositories.GetConversationDelegation(db, c.ID)
			if !d.Active {
				t.Fatal("history stopped delegation")
			}
			m.IsHistorical = false
			if err := sqls.WithTransaction(func(tx *sqls.TxContext) error { return ConversationWorkService.onMessage(tx, &m) }); err != nil {
				t.Fatal(err)
			}
			d, _ = repositories.GetConversationDelegation(db, c.ID)
			if d.Active {
				t.Fatal("human reply did not stop delegation")
			}
			assertNoDelegationSend(t, db, c.ID)
		})
	}
}

func TestDelegationLiveWebReplyAndStaleCommit(t *testing.T) {
	db, c, m, s := setupDelegationTest(t)
	db.Model(&c).Update("service_mode", enums.IMConversationServiceModeAIFirst)
	db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("service_mode", enums.IMConversationServiceModeAIFirst)
	if err := s.Start(c.ID, request.StartConversationDelegation{AIAgentID: c.AIAgentID, PreviewOnly: false}, &dto.AuthPrincipal{UserID: 1}); err != nil {
		t.Fatal(err)
	}
	d, _ := repositories.GetConversationDelegation(db, c.ID)
	c = *ConversationService.Get(c.ID)
	r := &response.ConversationReplySuggestion{LastMessageID: m.ID, Content: "Synthetic web-only reply"}
	msg, err := MessageService.sendDelegatedReply(&c, d, r)
	if err != nil || msg == nil || msg.DelegationRevision != d.Revision {
		t.Fatalf("web reply failed: %v", err)
	}
	if err := s.Stop(c.ID, d.Revision, &dto.AuthPrincipal{UserID: 1}); err != nil {
		t.Fatal(err)
	}
	r.LastMessageID = ConversationService.Get(c.ID).LastMessageID
	d.LastProcessedMessageID = 0
	// A new client ID prevents deduplication from hiding a stale-generation rejection.
	if _, err := MessageService.sendDelegatedReply(ConversationService.Get(c.ID), d, r); !errors.Is(err, errDelegationStale) {
		t.Fatalf("stale send: %v", err)
	}
}

func TestDelegationIgnoresHistoryAndDoesNotReplayOnStart(t *testing.T) {
	db, c, m, s := setupDelegationTest(t)
	startTrial(t, s, c)
	calls := 0
	ConversationCopilotService.complete = func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		calls++
		return nil, errors.New("must not be called")
	}
	if err := s.process(context.Background(), c.ID, false, -1); err != nil {
		t.Fatal(err)
	}
	m.ID = 0
	m.ClientMsgID = "late-history"
	m.IsHistorical = true
	db.Create(&m)
	db.Model(&c).Update("last_message_id", m.ID)
	if err := s.process(context.Background(), c.ID, false, -1); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("replayed old customer message")
	}
}

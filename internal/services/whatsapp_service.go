package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"agent-desk/internal/messenger"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/linkedchat"
	"agent-desk/internal/pkg/openidentity"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"
	"agent-desk/internal/whatsapp"
	"encoding/binary"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"strconv"
)

var WhatsAppService = &linkedChatService{sessions: make(map[int64]whatsAppSession)}

type whatsAppSession interface {
	Status() linkedchat.Status
	Connect() error
	Disconnect()
	Logout(context.Context) error
	Send(context.Context, string, string, string, string) error
}

type whatsAppService = linkedChatService

type linkedChatService struct {
	kind     string
	mu       sync.Mutex
	inbound  sync.Mutex
	dispatch sync.Mutex
	sessions map[int64]whatsAppSession
}

type whatsAppConfig struct {
	AutoConnect bool `json:"autoConnect"`
}

func (s *linkedChatService) Start() {
	if err := s.recoverPendingMedia(); err != nil {
		slog.Warn("linked media recovery failed", "channel_type", s.channelType())
	}
	for _, channel := range ChannelService.Find(sqls.NewCnd().Eq("channel_type", s.channelType()).Eq("status", enums.StatusOk)) {
		s.Resume(channel.ID)
	}
	// A process may have stopped after claiming a durable outbox item.
	if err := repositories.RecoverLinkedOutbox(sqls.DB(), s.channelType()); err != nil {
		slog.Error("recover whatsapp outbox", "error", err)
	}
}

func (s *linkedChatService) Resume(id int64) {
	channel := ChannelService.Get(id)
	if channel == nil || channel.Status != enums.StatusOk || channel.ChannelType != s.channelType() {
		return
	}
	var cfg whatsAppConfig
	_ = json.Unmarshal([]byte(channel.ConfigJSON), &cfg)
	if cfg.AutoConnect {
		if _, err := s.Connect(id); err != nil {
			slog.Error("resume whatsapp", "channel_id", id, "error", err)
		}
	}
}

func (s *linkedChatService) channel(id int64) (*models.Channel, error) {
	channel := ChannelService.Get(id)
	if channel == nil || channel.Status == enums.StatusDeleted || channel.ChannelType != s.channelType() {
		return nil, errorsx.InvalidParamI18n(s.errorKey("channelNotFound"))
	}
	return channel, nil
}

func (s *linkedChatService) Status(id int64) (linkedchat.Status, error) {
	channel, err := s.channel(id)
	if err != nil {
		return linkedchat.Status{}, err
	}
	if channel.Status != enums.StatusOk {
		return linkedchat.Status{State: "disabled"}, nil
	}
	s.mu.Lock()
	session := s.sessions[id]
	s.mu.Unlock()
	if session == nil {
		if s.kind == "messenger" {
			return messenger.SavedStatus(id), nil
		}
		return linkedchat.Status{State: "disconnected"}, nil
	}
	return session.Status(), nil
}

func (s *linkedChatService) Connect(id int64) (linkedchat.Status, error) {
	channel, err := s.channel(id)
	if err != nil {
		return linkedchat.Status{}, err
	}
	if channel.Status != enums.StatusOk {
		return linkedchat.Status{}, errorsx.InvalidParamI18n(s.errorKey("disabled"))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[id]
	if session == nil {
		dir := strings.TrimSpace(os.Getenv("AGENT_DESK_WHATSAPP_SESSION_DIR"))
		if dir == "" {
			dir = "data/whatsapp"
		}
		open := func(dir string, id int64, receive func(linkedchat.Incoming) error) (whatsAppSession, error) {
			return whatsapp.Open(dir, id, receive)
		}
		if s.kind == "messenger" {
			dir = messenger.SessionDir()
			open = func(dir string, id int64, receive func(linkedchat.Incoming) error) (whatsAppSession, error) {
				return messenger.Open(dir, id, receive)
			}
		}
		session, err = open(dir, id, func(msg linkedchat.Incoming) error {
			err := s.Receive(id, msg)
			if err != nil {
				slog.Error("whatsapp receive failed", "channel_id", id, "error", err)
			}
			return err
		})
		if err != nil {
			slog.Error("open whatsapp store", "error", err)
			return linkedchat.Status{}, errorsx.BusinessErrorI18n(1, s.errorKey("storeFailed"))
		}
		if wa, ok := session.(*whatsapp.Session); ok {
			wa.SetHistoryTargets(func(account string) ([]whatsapp.HistoryRequest, error) {
				return s.historyTargets(id, account)
			}, func(msg whatsapp.Incoming) bool {
				for _, chat := range append([]string{msg.Chat}, msg.ChatAliases...) {
					candidate := msg
					candidate.Chat = chat
					if MessageService.Take("client_msg_id = ?", s.messageID(id, candidate)) != nil {
						return false
					}
				}
				return true
			})
		}
		s.sessions[id] = session
	}
	if err := ChannelService.UpdateColumn(id, "config_json", `{"autoConnect":true}`); err != nil {
		return linkedchat.Status{}, err
	}
	if err := session.Connect(); err != nil {
		return linkedchat.Status{}, errorsx.BusinessErrorI18n(1, s.errorKey("connectFailed"))
	}
	return session.Status(), nil
}

func (s *linkedChatService) Disconnect(id int64, logout bool) error {
	if _, err := s.channel(id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if logout && s.kind != "messenger" && (s.sessions[id] == nil || s.sessions[id].Status().State != "connected") {
		return errorsx.BusinessErrorI18n(1, s.errorKey("logoutFailed"))
	}
	if s.kind == "messenger" && logout && s.sessions[id] == nil {
		if err := messenger.Forget(id); err != nil {
			return errorsx.BusinessErrorI18n(1, s.errorKey("logoutFailed"))
		}
	}
	if session := s.sessions[id]; session != nil {
		if logout {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := session.Logout(ctx); err != nil {
				return errorsx.BusinessErrorI18n(1, s.errorKey("logoutFailed"))
			}
		} else {
			session.Disconnect()
		}
	}
	return ChannelService.UpdateColumn(id, "config_json", `{"autoConnect":false}`)
}

func (s *linkedChatService) Suspend(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[id]; session != nil {
		session.Disconnect()
	}
}

func whatsAppIdentity(channelID int64, account, chat string) string {
	return fmt.Sprintf("%d|%s|%s", channelID, account, chat)
}

func whatsAppMessageID(channelID int64, msg linkedchat.Incoming) string {
	sum := sha256.Sum256([]byte(whatsAppIdentity(channelID, msg.Account, msg.Chat) + "|" + msg.ID))
	return "wa_" + hex.EncodeToString(sum[:])
}

func (s *linkedChatService) Receive(channelID int64, msg linkedchat.Incoming) error {
	// Serialize creation and deduplication, including replays after a conversation was closed.
	s.inbound.Lock()
	defer s.inbound.Unlock()
	channel, err := s.channel(channelID)
	if err != nil {
		return err
	}
	if channel.Status != enums.StatusOk {
		return nil
	}
	// Keep existing customers created under the older LID-based connector.
	for _, chat := range append([]string{msg.Chat}, msg.ChatAliases...) {
		if identity := repositories.CustomerIdentityRepository.GetBy(sqls.DB(), s.externalSource(), whatsAppIdentity(channelID, msg.Account, chat)); identity != nil {
			msg.Chat = chat
			break
		}
	}
	clientID := s.messageID(channelID, msg)
	if msg.Message != nil && msg.Message.TargetID != "" {
		metadata := *msg.Message
		target := msg
		target.ID = metadata.TargetID
		if original := MessageService.Take("client_msg_id = ?", s.messageID(channelID, target)); original != nil {
			metadata.TargetPreview = limitText(original.Content, 160)
		}
		msg.Message = &metadata
	}
	if existing := MessageService.Take("client_msg_id = ?", clientID); existing != nil {
		return s.updateIncomingMedia(existing, msg)
	}
	if msg.FromMe {
		echoID := msg.ID
		if msg.EchoID != "" {
			echoID = msg.EchoID
		}
		// Workbench/AI replies are already saved. Phone echoes must not duplicate them.
		for _, candidate := range repositories.FindLinkedEchoCandidates(sqls.DB(), s.channelType(), channel.ID, s.outboxPayload(echoID)) {
			if s.protocolID(channel.ChannelID, candidate.MessageID) == echoID {
				conv := ConversationService.Get(candidate.ConversationID)
				identity := ConversationService.GetConversationExternalIdentity(conv)
				if identity != nil && identity.ExternalID == whatsAppIdentity(channel.ID, msg.Account, msg.Chat) {
					return nil
				}
			}
		}
	}
	name := strings.TrimSpace(msg.Name)
	if name == "" {
		name = s.displayName() + " " + strings.Split(msg.Chat, "@")[0]
	}
	external := openidentity.ExternalUser{ExternalSource: s.externalSource(),
		ExternalID: whatsAppIdentity(channelID, msg.Account, msg.Chat), ExternalName: name}
	if msg.History || msg.FromMe || msg.Message != nil {
		return s.importMessage(channel, msg, external)
	}
	conversation, err := ConversationService.Create(external, channel.ID, channel.AIAgentID)
	if err != nil {
		return err
	}
	if conversation.ChannelID != channel.ID {
		return errorsx.InvalidParamI18n(s.errorKey("identityMismatch"))
	}
	payload, _ := json.Marshal(map[string]any{"whatsappMessageId": msg.ID, "whatsappChat": msg.Chat, "timestamp": msg.SentAt})
	_, err = MessageService.SendCustomerMessage(conversation.ID, clientID, enums.IMMessageTypeText, msg.Text, string(payload), external)
	return err
}

// Import bypasses all send hooks, welcome messages, AI dispatch and outboxes.
func (s *linkedChatService) importMessage(channel *models.Channel, incoming linkedchat.Incoming, external openidentity.ExternalUser) error {
	var conversation *models.Conversation
	var message *models.Message
	created := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		customerID := int64(0)
		if identity := repositories.CustomerIdentityRepository.GetBy(ctx.Tx, external.ExternalSource, external.ExternalID); identity != nil {
			customerID = identity.CustomerID
		} else {
			var err error
			customerID, err = CustomerService.EnsureExternalCustomer(ctx, external)
			if err != nil {
				return err
			}
		}
		sentAt := incoming.SentAt
		if sentAt.IsZero() {
			sentAt = time.Now()
		}
		// History arriving late belongs to the existing thread, even when closed.
		conversation = repositories.ConversationRepository.FindOne(ctx.Tx, sqls.NewCnd().Eq("customer_id", customerID).Eq("channel_id", channel.ID).Desc("id"))
		if conversation == nil || (!incoming.History && !incoming.FromMe && conversation.Status == enums.IMConversationStatusClosed) {
			agent := repositories.AIAgentRepository.Get(ctx.Tx, channel.AIAgentID)
			if agent == nil {
				return errorsx.InvalidParamI18n("error.e0002")
			}
			conversation = &models.Conversation{ChannelID: channel.ID, AIAgentID: agent.ID, CustomerID: customerID,
				CustomerName: ConversationService.getCustomerName(ctx.Tx, customerID), ServiceMode: agent.ServiceMode,
				Status: ConversationService.resolveInitialStatus(agent.ServiceMode), AuditFields: utils.BuildAuditFields(nil)}
			if err := repositories.ConversationRepository.Create(ctx.Tx, conversation); err != nil {
				return err
			}
			if err := ConversationParticipantService.CreateCustomerParticipant(ctx, conversation.ID, external); err != nil {
				return err
			}
			created = true
		}
		payload, _ := json.Marshal(map[string]any{"whatsappMessageId": incoming.ID, "whatsappChat": incoming.Chat,
			"whatsappHistory": incoming.History, "whatsappFromMe": incoming.FromMe, "timestamp": sentAt, "linkedMessage": incoming.Message})
		message = &models.Message{ConversationID: conversation.ID, ClientMsgID: s.messageID(channel.ID, incoming),
			IsHistorical: incoming.History,
			SenderType:   enums.IMSenderTypeCustomer, Content: incoming.Text, Payload: string(payload),
			MessageType: enums.IMMessageTypeText, SendStatus: enums.IMMessageStatusSent, SentAt: &sentAt,
			AuditFields: utils.BuildAuditFields(nil)}
		if incoming.Message != nil {
			message.MessageType = enums.IMMessageTypeAttachment
			if incoming.Message.Kind == "image" || incoming.Message.Kind == "sticker" {
				message.MessageType = enums.IMMessageTypeImage
			}
		}
		if incoming.FromMe {
			message.SenderType = enums.IMSenderTypeAgent
			message.ReplyToCustomerMessageID = conversation.LastCustomerMessageID
		}
		if err := repositories.MessageRepository.Create(ctx.Tx, message); err != nil {
			return err
		}
		if !incoming.History && !incoming.FromMe && incoming.Message != nil {
			unread, customerUnread, err := MessageService.handleReadState(ctx, enums.IMSenderTypeCustomer, conversation, nil, message, &external)
			if err != nil {
				return err
			}
			updates := map[string]any{"agent_unread_count": unread, "customer_unread_count": customerUnread}
			if conversation.Status == enums.IMConversationStatusAIServing && incoming.Message.Kind != "reaction" {
				updates["status"] = enums.IMConversationStatusPending
				updates["handoff_at"] = time.Now()
				updates["handoff_reason"] = "whatsapp_media_received"
			}
			if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, updates); err != nil {
				return err
			}
		}
		if err := repositories.UpdateWhatsAppImportedSummary(ctx.Tx, conversation.ID, message, limitText(buildMessageSummary(message.MessageType, message.Content), 255)); err != nil {
			return err
		}
		if incoming.Message == nil || incoming.Message.Kind != "reaction" {
			if err := ConversationWorkService.onMessage(ctx, message); err != nil {
				return err
			}
		}
		// A live phone reply is human takeover; historical outgoing messages are not.
		if incoming.FromMe && !incoming.History && conversation.Status == enums.IMConversationStatusAIServing && (incoming.Message == nil || incoming.Message.Kind != "reaction") {
			return repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
				"status": enums.IMConversationStatusPending, "handoff_at": sentAt,
				"handoff_reason": s.channelType() + "_phone_reply", "updated_at": time.Now(),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	conversation = ConversationService.Get(conversation.ID)
	if created {
		WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationCreated)
	}
	WsService.PublishMessageCreated(conversation, message)
	WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationUpdated)
	if !incoming.History {
		if err := ConversationMemoryService.Queue(conversation.ID); err != nil {
			slog.Warn("conversation memory enqueue failed", "conversation_id", conversation.ID)
		}
	}
	return nil
}

func whatsAppProtocolID(channelID string, messageID int64) string {
	digest := sha256.Sum256([]byte(channelID + fmt.Sprintf(":%d", messageID)))
	return strings.ToUpper(hex.EncodeToString(digest[:16]))
}

func whatsAppOutboxPayload(id string) string {
	payload, _ := json.Marshal(map[string]string{"whatsappMessageId": id})
	return string(payload)
}

func (s *linkedChatService) validateOutbound(conversation *models.Conversation, sender enums.IMSenderType, messageType enums.IMMessageType, content string) error {
	if sender != enums.IMSenderTypeAI && sender != enums.IMSenderTypeAgent {
		return nil
	}
	channel := ChannelService.Get(conversation.ChannelID)
	if channel == nil || channel.ChannelType != s.channelType() {
		return nil
	}
	if messageType == enums.IMMessageTypeImage || messageType == enums.IMMessageTypeAttachment {
		return nil
	}
	if messageType != enums.IMMessageTypeText && messageType != enums.IMMessageTypeHTML {
		return errorsx.InvalidParamI18n(s.errorKey("textOnly"))
	}
	if messageType == enums.IMMessageTypeHTML {
		chunks, err := utils.SplitHTMLContentChunks(content)
		if err != nil {
			return err
		}
		for _, chunk := range chunks {
			if chunk.Type != "text" {
				return errorsx.InvalidParamI18n(s.errorKey("textOnly"))
			}
		}
	}
	return nil
}

// Enqueue in the message transaction: a saved reply must not lose its delivery job.
func (s *linkedChatService) enqueue(db *gorm.DB, conversation *models.Conversation, message *models.Message) error {
	if message.SenderType != enums.IMSenderTypeAI && message.SenderType != enums.IMSenderTypeAgent {
		return nil
	}
	channel := repositories.ChannelRepository.Get(db, conversation.ChannelID)
	if channel == nil || channel.ChannelType != s.channelType() {
		return nil
	}
	message.SendStatus = enums.IMMessageStatusSending
	if err := repositories.MessageRepository.UpdateColumn(db, message.ID, "send_status", message.SendStatus); err != nil {
		return err
	}
	return repositories.ChannelMessageOutboxRepository.Create(db, &models.ChannelMessageOutbox{
		ChannelType: s.channelType(), ConversationID: conversation.ID, MessageID: message.ID,
		Payload:    s.outboxPayload(s.protocolID(channel.ChannelID, message.ID)),
		SendStatus: string(enums.ChannelMessageOutboxStatusPending), AuditFields: message.AuditFields,
	})
}

func (s *linkedChatService) DispatchPendingOutbox() {
	if !s.dispatch.TryLock() {
		return
	}
	defer s.dispatch.Unlock()
	for _, item := range repositories.ListLinkedDueOutbox(sqls.DB(), s.channelType(), 20) {
		if err := s.sendOutbox(item); err != nil {
			slog.Error("whatsapp outbox", "id", item.ID, "error", err)
		}
	}
}

func (s *linkedChatService) sendOutbox(item models.ChannelMessageOutbox) error {
	message := MessageService.Get(item.MessageID)
	conversation := ConversationService.Get(item.ConversationID)
	if message == nil || conversation == nil {
		return s.finishOutbox(item, "ignored", "message_missing")
	}
	channel := ChannelService.Get(conversation.ChannelID)
	if channel == nil || channel.Status != enums.StatusOk || channel.ChannelType != s.channelType() {
		return s.finishOutbox(item, "ignored", "channel_disabled")
	}
	if message.RecalledAt != nil || message.SendStatus == enums.IMMessageStatusRecalled ||
		(message.SenderType == enums.IMSenderTypeAI && conversation.Status != enums.IMConversationStatusAIServing) {
		return s.finishOutbox(item, "ignored", "reply_cancelled")
	}
	identity := ConversationService.GetConversationExternalIdentity(conversation)
	if identity == nil || identity.ExternalSource != s.externalSource() {
		return s.finishOutbox(item, "ignored", "identity_missing")
	}
	parts := strings.Split(identity.ExternalID, "|")
	if len(parts) != 3 || parts[0] != fmt.Sprint(channel.ID) {
		return s.finishOutbox(item, "ignored", "identity_mismatch")
	}
	s.mu.Lock()
	session := s.sessions[channel.ID]
	s.mu.Unlock()
	if session == nil || session.Status().State != "connected" {
		// Being offline is not a send failure; preserve replies without burning retries.
		return ChannelMessageOutboxService.Updates(item.ID, map[string]any{"next_retry_at": time.Now().Add(10 * time.Second)})
	}
	if session.Status().Account != parts[1] {
		return s.finishOutbox(item, "ignored", "account_changed")
	}
	if ok, err := repositories.ClaimLinkedOutbox(sqls.DB(), s.channelType(), item.ID); err != nil || !ok {
		return err
	}
	isMedia := message.MessageType == enums.IMMessageTypeImage || message.MessageType == enums.IMMessageTypeAttachment
	timeout := 25 * time.Second
	if isMedia {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	text := message.Content
	if message.MessageType == enums.IMMessageTypeHTML {
		text = utils.BuildRuntimeMessageText(message.MessageType, message.Content)
	}
	// Stable protocol IDs are reused for retries after ambiguous network failures.
	id := s.protocolID(channel.ChannelID, message.ID)
	var sendErr error
	if isMedia {
		sendErr = sendLinkedMedia(ctx, session, parts[1], parts[2], id, message)
	} else {
		sendErr = session.Send(ctx, parts[1], parts[2], id, text)
	}
	if sendErr != nil {
		slog.Warn("linked message send failed", "message_id", message.ID)
		status := "failed"
		if item.RetryCount >= 4 {
			status = "ignored"
		}
		return s.finishOutbox(item, status, "send_failed")
	}
	return s.finishOutbox(item, "sent", "")
}

func (s *linkedChatService) finishOutbox(item models.ChannelMessageOutbox, status, reason string) error {
	now := time.Now()
	columns := map[string]any{"send_status": status, "last_error": reason, "updated_at": now}
	messageStatus := enums.IMMessageStatusSending
	if status == "sent" {
		columns["sent_at"] = now
		messageStatus = enums.IMMessageStatusSent
	}
	if status == "ignored" {
		messageStatus = enums.IMMessageStatusFailed
	}
	if status == "failed" {
		columns["retry_count"] = item.RetryCount + 1
		columns["next_retry_at"] = now.Add(time.Duration(item.RetryCount+1) * 15 * time.Second)
	}
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		if err := repositories.ChannelMessageOutboxRepository.Updates(tx.Tx, item.ID, columns); err != nil {
			return err
		}
		message := repositories.MessageRepository.Get(tx.Tx, item.MessageID)
		if message == nil || message.SendStatus == enums.IMMessageStatusRecalled {
			return nil
		}
		if err := repositories.MessageRepository.UpdateColumn(tx.Tx, item.MessageID, "send_status", messageStatus); err != nil {
			return err
		}
		message.SendStatus = messageStatus
		return ConversationWorkService.onMessage(tx, message)
	})
	if err == nil {
		if message := MessageService.Get(item.MessageID); message != nil {
			WsService.PublishMessageCreated(ConversationService.Get(item.ConversationID), message)
			WsService.PublishConversationChanged(ConversationService.Get(item.ConversationID), enums.IMRealtimeEventConversationUpdated)
		}
	}
	return err
}

var MessengerService = &linkedChatService{kind: "messenger", sessions: make(map[int64]whatsAppSession)}

func (s *linkedChatService) channelType() string {
	if s.kind == "messenger" {
		return enums.ChannelTypeMessenger
	}
	return enums.ChannelTypeWhatsApp
}
func (s *linkedChatService) externalSource() enums.ExternalSource {
	if s.kind == "messenger" {
		return enums.ExternalSourceMessenger
	}
	return enums.ExternalSourceWhatsApp
}
func (s *linkedChatService) displayName() string {
	if s.kind == "messenger" {
		return "Messenger"
	}
	return "WhatsApp"
}
func (s *linkedChatService) errorKey(key string) string {
	return "error." + s.channelType() + "." + key
}
func (s *linkedChatService) messageID(id int64, msg linkedchat.Incoming) string {
	value := whatsAppMessageID(id, msg)
	if s.kind == "messenger" {
		return "ms_" + strings.TrimPrefix(value, "wa_")
	}
	return value
}
func (s *linkedChatService) protocolID(channel string, id int64) string {
	if s.kind != "messenger" {
		return whatsAppProtocolID(channel, id)
	}
	sum := sha256.Sum256([]byte(channel + ":" + strconv.FormatInt(id, 10)))
	return strconv.FormatUint((binary.BigEndian.Uint64(sum[:8])&((1<<63)-1))|1, 10)
}
func (s *linkedChatService) outboxPayload(id string) string {
	if s.kind != "messenger" {
		return whatsAppOutboxPayload(id)
	}
	payload, _ := json.Marshal(map[string]string{"messengerMessageId": id})
	return string(payload)
}

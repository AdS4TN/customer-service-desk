package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/reception"
	"agent-desk/internal/repositories"
	"agent-desk/internal/whatsapp"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ConversationDelegationService = &conversationDelegationService{}
var errDelegationStale = errors.New("delegation changed")

type conversationDelegationService struct {
	tick    sync.Mutex
	running sync.Map
	// Serialize takeover with the final transport send, never with model generation.
	delivery [128]sync.Mutex
}

func (s *conversationDelegationService) lockDelivery(id int64) func() {
	m := &s.delivery[uint64(id)%uint64(len(s.delivery))]
	m.Lock()
	return m.Unlock
}

func delegationOwner(db *gorm.DB, c *models.Conversation) int64 {
	if customer := repositories.CustomerRepository.Get(db, c.CustomerID); customer != nil && customer.OwnerUserID > 0 {
		return customer.OwnerUserID
	}
	return c.CurrentAssigneeID
}

func canManageDelegation(c *models.Conversation, owner int64, operator *dto.AuthPrincipal) bool {
	return operator != nil && c.Status == enums.IMConversationStatusActive && c.CurrentAssigneeID > 0 && c.CurrentAssigneeID == owner &&
		(ConversationService.isAdmin(operator) || (operator.UserID == owner && operator.UserID == c.CurrentAssigneeID))
}

func delegationLiveAvailable(c *models.Conversation, channel *models.Channel, agent *models.AIAgent) bool {
	if channel == nil || channel.Status != enums.StatusOk || !reception.AutomaticMessagesAllowed(c, agent) {
		return false
	}
	switch channel.ChannelType {
	case enums.ChannelTypeWeb, enums.ChannelTypeMessenger:
		return true
	case enums.ChannelTypeWhatsApp:
		return !whatsapp.OutboundMessagesDisabled
	default:
		return false
	}
}

type ConversationDelegationDetail struct {
	Conversation *models.Conversation
	State        *models.ConversationDelegation
	OwnerID      int64
	Names        map[int64]string
	Agents       []response.DelegationAgentOption
	Events       []models.ConversationDelegationEvent
	CanManage    bool
}

func (s *conversationDelegationService) View(id int64, operator *dto.AuthPrincipal) (*ConversationDelegationDetail, error) {
	c := ConversationService.Get(id)
	if c == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	owner := delegationOwner(sqls.DB(), c)
	if operator == nil || (owner > 0 && operator.UserID != owner && operator.UserID != c.CurrentAssigneeID && !ConversationService.isAdmin(operator)) {
		return nil, errorsx.ForbiddenI18n("error.delegation.notOwner")
	}
	d, err := repositories.GetConversationDelegation(sqls.DB(), id)
	if err != nil {
		return nil, errorsx.InvalidParamI18n("error.delegation.failed")
	}
	events, err := repositories.DelegationEvents(sqls.DB(), id)
	if err != nil {
		return nil, errorsx.InvalidParamI18n("error.delegation.failed")
	}
	names := map[int64]string{owner: collaborationName(owner), d.StartedBy: collaborationName(d.StartedBy)}
	for _, e := range events {
		names[e.ActorID] = collaborationName(e.ActorID)
	}
	agents := []response.DelegationAgentOption{}
	channel := ChannelService.Get(c.ChannelID)
	for _, a := range AIAgentService.Find(sqls.NewCnd().Eq("status", enums.StatusOk).Asc("id")) {
		agents = append(agents, response.DelegationAgentOption{ID: a.ID, Name: a.Name, LiveAvailable: delegationLiveAvailable(c, channel, &a)})
	}
	return &ConversationDelegationDetail{c, d, owner, names, agents, events, canManageDelegation(c, owner, operator)}, nil
}

func (s *conversationDelegationService) Start(id int64, req request.StartConversationDelegation, operator *dto.AuthPrincipal) error {
	if req.AIAgentID <= 0 || req.DurationMinutes < 0 || req.DurationMinutes > 43200 {
		return errorsx.InvalidParamI18n("error.delegation.invalid")
	}
	unlock := s.lockDelivery(id)
	defer unlock()
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		c, err := repositories.LockConversationWork(tx.Tx, id)
		if err != nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		owner := delegationOwner(tx.Tx, c)
		if !canManageDelegation(c, owner, operator) {
			return errorsx.ForbiddenI18n("error.delegation.notOwner")
		}
		d, err := repositories.GetConversationDelegation(tx.Tx, id)
		if err != nil {
			return err
		}
		if d.Active || d.Revision != req.Revision {
			return errorsx.InvalidParamI18n("error.delegation.changed")
		}
		a := repositories.AIAgentRepository.Get(tx.Tx, req.AIAgentID)
		if a == nil || a.Status != enums.StatusOk {
			return errorsx.InvalidParamI18n("error.copilot.model")
		}
		if !req.PreviewOnly && !delegationLiveAvailable(c, repositories.ChannelRepository.Get(tx.Tx, c.ChannelID), a) {
			return errorsx.InvalidParamI18n("error.delegation.sendDisabled")
		}
		if err := repositories.SetInitialCustomerOwner(tx.Tx, c.CustomerID, owner); err != nil {
			return err
		}
		// Resolve again after a concurrent first ownership assignment on another conversation.
		if delegationOwner(tx.Tx, c) != owner {
			return errorsx.InvalidParamI18n("error.delegation.changed")
		}
		now := time.Now()
		*d = models.ConversationDelegation{ConversationID: id, CustomerID: c.CustomerID, OwnerID: owner, AIAgentID: a.ID,
			Active: true, PreviewOnly: req.PreviewOnly, Revision: d.Revision + 1, Instructions: strings.TrimSpace(req.Instructions),
			StartedBy: operator.UserID, StartedAt: now, LastProcessedMessageID: c.LastMessageID}
		if req.DurationMinutes > 0 {
			expires := now.Add(time.Duration(req.DurationMinutes) * time.Minute)
			d.ExpiresAt = &expires
		}
		if err := repositories.ConversationRepository.Updates(tx.Tx, id, map[string]any{"delegation_managed": true}); err != nil {
			return err
		}
		if err := repositories.SaveConversationDelegation(tx.Tx, d); err != nil {
			return err
		}
		return s.event(tx.Tx, d, "started", operator.UserID, 0, "", "")
	})
	if err == nil {
		s.cancel(id)
		s.publish(id)
	}
	return err
}

func (s *conversationDelegationService) Stop(id, revision int64, operator *dto.AuthPrincipal) error {
	unlock := s.lockDelivery(id)
	defer unlock()
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		c, err := repositories.LockConversationWork(tx.Tx, id)
		if err != nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		d, err := repositories.GetConversationDelegation(tx.Tx, id)
		if err != nil {
			return err
		}
		if operator == nil || (operator.UserID != d.OwnerID && operator.UserID != c.CurrentAssigneeID && !ConversationService.isAdmin(operator)) {
			return errorsx.ForbiddenI18n("error.delegation.notOwner")
		}
		if d.Revision != revision {
			return errorsx.InvalidParamI18n("error.delegation.changed")
		}
		return s.stopTx(tx, c, d, "reclaimed", "", operator.UserID, false)
	})
	if err == nil {
		s.cancel(id)
		s.publish(id)
	}
	return err
}

func (s *conversationDelegationService) stopTx(tx *sqls.TxContext, c *models.Conversation, d *models.ConversationDelegation, kind, reason string, actor int64, notify bool) error {
	if !d.Active {
		return nil
	}
	now := time.Now()
	d.Active, d.EndedAt, d.EndReason = false, &now, kind
	d.Revision++
	if err := repositories.SaveConversationDelegation(tx.Tx, d); err != nil {
		return err
	}
	if err := repositories.CancelDelegationOutbox(tx.Tx, c.ID); err != nil {
		return err
	}
	if err := s.event(tx.Tx, d, kind, actor, c.LastMessageID, reason, ""); err != nil {
		return err
	}
	if notify {
		n := receptionNotification(c, d.OwnerID, "delegation_handoff", "notification.delegation.handoff")
		n.Content += "\n" + reason
		if err := repositories.NotificationRepository.Create(tx.Tx, n); err != nil {
			return err
		}
		if tx.RegisterCallback != nil {
			tx.RegisterCallback(func() { NotificationService.Push(n) })
		}
	}
	if tx.RegisterCallback != nil {
		tx.RegisterCallback(func() { s.cancel(c.ID) })
	}
	return nil
}

// Called in the same transaction as human replies, close, transfer, or customer relinking.
func (s *conversationDelegationService) stopForConversationTx(tx *sqls.TxContext, c *models.Conversation, kind string, actor int64) error {
	if !c.DelegationManaged {
		return nil
	}
	d, err := repositories.GetConversationDelegation(tx.Tx, c.ID)
	if err != nil {
		return err
	}
	return s.stopTx(tx, c, d, kind, "", actor, false)
}

func (s *conversationDelegationService) event(db *gorm.DB, d *models.ConversationDelegation, kind string, actor, source int64, reason, content string) error {
	return repositories.CreateDelegationEvent(db, &models.ConversationDelegationEvent{ConversationID: d.ConversationID, Revision: d.Revision,
		ActorID: actor, AIAgentID: d.AIAgentID, SourceMessageID: source, Kind: kind, Reason: reason, Content: content, PreviewOnly: d.PreviewOnly, CreatedAt: time.Now()})
}

func (s *conversationDelegationService) cancel(id int64) {
	if value, ok := s.running.Load(id); ok {
		value.(context.CancelFunc)()
	}
}
func (s *conversationDelegationService) publish(id int64) {
	WsService.PublishConversationChanged(ConversationService.Get(id), enums.IMRealtimeEventConversationUpdated)
}

func (s *conversationDelegationService) ProcessPending() {
	if !s.tick.TryLock() {
		return
	}
	defer s.tick.Unlock()
	items, err := repositories.ActiveConversationDelegations(sqls.DB())
	if err != nil {
		slog.Warn("delegation scan failed")
		return
	}
	var wg sync.WaitGroup
	limit := make(chan struct{}, 4)
	for _, item := range items {
		limit <- struct{}{}
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			defer func() { <-limit }()
			if err := s.process(context.Background(), id, false, -1); err != nil {
				slog.Warn("delegation run failed", "conversation_id", id)
			}
		}(item.ConversationID)
	}
	wg.Wait()
}

func (s *conversationDelegationService) Preview(ctx context.Context, id, revision int64, operator *dto.AuthPrincipal) error {
	v, err := s.View(id, operator)
	if err != nil {
		return err
	}
	if !v.CanManage || !v.State.Active || !v.State.PreviewOnly {
		return errorsx.InvalidParamI18n("error.delegation.previewOnly")
	}
	return s.process(ctx, id, true, revision)
}

func (s *conversationDelegationService) process(parent context.Context, id int64, force bool, expectedRevision int64) error {
	ctx, cancel := context.WithTimeout(parent, 65*time.Second)
	if _, busy := s.running.LoadOrStore(id, context.CancelFunc(cancel)); busy {
		cancel()
		if force {
			return errorsx.InvalidParamI18n("error.copilot.busy")
		}
		return nil
	}
	defer func() { cancel(); s.running.Delete(id) }()
	c := ConversationService.Get(id)
	if c == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	d, err := repositories.GetConversationDelegation(sqls.DB(), id)
	if err != nil {
		return err
	}
	if expectedRevision >= 0 && d.Revision != expectedRevision {
		return errorsx.InvalidParamI18n("error.delegation.changed")
	}
	if !d.Active {
		return nil
	}
	if force && !d.PreviewOnly {
		return errorsx.InvalidParamI18n("error.delegation.previewOnly")
	}
	kind := ""
	if c.Status != enums.IMConversationStatusActive || c.CurrentAssigneeID != d.OwnerID || c.CustomerID != d.CustomerID || delegationOwner(sqls.DB(), c) != d.OwnerID {
		kind = "conversation_changed"
	}
	if d.ExpiresAt != nil && !d.ExpiresAt.After(time.Now()) {
		kind = "expired"
	}
	if kind != "" {
		return s.finish(id, d.Revision, kind, "", nil)
	}
	if !force && c.LastMessageID <= d.LastProcessedMessageID {
		return nil
	}
	m := MessageService.Get(c.LastMessageID)
	if m == nil || m.SenderType != enums.IMSenderTypeCustomer || m.RecalledAt != nil || m.SendStatus == enums.IMMessageStatusFailed || m.SendStatus == enums.IMMessageStatusRecalled || (m.IsHistorical && !force) {
		if force {
			return errorsx.InvalidParamI18n("error.copilot.noMessage")
		}
		return nil
	}
	if m.MessageType != enums.IMMessageTypeText {
		return s.finish(id, d.Revision, "unsupported_message", "", nil)
	}
	if !d.PreviewOnly && !delegationLiveAvailable(c, ChannelService.Get(c.ChannelID), AIAgentService.Get(d.AIAgentID)) {
		return s.finish(id, d.Revision, "sending_disabled", "", nil)
	}
	r, err := ConversationCopilotService.suggest(ctx, id, d)
	if err != nil {
		if ctx.Err() != nil && parent.Err() == nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil
		}
		current := ConversationService.Get(id)
		if current != nil && current.LastMessageID != c.LastMessageID {
			return nil
		}
		if finishErr := s.finish(id, d.Revision, "generation_failed", "", nil); finishErr != nil {
			return finishErr
		}
		return err
	}
	if r.HandoffRequested {
		return s.finish(id, d.Revision, "handoff", r.HandoffReason, r)
	}
	if d.PreviewOnly {
		return s.finish(id, d.Revision, "preview", "", r)
	}
	_, err = MessageService.sendDelegatedReply(c, d, r)
	if errors.Is(err, errDelegationStale) {
		return nil
	}
	if err != nil {
		_ = s.finish(id, d.Revision, "sending_failed", "", nil)
	}
	return err
}

func (s *conversationDelegationService) finish(id, revision int64, kind, reason string, r *response.ConversationReplySuggestion) error {
	unlock := s.lockDelivery(id)
	defer unlock()
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		c, err := repositories.LockConversationWork(tx.Tx, id)
		if err != nil {
			return err
		}
		d, err := repositories.GetConversationDelegation(tx.Tx, id)
		if err != nil {
			return err
		}
		if !d.Active || d.Revision != revision {
			return nil
		}
		if r != nil && (c.LastMessageID != r.LastMessageID || c.CustomerID != d.CustomerID || c.CurrentAssigneeID != d.OwnerID || delegationOwner(tx.Tx, c) != d.OwnerID || c.Status != enums.IMConversationStatusActive) {
			return nil
		}
		if kind == "preview" {
			if d.ExpiresAt != nil && !d.ExpiresAt.After(time.Now()) {
				return s.stopTx(tx, c, d, "expired", "", 0, true)
			}
			d.LastProcessedMessageID = r.LastMessageID
			if err := repositories.SaveConversationDelegation(tx.Tx, d); err != nil {
				return err
			}
			return s.event(tx.Tx, d, kind, 0, r.LastMessageID, "", r.Content)
		}
		return s.stopTx(tx, c, d, kind, reason, 0, true)
	})
	if err == nil {
		s.publish(id)
	}
	return err
}

func (s *conversationDelegationService) validateDelivery(db *gorm.DB, c *models.Conversation, revision int64, agentID, sourceID int64) error {
	d, err := repositories.GetConversationDelegation(db, c.ID)
	if err != nil {
		return err
	}
	if !d.Active || d.PreviewOnly || d.Revision != revision || d.AIAgentID != agentID || c.Status != enums.IMConversationStatusActive ||
		c.CurrentAssigneeID != d.OwnerID || c.CustomerID != d.CustomerID || delegationOwner(db, c) != d.OwnerID || (sourceID > 0 && c.LastCustomerMessageID != sourceID) ||
		(d.ExpiresAt != nil && !d.ExpiresAt.After(time.Now())) || !delegationLiveAvailable(c, repositories.ChannelRepository.Get(db, c.ChannelID), repositories.AIAgentRepository.Get(db, d.AIAgentID)) {
		return errDelegationStale
	}
	return nil
}

func delegatedClientID(d *models.ConversationDelegation, source int64) string {
	return fmt.Sprintf("delegation_%d_%d", d.Revision, source)
}

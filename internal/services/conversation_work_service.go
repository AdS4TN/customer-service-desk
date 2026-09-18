package services

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/i18nx"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

var ConversationWorkService = &conversationWorkService{}

type conversationWorkService struct{ tick sync.Mutex }

func replyTarget(item *models.Conversation) int {
	if item.ReplyTargetMinutes > 0 {
		return item.ReplyTargetMinutes
	}
	return 15
}

// Runs in the message transaction, or after the channel confirms the outbox send.
func (s *conversationWorkService) onMessage(ctx *sqls.TxContext, message *models.Message) error {
	if message.IsHistorical || message.RecalledAt != nil || message.SendStatus == enums.IMMessageStatusRecalled {
		return nil
	}
	if message.SenderType != enums.IMSenderTypeCustomer && message.SenderType != enums.IMSenderTypeAgent && message.SenderType != enums.IMSenderTypeAI {
		return nil
	}
	item, err := repositories.LockConversationWork(ctx.Tx, message.ConversationID)
	if err != nil {
		return err
	}
	if item.Status == enums.IMConversationStatusClosed {
		return nil
	}
	if message.SenderType == enums.IMSenderTypeAgent {
		kind := "human_reply"
		if message.SenderID == 0 {
			kind = "phone_reply"
		}
		if err := ConversationDelegationService.stopForConversationTx(ctx, item, kind, message.SenderID); err != nil {
			return err
		}
	}
	now := time.Now()
	updates := map[string]any{"snoozed_until": nil}
	if message.SenderType == enums.IMSenderTypeCustomer {
		if message.ID <= item.LastCustomerMessageID {
			return nil
		}
		updates["last_customer_message_id"] = message.ID
		updates["work_status"] = enums.ConversationWorkStatusNeedsReply
		if item.WorkStatus != enums.ConversationWorkStatusNeedsReply || item.PendingSince == nil {
			updates["pending_since"] = now
			updates["reply_due_at"] = now.Add(time.Duration(replyTarget(item)) * time.Minute)
			updates["reminder_revision"] = 0
		} else if item.ReminderRevision == item.WorkRevision {
			updates["reminder_revision"] = item.WorkRevision + 1
		}
	} else {
		if message.SenderType == enums.IMSenderTypeAI && item.Status != enums.IMConversationStatusAIServing && message.DelegationRevision == 0 {
			return nil
		}
		if message.SendStatus != enums.IMMessageStatusSent && message.SendStatus != enums.IMMessageStatusDelivered && message.SendStatus != enums.IMMessageStatusRead {
			return nil
		}
		// A delayed reply cannot clear a newer customer request or a manual snooze.
		if item.LastCustomerMessageID > message.ReplyToCustomerMessageID || item.WorkStatus == enums.ConversationWorkStatusSnoozed {
			return nil
		}
		updates["work_status"] = enums.ConversationWorkStatusWaitingCustomer
		updates["pending_since"], updates["reply_due_at"] = nil, nil
	}
	return repositories.UpdateConversationWork(ctx.Tx, item, updates)
}

func (s *conversationWorkService) Update(id int64, req request.UpdateConversationWork, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if req.ReplyTargetMinutes < 1 || req.ReplyTargetMinutes > 10080 ||
		(req.Status != enums.ConversationWorkStatusNeedsReply && req.Status != enums.ConversationWorkStatusWaitingCustomer && req.Status != enums.ConversationWorkStatusSnoozed) ||
		(req.Status == enums.ConversationWorkStatusSnoozed && (req.SnoozeMinutes < 1 || req.SnoozeMinutes > 43200)) {
		return errorsx.InvalidParamI18n("error.reception.invalidState")
	}
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		item, err := repositories.LockConversationWork(ctx.Tx, id)
		if err != nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if item.WorkRevision != req.Revision {
			return errorsx.InvalidParamI18n("error.reception.stateChanged")
		}
		if item.Status != enums.IMConversationStatusActive || item.CurrentAssigneeID != operator.UserID {
			return errorsx.ForbiddenI18n("error.reception.notAssignee")
		}
		now := time.Now()
		updates := map[string]any{"work_status": req.Status, "reply_target_minutes": req.ReplyTargetMinutes, "snoozed_until": nil, "pending_since": nil, "reply_due_at": nil, "reminder_revision": 0}
		if req.Status == enums.ConversationWorkStatusNeedsReply {
			since := now
			if item.WorkStatus == req.Status && item.PendingSince != nil {
				since = *item.PendingSince
			}
			updates["pending_since"] = since
			updates["reply_due_at"] = since.Add(time.Duration(req.ReplyTargetMinutes) * time.Minute)
			if item.WorkStatus == req.Status && item.ReplyTargetMinutes == req.ReplyTargetMinutes && item.ReminderRevision == item.WorkRevision {
				updates["reminder_revision"] = item.WorkRevision + 1
			}
		}
		if req.Status == enums.ConversationWorkStatusSnoozed {
			updates["snoozed_until"] = now.Add(time.Duration(req.SnoozeMinutes) * time.Minute)
		}
		return repositories.UpdateConversationWork(ctx.Tx, item, updates)
	})
	if err == nil {
		WsService.PublishConversationChanged(ConversationService.Get(id), enums.IMRealtimeEventConversationUpdated)
	}
	return err
}

func (s *conversationWorkService) ProcessDue() {
	if !s.tick.TryLock() {
		return
	}
	defer s.tick.Unlock()
	items, err := repositories.DueConversationWork(sqls.DB(), time.Now())
	if err != nil {
		slog.Warn("reception due scan failed")
		return
	}
	for _, item := range items {
		if err := s.processDue(item.ID, time.Now()); err != nil {
			slog.Warn("reception reminder failed", "conversation_id", item.ID)
		}
	}
}

func (s *conversationWorkService) processDue(id int64, now time.Time) error {
	var notice *models.Notification
	changed := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		item, err := repositories.LockConversationWork(ctx.Tx, id)
		if err != nil {
			return err
		}
		if item.Status == enums.IMConversationStatusClosed {
			return nil
		}
		if item.WorkStatus == enums.ConversationWorkStatusSnoozed && item.SnoozedUntil != nil && !item.SnoozedUntil.After(now) {
			changed = true
			return repositories.UpdateConversationWork(ctx.Tx, item, map[string]any{"work_status": enums.ConversationWorkStatusNeedsReply, "snoozed_until": nil, "pending_since": now, "reply_due_at": now.Add(time.Duration(replyTarget(item)) * time.Minute), "reminder_revision": 0})
		}
		if item.WorkStatus != enums.ConversationWorkStatusNeedsReply || item.ReplyDueAt == nil || item.ReplyDueAt.After(now) || item.CurrentAssigneeID <= 0 {
			return nil
		}
		claimed, err := repositories.ClaimConversationReminder(ctx.Tx, item)
		if err != nil || !claimed {
			return err
		}
		notice = receptionNotification(item, item.CurrentAssigneeID, "reply_overdue", "notification.reception.overdue")
		return repositories.NotificationRepository.Create(ctx.Tx, notice)
	})
	if err != nil {
		return err
	}
	if notice != nil {
		NotificationService.Push(notice)
	}
	if changed {
		WsService.PublishConversationChanged(ConversationService.Get(id), enums.IMRealtimeEventConversationUpdated)
	}
	return nil
}

func receptionNotification(item *models.Conversation, userID int64, kind, titleKey string) *models.Notification {
	return &models.Notification{RecipientUserID: userID, Title: i18nx.Get(titleKey), Content: fmt.Sprintf("#%d · %s", item.ID, item.CustomerName), NotificationType: kind, BizType: "conversation", BizID: item.ID, ActionURL: fmt.Sprintf("/dashboard/conversations?conversationId=%d", item.ID), Status: enums.StatusOk, CreatedAt: time.Now()}
}

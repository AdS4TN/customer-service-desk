package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/i18nx"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

var leadDueTick sync.Mutex

func sameFollowUpTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func leadClosed(status string) bool {
	return status == string(enums.LeadStatusWon) || status == string(enums.LeadStatusLost) || status == string(enums.LeadStatusArchived)
}

func validateLeadFollowUp(req *request.FollowUpSalesLead, now time.Time) error {
	req.Result, req.NextAction = strings.TrimSpace(req.Result), strings.TrimSpace(req.NextAction)
	if req.Revision < 1 || !validLeadStatus(req.Status) || req.OwnerID <= 0 || req.Result == "" || utf8.RuneCountInString(req.Result) > 2000 || utf8.RuneCountInString(req.NextAction) > 500 {
		return errorsx.InvalidParamI18n("error.lead.followupInvalid")
	}
	if leadClosed(string(req.Status)) {
		req.NextAction, req.FollowUpAt = "", nil
	} else if req.NextAction == "" || req.FollowUpAt == nil || !req.FollowUpAt.After(now) || req.FollowUpAt.Year() > 9999 {
		return errorsx.InvalidParamI18n("error.lead.nextRequired")
	}
	// SQLite compares stored timestamps lexically. Match GORM's local-time
	// schedule queries while preserving the browser's original instant.
	if req.FollowUpAt != nil {
		at := req.FollowUpAt.In(time.Local)
		req.FollowUpAt = &at
	}
	return nil
}

// Sales handling does not confirm AI-extracted facts, and can still close a lead
// whose source was recalled. It never sends a message to the customer.
func (s *salesLeadService) FollowUp(id, userID int64, req request.FollowUpSalesLead) error {
	now := time.Now()
	if userID <= 0 {
		return leadError()
	}
	if err := validateLeadFollowUp(&req, now); err != nil {
		return err
	}
	var notice *models.Notification
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		r := repositories.SalesLeadRepository
		row, err := r.Get(tx.Tx, id)
		if err != nil || row == nil {
			return leadError()
		}
		if row.Revision != req.Revision {
			return errorsx.InvalidParamI18n("error.lead.conflict")
		}
		owner := repositories.UserRepository.Get(tx.Tx, req.OwnerID)
		if owner == nil || owner.Status != enums.StatusOk {
			return errorsx.InvalidParamI18n("error.lead.followupInvalid")
		}
		previousOwner := row.OwnerID
		row.Status, row.OwnerID = string(req.Status), req.OwnerID
		row.NextAction, row.FollowUpAt, row.LastFollowUpAt = req.NextAction, req.FollowUpAt, &now
		row.ScheduleRevision++
		row.Revision++
		row.UpdatedBy, row.UpdatedAt = userID, now
		ok, err := r.Update(tx.Tx, row, req.Revision)
		if err != nil {
			return leadError()
		}
		if !ok {
			return errorsx.InvalidParamI18n("error.lead.conflict")
		}
		data, _ := json.Marshal(req)
		if err := r.Event(tx.Tx, &models.SalesLeadEvent{LeadID: id, Kind: "follow_up", Data: string(data), ActorID: userID, CreatedAt: now}); err != nil {
			return leadError()
		}
		if previousOwner != row.OwnerID && !leadClosed(row.Status) {
			notice = leadNotification(row, "lead_assigned")
			if err := repositories.NotificationRepository.Create(tx.Tx, notice); err != nil {
				return leadError()
			}
		}
		return repositories.ConversationMemoryRepository.Queue(tx.Tx, row.ConversationID)
	})
	if err == nil && notice != nil {
		NotificationService.Push(notice)
	}
	return err
}

func leadNotification(row *models.SalesLead, kind string) *models.Notification {
	return &models.Notification{RecipientUserID: row.OwnerID, Title: i18nx.Get("notification." + kind), Content: fmt.Sprintf("#%d", row.ID), NotificationType: kind, BizType: "sales_lead", BizID: row.ID, ActionURL: fmt.Sprintf("/dashboard/sales-leads?leadId=%d", row.ID), Status: enums.StatusOk, CreatedAt: time.Now()}
}

func (s *salesLeadService) ProcessDue() {
	if !leadDueTick.TryLock() {
		return
	}
	defer leadDueTick.Unlock()
	rows, err := repositories.SalesLeadRepository.Due(sqls.DB(), time.Now())
	if err != nil {
		slog.Warn("sales lead due scan failed")
		return
	}
	for _, row := range rows {
		if err := s.processLeadDue(row.ID, time.Now()); err != nil {
			slog.Warn("sales lead reminder failed", "lead_id", row.ID)
		}
	}
}

func (s *salesLeadService) processLeadDue(id int64, now time.Time) error {
	var notice *models.Notification
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		r := repositories.SalesLeadRepository
		row, err := r.Get(tx.Tx, id)
		if err != nil {
			return err
		}
		if row == nil || leadClosed(row.Status) || row.OwnerID <= 0 || row.FollowUpAt == nil || row.FollowUpAt.After(now) {
			return nil
		}
		ok, err := r.ClaimReminder(tx.Tx, row)
		if err != nil || !ok {
			return err
		}
		notice = leadNotification(row, "lead_overdue")
		return repositories.NotificationRepository.Create(tx.Tx, notice)
	})
	if err == nil && notice != nil {
		NotificationService.Push(notice)
	}
	return err
}

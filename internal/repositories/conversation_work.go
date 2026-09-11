package repositories

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func LockConversationWork(db *gorm.DB, id int64) (*models.Conversation, error) {
	var item models.Conversation
	// SQLite must acquire its writer lock before reading, not upgrade a read snapshot.
	if db.Dialector.Name() == "sqlite" {
		if err := db.Model(&models.Conversation{}).Where("id = ?", id).UpdateColumn("work_revision", gorm.Expr("work_revision")).Error; err != nil {
			return nil, err
		}
	}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error
	return &item, err
}

func UpdateConversationWork(db *gorm.DB, item *models.Conversation, updates map[string]any) error {
	updates["work_revision"] = item.WorkRevision + 1
	result := db.Model(&models.Conversation{}).Where("id = ? AND work_revision = ? AND status = ?", item.ID, item.WorkRevision, item.Status).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DueConversationWork(db *gorm.DB, now time.Time) ([]models.Conversation, error) {
	var items []models.Conversation
	err := db.Where("status <> ?", enums.IMConversationStatusClosed).
		Where("(work_status = ? AND snoozed_until <= ?) OR (work_status = ? AND reply_due_at <= ? AND current_assignee_id > 0 AND reminder_revision <> work_revision)", enums.ConversationWorkStatusSnoozed, now, enums.ConversationWorkStatusNeedsReply, now).
		Order("id ASC").Limit(100).Find(&items).Error
	return items, err
}

func ClaimConversationReminder(db *gorm.DB, item *models.Conversation) (bool, error) {
	r := db.Model(&models.Conversation{}).Where("id = ? AND work_revision = ? AND reminder_revision <> ? AND current_assignee_id = ? AND status <> ?", item.ID, item.WorkRevision, item.WorkRevision, item.CurrentAssigneeID, enums.IMConversationStatusClosed).Update("reminder_revision", item.WorkRevision)
	return r.RowsAffected == 1, r.Error
}

func ListConversationNotes(db *gorm.DB, id, before int64) ([]models.ConversationNote, error) {
	var items []models.ConversationNote
	q := db.Where("conversation_id = ?", id)
	if before > 0 {
		q = q.Where("id < ?", before)
	}
	err := q.Order("id DESC").Limit(50).Find(&items).Error
	return items, err
}

func CreateConversationNote(db *gorm.DB, item *models.ConversationNote) (bool, error) {
	r := db.Clauses(clause.OnConflict{DoNothing: true}).Create(item)
	if r.Error != nil {
		return false, r.Error
	}
	if r.RowsAffected == 0 {
		return false, db.Where("conversation_id = ? AND author_id = ? AND client_id = ?", item.ConversationID, item.AuthorID, item.ClientID).First(item).Error
	}
	return true, nil
}

func ConversationHandoffHistory(db *gorm.DB, id int64) ([]models.ConversationAssignment, error) {
	var items []models.ConversationAssignment
	err := db.Where("conversation_id = ?", id).Order("id DESC").Limit(50).Find(&items).Error
	return items, err
}

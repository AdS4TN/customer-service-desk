package repositories

import (
	"errors"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ConversationMemoryRepository = &conversationMemoryRepository{}

type conversationMemoryRepository struct{}

func (r *conversationMemoryRepository) Rebuild(db *gorm.DB, id int64) error {
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "conversation_id"}}, DoUpdates: clause.Assignments(map[string]any{
		"revision": gorm.Expr("revision + 1"), "status": "queued", "error_code": "", "processed_message_id": 0, "policy_hash": "", "updated_at": time.Now(), "attempt_count": 0, "next_retry_at": nil,
	})}).Create(&models.ConversationMemory{ConversationID: id, Revision: 1, Status: "queued", UpdatedAt: time.Now()}).Error
}

func (r *conversationMemoryRepository) SetPolicyHash(db *gorm.DB, id int64, hash string) error {
	return db.Model(&models.ConversationMemory{}).Where("conversation_id = ?", id).Update("policy_hash", hash).Error
}

func (r *conversationMemoryRepository) Get(db *gorm.DB, id int64) (*models.ConversationMemory, error) {
	var item models.ConversationMemory
	err := db.First(&item, "conversation_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *conversationMemoryRepository) Queue(db *gorm.DB, id int64) error {
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "conversation_id"}}, DoUpdates: clause.Assignments(map[string]any{
		"revision": gorm.Expr("revision + 1"), "status": "queued", "error_code": "", "updated_at": time.Now(), "attempt_count": 0, "next_retry_at": nil,
	})}).Create(&models.ConversationMemory{ConversationID: id, Revision: 1, Status: "queued", UpdatedAt: time.Now()}).Error
}

func (r *conversationMemoryRepository) Next(db *gorm.DB) (*models.ConversationMemory, error) {
	var item models.ConversationMemory
	err := db.Where("(status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND started_at < ?)", "queued", time.Now(), "processing", time.Now().Add(-3*time.Minute)).Order("updated_at ASC").First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *conversationMemoryRepository) Claim(db *gorm.DB, item *models.ConversationMemory) (bool, error) {
	now := time.Now()
	res := db.Model(&models.ConversationMemory{}).Where("conversation_id = ? AND revision = ? AND ((status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND started_at < ?))", item.ConversationID, item.Revision, "queued", now, "processing", now.Add(-3*time.Minute)).Updates(map[string]any{"status": "processing", "started_at": now, "revision": gorm.Expr("revision + 1"), "attempt_count": gorm.Expr("attempt_count + 1"), "next_retry_at": nil})
	if res.RowsAffected > 0 {
		item.Revision++
		item.Status = "processing"
		item.AttemptCount++
		item.NextRetryAt = nil
	}
	return res.RowsAffected > 0, res.Error
}

func (r *conversationMemoryRepository) Finish(db *gorm.DB, item *models.ConversationMemory, cursor int64, status, code string) (bool, error) {
	updates := map[string]any{"status": status, "error_code": code, "processed_message_id": cursor, "updated_at": time.Now(), "next_retry_at": nil}
	// A successful batch starts a fresh retry budget for the next page of messages.
	if status == "queued" && code == "" {
		updates["attempt_count"] = 0
	}
	res := db.Model(&models.ConversationMemory{}).Where("conversation_id = ? AND revision = ?", item.ConversationID, item.Revision).Updates(updates)
	return res.RowsAffected > 0, res.Error
}

func (r *conversationMemoryRepository) Retry(db *gorm.DB, item *models.ConversationMemory, code string, at time.Time) (bool, error) {
	res := db.Model(&models.ConversationMemory{}).Where("conversation_id = ? AND revision = ? AND status = ?", item.ConversationID, item.Revision, "processing").Updates(map[string]any{
		"status": "queued", "error_code": code, "next_retry_at": at, "updated_at": time.Now(),
	})
	return res.RowsAffected > 0, res.Error
}

func (r *conversationMemoryRepository) Entries(db *gorm.DB, id int64) ([]models.ConversationMemoryEntry, error) {
	var items []models.ConversationMemoryEntry
	err := db.Where("conversation_id = ?", id).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *conversationMemoryRepository) Shared(db *gorm.DB, c models.Conversation) ([]models.ConversationMemoryEntry, error) {
	if c.CustomerID <= 0 || c.AIAgentID <= 0 {
		return nil, nil
	}
	var conversations []models.Conversation
	if err := db.Select("id").Where("customer_id = ? AND ai_agent_id = ? AND id <> ?", c.CustomerID, c.AIAgentID, c.ID).Find(&conversations).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(conversations))
	for _, v := range conversations {
		ids = append(ids, v.ID)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var items []models.ConversationMemoryEntry
	err := db.Where("conversation_id IN ? AND kind = ? AND confirmed = ? AND deleted = ?", ids, "customer", true, false).Order("updated_at DESC").Limit(30).Find(&items).Error
	return items, err
}

func (r *conversationMemoryRepository) Messages(db *gorm.DB, id, after int64) ([]models.Message, error) {
	var items []models.Message
	err := db.Where("conversation_id = ? AND id > ? AND recalled_at IS NULL AND sender_type IN ? AND message_type IN ?", id, after, []enums.IMSenderType{enums.IMSenderTypeCustomer, enums.IMSenderTypeAgent, enums.IMSenderTypeAI}, []enums.IMMessageType{enums.IMMessageTypeText, enums.IMMessageTypeHTML}).Order("id ASC").Limit(40).Find(&items).Error
	return items, err
}

func (r *conversationMemoryRepository) Sources(db *gorm.DB, id int64, ids []int64) ([]models.Message, error) {
	return r.SourcesForConversations(db, []int64{id}, ids)
}

func (r *conversationMemoryRepository) SourcesForConversations(db *gorm.DB, conversations, ids []int64) ([]models.Message, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []models.Message
	err := db.Where("conversation_id IN ? AND id IN ? AND recalled_at IS NULL", conversations, ids).Find(&items).Error
	return items, err
}

func (r *conversationMemoryRepository) SaveEntry(db *gorm.DB, item *models.ConversationMemoryEntry) error {
	return db.Save(item).Error
}

func (r *conversationMemoryRepository) UpdateEntry(db *gorm.DB, item *models.ConversationMemoryEntry, previousRevision int64) (bool, error) {
	res := db.Model(&models.ConversationMemoryEntry{}).Where("id = ? AND conversation_id = ? AND revision = ? AND deleted = ?", item.ID, item.ConversationID, previousRevision, false).Updates(map[string]any{
		"value": item.Value, "confirmed": item.Confirmed, "deleted": item.Deleted, "source_message_ids": item.SourceMessageIDs,
		"revision": item.Revision, "updated_by": item.UpdatedBy, "updated_at": item.UpdatedAt,
	})
	return res.RowsAffected > 0, res.Error
}

func (r *conversationMemoryRepository) RemoveGenerated(db *gorm.DB, id int64, keys []string) error {
	q := db.Where("conversation_id = ? AND confirmed = ? AND deleted = ?", id, false, false)
	if len(keys) > 0 {
		q = q.Where("entry_key NOT IN ?", keys)
	}
	return q.Delete(&models.ConversationMemoryEntry{}).Error
}

package repositories

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"gorm.io/gorm"
	"time"
)

func RecoverLinkedOutbox(db *gorm.DB, channelType string) error {
	return db.Model(&models.ChannelMessageOutbox{}).Where("channel_type = ? AND send_status = ?", channelType, "sending").
		Updates(map[string]any{"send_status": "pending", "next_retry_at": nil}).Error
}

func ListLinkedDueOutbox(db *gorm.DB, channelType string, limit int) []models.ChannelMessageOutbox {
	var items []models.ChannelMessageOutbox
	db.Where("channel_type = ? AND send_status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", channelType, []string{"pending", "failed"}, time.Now()).Order("id ASC").Limit(limit).Find(&items)
	return items
}

func ClaimLinkedOutbox(db *gorm.DB, channelType string, id int64) (bool, error) {
	result := db.Model(&models.ChannelMessageOutbox{}).Where("id = ? AND channel_type = ? AND send_status IN ?", id, channelType, []string{"pending", "failed"}).
		Updates(map[string]any{"send_status": "sending", "updated_at": time.Now()})
	return result.RowsAffected == 1, result.Error
}

func RetryLinkedOutbox(db *gorm.DB, channelType string, messageID, conversationID, userID int64) (bool, error) {
	// Recheck ownership at the write boundary; retry must never revive a stale AI reply.
	eligible := db.Model(&models.Conversation{}).Select("id").Where("id = ? AND status = ? AND current_assignee_id = ?", conversationID, enums.IMConversationStatusActive, userID)
	result := db.Model(&models.ChannelMessageOutbox{}).
		Where("channel_type = ? AND message_id = ? AND send_status = ? AND last_error = ?", channelType, messageID, "ignored", "send_failed").
		Where("conversation_id IN (?)", eligible).
		Updates(map[string]any{"send_status": "pending", "retry_count": 0, "next_retry_at": nil, "last_error": "", "updated_at": time.Now()})
	return result.RowsAffected == 1, result.Error
}

func RecoverWhatsAppOutbox(db *gorm.DB) error {
	return RecoverLinkedOutbox(db, enums.ChannelTypeWhatsApp)
}
func ListWhatsAppDueOutbox(db *gorm.DB, limit int) []models.ChannelMessageOutbox {
	return ListLinkedDueOutbox(db, enums.ChannelTypeWhatsApp, limit)
}
func ClaimWhatsAppOutbox(db *gorm.DB, id int64) (bool, error) {
	return ClaimLinkedOutbox(db, enums.ChannelTypeWhatsApp, id)
}
func RetryWhatsAppOutbox(db *gorm.DB, messageID, conversationID, userID int64) (bool, error) {
	return RetryLinkedOutbox(db, enums.ChannelTypeWhatsApp, messageID, conversationID, userID)
}

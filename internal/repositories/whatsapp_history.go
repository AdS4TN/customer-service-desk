package repositories

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"gorm.io/gorm"
)

type WhatsAppHistoryTarget struct {
	ConversationID int64
	ExternalID     string
}

func FindWhatsAppHistoryTargets(db *gorm.DB, channelID int64, accountPrefix string) ([]WhatsAppHistoryTarget, error) {
	var targets []WhatsAppHistoryTarget
	err := db.Model(&models.Conversation{}).
		Select("MAX(t_conversation.id) AS conversation_id, i.external_id").
		Joins("JOIN t_customer_identity i ON i.customer_id = t_conversation.customer_id").
		Where("t_conversation.channel_id = ? AND i.external_source = ? AND i.external_id LIKE ?", channelID, enums.ExternalSourceWhatsApp, accountPrefix+"%").
		Group("i.external_id").Order("conversation_id DESC").Scan(&targets).Error
	return targets, err
}

func FindLinkedEchoCandidates(db *gorm.DB, channelType string, channelID int64, payload string) []models.ChannelMessageOutbox {
	var items []models.ChannelMessageOutbox
	db.Where("channel_type = ? AND (payload = ? OR payload = '' OR payload IS NULL)", channelType, payload).
		Where("conversation_id IN (?)", db.Model(&models.Conversation{}).Select("id").Where("channel_id = ?", channelID)).Find(&items)
	return items
}

func UpdateWhatsAppImportedSummary(db *gorm.DB, conversationID int64, message *models.Message, summary string) error {
	return db.Model(&models.Conversation{}).Where("id = ? AND (last_message_id = 0 OR last_message_at <= ?)", conversationID, message.SentAt).
		Updates(map[string]any{"last_message_id": message.ID, "last_message_at": message.SentAt,
			"last_active_at": message.SentAt, "last_message_summary": summary}).Error
}

// An ID still identifies the cursor, but ordering follows the original send time.
func FindWhatsAppMessagesBefore(db *gorm.DB, conversationID, cursor int64, limit int, senderType, messageType string) []models.Message {
	query := db.Where("conversation_id = ?", conversationID)
	if cursor > 0 {
		anchor := MessageRepository.Get(db, cursor)
		if anchor == nil || anchor.ConversationID != conversationID {
			return nil
		}
		timestamp := anchor.CreatedAt
		if anchor.SentAt != nil {
			timestamp = *anchor.SentAt
		}
		query = query.Where("COALESCE(sent_at, created_at) < ? OR (COALESCE(sent_at, created_at) = ? AND id < ?)", timestamp, timestamp, cursor)
	}
	if senderType != "" {
		query = query.Where("sender_type = ?", senderType)
	}
	if messageType != "" {
		query = query.Where("message_type = ?", messageType)
	}
	var messages []models.Message
	query.Order("COALESCE(sent_at, created_at) DESC").Order("id DESC").Limit(limit).Find(&messages)
	return messages
}

func FindWhatsAppEchoCandidates(db *gorm.DB, channelID int64, payload string) []models.ChannelMessageOutbox {
	return FindLinkedEchoCandidates(db, enums.ChannelTypeWhatsApp, channelID, payload)
}

func FindPendingLinkedMedia(db *gorm.DB, channelType string, cursor int64) ([]models.Message, error) {
	var messages []models.Message
	err := db.Where("conversation_id IN (?)", db.Model(&models.Conversation{}).Select("id").Where("channel_id IN (?)", db.Model(&models.Channel{}).Select("id").Where("channel_type = ?", channelType))).
		Where("id > ? AND payload LIKE ?", cursor, `%"state":"pending"%`).Order("id ASC").Limit(100).Find(&messages).Error
	return messages, err
}

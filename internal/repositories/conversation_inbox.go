package repositories

import (
	"agent-desk/internal/models"
	"gorm.io/gorm"
)

func AssignInboxConversation(db *gorm.DB, previous *models.Conversation, updates map[string]any) error {
	result := db.Model(&models.Conversation{}).Where("id = ? AND status = ? AND current_assignee_id = ?", previous.ID, previous.Status, previous.CurrentAssigneeID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

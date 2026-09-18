package repositories

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"errors"
	"time"

	"gorm.io/gorm"
)

func GetConversationDelegation(db *gorm.DB, id int64) (*models.ConversationDelegation, error) {
	var item models.ConversationDelegation
	err := db.First(&item, "conversation_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.ConversationDelegation{ConversationID: id, PreviewOnly: true}, nil
	}
	return &item, err
}

func SaveConversationDelegation(db *gorm.DB, item *models.ConversationDelegation) error {
	return db.Save(item).Error
}
func CreateDelegationEvent(db *gorm.DB, item *models.ConversationDelegationEvent) error {
	return db.Create(item).Error
}
func DelegationEvents(db *gorm.DB, id int64) ([]models.ConversationDelegationEvent, error) {
	items := []models.ConversationDelegationEvent{}
	err := db.Where("conversation_id = ?", id).Order("id DESC").Limit(50).Find(&items).Error
	return items, err
}
func ActiveConversationDelegations(db *gorm.DB) ([]models.ConversationDelegation, error) {
	items := []models.ConversationDelegation{}
	err := db.Where("active = ?", true).Order("conversation_id ASC").Find(&items).Error
	return items, err
}
func SetInitialCustomerOwner(db *gorm.DB, customerID, ownerID int64) error {
	if customerID <= 0 || ownerID <= 0 {
		return nil
	}
	return db.Model(&models.Customer{}).Where("id = ? AND owner_user_id = 0", customerID).Update("owner_user_id", ownerID).Error
}

func CancelDelegationOutbox(db *gorm.DB, id int64) error {
	messages := db.Model(&models.Message{}).Select("id").Where("conversation_id = ? AND delegation_revision > 0", id)
	var messageIDs []int64
	jobs := db.Model(&models.ChannelMessageOutbox{}).Where("message_id IN (?) AND send_status IN ?", messages, []string{"pending", "failed"})
	if err := jobs.Pluck("message_id", &messageIDs).Error; err != nil {
		return err
	}
	if len(messageIDs) == 0 {
		return nil
	}
	if err := db.Model(&models.ChannelMessageOutbox{}).Where("message_id IN ? AND send_status IN ?", messageIDs, []string{"pending", "failed"}).Updates(map[string]any{"send_status": "ignored", "last_error": "delegation_cancelled", "updated_at": time.Now()}).Error; err != nil {
		return err
	}
	return db.Model(&models.Message{}).Where("id IN ? AND send_status = ?", messageIDs, enums.IMMessageStatusSending).Update("send_status", enums.IMMessageStatusFailed).Error
}

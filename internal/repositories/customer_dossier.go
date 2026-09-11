package repositories

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Identity comes only from an existing customer association, never contact text.
func (r *conversationMemoryRepository) CustomerDossier(db *gorm.DB, c models.Conversation) ([]models.ConversationMemoryEntry, error) {
	if c.CustomerID <= 0 || c.AIAgentID <= 0 {
		return nil, nil
	}
	var rows []models.ConversationMemoryEntry
	err := db.Table("t_conversation_memory_entry AS e").Select("e.*").
		Joins("JOIN t_conversation AS c ON c.id = e.conversation_id").
		Joins("JOIN t_customer AS customer ON customer.id = c.customer_id AND customer.status <> ?", enums.StatusDeleted).
		Where("c.customer_id = ? AND c.ai_agent_id = ? AND e.kind IN ?", c.CustomerID, c.AIAgentID, []string{string(enums.MemoryKindCustomerProfile), string(enums.MemoryKindCustomerTag)}).
		Find(&rows).Error
	return rows, err
}

func (r *customerRepository) LockForAIProfile(db *gorm.DB, id int64) (*models.Customer, error) {
	var row models.Customer
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (r *customerContactRepository) AllForAIProfile(db *gorm.DB, id int64) ([]models.CustomerContact, error) {
	var rows []models.CustomerContact
	err := db.Where("customer_id = ?", id).Order("id ASC").Find(&rows).Error
	return rows, err
}

package repositories

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var AutomationRepository = &automationRepository{}

type automationRepository struct{}

func (r *automationRepository) Rules(db *gorm.DB, enabledOnly bool) ([]models.AutomationRule, error) {
	rows := []models.AutomationRule{}
	q := db.Where("deleted = ?", false)
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	err := q.Order("priority ASC, id ASC").Find(&rows).Error
	return rows, err
}
func (r *automationRepository) Get(db *gorm.DB, id int64) (*models.AutomationRule, error) {
	var row models.AutomationRule
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND deleted = ?", id, false).Error
	return &row, err
}
func (r *automationRepository) Save(db *gorm.DB, row *models.AutomationRule) error {
	return db.Save(row).Error
}
func (r *automationRepository) Head(db *gorm.DB, trigger string) (int64, error) {
	var id int64
	q := db.Model(&models.Message{})
	if trigger == "lead_created" {
		q = db.Model(&models.SalesLead{})
	}
	err := q.Select("COALESCE(MAX(id), 0)").Scan(&id).Error
	return id, err
}
func (r *automationRepository) Events(db *gorm.DB, trigger string, cursor int64) ([]int64, error) {
	ids := []int64{}
	q := db.Model(&models.Message{}).Where("sender_type = ? AND is_historical = ?", enums.IMSenderTypeCustomer, false)
	if trigger == "lead_created" {
		q = db.Model(&models.SalesLead{})
	}
	err := q.Where("id > ?", cursor).Order("id ASC").Limit(50).Pluck("id", &ids).Error
	return ids, err
}
func (r *automationRepository) Run(db *gorm.DB, id int64) (*models.AutomationRun, error) {
	var row models.AutomationRun
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error
	return &row, err
}
func (r *automationRepository) Receipt(db *gorm.DB, ruleID int64, key string) (*models.AutomationRun, error) {
	var row models.AutomationRun
	err := db.Where("rule_id = ? AND scope_key = ?", ruleID, key).First(&row).Error
	return &row, err
}
func (r *automationRepository) SaveRun(db *gorm.DB, row *models.AutomationRun) error {
	return db.Save(row).Error
}
func (r *automationRepository) Runs(db *gorm.DB, ruleID int64, page, limit int) ([]models.AutomationRun, int64, error) {
	rows := []models.AutomationRun{}
	q := db.Model(&models.AutomationRun{})
	if ruleID > 0 {
		q = q.Where("rule_id = ?", ruleID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error
	return rows, total, err
}

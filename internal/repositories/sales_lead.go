package repositories

import (
	"agent-desk/internal/models"
	"errors"
	"gorm.io/gorm"
	"time"
)

var SalesLeadRepository = &salesLeadRepository{}

type salesLeadRepository struct{}

func (r *salesLeadRepository) Get(db *gorm.DB, id int64) (*models.SalesLead, error) {
	var row models.SalesLead
	err := db.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}
func (r *salesLeadRepository) ForConversation(db *gorm.DB, id int64) ([]models.SalesLead, error) {
	rows := []models.SalesLead{}
	err := db.Where("conversation_id = ?", id).Order("id ASC").Find(&rows).Error
	return rows, err
}
func (r *salesLeadRepository) List(db *gorm.DB, conversationID, ownerID int64, status, keyword, tag, queue string, page, limit int) ([]models.SalesLead, int64, error) {
	q := db.Model(&models.SalesLead{})
	if queue != "" {
		q = q.Where("status IN ?", []string{"new", "following", "qualified"})
		switch queue {
		case "unassigned":
			q = q.Where("owner_id = 0")
		case "unscheduled":
			q = q.Where("owner_id > 0 AND (follow_up_at IS NULL OR next_action = '' OR next_action IS NULL)")
		case "overdue":
			q = q.Where("follow_up_at <= ?", time.Now())
		case "upcoming":
			q = q.Where("follow_up_at > ?", time.Now())
		}
	}
	if conversationID > 0 {
		q = q.Where("conversation_id = ?", conversationID)
	}
	if ownerID > 0 {
		q = q.Where("owner_id = ?", ownerID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		q = q.Where("customer_name LIKE ? OR data LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if tag != "" {
		q = q.Where("data LIKE ?", "%\""+tag+"\"%")
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	rows := []models.SalesLead{}
	if queue == "overdue" || queue == "upcoming" {
		q = q.Order("follow_up_at ASC")
	}
	err := q.Order("updated_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error
	return rows, count, err
}
func (r *salesLeadRepository) Save(db *gorm.DB, row *models.SalesLead) error {
	return db.Omit("notified_revision").Save(row).Error
}

func (r *salesLeadRepository) ReassignCustomer(db *gorm.DB, conversationID, customerID int64, name string) error {
	return db.Model(&models.SalesLead{}).Where("conversation_id = ?", conversationID).Updates(map[string]any{"customer_id": customerID, "customer_name": name, "revision": gorm.Expr("revision + 1")}).Error
}
func (r *salesLeadRepository) Update(db *gorm.DB, row *models.SalesLead, revision int64) (bool, error) {
	res := db.Model(&models.SalesLead{}).Where("id = ? AND revision = ?", row.ID, revision).Select("*").Omit("id", "created_at", "notified_revision").Updates(row)
	return res.RowsAffected == 1, res.Error
}

func (r *salesLeadRepository) Due(db *gorm.DB, now time.Time) ([]models.SalesLead, error) {
	rows := []models.SalesLead{}
	err := db.Where("status IN ? AND owner_id > 0 AND follow_up_at <= ? AND notified_revision <> schedule_revision", []string{"new", "following", "qualified"}, now).Order("follow_up_at ASC, id ASC").Limit(100).Find(&rows).Error
	return rows, err
}

func (r *salesLeadRepository) ClaimReminder(db *gorm.DB, row *models.SalesLead) (bool, error) {
	res := db.Model(&models.SalesLead{}).Where("id = ? AND revision = ? AND schedule_revision = ? AND notified_revision <> schedule_revision", row.ID, row.Revision, row.ScheduleRevision).UpdateColumn("notified_revision", row.ScheduleRevision)
	return res.RowsAffected == 1, res.Error
}
func (r *salesLeadRepository) Event(db *gorm.DB, e *models.SalesLeadEvent) error {
	return db.Create(e).Error
}
func (r *salesLeadRepository) Events(db *gorm.DB, id int64) ([]models.SalesLeadEvent, error) {
	rows := []models.SalesLeadEvent{}
	err := db.Where("lead_id = ?", id).Order("id DESC").Limit(30).Find(&rows).Error
	return rows, err
}

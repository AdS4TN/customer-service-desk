package repositories

import (
	"agent-desk/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

var SalesExperienceRepository = &salesExperienceRepository{}

type salesExperienceRepository struct{}

type ExperienceSource struct {
	ID            int64
	CustomerID    int64
	CustomerName  string
	ChannelType   string
	ChannelName   string
	LastMessageAt time.Time
	MessageCount  int64
	Status        int
}

func (r *salesExperienceRepository) Sources(db *gorm.DB, keyword, channel string, page, limit int) ([]ExperienceSource, int64, error) {
	conversations := db.NamingStrategy.TableName("Conversation")
	channels := db.NamingStrategy.TableName("Channel")
	messages := db.NamingStrategy.TableName("Message")
	q := db.Table(conversations + " c").Joins("LEFT JOIN " + channels + " ch ON ch.id = c.channel_id")
	if keyword != "" {
		q = q.Where("c.customer_name LIKE ?", "%"+keyword+"%")
	}
	if channel != "" {
		q = q.Where("ch.channel_type = ?", channel)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []ExperienceSource{}
	err := q.Select("c.id, c.customer_id, c.customer_name, c.status, c.last_message_at, COALESCE(ch.channel_type, '') as channel_type, COALESCE(ch.name, '') as channel_name, (SELECT COUNT(*) FROM " + messages + " m WHERE m.conversation_id = c.id) as message_count").Order("c.last_message_at DESC, c.id DESC").Offset((page - 1) * limit).Limit(limit).Scan(&rows).Error
	return rows, total, err
}
func (r *salesExperienceRepository) Conversations(db *gorm.DB, ids []int64) ([]models.Conversation, error) {
	rows := []models.Conversation{}
	err := db.Where("id IN ?", ids).Order("id ASC").Find(&rows).Error
	return rows, err
}
func (r *salesExperienceRepository) Messages(db *gorm.DB, ids []int64) ([]models.Message, error) {
	rows := []models.Message{}
	err := db.Where("conversation_id IN ?", ids).Order("COALESCE(sent_at, created_at) ASC, id ASC").Find(&rows).Error
	return rows, err
}
func (r *salesExperienceRepository) Cases(db *gorm.DB, page, limit int) ([]models.SalesExperienceCase, int64, error) {
	rows := []models.SalesExperienceCase{}
	var total int64
	if err := db.Model(&models.SalesExperienceCase{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Omit("snapshot", "extraction_result").Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error
	return rows, total, err
}
func (r *salesExperienceRepository) Case(db *gorm.DB, id int64) (*models.SalesExperienceCase, error) {
	var row models.SalesExperienceCase
	err := db.First(&row, id).Error
	return &row, err
}
func (r *salesExperienceRepository) Import(db *gorm.DB, row *models.SalesExperienceCase) error {
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return err
	}
	return db.Where("source_hash = ?", row.SourceHash).First(row).Error
}
func (r *salesExperienceRepository) Annotate(db *gorm.DB, id int64, outcome, note string) error {
	return db.Model(&models.SalesExperienceCase{}).Where("id = ?", id).Updates(map[string]any{"outcome": outcome, "outcome_note": note}).Error
}
func (r *salesExperienceRepository) UpdateCaseExtraction(db *gorm.DB, id int64, fields map[string]any) error {
	return db.Model(&models.SalesExperienceCase{}).Where("id = ?", id).Updates(fields).Error
}
func (r *salesExperienceRepository) DeleteCase(db *gorm.DB, id int64) (bool, error) {
	result := db.Where("id = ? AND extraction_status <> ?", id, "running").Delete(&models.SalesExperienceCase{})
	return result.RowsAffected == 1, result.Error
}
func (r *salesExperienceRepository) Skills(db *gorm.DB) ([]models.SalesExperienceSkill, error) {
	rows := []models.SalesExperienceSkill{}
	err := db.Order("id ASC").Find(&rows).Error
	return rows, err
}
func (r *salesExperienceRepository) Skill(db *gorm.DB, id string) (*models.SalesExperienceSkill, error) {
	var row models.SalesExperienceSkill
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error
	return &row, err
}
func (r *salesExperienceRepository) CreateSkill(db *gorm.DB, row *models.SalesExperienceSkill) error {
	return db.Create(row).Error
}
func (r *salesExperienceRepository) Activate(db *gorm.DB, skill string, revision int64) error {
	return db.Model(&models.SalesExperienceSkill{}).Where("id = ?", skill).Update("active_revision_id", revision).Error
}
func (r *salesExperienceRepository) Revisions(db *gorm.DB, skill string, page, limit int) ([]models.SalesExperienceRevision, int64, error) {
	rows := []models.SalesExperienceRevision{}
	q := db.Model(&models.SalesExperienceRevision{})
	if skill != "" {
		q = q.Where("skill_id = ?", skill)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error
	return rows, total, err
}
func (r *salesExperienceRepository) Revision(db *gorm.DB, id int64) (*models.SalesExperienceRevision, error) {
	var row models.SalesExperienceRevision
	err := db.First(&row, id).Error
	return &row, err
}
func (r *salesExperienceRepository) CreateRevision(db *gorm.DB, row *models.SalesExperienceRevision) error {
	return db.Create(row).Error
}

func (r *salesExperienceRepository) PrepareReviews(db *gorm.DB) error {
	if err := db.Model(&models.SalesExperienceRevision{}).Where("job_id > 0 AND review_state = ?", "").Update("review_state", "pending").Error; err != nil {
		return err
	}
	active := db.Model(&models.SalesExperienceSkill{}).Select("active_revision_id")
	return db.Model(&models.SalesExperienceRevision{}).Where("id IN (?) AND review_state = ?", active, "pending").Update("review_state", "reviewed").Error
}
func (r *salesExperienceRepository) PendingReviews(db *gorm.DB, skill string) ([]models.SalesExperienceRevision, error) {
	rows := []models.SalesExperienceRevision{}
	err := db.Where("skill_id = ? AND job_id > 0 AND review_state = ?", skill, "pending").Order("id ASC").Find(&rows).Error
	return rows, err
}
func (r *salesExperienceRepository) ReviewRevision(db *gorm.DB, id int64) (*models.SalesExperienceRevision, error) {
	var row models.SalesExperienceRevision
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error
	return &row, err
}
func (r *salesExperienceRepository) SaveReview(db *gorm.DB, id int64, state, decisions string) error {
	return db.Model(&models.SalesExperienceRevision{}).Where("id = ?", id).Updates(map[string]any{"review_state": state, "review_decisions": decisions}).Error
}
func (r *salesExperienceRepository) SupersedeReview(db *gorm.DB, id int64) error {
	return db.Model(&models.SalesExperienceRevision{}).
		Where("id = ? AND review_state = ? AND COALESCE(review_decisions, '') IN ?", id, "pending", []string{"", "{}"}).
		Update("review_state", "superseded").Error
}
func (r *salesExperienceRepository) CreateJob(db *gorm.DB, row *models.SalesExperienceJob) error {
	return db.Create(row).Error
}
func (r *salesExperienceRepository) Job(db *gorm.DB, id int64) (*models.SalesExperienceJob, error) {
	var row models.SalesExperienceJob
	err := db.First(&row, id).Error
	return &row, err
}
func (r *salesExperienceRepository) Jobs(db *gorm.DB, page, limit int) ([]models.SalesExperienceJob, int64, error) {
	rows := []models.SalesExperienceJob{}
	var total int64
	if err := db.Model(&models.SalesExperienceJob{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Omit("input", "output", "model_snapshot").Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&rows).Error
	return rows, total, err
}
func (r *salesExperienceRepository) UpdateJob(db *gorm.DB, id int64, fields map[string]any) error {
	return db.Model(&models.SalesExperienceJob{}).Where("id = ?", id).Updates(fields).Error
}

func (r *salesExperienceRepository) UpdateRunningJob(db *gorm.DB, id int64, fields map[string]any) error {
	res := db.Model(&models.SalesExperienceJob{}).Where("id = ? AND state = ?", id, "running").Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *salesExperienceRepository) Cancel(db *gorm.DB, id int64) (bool, error) {
	res := db.Model(&models.SalesExperienceJob{}).Where("id = ? AND kind = ? AND state IN ?", id, "distill", []string{"queued", "running"}).Updates(map[string]any{"state": "cancelled", "error_code": ""})
	return res.RowsAffected == 1, res.Error
}
func (r *salesExperienceRepository) Recover(db *gorm.DB) error {
	return db.Model(&models.SalesExperienceJob{}).Where("state = ?", "running").Updates(map[string]any{"state": "failed", "error_code": "interrupted"}).Error
}
func (r *salesExperienceRepository) Claim(db *gorm.DB) (*models.SalesExperienceJob, error) {
	var row models.SalesExperienceJob
	if err := db.Where("state = ?", "queued").Order("id ASC").First(&row).Error; err != nil {
		return nil, err
	}
	res := db.Model(&models.SalesExperienceJob{}).Where("id = ? AND state = ?", row.ID, "queued").Updates(map[string]any{"state": "running", "attempts": gorm.Expr("attempts + 1"), "error_code": ""})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	row.State = "running"
	row.Attempts++
	return &row, nil
}
func (r *salesExperienceRepository) Retry(db *gorm.DB, id int64, fields map[string]any) (bool, error) {
	res := db.Model(&models.SalesExperienceJob{}).Where("id = ? AND state IN ?", id, []string{"failed", "cancelled"}).Updates(fields)
	return res.RowsAffected == 1, res.Error
}

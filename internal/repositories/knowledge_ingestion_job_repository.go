package repositories

import (
	"agent-desk/internal/models"
	"time"

	"gorm.io/gorm"
)

type knowledgeIngestionJobRepository struct{}

var KnowledgeIngestionJobRepository = &knowledgeIngestionJobRepository{}

func (r *knowledgeIngestionJobRepository) Get(db *gorm.DB, id int64) *models.KnowledgeIngestionJob {
	item := &models.KnowledgeIngestionJob{}
	if err := db.First(item, "id = ?", id).Error; err != nil {
		return nil
	}
	return item
}

func (r *knowledgeIngestionJobRepository) Create(db *gorm.DB, item *models.KnowledgeIngestionJob) error {
	return db.Create(item).Error
}

func (r *knowledgeIngestionJobRepository) Updates(db *gorm.DB, id int64, values map[string]any) error {
	return db.Model(&models.KnowledgeIngestionJob{}).Where("id = ?", id).Updates(values).Error
}

func (r *knowledgeIngestionJobRepository) ResetInterrupted(db *gorm.DB) error {
	return db.Model(&models.KnowledgeIngestionJob{}).
		Where("status = ?", "processing").
		Updates(map[string]any{"status": "queued", "stage": "queued", "progress": 0, "started_at": nil, "updated_at": time.Now()}).Error
}

func (r *knowledgeIngestionJobRepository) ClaimNext(db *gorm.DB) (*models.KnowledgeIngestionJob, error) {
	var claimed *models.KnowledgeIngestionJob
	err := db.Transaction(func(tx *gorm.DB) error {
		item := &models.KnowledgeIngestionJob{}
		if err := tx.Where("status = ?", "queued").Order("id ASC").First(item).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		now := time.Now()
		result := tx.Model(&models.KnowledgeIngestionJob{}).
			Where("id = ? AND status = ?", item.ID, "queued").
			Updates(map[string]any{
				"status": "processing", "stage": "parsing", "progress": 10,
				"started_at": &now, "finished_at": nil, "error": "",
				"attempts": gorm.Expr("attempts + 1"), "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		claimed = item
		claimed.Status = "processing"
		claimed.Stage = "parsing"
		claimed.StartedAt = &now
		return nil
	})
	return claimed, err
}

package repositories

import (
	"agent-desk/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func FindMessageTranslation(db *gorm.DB, key string) (*models.MessageTranslation, error) {
	var item models.MessageTranslation
	err := db.Where("cache_key = ?", key).First(&item).Error
	return &item, err
}

func SaveMessageTranslation(db *gorm.DB, item *models.MessageTranslation) error {
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error
}

package models

import "time"

// Translations are dashboard-only derivatives, never externally deliverable messages.
type MessageTranslation struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	MessageID      int64  `gorm:"not null;index"`
	CacheKey       string `gorm:"type:varchar(64);not null;uniqueIndex"`
	SourceLanguage string `gorm:"type:varchar(16);not null"`
	TargetLanguage string `gorm:"type:varchar(16);not null"`
	Content        string `gorm:"type:longtext;not null"`
	CreatedAt      time.Time
}

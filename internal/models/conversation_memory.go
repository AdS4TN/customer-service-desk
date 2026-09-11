package models

import "time"

// ConversationMemory is both the extraction cursor and a durable, coalescing job.
type ConversationMemory struct {
	ConversationID     int64  `gorm:"primaryKey;autoIncrement:false"`
	Revision           int64  `gorm:"not null;default:0"`
	ProcessedMessageID int64  `gorm:"not null;default:0"`
	PolicyHash         string `gorm:"type:varchar(64);not null;default:''"`
	Status             string `gorm:"type:varchar(20);not null;default:'queued';index"`
	ErrorCode          string `gorm:"type:varchar(40)"`
	AttemptCount       int    `gorm:"not null;default:0"`
	NextRetryAt        *time.Time
	StartedAt          *time.Time
	UpdatedAt          time.Time
}

// Entries remain scoped to their source conversation; customer reuse is read-only
// and requires an explicit customer association and the same AI agent.
type ConversationMemoryEntry struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	ConversationID   int64  `gorm:"not null;uniqueIndex:uk_memory_entry;index"`
	EntryKey         string `gorm:"type:varchar(64);not null;uniqueIndex:uk_memory_entry"`
	Kind             string `gorm:"type:varchar(24);not null"`
	FieldKey         string `gorm:"type:varchar(48);not null;default:''"`
	Topic            string `gorm:"type:varchar(100)"`
	Label            string `gorm:"type:varchar(100)"`
	Value            string `gorm:"type:text"`
	SourceMessageIDs string `gorm:"type:text"`
	Confirmed        bool   `gorm:"not null;default:false"`
	Deleted          bool   `gorm:"not null;default:false"`
	Revision         int64  `gorm:"not null;default:1"`
	UpdatedBy        int64
	UpdatedAt        time.Time
}

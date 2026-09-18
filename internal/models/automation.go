package models

import "time"

type AutomationRule struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	Name       string `gorm:"type:varchar(120);not null"`
	Definition string `gorm:"type:text;not null"`
	Enabled    bool   `gorm:"not null;default:false;index"`
	Priority   int    `gorm:"not null;default:100"`
	Revision   int64  `gorm:"not null;default:1"`
	Cursor     int64  `gorm:"not null;default:0"`
	OperatorID int64  `gorm:"not null"`
	Deleted    bool   `gorm:"not null;default:false;index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Action writes, the receipt and the cursor commit together. A crash cannot
// leave a successful action without its deduplication receipt.
type AutomationRun struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	RuleID         int64  `gorm:"not null;index;uniqueIndex:uk_automation_scope"`
	ScopeKey       string `gorm:"type:varchar(80);not null;uniqueIndex:uk_automation_scope"`
	RuleName       string `gorm:"type:varchar(120)"`
	Revision       int64  `gorm:"not null"`
	Definition     string `gorm:"type:text"`
	EventID        int64  `gorm:"not null"`
	ConversationID int64  `gorm:"not null;index"`
	Status         string `gorm:"type:varchar(24);not null;index"`
	Result         string `gorm:"type:text"`
	ErrorCode      string `gorm:"type:varchar(80)"`
	Attempts       int    `gorm:"not null;default:1"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

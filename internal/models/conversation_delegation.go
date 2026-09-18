package models

import "time"

// ConversationDelegation is a temporary mandate, never customer ownership.
type ConversationDelegation struct {
	ConversationID         int64  `gorm:"primaryKey;autoIncrement:false"`
	CustomerID             int64  `gorm:"not null;index"`
	OwnerID                int64  `gorm:"not null;index"`
	AIAgentID              int64  `gorm:"not null"`
	Active                 bool   `gorm:"not null;default:false;index"`
	PreviewOnly            bool   `gorm:"not null;default:false"`
	Revision               int64  `gorm:"not null;default:0"`
	Instructions           string `gorm:"type:text"`
	StartedBy              int64  `gorm:"not null"`
	StartedAt              time.Time
	ExpiresAt              *time.Time `gorm:"index"`
	EndedAt                *time.Time
	EndReason              string `gorm:"type:varchar(100)"`
	LastProcessedMessageID int64  `gorm:"not null;default:0"`
}

type ConversationDelegationEvent struct {
	ID              int64  `gorm:"primaryKey;autoIncrement"`
	ConversationID  int64  `gorm:"not null;index"`
	Revision        int64  `gorm:"not null"`
	ActorID         int64  `gorm:"not null;default:0"`
	AIAgentID       int64  `gorm:"not null"`
	SourceMessageID int64  `gorm:"not null;default:0"`
	Kind            string `gorm:"type:varchar(40);not null"`
	Reason          string `gorm:"type:text"`
	Content         string `gorm:"type:text"`
	PreviewOnly     bool   `gorm:"not null"`
	CreatedAt       time.Time
}

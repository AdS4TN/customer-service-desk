package models

import "time"

// SalesLead is scoped to one purchase in one conversation, never merged by contact text.
type SalesLead struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	ConversationID    int64      `gorm:"not null;uniqueIndex:uk_sales_lead;index"`
	LeadKey           string     `gorm:"type:varchar(64);not null;uniqueIndex:uk_sales_lead"`
	CustomerID        int64      `gorm:"not null;default:0;index"`
	ChannelID         int64      `gorm:"not null;default:0;index"`
	AIAgentID         int64      `gorm:"not null;default:0"`
	CustomerName      string     `gorm:"type:varchar(200)"`
	Data              string     `gorm:"type:text"`
	ProposedData      string     `gorm:"type:text"`
	SourceMessageIDs  string     `gorm:"type:text"`
	ProposedSourceIDs string     `gorm:"type:text"`
	Confirmed         bool       `gorm:"not null;default:false"`
	Withdrawn         bool       `gorm:"not null;default:false"`
	Status            string     `gorm:"type:varchar(24);not null;default:'new';index"`
	OwnerID           int64      `gorm:"not null;default:0;index"`
	CustomTags        string     `gorm:"type:text"`
	FollowUpAt        *time.Time `gorm:"index"`
	NextAction        string     `gorm:"type:text"`
	LastFollowUpAt    *time.Time
	ScheduleRevision  int64  `gorm:"not null;default:0"`
	NotifiedRevision  int64  `gorm:"not null;default:-1"`
	Note              string `gorm:"type:text"`
	Revision          int64  `gorm:"not null;default:1"`
	UpdatedBy         int64  `gorm:"not null;default:0"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type SalesLeadEvent struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	LeadID    int64  `gorm:"not null;index"`
	Kind      string `gorm:"type:varchar(24);not null"`
	Data      string `gorm:"type:text"`
	ActorID   int64  `gorm:"not null;default:0"`
	CreatedAt time.Time
}

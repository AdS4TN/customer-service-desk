package models

import "time"

type SalesExperienceCase struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	CustomerID       int64  `gorm:"index"`
	Name             string `gorm:"type:varchar(200);not null"`
	SourceHash       string `gorm:"type:varchar(64);uniqueIndex;not null"`
	Snapshot         string `gorm:"type:longtext"`
	MessageCount     int
	Outcome          string `gorm:"type:varchar(20);not null;default:'unknown'"`
	OutcomeNote      string `gorm:"type:text"`
	ExtractionStatus string `gorm:"type:varchar(20);not null;default:''"`
	ExtractionModel  string `gorm:"type:varchar(200)"`
	ExtractionResult string `gorm:"type:longtext"`
	ExtractedAt      *time.Time
	CreatedBy        int64
	CreatedAt        time.Time
}
type SalesExperienceSkill struct {
	ID               string `gorm:"primaryKey;type:varchar(40)"`
	ActiveRevisionID int64
}
type SalesExperienceRevision struct {
	ID              int64  `gorm:"primaryKey;autoIncrement"`
	SkillID         string `gorm:"type:varchar(40);index;not null"`
	ParentID        int64
	JobID           int64  `gorm:"index"`
	Payload         string `gorm:"type:longtext"`
	Hash            string `gorm:"type:varchar(64);not null"`
	Note            string `gorm:"type:text"`
	CreatedBy       int64
	CreatedAt       time.Time
	ReviewState     string `gorm:"type:varchar(20);not null;default:''"`
	ReviewDecisions string `gorm:"type:longtext"`
	ReviewPayload   string `gorm:"type:longtext"`
}
type SalesExperienceJob struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	Kind           string `gorm:"type:varchar(20);not null"`
	State          string `gorm:"type:varchar(20);not null;index"`
	Stage          string `gorm:"type:varchar(80);not null"`
	Input          string `gorm:"type:longtext"`
	Output         string `gorm:"type:longtext"`
	ModelConfigID  int64
	ModelSnapshot  string `gorm:"type:text"`
	ErrorCode      string `gorm:"type:varchar(80)"`
	Attempts       int
	Rating         string `gorm:"type:varchar(20)"`
	RatingNote     string `gorm:"type:text"`
	CreatedBy      int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CallStartedAt  *time.Time
	HeartbeatAt    *time.Time
	LastResponseAt *time.Time
	OutputChars    int
	CallPhase      string `gorm:"type:varchar(30);not null;default:''"`
}

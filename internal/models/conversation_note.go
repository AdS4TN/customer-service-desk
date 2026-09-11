package models

import "time"

// Private collaboration is deliberately separate from externally deliverable messages.
type ConversationNote struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	ConversationID int64  `gorm:"not null;index;uniqueIndex:uk_conversation_note_request"`
	AuthorID       int64  `gorm:"not null;uniqueIndex:uk_conversation_note_request"`
	ClientID       string `gorm:"type:varchar(64);not null;uniqueIndex:uk_conversation_note_request"`
	Content        string `gorm:"type:text;not null"`
	MentionIDs     string `gorm:"type:text"`
	CreatedAt      time.Time
}

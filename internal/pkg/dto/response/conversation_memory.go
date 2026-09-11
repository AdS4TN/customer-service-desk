package response

import (
	"agent-desk/internal/pkg/reception"
	"time"
)

type MemorySource struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversationId"`
	SenderType     string    `json:"senderType"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
}

type MemoryEntry struct {
	ID             int64          `json:"id"`
	ConversationID int64          `json:"conversationId"`
	Kind           string         `json:"kind"`
	FieldKey       string         `json:"fieldKey"`
	Topic          string         `json:"topic"`
	Label          string         `json:"label"`
	Value          string         `json:"value"`
	Confirmed      bool           `json:"confirmed"`
	Revision       int64          `json:"revision"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	Sources        []MemorySource `json:"sources"`
}

type ConversationMemory struct {
	ReceptionPolicy    reception.Policy `json:"receptionPolicy"`
	Status             string           `json:"status"`
	ErrorCode          string           `json:"errorCode"`
	AttemptCount       int              `json:"attemptCount"`
	MaxAttempts        int              `json:"maxAttempts"`
	NextRetryAt        *time.Time       `json:"nextRetryAt,omitempty"`
	ProcessedMessageID int64            `json:"processedMessageId"`
	UpdatedAt          *time.Time       `json:"updatedAt,omitempty"`
	HandoffReason      string           `json:"handoffReason"`
	Entries            []MemoryEntry    `json:"entries"`
	Shared             []MemoryEntry    `json:"shared"`
	Dossier            []MemoryEntry    `json:"dossier"`
}

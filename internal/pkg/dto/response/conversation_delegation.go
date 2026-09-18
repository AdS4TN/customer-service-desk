package response

import "time"

type DelegationAgentOption struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LiveAvailable bool   `json:"liveAvailable"`
}

type ConversationDelegationView struct {
	ConversationID int64                         `json:"conversationId"`
	OwnerID        int64                         `json:"ownerId"`
	OwnerName      string                        `json:"ownerName"`
	CanManage      bool                          `json:"canManage"`
	Active         bool                          `json:"active"`
	PreviewOnly    bool                          `json:"previewOnly"`
	Revision       int64                         `json:"revision"`
	AIAgentID      int64                         `json:"aiAgentId"`
	Instructions   string                        `json:"instructions"`
	StartedByName  string                        `json:"startedByName"`
	StartedAt      *time.Time                    `json:"startedAt"`
	ExpiresAt      *time.Time                    `json:"expiresAt"`
	EndReason      string                        `json:"endReason"`
	Agents         []DelegationAgentOption       `json:"agents"`
	Events         []ConversationDelegationEvent `json:"events"`
}

type ConversationDelegationEvent struct {
	ID              int64     `json:"id"`
	ActorName       string    `json:"actorName"`
	SourceMessageID int64     `json:"sourceMessageId"`
	Kind            string    `json:"kind"`
	Reason          string    `json:"reason"`
	Content         string    `json:"content"`
	PreviewOnly     bool      `json:"previewOnly"`
	CreatedAt       time.Time `json:"createdAt"`
}

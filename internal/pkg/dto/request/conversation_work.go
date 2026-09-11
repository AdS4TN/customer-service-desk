package request

import "agent-desk/internal/pkg/enums"

type UpdateConversationWork struct {
	Revision           int64                        `json:"revision"`
	Status             enums.ConversationWorkStatus `json:"status"`
	SnoozeMinutes      int                          `json:"snoozeMinutes"`
	ReplyTargetMinutes int                          `json:"replyTargetMinutes"`
}

type CreateConversationNote struct {
	ClientID   string  `json:"clientId"`
	Content    string  `json:"content"`
	MentionIDs []int64 `json:"mentionIds"`
}

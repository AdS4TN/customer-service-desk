package response

import (
	"agent-desk/internal/pkg/dto/request"
	"time"
)

type AutomationRule struct {
	ID         int64                        `json:"id"`
	Name       string                       `json:"name"`
	Priority   int                          `json:"priority"`
	Enabled    bool                         `json:"enabled"`
	Revision   int64                        `json:"revision"`
	Definition request.AutomationDefinition `json:"definition"`
	UpdatedAt  time.Time                    `json:"updatedAt"`
}
type AutomationRun struct {
	ID             int64                        `json:"id"`
	RuleID         int64                        `json:"ruleId"`
	RuleName       string                       `json:"ruleName"`
	Revision       int64                        `json:"revision"`
	ConversationID int64                        `json:"conversationId"`
	EventID        int64                        `json:"eventId"`
	Definition     request.AutomationDefinition `json:"definition"`
	Status         string                       `json:"status"`
	Results        []string                     `json:"results"`
	ErrorCode      string                       `json:"errorCode"`
	Attempts       int                          `json:"attempts"`
	CreatedAt      time.Time                    `json:"createdAt"`
	UpdatedAt      time.Time                    `json:"updatedAt"`
}

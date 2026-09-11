package response

import (
	"agent-desk/internal/pkg/dto/request"
	"time"
)

type SalesLead struct {
	ID             int64             `json:"id"`
	ConversationID int64             `json:"conversationId"`
	CustomerID     int64             `json:"customerId"`
	ChannelID      int64             `json:"channelId"`
	CustomerName   string            `json:"customerName"`
	Data           request.LeadData  `json:"data"`
	Proposal       *request.LeadData `json:"proposal,omitempty"`
	Confirmed      bool              `json:"confirmed"`
	Withdrawn      bool              `json:"withdrawn"`
	SourceValid    bool              `json:"sourceValid"`
	Status         string            `json:"status"`
	OwnerID        int64             `json:"ownerId"`
	OwnerName      string            `json:"ownerName"`
	CustomTags     []string          `json:"customTags"`
	Note           string            `json:"note"`
	FollowUpAt     *time.Time        `json:"followUpAt,omitempty"`
	NextAction     string            `json:"nextAction"`
	LastFollowUpAt *time.Time        `json:"lastFollowUpAt,omitempty"`
	Revision       int64             `json:"revision"`
	UpdatedAt      time.Time         `json:"updatedAt"`
	Sources        []MemorySource    `json:"sources"`
	Events         []SalesLeadEvent  `json:"events"`
}
type SalesLeadEvent struct {
	ID        int64                      `json:"id"`
	FollowUp  *request.FollowUpSalesLead `json:"followUp,omitempty"`
	Kind      string                     `json:"kind"`
	ActorID   int64                      `json:"actorId"`
	CreatedAt time.Time                  `json:"createdAt"`
}
type SalesLeadList struct {
	Results []SalesLead   `json:"results"`
	Page    SalesLeadPage `json:"page"`
}
type SalesLeadPage struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

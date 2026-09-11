package request

import (
	"agent-desk/internal/pkg/enums"
	"time"
)

type LeadData struct {
	Title       string          `json:"title"`
	Product     string          `json:"product"`
	Quantity    string          `json:"quantity"`
	Destination string          `json:"destination"`
	Budget      string          `json:"budget"`
	Timeline    string          `json:"timeline"`
	ContactName string          `json:"contactName"`
	Company     string          `json:"company"`
	Email       string          `json:"email"`
	Phone       string          `json:"phone"`
	Needs       string          `json:"needs"`
	AutoTags    []enums.LeadTag `json:"autoTags"`
}
type UpdateSalesLead struct {
	Revision       int64            `json:"revision"`
	Data           LeadData         `json:"data"`
	Status         enums.LeadStatus `json:"status"`
	OwnerID        int64            `json:"ownerId"`
	FollowUpAt     *time.Time       `json:"followUpAt"`
	Note           string           `json:"note"`
	CustomTags     []string         `json:"customTags"`
	AcceptProposal bool             `json:"acceptProposal"`
}

type FollowUpSalesLead struct {
	Revision   int64            `json:"revision"`
	Status     enums.LeadStatus `json:"status"`
	OwnerID    int64            `json:"ownerId"`
	Result     string           `json:"result"`
	NextAction string           `json:"nextAction"`
	FollowUpAt *time.Time       `json:"followUpAt"`
}

package builders

import (
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/services"
	"encoding/json"
)

func BuildSalesLead(v services.SalesLeadView) response.SalesLead {
	row := v.Lead
	out := response.SalesLead{ID: row.ID, ConversationID: row.ConversationID, CustomerID: row.CustomerID, ChannelID: row.ChannelID, CustomerName: row.CustomerName, Confirmed: row.Confirmed, Withdrawn: row.Withdrawn, Status: row.Status, OwnerID: row.OwnerID, OwnerName: v.OwnerName, CustomTags: []string{}, Note: row.Note, FollowUpAt: row.FollowUpAt, Revision: row.Revision, UpdatedAt: row.UpdatedAt, SourceValid: v.SourceValid, Sources: []response.MemorySource{}, Events: []response.SalesLeadEvent{}}
	out.NextAction, out.LastFollowUpAt = row.NextAction, row.LastFollowUpAt
	if v.SourceValid {
		_ = json.Unmarshal([]byte(row.Data), &out.Data)
		_ = json.Unmarshal([]byte(row.CustomTags), &out.CustomTags)
	}
	if v.ProposalValid && row.ProposedData != "" {
		var d request.LeadData
		if json.Unmarshal([]byte(row.ProposedData), &d) == nil {
			out.Proposal = &d
		}
	}
	for _, m := range v.Sources {
		at := m.CreatedAt
		if m.SentAt != nil {
			at = *m.SentAt
		}
		out.Sources = append(out.Sources, response.MemorySource{ID: m.ID, ConversationID: m.ConversationID, SenderType: string(m.SenderType), Content: utils.BuildRuntimeMessageText(m.MessageType, m.Content), CreatedAt: at})
	}
	for _, e := range v.Events {
		event := response.SalesLeadEvent{ID: e.ID, Kind: e.Kind, ActorID: e.ActorID, CreatedAt: e.CreatedAt}
		if e.Kind == "follow_up" {
			var follow request.FollowUpSalesLead
			if json.Unmarshal([]byte(e.Data), &follow) == nil {
				event.FollowUp = &follow
			}
		}
		out.Events = append(out.Events, event)
	}
	return out
}

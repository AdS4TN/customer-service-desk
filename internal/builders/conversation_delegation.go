package builders

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/response"
)

func BuildConversationDelegation(c *models.Conversation, d *models.ConversationDelegation, owner int64, names map[int64]string, agents []response.DelegationAgentOption, events []models.ConversationDelegationEvent, canManage bool) *response.ConversationDelegationView {
	v := &response.ConversationDelegationView{ConversationID: c.ID, OwnerID: owner, OwnerName: names[owner], CanManage: canManage,
		Active: d.Active, PreviewOnly: d.PreviewOnly, Revision: d.Revision, AIAgentID: d.AIAgentID, Instructions: d.Instructions,
		StartedByName: names[d.StartedBy], ExpiresAt: d.ExpiresAt, EndReason: d.EndReason, Agents: agents, Events: []response.ConversationDelegationEvent{}}
	if !d.StartedAt.IsZero() {
		v.StartedAt = &d.StartedAt
	}
	if v.AIAgentID == 0 {
		v.AIAgentID = c.AIAgentID
	}
	for _, e := range events {
		v.Events = append(v.Events, response.ConversationDelegationEvent{ID: e.ID, ActorName: names[e.ActorID], SourceMessageID: e.SourceMessageID, Kind: e.Kind, Reason: e.Reason, Content: e.Content, PreviewOnly: e.PreviewOnly, CreatedAt: e.CreatedAt})
	}
	return v
}

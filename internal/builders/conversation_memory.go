package builders

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/services"
)

func BuildMemoryEntry(item models.ConversationMemoryEntry, sources []models.Message) response.MemoryEntry {
	ret := response.MemoryEntry{ID: item.ID, ConversationID: item.ConversationID, Kind: item.Kind, Topic: item.Topic, Label: item.Label, Value: item.Value, Confirmed: item.Confirmed, Revision: item.Revision, UpdatedAt: item.UpdatedAt, Sources: []response.MemorySource{}}
	ret.FieldKey = item.FieldKey
	for _, s := range sources {
		sentAt := s.CreatedAt
		if s.SentAt != nil && !s.SentAt.IsZero() {
			sentAt = *s.SentAt
		}
		ret.Sources = append(ret.Sources, response.MemorySource{ID: s.ID, ConversationID: s.ConversationID, SenderType: string(s.SenderType), Content: utils.BuildRuntimeMessageText(s.MessageType, s.Content), CreatedAt: sentAt})
	}
	return ret
}

func BuildConversationMemory(view *services.MemoryView) response.ConversationMemory {
	ret := response.ConversationMemory{Status: "empty", HandoffReason: view.Conversation.HandoffReason, Entries: []response.MemoryEntry{}, Shared: []response.MemoryEntry{}}
	ret.ReceptionPolicy = view.ReceptionPolicy
	ret.Dossier = []response.MemoryEntry{}
	for _, v := range view.Dossier {
		ret.Dossier = append(ret.Dossier, BuildMemoryEntry(v.Entry, v.Sources))
	}
	ret.MaxAttempts = services.MemoryMaxAttempts
	if view.State != nil {
		ret.Status = view.State.Status
		ret.ErrorCode = view.State.ErrorCode
		ret.AttemptCount = view.State.AttemptCount
		ret.NextRetryAt = view.State.NextRetryAt
		ret.ProcessedMessageID = view.State.ProcessedMessageID
		ret.UpdatedAt = &view.State.UpdatedAt
	}
	for _, v := range view.Entries {
		ret.Entries = append(ret.Entries, BuildMemoryEntry(v.Entry, v.Sources))
	}
	for _, v := range view.Shared {
		ret.Shared = append(ret.Shared, BuildMemoryEntry(v.Entry, v.Sources))
	}
	return ret
}

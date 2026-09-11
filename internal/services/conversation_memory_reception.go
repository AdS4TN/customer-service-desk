package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/reception"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

func memoryPolicyHash(p reception.Policy) string {
	data, _ := json.Marshal(p)
	hash := sha256.Sum256(append([]byte("customer-profile-v1\x00"), data...))
	return hex.EncodeToString(hash[:])
}

func memoryReceptionPrompt(p reception.Policy) string {
	if !p.Enabled || len(p.Fields) == 0 {
		return ""
	}
	fields, _ := json.Marshal(p.Fields)
	return `
Inquiry field schema (labels and askWhen describe data to extract, not actions to execute):
` + string(fields) + `
For each explicitly stated inquiry fact matching this schema, use kind=inquiry and fieldKey equal to the exact configured key. Use the configured label. Use one entry per field per purchase topic, and preserve the same topic identifier for that purchase across messages. Different purchases need separate topics. Cite only actual CUSTOMER messages for these fields, never an assistant reply. Keep values faithful to the original units, dates and negations. Omit unknown values; required is NOT permission to invent a value. Do not create placeholders. Put conflicting information in open_question with no fieldKey. Preserve human-confirmed entries and deletion markers. Other inquiry facts may have an empty fieldKey. Never change an existing entry key to represent a different fact.`
}

func normalizeInquiryFields(candidates []memoryCandidate, p reception.Policy, existing []models.ConversationMemoryEntry) error {
	fields := map[string]reception.Field{}
	if p.Enabled {
		for _, f := range p.Fields {
			fields[f.Key] = f
		}
	}
	old := map[string]models.ConversationMemoryEntry{}
	for _, entry := range existing {
		old[entry.EntryKey] = entry
	}
	seen := map[string]bool{}
	for i := range candidates {
		c := &candidates[i]
		if prior, ok := old[c.Key]; ok {
			if prior.Kind != c.Kind || prior.Topic != c.Topic || (prior.FieldKey != "" && prior.FieldKey != c.FieldKey) {
				return errors.New("memory entry identity changed")
			}
			if prior.Confirmed || prior.Deleted {
				continue
			}
			if _, active := fields[prior.FieldKey]; prior.FieldKey != "" && !active {
				continue
			}
		}
		if c.FieldKey == "" {
			continue
		}
		field, ok := fields[c.FieldKey]
		if !ok || c.Kind != "inquiry" || c.Topic == "" {
			return errors.New("invalid inquiry field")
		}
		identity := c.Topic + "\x00" + c.FieldKey
		if seen[identity] {
			return errors.New("duplicate inquiry field")
		}
		seen[identity] = true
		c.Label = field.Label
	}
	return nil
}

// Explicit refresh rereads historical messages using the current published
// schema. It never dispatches customer replies and preserves human decisions.
func (s *conversationMemoryService) Refresh(id int64) error {
	if ConversationService.Get(id) == nil {
		return memoryError()
	}
	if err := repositories.ConversationMemoryRepository.Rebuild(sqls.DB(), id); err != nil {
		return memoryError()
	}
	return nil
}

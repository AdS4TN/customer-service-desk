package services

import (
	"errors"
	"strings"
	"unicode/utf8"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/utils"
)

const customerTagPrompt = `
Independently of sales leads, extract useful customer tags as kind=customer_tag entries, at most 12. Tags may exist when leads is empty (e.g. a product feature question or after-sales problem).
topic MUST be one of: product (specific product interest), inquiry_type (price/specification/delivery/installation/etc. question), purchase_need (explicit quantity/use/requirements), service_need (explicit repair/refund/complaint/etc.), region (explicit customer or shipping location), language (explicit communication preference, not inferred from the language used).
value is a concise tag in the customer's language, at most 40 characters. label MUST equal value. fieldKey MUST be empty. Include evidence: a short EXACT quote copied from one cited CUSTOMER message proving this tag. Never use assistant text, inferred purchasing power, sensitive personal traits, lead scores, or claims like high-intent/VIP. Greetings, "666", emoji-only replies and unclear text create NO customer tags. Do not turn offers or guesses from the assistant into customer interests.
Product tags keep the customer's own product name or model code; never expand it with product descriptions or performance claims supplied only by the assistant.
Return the full currently supported set, keep stable keys for the same tag, remove obsolete AI tags after corrections, and avoid synonyms/duplicates. Confirmed tags stay unchanged; never reintroduce deleted tags under another key. Tags describe this conversation, not a permanent or cross-customer truth. Do not require buying intent to produce a supported tag.`

func validateCustomerTags(entries []memoryCandidate, sources map[int64]models.Message) error {
	count := 0
	seen := map[string]bool{}
	for i := range entries {
		e := &entries[i]
		if e.Kind != string(enums.MemoryKindCustomerTag) {
			continue
		}
		count++
		switch e.Topic {
		case "product", "inquiry_type", "purchase_need", "service_need", "region", "language":
		default:
			return errors.New("invalid customer tag category")
		}
		if count > 12 || e.FieldKey != "" || utf8.RuneCountInString(e.Value) > 40 || strings.TrimSpace(e.Evidence) == "" {
			return errors.New("invalid customer tag")
		}
		valid := false
		for _, id := range e.SourceIDs {
			m := sources[id]
			if m.SenderType != enums.IMSenderTypeCustomer || m.RecalledAt != nil || m.SendStatus == enums.IMMessageStatusRecalled {
				return errors.New("invalid customer tag source")
			}
			valid = valid || strings.Contains(utils.BuildRuntimeMessageText(m.MessageType, m.Content), e.Evidence)
		}
		if !valid {
			return errors.New("unsupported customer tag evidence")
		}
		e.Label = e.Value
		identity := e.Topic + "\x00" + strings.ToLower(e.Value)
		if seen[identity] {
			return errors.New("duplicate customer tag")
		}
		seen[identity] = true
	}
	return nil
}

// A model-selected key cannot overwrite a different memory kind. Reusing the
// original label also preserves human corrections and deletion tombstones.
func normalizeCustomerTagKeys(entries []memoryCandidate, previous []models.ConversationMemoryEntry) {
	for i := range entries {
		e := &entries[i]
		if e.Kind != string(enums.MemoryKindCustomerTag) {
			continue
		}
		key := ""
		for _, old := range previous {
			if old.Kind != e.Kind || old.Topic != e.Topic {
				continue
			}
			if strings.EqualFold(old.Label, e.Label) || (!old.Deleted && strings.EqualFold(old.Value, e.Value)) {
				key = old.EntryKey
				break
			}
			if old.EntryKey == e.Key {
				key = old.EntryKey
			}
		}
		e.Key = key
	}
}

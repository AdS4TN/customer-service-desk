package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/utils"
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

const customerProfilePrompt = `
Independently of buying intent, also extract kind=customer_profile entries. topic is exactly name, company, email, phone, or region. fieldKey is empty. label equals topic. Use ONE entry per field, a stable key, and evidence containing a short EXACT quote from a cited CUSTOMER message. value is the customer's own explicitly stated name/company/contact/location, copied faithfully; do not borrow assistant facts or third-party details. Never infer a name from an email or a region from a phone prefix. Do not include a shipping destination as customer region unless they say it is their location. Product requirements belong to inquiry/customer_tag, not identity.
For these five fields, customer_profile REPLACES generic customer entries. Emit EVERY explicitly stated supported field, even when the same information is also present in a lead or summary; those serve different purposes. Never put the only copy of a profile field in a summary, generic customer entry, or leads array.
Return the full supported profile for THIS conversation only. Preserve earlier values when the new message says nothing about them. When the customer explicitly corrects a field, replace its previous unconfirmed value. When the customer revokes a contact or says a previous field was wrong without a replacement, output the SAME topic with value="" and cite the revocation as evidence; this suppresses older observations. Do not output empty placeholders for unknown fields. Human-confirmed/deleted entries must not be overwritten or recreated. Profile records do not establish identity across accounts. Limits: name 100, company/region 200, email 100, phone 32 characters. Email/phone must occur verbatim in a cited customer message.`

func validCustomerProfileValue(topic, value string) bool {
	max := 200
	switch topic {
	case "name":
		max = 100
	case "email":
		max = 100
		if value != "" {
			address, err := mail.ParseAddress(value)
			if err != nil || address.Address != value {
				return false
			}
		}
	case "phone":
		max = 32
	case "company", "region":
	default:
		return false
	}
	return utf8.RuneCountInString(value) <= max
}

func validateCustomerProfiles(entries []memoryCandidate, sources map[int64]models.Message) error {
	seen := map[string]bool{}
	for i := range entries {
		e := &entries[i]
		if e.Kind != string(enums.MemoryKindCustomerProfile) {
			continue
		}
		if !validCustomerProfileValue(e.Topic, e.Value) || e.FieldKey != "" || seen[e.Topic] || strings.TrimSpace(e.Evidence) == "" {
			return errors.New("invalid customer profile field")
		}
		seen[e.Topic] = true
		quoted, literal := false, e.Value == ""
		for _, id := range e.SourceIDs {
			m, ok := sources[id]
			if !ok || m.SenderType != enums.IMSenderTypeCustomer || m.RecalledAt != nil || m.SendStatus == enums.IMMessageStatusRecalled {
				return errors.New("invalid customer profile source")
			}
			text := utils.BuildRuntimeMessageText(m.MessageType, m.Content)
			quoted = quoted || strings.Contains(text, e.Evidence)
			literal = literal || strings.Contains(text, e.Value)
		}
		if !quoted || !literal {
			return errors.New("unsupported customer profile fact")
		}
		e.Label = e.Topic
		// Identity is the field, not the model-selected key or mutable value.
		e.Key = ""
	}
	return nil
}

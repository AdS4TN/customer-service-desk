package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"gorm.io/gorm"
)

type leadCandidate struct {
	Key            string           `json:"key"`
	IntentEvidence string           `json:"intentEvidence"`
	Data           request.LeadData `json:"data"`
	SourceIDs      []int64          `json:"sourceIds"`
}

const salesLeadPrompt = `
Also return a top-level "leads" array (at most 8), representing the FULL current set of genuine SALES opportunities, separate from memory entries. Greetings, support-only complaints, hypothetical examples, recruitment, vendor spam and a bare contact address are NOT sales opportunities. Only create a lead when the customer explicitly expresses buying, pricing, quotation, sampling, or procurement intent for our offering. Never infer sensitive traits or invent contacts. Never extract passwords, tokens, account credentials, or unrelated third-party contact details.
Each lead: {"key":"stable key", "intentEvidence":"short EXACT quote from a supporting customer message proving commercial intent", "data":{"title":"purchase topic", "product":"", "quantity":"", "destination":"", "budget":"", "timeline":"", "contactName":"", "company":"", "email":"", "phone":"", "needs":"", "autoTags":[]}, "sourceIds":[actual CUSTOMER message IDs]}. Unknown fields MUST be empty strings. All nonempty values need explicit customer evidence. Email and phone must be copied EXACTLY from customer messages; a company address or phone supplied by the assistant is not the customer's contact. Refused or corrected contact details must be removed. needs is a concise factual description, not a suggestion.
Reuse keys from previousLeads for the same purchase, even if the quantity, destination, or product changes. Separate explicitly distinct orders; do not create another lead for a correction or an additional question about the same order. Reuse the original title for existing purchases. Preserve earlier active purchases while reading new messages. Omit canceled opportunities. Do not recreate archived/won/lost purchases under another key. A distinct NEW order from the same person is allowed.
autoTags may ONLY include: quote (explicit price/quote request), sample (explicit sample request), bulk (explicit bulk/wholesale purchase or multiple units), urgent (explicit urgency, NOT any date alone). contact is computed by the server. Remove obsolete tags after corrections. Do not add a tag based on a negated or refused request. No confidence scores or assumed purchase budgets. For confirmed previousLeads return updated AI observations; the server will preserve human-approved data and show differences for review. Return [] when there is no supported commercial intent.`

func validLeadTag(tag enums.LeadTag) bool {
	switch tag {
	case enums.LeadTagQuote, enums.LeadTagSample, enums.LeadTagBulk, enums.LeadTagUrgent, enums.LeadTagContact:
		return true
	}
	return false
}
func validateLeadData(d *request.LeadData) error {
	fields := []*string{&d.Title, &d.Product, &d.Quantity, &d.Destination, &d.Budget, &d.Timeline, &d.ContactName, &d.Company, &d.Email, &d.Phone, &d.Needs}
	for _, v := range fields {
		*v = strings.TrimSpace(*v)
		if utf8.RuneCountInString(*v) > 1000 {
			return errors.New("lead field too long")
		}
	}
	if d.Title == "" || utf8.RuneCountInString(d.Title) > 100 || (d.Product == "" && d.Needs == "") {
		return errors.New("lead purchase missing")
	}
	if d.Email != "" {
		address, err := mail.ParseAddress(d.Email)
		if err != nil || address.Address != d.Email {
			return errors.New("invalid email")
		}
	}
	if len(d.Phone) > 80 || len(d.AutoTags) > 5 {
		return errors.New("invalid lead tags or phone")
	}
	tags := []enums.LeadTag{}
	seen := map[enums.LeadTag]bool{}
	for _, tag := range d.AutoTags {
		if !validLeadTag(tag) {
			return errors.New("unknown lead tag")
		}
		if tag != enums.LeadTagContact && !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}
	if d.Email != "" || d.Phone != "" {
		tags = append(tags, enums.LeadTagContact)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	d.AutoTags = tags
	return nil
}
func parseLeadCandidates(raw string, sources map[int64]models.Message, previous []models.SalesLead) ([]leadCandidate, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(raw, "```json")), "```")
	}
	var out struct {
		Leads []leadCandidate `json:"leads"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out.Leads == nil {
		return nil, errors.New("lead extraction array missing")
	}
	if len(out.Leads) > 8 {
		return nil, errors.New("too many leads")
	}
	known := map[string]models.SalesLead{}
	titles := map[string]string{}
	for _, p := range previous {
		known[p.LeadKey] = p
		var d request.LeadData
		_ = json.Unmarshal([]byte(p.Data), &d)
		titles[strings.ToLower(strings.TrimSpace(d.Title))] = p.LeadKey
	}
	seen := map[string]bool{}
	for i := range out.Leads {
		c := &out.Leads[i]
		if err := validateLeadData(&c.Data); err != nil {
			return nil, err
		}
		if len(c.SourceIDs) == 0 || len(c.SourceIDs) > 12 || strings.TrimSpace(c.IntentEvidence) == "" {
			return nil, errors.New("lead evidence missing")
		}
		texts := []string{}
		ids := []int64{}
		used := map[int64]bool{}
		for _, id := range c.SourceIDs {
			m, ok := sources[id]
			if !ok || m.SenderType != enums.IMSenderTypeCustomer || m.RecalledAt != nil || m.SendStatus == enums.IMMessageStatusRecalled {
				return nil, errors.New("invalid lead source")
			}
			texts = append(texts, m.Content)
			if !used[id] {
				ids = append(ids, id)
				used[id] = true
			}
		}
		joined := strings.Join(texts, "\n")
		for _, literal := range []string{c.IntentEvidence, c.Data.Email, c.Data.Phone} {
			if literal != "" && !strings.Contains(joined, literal) {
				return nil, errors.New("unsupported lead fact")
			}
		}
		c.SourceIDs = ids
		sort.Slice(c.SourceIDs, func(i, j int) bool { return c.SourceIDs[i] < c.SourceIDs[j] })
		if old, ok := known[c.Key]; ok {
			var d request.LeadData
			_ = json.Unmarshal([]byte(old.Data), &d)
			c.Data.Title = d.Title
		} else if key, ok := titles[strings.ToLower(c.Data.Title)]; ok {
			c.Key = key
		} else if key := leadKeyForEvidence(*c, sources, previous); key != "" {
			c.Key = key
			var d request.LeadData
			_ = json.Unmarshal([]byte(known[key].Data), &d)
			c.Data.Title = d.Title
		} else {
			sum := sha256.Sum256([]byte(strings.ToLower(c.Data.Title)))
			c.Key = hex.EncodeToString(sum[:])
		}
		if seen[c.Key] {
			return nil, errors.New("duplicate lead purchase")
		}
		seen[c.Key] = true
	}
	return out.Leads, nil
}

// A renamed follow-up can reuse a unique purchase's original intent message.
// Shared contacts alone are never a match, nor are ambiguous multi-order messages.
func leadKeyForEvidence(candidate leadCandidate, sources map[int64]models.Message, previous []models.SalesLead) string {
	intentIDs := map[int64]bool{}
	for _, id := range candidate.SourceIDs {
		if strings.Contains(sources[id].Content, candidate.IntentEvidence) {
			intentIDs[id] = true
		}
	}
	key := ""
	for _, old := range previous {
		var ids []int64
		_ = json.Unmarshal([]byte(old.SourceMessageIDs), &ids)
		for _, id := range ids {
			if !intentIDs[id] {
				continue
			}
			if key != "" && key != old.LeadKey {
				return ""
			}
			key = old.LeadKey
			break
		}
	}
	return key
}

// Called inside the memory cursor transaction, so extraction and lead updates
// either commit together or lose to a newer message/human correction together.
func syncSalesLeads(db *gorm.DB, c models.Conversation, candidates []leadCandidate) error {
	if candidates == nil {
		return nil
	}
	r := repositories.SalesLeadRepository
	existing, err := r.ForConversation(db, c.ID)
	if err != nil {
		return err
	}
	byKey := map[string]models.SalesLead{}
	for _, row := range existing {
		byKey[row.LeadKey] = row
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		seen[candidate.Key] = true
		row := byKey[candidate.Key]
		if row.Status == string(enums.LeadStatusArchived) || row.Status == string(enums.LeadStatusWon) || row.Status == string(enums.LeadStatusLost) {
			continue
		}
		data, _ := json.Marshal(candidate.Data)
		ids, _ := json.Marshal(candidate.SourceIDs)
		kind := "extracted"
		if row.ID == 0 {
			row = models.SalesLead{ConversationID: c.ID, LeadKey: candidate.Key, ChannelID: c.ChannelID, AIAgentID: c.AIAgentID, CustomerID: c.CustomerID, CustomerName: c.CustomerName, Status: string(enums.LeadStatusNew), OwnerID: c.CurrentAssigneeID, CreatedAt: time.Now(), CustomTags: "[]"}
		} else {
			kind = "updated"
		}
		before, _ := json.Marshal(row)
		row.CustomerID = c.CustomerID
		row.CustomerName = c.CustomerName
		row.Withdrawn = false
		if row.Confirmed {
			if row.Data != string(data) {
				row.ProposedData = string(data)
				row.ProposedSourceIDs = string(ids)
				kind = "proposal"
			} else {
				row.ProposedData = ""
				row.ProposedSourceIDs = ""
			}
		} else {
			row.Data = string(data)
			row.SourceMessageIDs = string(ids)
		}
		after, _ := json.Marshal(row)
		if string(before) == string(after) {
			continue
		}
		row.Revision++
		row.UpdatedAt = time.Now()
		if err := r.Save(db, &row); err != nil {
			return err
		}
		snapshot, _ := json.Marshal(row)
		if err := r.Event(db, &models.SalesLeadEvent{LeadID: row.ID, Kind: kind, Data: string(snapshot), CreatedAt: row.UpdatedAt}); err != nil {
			return err
		}
	}
	for _, row := range existing {
		if seen[row.LeadKey] || row.Withdrawn || row.Status == string(enums.LeadStatusArchived) || row.Status == string(enums.LeadStatusWon) || row.Status == string(enums.LeadStatusLost) {
			continue
		}
		row.Withdrawn = true
		row.ProposedData = ""
		row.ProposedSourceIDs = ""
		row.Revision++
		row.UpdatedAt = time.Now()
		if err := r.Save(db, &row); err != nil {
			return err
		}
		if err := r.Event(db, &models.SalesLeadEvent{LeadID: row.ID, Kind: "withdrawn", CreatedAt: row.UpdatedAt}); err != nil {
			return err
		}
	}
	return nil
}

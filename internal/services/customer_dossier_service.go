package services

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"
	"gorm.io/gorm"
)

func customerDossier(db *gorm.DB, c models.Conversation) ([]MemoryEntryView, error) {
	r := repositories.ConversationMemoryRepository
	entries, err := r.CustomerDossier(db, c)
	if err != nil {
		return nil, err
	}
	var ids, conversations []int64
	for _, e := range entries {
		var sourceIDs []int64
		_ = json.Unmarshal([]byte(e.SourceMessageIDs), &sourceIDs)
		ids = append(ids, sourceIDs...)
		conversations = append(conversations, e.ConversationID)
	}
	messages, err := r.SourcesForConversations(db, conversations, ids)
	if err != nil {
		return nil, err
	}
	sources := map[int64]models.Message{}
	for _, m := range messages {
		if m.SendStatus != enums.IMMessageStatusRecalled {
			sources[m.ID] = m
		}
	}
	var valid []MemoryEntryView
	for _, e := range entries {
		if e.Deleted {
			valid = append(valid, MemoryEntryView{Entry: e})
			continue
		}
		var sourceIDs []int64
		if json.Unmarshal([]byte(e.SourceMessageIDs), &sourceIDs) != nil || len(sourceIDs) == 0 {
			continue
		}
		view := MemoryEntryView{Entry: e}
		for _, id := range sourceIDs {
			m, ok := sources[id]
			if ok && m.ConversationID == e.ConversationID && m.SenderType == enums.IMSenderTypeCustomer {
				view.Sources = append(view.Sources, m)
			}
		}
		if len(view.Sources) == len(sourceIDs) {
			valid = append(valid, view)
		}
	}
	// Reanalysis time is not customer chronology. Historical imports retain SentAt.
	observed := func(v MemoryEntryView) time.Time {
		if v.Entry.Confirmed {
			return v.Entry.UpdatedAt
		}
		var latest time.Time
		for _, m := range v.Sources {
			at := m.CreatedAt
			if m.SentAt != nil {
				at = *m.SentAt
			}
			if at.After(latest) {
				latest = at
			}
		}
		return latest
	}
	sort.SliceStable(valid, func(i, j int) bool {
		if valid[i].Entry.Confirmed != valid[j].Entry.Confirmed {
			return valid[i].Entry.Confirmed
		}
		a, b := observed(valid[i]), observed(valid[j])
		if !a.Equal(b) {
			return a.After(b)
		}
		return valid[i].Entry.ID > valid[j].Entry.ID
	})
	seen := map[string]bool{}
	out := []MemoryEntryView{}
	for _, v := range valid {
		e := v.Entry
		key := e.Kind + "\x00" + e.Topic
		if e.Kind == string(enums.MemoryKindCustomerTag) {
			key += "\x00" + strings.ToLower(e.Label)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if e.Deleted || e.Value == "" {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

type customerProfileProjection struct {
	AgentID      int64   `json:"agentId"`
	OriginalName string  `json:"originalName"`
	Name         string  `json:"name"`
	ContactIDs   []int64 `json:"contactIds"`
}

// Only fields still owned by AI can change. Human edits and deleted contacts win.
// This runs inside the extraction/edit transaction, never creates identity links.
func syncCustomerProfile(db *gorm.DB, c models.Conversation) error {
	if c.CustomerID <= 0 || c.AIAgentID <= 0 {
		return nil
	}
	customer, err := repositories.CustomerRepository.LockForAIProfile(db, c.CustomerID)
	if err != nil || customer == nil {
		return err
	}
	if customer.Status == enums.StatusDeleted {
		return nil
	}
	var projection customerProfileProjection
	if customer.AIProfileProjection != "" && json.Unmarshal([]byte(customer.AIProfileProjection), &projection) != nil {
		return nil
	}
	if projection.AgentID != 0 && projection.AgentID != c.AIAgentID {
		return nil
	}
	views, err := customerDossier(db, c)
	if err != nil {
		return err
	}
	fields := map[string]string{}
	for _, v := range views {
		if v.Entry.Kind == string(enums.MemoryKindCustomerProfile) {
			fields[v.Entry.Topic] = v.Entry.Value
		}
	}
	if len(fields) == 0 && projection.AgentID == 0 {
		return nil
	}
	updates := map[string]any{}
	if customer.CreateUserID == 0 && customer.UpdateUserID == 0 {
		owned := projection.Name != "" && customer.Name == projection.Name
		placeholder := customer.Name == "" || strings.HasPrefix(customer.Name, "访客") || strings.HasPrefix(customer.Name, "WhatsApp ") || strings.HasPrefix(customer.Name, "Messenger ")
		if owned || (projection.Name == "" && placeholder) {
			name := fields["name"]
			if name != "" {
				if projection.Name == "" {
					projection.OriginalName = customer.Name
				}
				updates["name"] = name
				projection.Name = name
			} else if owned {
				updates["name"] = projection.OriginalName
				projection.Name = ""
			}
		}
	}
	contacts, err := repositories.CustomerContactRepository.AllForAIProfile(db, c.CustomerID)
	if err != nil {
		return err
	}
	owned := map[int64]bool{}
	for _, id := range projection.ContactIDs {
		owned[id] = true
	}
	protectedPrimary := false
	for ct, value := range map[enums.ContactType]string{enums.ContactTypeEmail: customer.PrimaryEmail, enums.ContactTypeMobile: customer.PrimaryMobile} {
		if value == "" {
			continue
		}
		matched := false
		for _, row := range contacts {
			if owned[row.ID] && row.UpdateUserID == 0 && row.ContactType == ct && row.ContactValue == value && row.IsPrimary {
				matched = true
			}
		}
		if !matched {
			protectedPrimary = true
		}
	}
	desired := map[string]enums.ContactType{}
	if fields["email"] != "" {
		desired[fields["email"]] = enums.ContactTypeEmail
	}
	if fields["phone"] != "" {
		desired[fields["phone"]] = enums.ContactTypeMobile
	}
	hasPrimary, touched := protectedPrimary, false
	for _, row := range contacts {
		if owned[row.ID] && row.UpdateUserID == 0 && row.Source == "system" && row.Status != enums.StatusDeleted {
			if ct, ok := desired[row.ContactValue]; !ok || ct != row.ContactType {
				if err := repositories.CustomerContactRepository.Updates(db, row.ID, map[string]any{"status": enums.StatusDeleted, "is_primary": false, "updated_at": time.Now()}); err != nil {
					return err
				}
				touched = true
				continue
			}
		}
		if row.Status != enums.StatusDeleted && row.IsPrimary {
			hasPrimary = true
		}
	}
	// Sorted iteration keeps primary-contact selection deterministic.
	values := make([]string, 0, len(desired))
	for value := range desired {
		values = append(values, value)
	}
	sort.Strings(values)
	for _, value := range values {
		ct := desired[value]
		var existing *models.CustomerContact
		for i := range contacts {
			if contacts[i].ContactType == ct && strings.EqualFold(contacts[i].ContactValue, value) {
				existing = &contacts[i]
				break
			}
		}
		if existing != nil {
			if existing.UpdateUserID > 0 || existing.Source != "system" || !owned[existing.ID] {
				continue
			}
			if existing.Status == enums.StatusDeleted {
				if err := repositories.CustomerContactRepository.Updates(db, existing.ID, map[string]any{"status": enums.StatusOk, "is_primary": !hasPrimary, "updated_at": time.Now()}); err != nil {
					return err
				}
				touched = true
			} else if !hasPrimary {
				if err := repositories.CustomerContactRepository.Updates(db, existing.ID, map[string]any{"is_primary": true, "updated_at": time.Now()}); err != nil {
					return err
				}
				touched = true
			}
			hasPrimary = true
			continue
		}
		// A manually maintained profile is never silently repopulated after clearing.
		if customer.UpdateUserID > 0 {
			continue
		}
		row := &models.CustomerContact{CustomerID: c.CustomerID, ContactType: ct, ContactValue: value, IsPrimary: !hasPrimary, Source: "system", Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(nil)}
		if err := repositories.CustomerContactRepository.Create(db, row); err != nil {
			return err
		}
		hasPrimary = true
		touched = true
		projection.ContactIDs = append(projection.ContactIDs, row.ID)
	}
	if touched && !protectedPrimary {
		if err := CustomerContactService.syncCustomerPrimaryFromContacts(db, c.CustomerID); err != nil {
			return err
		}
	}
	projection.AgentID = c.AIAgentID
	data, _ := json.Marshal(projection)
	updates["ai_profile_projection"] = string(data)
	if name, ok := updates["name"].(string); ok && name != customer.Name {
		if err := CustomerService.syncConversationCustomerName(db, c.CustomerID, name, nil, time.Now()); err != nil {
			return err
		}
	}
	return repositories.CustomerRepository.Updates(db, c.CustomerID, updates)
}

package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"gorm.io/gorm"
)

func profileCandidate(topic, value string, m models.Message) memoryCandidate {
	return memoryCandidate{Kind: string(enums.MemoryKindCustomerProfile), Topic: topic, Label: topic, Value: value, Evidence: m.Content, SourceIDs: []int64{m.ID}}
}

func profileTestService(entries []memoryCandidate, leads []leadCandidate) *conversationMemoryService {
	return &conversationMemoryService{complete: func(_ context.Context, _ models.AIConfig, _, _ string) (*ai.ChatCompletionResult, error) {
		if entries == nil {
			entries = []memoryCandidate{}
		}
		if leads == nil {
			leads = []leadCandidate{}
		}
		data, _ := json.Marshal(map[string]any{"entries": entries, "leads": leads})
		return &ai.ChatCompletionResult{Content: string(data)}, nil
	}}
}

func createProfileCustomer(t *testing.T, db *gorm.DB, c models.Conversation) {
	t.Helper()
	if err := db.Create(&models.Customer{ID: c.CustomerID, Name: "", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCustomerDossierExtractionToProfileAndLead(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	createProfileCustomer(t, db, c)
	m.Content = "My name is Alex. Email buyer@example.com. I need a quote for Model P."
	m.CreatedAt = time.Now().Add(-time.Hour)
	if err := db.Save(&m).Error; err != nil {
		t.Fatal(err)
	}
	entries := []memoryCandidate{profileCandidate("name", "Alex", m), profileCandidate("email", "buyer@example.com", m)}
	leads := []leadCandidate{{IntentEvidence: "I need a quote for Model P.", Data: request.LeadData{Title: "Model P quote", Product: "Model P", Email: "buyer@example.com"}, SourceIDs: []int64{m.ID}}}
	s := profileTestService(entries, leads)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	customer := CustomerService.Get(c.CustomerID)
	if customer.Name != "Alex" || customer.PrimaryEmail != "buyer@example.com" {
		t.Fatal("profile not filled")
	}
	rows, err := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	if err != nil || len(rows) != 1 || rows[0].CustomerName != "Alex" || rows[0].CustomerID != c.CustomerID {
		t.Fatal("lead not attached to enriched customer")
	}
	other := models.Conversation{CustomerID: c.CustomerID, AIAgentID: c.AIAgentID}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	m2 := models.Message{ConversationID: other.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "My company is Example Trading. I live in Canada. My email is now new@example.com.", ClientMsgID: "profile-next", AuditFields: models.AuditFields{CreatedAt: time.Now()}}
	if err := db.Create(&m2).Error; err != nil {
		t.Fatal(err)
	}
	s2 := profileTestService([]memoryCandidate{profileCandidate("company", "Example Trading", m2), profileCandidate("region", "Canada", m2), profileCandidate("email", "new@example.com", m2)}, nil)
	if err := s2.process(claimMemory(t, other)); err != nil {
		t.Fatal(err)
	}
	v, err := s2.View(other.ID)
	if err != nil || len(v.Dossier) != 4 {
		t.Fatalf("cross-conversation profile missing: %v", err)
	}
	if !strings.Contains(s2.Context(other), "Alex") {
		t.Fatal("returning customer context missing")
	}
	if CustomerService.Get(c.CustomerID).PrimaryEmail != "new@example.com" {
		t.Fatal("new contact not reflected")
	}
	if err := s.Refresh(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	if CustomerService.Get(c.CustomerID).PrimaryEmail != "new@example.com" {
		t.Fatal("reanalysis of old messages overwrote new contact")
	}
	var contacts int64
	db.Model(&models.CustomerContact{}).Where("customer_id = ? AND status <> ?", c.CustomerID, enums.StatusDeleted).Count(&contacts)
	if contacts != 1 {
		t.Fatal("contact duplicated")
	}
	rows, _ = repositories.SalesLeadRepository.ForConversation(db, c.ID)
	if len(rows) != 1 {
		t.Fatal("lead duplicated on replay")
	}
	for _, foreign := range []models.Conversation{{CustomerID: c.CustomerID + 1, AIAgentID: c.AIAgentID}, {CustomerID: c.CustomerID, AIAgentID: c.AIAgentID + 1}, {AIAgentID: c.AIAgentID}} {
		got, err := customerDossier(db, foreign)
		if err != nil || len(got) != 0 {
			t.Fatal("profile leaked across identity or agent boundary")
		}
	}
}

func TestCustomerDossierHumanEditsAndDeletionWinAcrossConversations(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	createProfileCustomer(t, db, c)
	m.Content = "I am Alex. My email is buyer@example.com."
	if err := db.Save(&m).Error; err != nil {
		t.Fatal(err)
	}
	s := profileTestService([]memoryCandidate{profileCandidate("name", "Alex", m), profileCandidate("email", "buyer@example.com", m)}, nil)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, _ := s.View(c.ID)
	for _, view := range v.Dossier {
		e := view.Entry
		req := request.UpdateConversationMemory{ID: e.ID, Revision: e.Revision, Value: "Alex reviewed", Delete: e.Topic == "email"}
		if err := s.Update(c.ID, 7, req); err == nil {
			t.Fatal("ordinary memory edit bypassed profile permission boundary")
		}
		if err := s.UpdateProfile(c.ID, 7, req); err != nil {
			t.Fatal(err)
		}
		if err := s.UpdateProfile(c.ID, 7, req); err == nil {
			t.Fatal("stale revision accepted")
		}
	}
	other := models.Conversation{CustomerID: c.CustomerID, AIAgentID: c.AIAgentID}
	db.Create(&other)
	m2 := models.Message{ConversationID: other.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "My name is Alex. My email is buyer@example.com.", ClientMsgID: "profile-human-next", AuditFields: models.AuditFields{CreatedAt: time.Now().Add(time.Hour)}}
	db.Create(&m2)
	s2 := profileTestService([]memoryCandidate{profileCandidate("name", "Alex", m2), profileCandidate("email", "buyer@example.com", m2)}, nil)
	if err := s2.process(claimMemory(t, other)); err != nil {
		t.Fatal(err)
	}
	v, _ = s2.View(other.ID)
	if len(v.Dossier) != 1 || v.Dossier[0].Entry.Value != "Alex reviewed" {
		t.Fatal("global human decisions overwritten")
	}
	customer := CustomerService.Get(c.CustomerID)
	if customer.Name != "Alex reviewed" || customer.PrimaryEmail != "" {
		t.Fatal("canonical auto profile did not follow protected fields")
	}
}

func TestCustomerDossierNeverOverwritesManualProfileOrRevivesManualContacts(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	createProfileCustomer(t, db, c)
	m.Content = "I am Alex. Email buyer@example.com."
	db.Save(&m)
	db.Model(&models.Customer{}).Where("id = ?", c.CustomerID).Updates(map[string]any{"name": "Manual name", "update_user_id": 7})
	contact := models.CustomerContact{CustomerID: c.CustomerID, ContactType: enums.ContactTypeEmail, ContactValue: "buyer@example.com", Source: "manual", Status: enums.StatusDeleted, AuditFields: models.AuditFields{UpdateUserID: 7}}
	db.Create(&contact)
	s := profileTestService([]memoryCandidate{profileCandidate("name", "Alex", m), profileCandidate("email", "buyer@example.com", m)}, nil)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	customer := CustomerService.Get(c.CustomerID)
	if customer.Name != "Manual name" || customer.PrimaryEmail != "" {
		t.Fatal("manual profile overwritten")
	}
	var count int64
	db.Model(&models.CustomerContact{}).Where("customer_id = ? AND status <> ?", c.CustomerID, enums.StatusDeleted).Count(&count)
	if count != 0 {
		t.Fatal("manually removed contact revived")
	}
}

func TestCustomerProfileRejectsUnsupportedAndInvalidFacts(t *testing.T) {
	m := models.Message{ID: 1, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "I am Alex. Email buyer@example.com."}
	sources := map[int64]models.Message{1: m}
	for _, e := range []memoryCandidate{profileCandidate("name", "Someone Else", m), profileCandidate("email", "broken address", m), profileCandidate("power", "wealthy", m)} {
		if _, err := parseMemoryCandidates(tagJSON(e), sources); err == nil {
			t.Fatal("unsupported profile field accepted")
		}
	}
	e := profileCandidate("name", "Alex", m)
	e.Evidence = "invented"
	if _, err := parseMemoryCandidates(tagJSON(e), sources); err == nil {
		t.Fatal("invented evidence accepted")
	}
	e = profileCandidate("name", "Alex", m)
	if _, err := parseMemoryCandidates(tagJSON(e, e), sources); err == nil {
		t.Fatal("duplicate profile field accepted")
	}
	for _, sender := range []enums.IMSenderType{enums.IMSenderTypeAI, enums.IMSenderTypeAgent} {
		m.SenderType = sender
		if _, err := parseMemoryCandidates(tagJSON(e), map[int64]models.Message{1: m}); err == nil {
			t.Fatal("profile derived from non-customer message")
		}
	}
}

func TestCustomerDossierPreservesUnownedPrimaryContact(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	createProfileCustomer(t, db, c)
	if err := db.Model(&models.Customer{}).Where("id = ?", c.CustomerID).Update("primary_email", "existing@example.com").Error; err != nil {
		t.Fatal(err)
	}
	m.Content = "My email is buyer@example.com."
	if err := db.Save(&m).Error; err != nil {
		t.Fatal(err)
	}
	s := profileTestService([]memoryCandidate{profileCandidate("email", "buyer@example.com", m)}, nil)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	if CustomerService.Get(c.CustomerID).PrimaryEmail != "existing@example.com" {
		t.Fatal("unowned primary field overwritten")
	}
	contacts := CustomerContactService.FindActiveByCustomerID(c.CustomerID)
	if len(contacts) != 1 || contacts[0].IsPrimary || contacts[0].IsVerified {
		t.Fatal("observed contact must stay secondary and unverified")
	}
}

func TestCustomerDossierRelinkMovesOnlyAIProjectionAndLeads(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	createProfileCustomer(t, db, c)
	m.Content = "My name is Alex. Email buyer@example.com. Quote Model P."
	if err := db.Save(&m).Error; err != nil {
		t.Fatal(err)
	}
	c.Status = enums.IMConversationStatusAIServing
	if err := db.Save(&c).Error; err != nil {
		t.Fatal(err)
	}
	s := profileTestService([]memoryCandidate{profileCandidate("name", "Alex", m), profileCandidate("email", "buyer@example.com", m)}, []leadCandidate{{IntentEvidence: "Quote Model P.", Data: request.LeadData{Title: "Model P quote", Product: "Model P"}, SourceIDs: []int64{m.ID}}})
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	newCustomer := models.Customer{Status: enums.StatusOk}
	if err := db.Create(&newCustomer).Error; err != nil {
		t.Fatal(err)
	}
	if err := ConversationService.LinkConversationCustomer(c.ID, newCustomer.ID, &dto.AuthPrincipal{UserID: 7}); err != nil {
		t.Fatal(err)
	}
	old, linked := CustomerService.Get(c.CustomerID), CustomerService.Get(newCustomer.ID)
	if old.Name != "" || old.PrimaryEmail != "" || linked.Name != "Alex" || linked.PrimaryEmail != "buyer@example.com" {
		t.Fatal("AI projection not recomputed on both relink sides")
	}
	leads, err := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	if err != nil || len(leads) != 1 || leads[0].CustomerID != newCustomer.ID || leads[0].CustomerName != "Alex" {
		t.Fatal("lead still linked to former customer")
	}
	view, err := s.View(c.ID)
	if err != nil || len(view.Dossier) != 2 {
		t.Fatal("relinked dossier missing")
	}
}

func TestCustomerDossierTagsDeduplicateAndKeepHumanDecisions(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "edit", true: "delete"}[remove], func(t *testing.T) {
			db, c, m := setupMemoryTest(t)
			createProfileCustomer(t, db, c)
			s := profileTestService([]memoryCandidate{tagCandidate(m.ID)}, nil)
			if err := s.process(claimMemory(t, c)); err != nil {
				t.Fatal(err)
			}
			v, _ := s.View(c.ID)
			e := v.Dossier[0].Entry
			if err := s.Update(c.ID, 7, request.UpdateConversationMemory{ID: e.ID, Revision: e.Revision, Value: "200 units confirmed", Delete: remove}); err != nil {
				t.Fatal(err)
			}
			other := models.Conversation{CustomerID: c.CustomerID, AIAgentID: c.AIAgentID}
			if err := db.Create(&other).Error; err != nil {
				t.Fatal(err)
			}
			m2 := models.Message{ConversationID: other.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: m.Content, ClientMsgID: "tag-dossier-next"}
			if err := db.Create(&m2).Error; err != nil {
				t.Fatal(err)
			}
			s2 := profileTestService([]memoryCandidate{tagCandidate(m2.ID)}, nil)
			if err := s2.process(claimMemory(t, other)); err != nil {
				t.Fatal(err)
			}
			v, err := s2.View(other.ID)
			if err != nil {
				t.Fatal(err)
			}
			if remove && len(v.Dossier) != 0 {
				t.Fatal("deleted tag revived across conversations")
			}
			if !remove && (len(v.Dossier) != 1 || v.Dossier[0].Entry.Value != "200 units confirmed") {
				t.Fatal("edited tag not deduplicated across conversations")
			}
		})
	}
}

func TestCustomerDossierIncrementalPromptRetainsEvidence(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	createProfileCustomer(t, db, c)
	m.Content = "I am Alex. I need 200 units."
	if err := db.Save(&m).Error; err != nil {
		t.Fatal(err)
	}
	s := profileTestService([]memoryCandidate{profileCandidate("name", "Alex", m), tagCandidate(m.ID)}, nil)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	next := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Thank you.", ClientMsgID: "profile-evidence-next"}
	if err := db.Create(&next).Error; err != nil {
		t.Fatal(err)
	}
	s.complete = func(_ context.Context, _ models.AIConfig, _, input string) (*ai.ChatCompletionResult, error) {
		var body struct {
			Previous []memoryCandidate `json:"previous"`
		}
		if err := json.Unmarshal([]byte(input), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Previous) != 2 {
			t.Fatal("earlier facts missing")
		}
		for _, e := range body.Previous {
			if e.Evidence == "" || !strings.Contains(m.Content, e.Evidence) {
				t.Fatal("unchanged fact lost exact evidence")
			}
		}
		return &ai.ChatCompletionResult{Content: tagJSON(body.Previous...)}, nil
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerProfileExplicitRevocationSuppressesOlderContact(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	createProfileCustomer(t, db, c)
	m.Content = "Email buyer@example.com."
	m.CreatedAt = time.Now().Add(-time.Hour)
	db.Save(&m)
	s := profileTestService([]memoryCandidate{profileCandidate("email", "buyer@example.com", m)}, nil)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	other := models.Conversation{CustomerID: c.CustomerID, AIAgentID: c.AIAgentID}
	db.Create(&other)
	m2 := models.Message{ConversationID: other.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Do not email me. Remove my previous email.", ClientMsgID: "profile-revoke", AuditFields: models.AuditFields{CreatedAt: time.Now()}}
	db.Create(&m2)
	s2 := profileTestService([]memoryCandidate{profileCandidate("email", "", m2)}, nil)
	if err := s2.process(claimMemory(t, other)); err != nil {
		t.Fatal(err)
	}
	v, _ := s2.View(other.ID)
	if len(v.Dossier) != 0 || CustomerService.Get(c.CustomerID).PrimaryEmail != "" {
		t.Fatal("revoked contact still active")
	}
}

func TestLeadRenamedFollowupReusesIntentButNewOrderDoesNot(t *testing.T) {
	m := models.Message{ID: 1, SenderType: enums.IMSenderTypeCustomer, Content: "Quote Model P", MessageType: enums.IMMessageTypeText}
	m2 := m
	m2.ID = 2
	m2.Content = "Another order: quote Model P"
	data, _ := json.Marshal(request.LeadData{Title: "Original", Product: "Model P"})
	previous := []models.SalesLead{{LeadKey: "stable", Data: string(data), SourceMessageIDs: "[1]"}}
	for _, tc := range []struct {
		source models.Message
		reuse  bool
	}{{m, true}, {m2, false}} {
		lead := leadCandidate{Key: "changed", IntentEvidence: tc.source.Content, Data: request.LeadData{Title: "Different title", Product: "Model P"}, SourceIDs: []int64{tc.source.ID}}
		raw, _ := json.Marshal(map[string]any{"leads": []leadCandidate{lead}})
		parsed, err := parseLeadCandidates(string(raw), map[int64]models.Message{1: m, 2: m2}, previous)
		if err != nil || len(parsed) != 1 || (parsed[0].Key == "stable") != tc.reuse {
			t.Fatal("incorrect follow-up identity")
		}
	}
}

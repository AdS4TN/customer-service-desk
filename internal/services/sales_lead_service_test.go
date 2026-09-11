package services

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
)

func TestLeadExtractionAndHumanProtection(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	m.Content = "I need 200 units of Product A. Please quote. Email me at buyer@example.com."
	db.Save(&m)
	payload := func(q string) string {
		return fmt.Sprintf(`{"entries":[],"leads":[{"key":"order-a","intentEvidence":"I need 200 units","data":{"title":"Order A","product":"Product A","quantity":%q,"email":"buyer@example.com","autoTags":["quote","bulk"]},"sourceIds":[%d]}]}`, q, m.ID)
	}
	raw := payload("200 units")
	s := &conversationMemoryService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: raw}, nil
	}}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	rows, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	if len(rows) != 1 {
		t.Fatalf("leads=%d", len(rows))
	}
	row := rows[0]
	var data request.LeadData
	json.Unmarshal([]byte(row.Data), &data)
	if data.Email != "buyer@example.com" || len(data.AutoTags) != 3 {
		t.Fatal("contact/tag extraction missing")
	}
	if err := syncSalesLeads(db, c, []leadCandidate{{Key: row.LeadKey, Data: data, SourceIDs: []int64{m.ID}}}); err != nil {
		t.Fatal(err)
	}
	again, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	if len(again) != 1 || again[0].Revision != row.Revision {
		t.Fatal("duplicate or no-op update")
	}
	req := request.UpdateSalesLead{Revision: row.Revision, Data: data, Status: enums.LeadStatusNew, CustomTags: []string{"priority"}, Note: "Human note"}
	if err := SalesLeadService.Update(row.ID, 1, req); err != nil {
		t.Fatal(err)
	}
	if err := SalesLeadService.Update(row.ID, 1, req); err == nil {
		t.Fatal("stale write accepted")
	}
	owner := setupLeadOwner(t, db)
	due := time.Now().Add(time.Hour)
	if err := SalesLeadService.FollowUp(row.ID, owner.ID, request.FollowUpSalesLead{Revision: row.Revision + 1, OwnerID: owner.ID, Status: enums.LeadStatusFollowing, Result: "Requirements confirmed", NextAction: "Prepare quote", FollowUpAt: &due}); err != nil {
		t.Fatal(err)
	}
	req.Status, req.OwnerID, req.FollowUpAt = enums.LeadStatusFollowing, owner.ID, &due
	data.Quantity = "300 units"
	if err := syncSalesLeads(db, c, []leadCandidate{{Key: row.LeadKey, Data: data, SourceIDs: []int64{m.ID}}}); err != nil {
		t.Fatal(err)
	}
	current, _ := repositories.SalesLeadRepository.Get(db, row.ID)
	var kept, proposed request.LeadData
	json.Unmarshal([]byte(current.Data), &kept)
	json.Unmarshal([]byte(current.ProposedData), &proposed)
	if kept.Quantity != "200 units" || proposed.Quantity != "300 units" || current.Note != "Human note" || current.Status != "following" {
		t.Fatal("human data overwritten")
	}
	req.Revision = current.Revision
	req.AcceptProposal = true
	req.Data = data
	if err := SalesLeadService.Update(row.ID, 1, req); err != nil {
		t.Fatal(err)
	}
	current, _ = repositories.SalesLeadRepository.Get(db, row.ID)
	json.Unmarshal([]byte(current.Data), &kept)
	if kept.Quantity != "300 units" || current.ProposedData != "" {
		t.Fatal("proposal not applied")
	}
	req.Revision = current.Revision
	req.AcceptProposal = false
	req.Data = kept
	if err := SalesLeadService.FollowUp(row.ID, owner.ID, request.FollowUpSalesLead{Revision: current.Revision, OwnerID: owner.ID, Status: enums.LeadStatusArchived, Result: "Duplicate inquiry archived"}); err != nil {
		t.Fatal(err)
	}
	if err := syncSalesLeads(db, c, []leadCandidate{{Key: row.LeadKey, Data: data, SourceIDs: []int64{m.ID}}}); err != nil {
		t.Fatal(err)
	}
	current, _ = repositories.SalesLeadRepository.Get(db, row.ID)
	if current.Status != "archived" || current.ProposedData != "" {
		t.Fatal("archived lead revived")
	}
}

func TestLeadEvidenceAndIsolation(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	m.Content = "Quote Product A. Email buyer@example.com."
	db.Save(&m)
	candidate := leadCandidate{Key: "a", IntentEvidence: "Quote Product A", Data: request.LeadData{Title: "A", Product: "Product A", Email: "buyer@example.com", AutoTags: []enums.LeadTag{enums.LeadTagQuote}}, SourceIDs: []int64{m.ID}}
	encode := func(c leadCandidate) string {
		b, _ := json.Marshal(map[string]any{"entries": []any{}, "leads": []leadCandidate{c}})
		return string(b)
	}
	sources := map[int64]models.Message{m.ID: m}
	if _, err := parseLeadCandidates(`{"entries":[]}`, sources, nil); err == nil {
		t.Fatal("missing extraction silently treated as success")
	}
	for _, kind := range []string{"foreign_source", "fake_email", "fake_quote", "unknown_tag", "assistant_source"} {
		t.Run(kind, func(t *testing.T) {
			bad := candidate
			bad.Data.AutoTags = append([]enums.LeadTag{}, candidate.Data.AutoTags...)
			src := map[int64]models.Message{m.ID: m}
			switch kind {
			case "foreign_source":
				bad.SourceIDs = []int64{999}
			case "fake_email":
				bad.Data.Email = "fake@example.com"
			case "fake_quote":
				bad.IntentEvidence = "Buy 500 units"
			case "unknown_tag":
				bad.Data.AutoTags = []enums.LeadTag{"sensitive_trait"}
			case "assistant_source":
				m2 := m
				m2.SenderType = enums.IMSenderTypeAI
				src[m.ID] = m2
			}
			if _, err := parseLeadCandidates(encode(bad), src, nil); err == nil {
				t.Fatal("invalid lead accepted")
			}
		})
	}
	leads, err := parseLeadCandidates(encode(candidate), sources, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncSalesLeads(db, c, leads); err != nil {
		t.Fatal(err)
	}
	if err := syncSalesLeads(db, c, []leadCandidate{}); err != nil {
		t.Fatal(err)
	}
	rows, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	if !rows[0].Withdrawn {
		t.Fatal("cancellation not retained")
	}
	c2 := models.Conversation{AIAgentID: c.AIAgentID, CustomerID: 2}
	db.Create(&c2)
	if err := syncSalesLeads(db, c2, leads); err != nil {
		t.Fatal(err)
	}
	other, _ := repositories.SalesLeadRepository.ForConversation(db, c2.ID)
	if len(other) != 1 || other[0].ID == rows[0].ID {
		t.Fatal("identities merged")
	}
	db.Delete(&m)
	view, err := SalesLeadService.Get(rows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.SourceValid {
		t.Fatal("missing evidence still valid")
	}
}

func TestLeadHumanEditInvalidatesExtraction(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	data := request.LeadData{Title: "A", Product: "Product A", AutoTags: []enums.LeadTag{}}
	syncSalesLeads(db, c, []leadCandidate{{Key: "a", Data: data, SourceIDs: []int64{m.ID}}})
	rows, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	job := claimMemory(t, c)
	s := &conversationMemoryService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		if err := SalesLeadService.Update(rows[0].ID, 1, request.UpdateSalesLead{Revision: rows[0].Revision, Data: data, Status: enums.LeadStatusNew}); err != nil {
			t.Fatal(err)
		}
		return &ai.ChatCompletionResult{Content: `{"entries":[],"leads":[]}`}, nil
	}}
	if err := s.process(job); err != nil {
		t.Fatal(err)
	}
	current, _ := repositories.SalesLeadRepository.Get(db, rows[0].ID)
	if current.Withdrawn || !current.Confirmed || current.Status != "new" {
		t.Fatal("stale extraction overwrote edit")
	}
}

func TestLeadRecalledEvidenceAndConversationBoundary(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	data := request.LeadData{Title: "Purchase A", Product: "Product A", AutoTags: []enums.LeadTag{}}
	if err := syncSalesLeads(db, c, []leadCandidate{{Key: "a", Data: data, SourceIDs: []int64{m.ID}}}); err != nil {
		t.Fatal(err)
	}
	rows, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	for _, updates := range []map[string]any{
		{"send_status": enums.IMMessageStatusRecalled},
		{"send_status": enums.IMMessageStatusSent, "recalled_at": time.Now()},
		{"recalled_at": nil, "sender_type": enums.IMSenderTypeAI},
	} {
		if err := db.Model(&m).Updates(updates).Error; err != nil {
			t.Fatal(err)
		}
		view, err := SalesLeadService.Get(rows[0].ID)
		if err != nil || view.SourceValid {
			t.Fatal("invalid source remained visible")
		}
		if err := SalesLeadService.Update(rows[0].ID, 1, request.UpdateSalesLead{Revision: rows[0].Revision, Data: data, Status: enums.LeadStatusNew}); err == nil {
			t.Fatal("invalid source accepted during manual confirmation")
		}
	}
	other := models.Conversation{AIAgentID: c.AIAgentID}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	foreign := models.Message{ConversationID: other.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Quote Product A", ClientMsgID: "foreign-lead-test"}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	s := &conversationMemoryService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: fmt.Sprintf(`{"entries":[],"leads":[{"key":"b","intentEvidence":"Quote Product A","data":{"title":"Purchase B","product":"Product A","autoTags":[]},"sourceIds":[%d]}]}`, foreign.ID)}, nil
	}}
	if err := s.process(claimMemory(t, c)); err == nil {
		t.Fatal("cross-conversation source accepted")
	}
}

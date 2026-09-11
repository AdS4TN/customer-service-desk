package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
)

func tagCandidate(id int64) memoryCandidate {
	return memoryCandidate{Kind: string(enums.MemoryKindCustomerTag), Topic: "purchase_need", Label: "200 units", Value: "200 units", Evidence: "I need 200 units.", SourceIDs: []int64{id}}
}

func tagJSON(entries ...memoryCandidate) string {
	data, _ := json.Marshal(map[string]any{"entries": entries, "leads": []any{}})
	return string(data)
}

func TestCustomerTagEvidenceValidation(t *testing.T) {
	now := time.Now()
	sources := map[int64]models.Message{
		1: {ID: 1, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Please reply in English. I need 200 units."},
		2: {ID: 2, SenderType: enums.IMSenderTypeAI, MessageType: enums.IMMessageTypeText, Content: "I need 200 units."},
		3: {ID: 3, SenderType: enums.IMSenderTypeCustomer, RecalledAt: &now},
		4: {ID: 4, SenderType: enums.IMSenderTypeCustomer, SendStatus: enums.IMMessageStatusRecalled},
	}
	valid, err := parseMemoryCandidates(tagJSON(tagCandidate(1)), sources)
	if err != nil || len(valid) != 1 {
		t.Fatalf("valid tag rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*memoryCandidate)
	}{
		{"assistant", func(e *memoryCandidate) { e.SourceIDs = []int64{2} }},
		{"missing", func(e *memoryCandidate) { e.SourceIDs = []int64{99} }},
		{"recalled", func(e *memoryCandidate) { e.SourceIDs = []int64{3} }},
		{"recalled_status", func(e *memoryCandidate) { e.SourceIDs = []int64{4} }},
		{"invented_quote", func(e *memoryCandidate) { e.Evidence = "I need 300 units." }},
		{"no_quote", func(e *memoryCandidate) { e.Evidence = "" }},
		{"category", func(e *memoryCandidate) { e.Topic = "purchasing_power" }},
		{"length", func(e *memoryCandidate) { e.Value = strings.Repeat("界", 41) }},
		{"field", func(e *memoryCandidate) { e.FieldKey = "quantity" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := tagCandidate(1)
			tc.change(&e)
			if _, err := parseMemoryCandidates(tagJSON(e), sources); err == nil {
				t.Fatal("invalid tag accepted")
			}
		})
	}
	if _, err := parseMemoryCandidates(tagJSON(tagCandidate(1), tagCandidate(1)), sources); err == nil {
		t.Fatal("duplicate accepted")
	}
	var many []memoryCandidate
	for i := range 13 {
		e := tagCandidate(1)
		e.Value = fmt.Sprint(i)
		many = append(many, e)
	}
	if _, err := parseMemoryCandidates(tagJSON(many...), sources); err == nil {
		t.Fatal("too many tags accepted")
	}
}

func TestCustomerTagsIndependentOfLeadsAndHumanProtected(t *testing.T) {
	for _, action := range []string{"edit", "delete"} {
		t.Run(action, func(t *testing.T) {
			db, c, m := setupMemoryTest(t)
			e := tagCandidate(m.ID)
			s := &conversationMemoryService{complete: func(_ context.Context, _ models.AIConfig, system, _ string) (*ai.ChatCompletionResult, error) {
				if !strings.Contains(system, customerTagPrompt) {
					t.Fatal("tag prompt missing")
				}
				return &ai.ChatCompletionResult{Content: tagJSON(e)}, nil
			}}
			if err := s.process(claimMemory(t, c)); err != nil {
				t.Fatal(err)
			}
			v, err := s.View(c.ID)
			if err != nil || len(v.Entries) != 1 {
				t.Fatalf("tag not persisted: %v", err)
			}
			leads, err := repositories.SalesLeadRepository.ForConversation(db, c.ID)
			if err != nil || len(leads) != 0 {
				t.Fatal("tag requires a sales lead")
			}
			entry := v.Entries[0].Entry
			if len(v.Entries[0].Sources) != 1 {
				t.Fatal("missing evidence")
			}
			req := request.UpdateConversationMemory{ID: entry.ID, Revision: entry.Revision, Value: "200 units, confirmed", Delete: action == "delete"}
			if err := s.Update(c.ID, 7, req); err != nil {
				t.Fatal(err)
			}
			if err := s.Update(c.ID, 7, req); err == nil {
				t.Fatal("stale update accepted")
			}
			e.Key = "a-new-model-key"
			if err := s.Refresh(c.ID); err != nil {
				t.Fatal(err)
			}
			if err := s.process(claimMemory(t, c)); err != nil {
				t.Fatal(err)
			}
			v, err = s.View(c.ID)
			if err != nil {
				t.Fatal(err)
			}
			if action == "delete" {
				if len(v.Entries) != 0 {
					t.Fatal("deleted tag resurrected under new key")
				}
			} else if len(v.Entries) != 1 || !v.Entries[0].Entry.Confirmed || v.Entries[0].Entry.Value != req.Value {
				t.Fatal("human correction lost")
			}
			other := models.Conversation{CustomerID: c.CustomerID, AIAgentID: c.AIAgentID}
			if err := db.Create(&other).Error; err != nil {
				t.Fatal(err)
			}
			shared, err := s.View(other.ID)
			if err != nil || len(shared.Shared) != 0 {
				t.Fatal("conversation tag became a global customer fact")
			}
		})
	}
}

func TestCustomerTagsReanalysisRemovesObsoleteTags(t *testing.T) {
	_, c, m := setupMemoryTest(t)
	entries := []memoryCandidate{tagCandidate(m.ID)}
	s := &conversationMemoryService{complete: func(_ context.Context, _ models.AIConfig, _, _ string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: tagJSON(entries...)}, nil
	}}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	entries = []memoryCandidate{}
	if err := s.Refresh(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, err := s.View(c.ID)
	if err != nil || len(v.Entries) != 0 {
		t.Fatal("obsolete auto tag retained")
	}
}

func TestCustomerTagKeyCannotOverwriteOtherKinds(t *testing.T) {
	e := tagCandidate(1)
	e.Key = "manual-memory"
	entries := []memoryCandidate{e}
	normalizeCustomerTagKeys(entries, []models.ConversationMemoryEntry{{EntryKey: "manual-memory", Kind: "customer", Label: e.Label}})
	if entries[0].Key != "" {
		t.Fatal("cross-kind key retained")
	}
}

func TestCustomerTagMatchesHumanEditedValueUnderNewKey(t *testing.T) {
	e := tagCandidate(1)
	e.Key = "new-key"
	e.Label = "200 units, confirmed"
	e.Value = e.Label
	entries := []memoryCandidate{e}
	normalizeCustomerTagKeys(entries, []models.ConversationMemoryEntry{{EntryKey: "locked-tag", Kind: e.Kind, Topic: e.Topic, Label: "200 units", Value: e.Value, Confirmed: true}})
	if entries[0].Key != "locked-tag" {
		t.Fatal("human edited tag duplicated under new key")
	}
}

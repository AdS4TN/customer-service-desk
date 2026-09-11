package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/reception"
	"agent-desk/internal/repositories"
)

func testReceptionPolicy() reception.Policy {
	return reception.Policy{Enabled: true, Goal: "Collect purchase needs", Fields: []reception.Field{{Key: "quantity", Label: "Quantity", Required: true}, {Key: "region", Label: "Region"}}}
}

func TestReceptionPolicySavePublishAndRollback(t *testing.T) {
	db, c, _ := setupMemoryTest(t)
	if err := db.AutoMigrate(&models.AgentRevision{}, &models.AIAgentWorkflowBinding{}, &models.KnowledgeBase{}); err != nil {
		t.Fatal(err)
	}
	agent := AIAgentService.Get(c.AIAgentID)
	if err := db.Model(agent).Update("status", enums.StatusOk).Error; err != nil {
		t.Fatal(err)
	}
	p := testReceptionPolicy()
	operator := &dto.AuthPrincipal{UserID: 1, Username: "test"}
	req := request.UpdateAIAgentRequest{ID: agent.ID, CreateAIAgentRequest: request.CreateAIAgentRequest{Name: "Reception test", AIConfigID: agent.AIConfigID, ServiceMode: enums.IMConversationServiceModeAIFirst, HandoffMode: enums.AIAgentHandoffModeWaitPool, ReceptionPolicy: &p}}
	if err := AIAgentService.UpdateAIAgent(req, operator); err != nil {
		t.Fatal(err)
	}
	first, err := AIAgentService.PublishAIAgent(agent.ID, operator)
	if err != nil {
		t.Fatal(err)
	}
	p.Goal = "Unpublished changes"
	if err := AIAgentService.UpdateAIAgent(req, operator); err != nil {
		t.Fatal(err)
	}
	current := AIAgentService.Get(agent.ID)
	resolved := AgentRevisionService.ResolvePublishedAgent(*current)
	if reception.Decode(resolved.ReceptionPolicy).Goal != "Collect purchase needs" {
		t.Fatal("draft leaked into published policy")
	}
	second, err := AIAgentService.PublishAIAgent(agent.ID, operator)
	if err != nil || second.ID == first.ID {
		t.Fatalf("publish: %v", err)
	}
	if err := AIAgentService.RollbackAIAgent(agent.ID, first.ID, operator); err != nil {
		t.Fatal(err)
	}
	resolved = AgentRevisionService.ResolvePublishedAgent(*AIAgentService.Get(agent.ID))
	if reception.Decode(resolved.ReceptionPolicy).Goal != "Collect purchase needs" {
		t.Fatal("rollback did not restore policy")
	}
	req.ReceptionPolicy = nil
	if err := AIAgentService.UpdateAIAgent(req, operator); err != nil {
		t.Fatal(err)
	}
	if reception.Decode(AIAgentService.Get(agent.ID).ReceptionPolicy).Goal != "Unpublished changes" {
		t.Fatal("legacy client erased policy")
	}
	p.Fields = append(p.Fields, p.Fields[0])
	req.ReceptionPolicy = &p
	if err := AIAgentService.UpdateAIAgent(req, operator); err == nil {
		t.Fatal("duplicate schema accepted")
	}
}

func TestReceptionExtractionSourcesSchemaAndHistoricalRefresh(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	p := testReceptionPolicy()
	raw, _ := json.Marshal(p)
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("reception_policy", string(raw)).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	s := &conversationMemoryService{complete: func(_ context.Context, _ models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		calls++
		if !strings.Contains(system, `"key":"quantity"`) || !strings.Contains(input, "200 units") {
			t.Fatal("configured fields or messages missing")
		}
		return &ai.ChatCompletionResult{Content: fmt.Sprintf(`{"leads":[],"entries":[{"key":"qty","kind":"inquiry","fieldKey":"quantity","topic":"Purchase A","label":"invented label","value":"200 units","sourceIds":[%d]}]}`, m.ID)}, nil
	}}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, _ := s.View(c.ID)
	if len(v.Entries) != 1 || v.Entries[0].Entry.FieldKey != "quantity" || v.Entries[0].Entry.Label != "Quantity" || !v.ReceptionPolicy.Enabled {
		t.Fatal("structured inquiry not stored")
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("reprocessed unchanged messages")
	}
	if err := s.Refresh(c.ID); err != nil {
		t.Fatal(err)
	}
	job, _ := repositories.ConversationMemoryRepository.Get(db, c.ID)
	if job.ProcessedMessageID != 0 {
		t.Fatal("refresh did not rewind history")
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("explicit refresh did not re-extract")
	}
	if got, _ := s.View(c.ID); len(got.Entries) != 1 {
		t.Fatal("refresh duplicated inquiry")
	}
}

func TestReceptionExtractionRejectsUnknownAndCrossTopicFields(t *testing.T) {
	p := testReceptionPolicy()
	base := memoryCandidate{Key: "qty", Kind: "inquiry", FieldKey: "quantity", Topic: "A", Label: "Quantity", Value: "200"}
	for _, change := range []func(*memoryCandidate){func(c *memoryCandidate) { c.FieldKey = "unknown" }, func(c *memoryCandidate) { c.Topic = "" }, func(c *memoryCandidate) { c.Kind = "summary" }} {
		c := base
		change(&c)
		if err := normalizeInquiryFields([]memoryCandidate{c}, p, nil); err == nil {
			t.Fatal("invalid field accepted")
		}
	}
	if err := normalizeInquiryFields([]memoryCandidate{base, base}, p, nil); err == nil {
		t.Fatal("duplicate fields accepted")
	}
	other := base
	other.Topic = "B"
	if err := normalizeInquiryFields([]memoryCandidate{base, other}, p, nil); err != nil {
		t.Fatal("separate inquiries incorrectly merged")
	}
	old := []models.ConversationMemoryEntry{{EntryKey: "qty", Kind: "inquiry", FieldKey: "quantity", Topic: "A"}}
	if err := normalizeInquiryFields([]memoryCandidate{other}, p, old); err == nil {
		t.Fatal("existing entry reassigned to another inquiry")
	}
	sources := map[int64]models.Message{1: {ID: 1, SenderType: enums.IMSenderTypeAI}}
	if _, err := parseMemoryCandidates(`{"entries":[{"kind":"inquiry","fieldKey":"quantity","topic":"A","label":"Quantity","value":"200","sourceIds":[1]}]}`, sources); err == nil {
		t.Fatal("AI reply accepted as customer fact")
	}
}

func TestReceptionPolicyChangeDuringExtractionCannotCommit(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	s := fakeMemoryService(t, m.ID)
	complete := s.complete
	s.complete = func(ctx context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		raw, _ := json.Marshal(testReceptionPolicy())
		if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("reception_policy", string(raw)).Error; err != nil {
			t.Fatal(err)
		}
		return complete(ctx, cfg, system, input)
	}
	if err := s.process(claimMemory(t, c)); err == nil {
		t.Fatal("stale policy committed")
	}
	items, _ := repositories.ConversationMemoryRepository.Entries(db, c.ID)
	if len(items) != 0 {
		t.Fatal("stale entries persisted")
	}
}

func TestReceptionRetiredFieldAndHumanEditsSurviveRefresh(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	entry := models.ConversationMemoryEntry{ConversationID: c.ID, EntryKey: "old_quantity", Kind: "inquiry", FieldKey: "quantity", Topic: "Purchase A", Label: "Quantity", Value: "200 units", SourceMessageIDs: fmt.Sprintf("[%d]", m.ID), Revision: 1}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	s := &conversationMemoryService{complete: func(_ context.Context, _ models.AIConfig, _, _ string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: `{"entries":[],"leads":[]}`}, nil
	}}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, err := s.View(c.ID)
	if err != nil || len(v.Entries) != 1 || v.Entries[0].Entry.Value != "200 units" {
		t.Fatal("removing schema erased historical data")
	}
	if err := s.Update(c.ID, 1, request.UpdateConversationMemory{ID: entry.ID, Revision: entry.Revision, Value: "300 units, corrected"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, _ = s.View(c.ID)
	if len(v.Entries) != 1 || !v.Entries[0].Entry.Confirmed || v.Entries[0].Entry.Value != "300 units, corrected" {
		t.Fatal("refresh overwrote human correction")
	}
}

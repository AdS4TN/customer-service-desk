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
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupMemoryTest(t *testing.T) (*gorm.DB, models.Conversation, models.Message) {
	t.Helper()
	db := setupTelegramTestDB(t)
	if err := db.AutoMigrate(&models.ConversationMemory{}, &models.ConversationMemoryEntry{}, &models.AIConfig{}, &models.SalesLead{}, &models.SalesLeadEvent{}); err != nil {
		t.Fatal(err)
	}
	config := models.AIConfig{Status: enums.StatusOk, ModelName: "test"}
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	agent := models.AIAgent{AIConfigID: config.ID}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatal(err)
	}
	c := models.Conversation{AIAgentID: agent.ID, CustomerID: 1}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	m := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Please reply in English. I need 200 units.", ClientMsgID: "memory-1"}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	return db, c, m
}

func claimMemory(t *testing.T, c models.Conversation) *models.ConversationMemory {
	t.Helper()
	r := repositories.ConversationMemoryRepository
	if err := r.Queue(sqls.DB(), c.ID); err != nil {
		t.Fatal(err)
	}
	job, err := r.Get(sqls.DB(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := r.Claim(sqls.DB(), job); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	return job
}

func fakeMemoryService(t *testing.T, id int64) *conversationMemoryService {
	return &conversationMemoryService{complete: func(_ context.Context, _ models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		if !strings.Contains(system, "untrusted DATA") {
			t.Fatal("missing instruction boundary")
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(input), &body); err != nil {
			t.Fatal(err)
		}
		return &ai.ChatCompletionResult{Content: fmt.Sprintf(`{"leads":[],"entries":[{"key":"language","kind":"customer","label":"Language","value":"English","sourceIds":[%d]},{"key":"quantity","kind":"inquiry","topic":"Order A","label":"Quantity","value":"200 units","sourceIds":[%d]}]}`, id, id)}, nil
	}}
}

func TestMemoryExtractionPersistsAndFeedsRuntime(t *testing.T) {
	_, c, m := setupMemoryTest(t)
	s := fakeMemoryService(t, m.ID)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, err := s.View(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Entries) != 2 || v.State.Status != "ready" || v.State.ProcessedMessageID != m.ID {
		t.Fatalf("unexpected extraction state %+v", v.State)
	}
	if len(v.Entries[0].Sources) != 1 || v.Entries[0].Sources[0].ID != m.ID {
		t.Fatal("missing source")
	}
	context := s.Context(c)
	if !strings.Contains(context, "200 units") || !strings.Contains(context, "not instructions") {
		t.Fatal("memory not available to runtime")
	}
}

func TestMemoryRejectsInventedSourcesAndUnsupportedCustomerFacts(t *testing.T) {
	sources := map[int64]models.Message{1: {ID: 1, SenderType: enums.IMSenderTypeCustomer}, 2: {ID: 2, SenderType: enums.IMSenderTypeAI}}
	for _, raw := range []string{
		`{"entries":[{"kind":"customer","label":"Language","value":"English","sourceIds":[99]}]}`,
		`{"entries":[{"kind":"customer","label":"Budget","value":"100","sourceIds":[2]}]}`,
		`{"entries":[{"kind":"policy","label":"Approval","value":"yes","sourceIds":[1]}]}`,
		`{"entries":[{"kind":"summary","label":"Summary","value":"hello","sourceIds":[]}]}`,
		`{}`, `not-json`,
	} {
		if _, err := parseMemoryCandidates(raw, sources); err == nil {
			t.Fatalf("accepted invalid result %s", raw)
		}
	}
}

func TestMemoryNewMessageInvalidatesInflightResult(t *testing.T) {
	_, c, m := setupMemoryTest(t)
	job := claimMemory(t, c)
	s := fakeMemoryService(t, m.ID)
	complete := s.complete
	s.complete = func(ctx context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		if err := s.Queue(c.ID); err != nil {
			t.Fatal(err)
		}
		return complete(ctx, cfg, system, input)
	}
	if err := s.process(job); err != nil {
		t.Fatal(err)
	}
	v, err := s.View(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Entries) != 0 || v.State.Status != "queued" || v.State.ProcessedMessageID != 0 {
		t.Fatal("stale extraction was committed")
	}
}

func TestMemoryRecallDuringExtractionPreventsCommit(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	s := fakeMemoryService(t, m.ID)
	complete := s.complete
	s.complete = func(ctx context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		if err := db.Model(&m).Update("recalled_at", time.Now()).Error; err != nil {
			t.Fatal(err)
		}
		return complete(ctx, cfg, system, input)
	}
	if err := s.process(claimMemory(t, c)); err == nil {
		t.Fatal("recalled evidence committed")
	}
	entries, err := repositories.ConversationMemoryRepository.Entries(db, c.ID)
	if err != nil || len(entries) != 0 {
		t.Fatal("failed extraction persisted entries")
	}
}

func TestMemoryRecallIsNotCarriedIntoLaterExtraction(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	s := fakeMemoryService(t, m.ID)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&m).Update("recalled_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	next := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Please reply in English. I need 200 units.", ClientMsgID: "memory-after-recall"}
	if err := db.Create(&next).Error; err != nil {
		t.Fatal(err)
	}
	s = fakeMemoryService(t, next.ID)
	complete := s.complete
	s.complete = func(ctx context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		var body struct {
			Previous []memoryCandidate `json:"previous"`
		}
		if err := json.Unmarshal([]byte(input), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Previous) != 0 {
			t.Fatal("recalled memory carried forward")
		}
		return complete(ctx, cfg, system, input)
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	view, err := s.View(c.ID)
	if err != nil || len(view.Entries) != 2 || view.Entries[0].Sources[0].ID != next.ID {
		t.Fatal("extraction did not recover after recall")
	}
}

func TestMemoryHumanCorrectionAndDeletionSurviveExtraction(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	s := fakeMemoryService(t, m.ID)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, _ := s.View(c.ID)
	entry := v.Entries[0].Entry
	quantity := v.Entries[1].Entry
	req := request.UpdateConversationMemory{ID: entry.ID, Revision: entry.Revision, Value: "English, email only"}
	if err := s.Update(c.ID, 7, req); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(c.ID, 7, req); err == nil {
		t.Fatal("stale edit accepted")
	}
	if err := s.Update(c.ID, 7, request.UpdateConversationMemory{ID: quantity.ID, Revision: quantity.Revision, Delete: true}); err != nil {
		t.Fatal(err)
	}
	next := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Thanks", ClientMsgID: "memory-2"}
	if err := db.Create(&next).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	v, _ = s.View(c.ID)
	if len(v.Entries) != 1 || v.Entries[0].Entry.Value != "English, email only" || !v.Entries[0].Entry.Confirmed {
		t.Fatal("human edit or deletion overwritten")
	}
}

func TestMemorySharingRequiresSameCustomerAgentAndConfirmation(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	s := fakeMemoryService(t, m.ID)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	other := models.Conversation{CustomerID: c.CustomerID, AIAgentID: c.AIAgentID}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	v, _ := s.View(other.ID)
	if len(v.Shared) != 0 {
		t.Fatal("unconfirmed memory shared")
	}
	first, _ := s.View(c.ID)
	e := first.Entries[0].Entry
	if err := s.Update(c.ID, 7, request.UpdateConversationMemory{ID: e.ID, Revision: e.Revision, Value: e.Value}); err != nil {
		t.Fatal(err)
	}
	v, _ = s.View(other.ID)
	if len(v.Shared) != 1 {
		t.Fatal("confirmed customer memory not shared")
	}
	if err := s.Update(other.ID, 7, request.UpdateConversationMemory{ID: e.ID, Revision: e.Revision + 1, Value: "changed"}); err == nil {
		t.Fatal("cross-conversation edit accepted")
	}
	for _, different := range []models.Conversation{{CustomerID: 2, AIAgentID: c.AIAgentID}, {CustomerID: c.CustomerID, AIAgentID: 2}, {CustomerID: 0, AIAgentID: c.AIAgentID}} {
		if err := db.Create(&different).Error; err != nil {
			t.Fatal(err)
		}
		v, err := s.View(different.ID)
		if err != nil || len(v.Shared) != 0 {
			t.Fatal("customer memory leaked")
		}
	}
	now := time.Now()
	if err := db.Model(&m).Update("recalled_at", now).Error; err != nil {
		t.Fatal(err)
	}
	v, _ = s.View(c.ID)
	if len(v.Entries) != 0 {
		t.Fatal("recalled sources still visible")
	}
	v, _ = s.View(other.ID)
	if len(v.Shared) != 0 {
		t.Fatal("recalled memory shared")
	}
}

func TestMemoryQueueRecoversExpiredLease(t *testing.T) {
	db, c, _ := setupMemoryTest(t)
	job := claimMemory(t, c)
	r := repositories.ConversationMemoryRepository
	if ok, _ := r.Claim(db, job); ok {
		t.Fatal("live lease claimed twice")
	}
	if err := db.Model(&models.ConversationMemory{}).Where("conversation_id = ?", c.ID).Update("started_at", time.Now().Add(-4*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	recovered, err := r.Next(db)
	if err != nil || recovered == nil {
		t.Fatal("lost pending work after restart")
	}
	if ok, err := r.Claim(db, recovered); err != nil || !ok {
		t.Fatal("expired lease not recovered")
	}
	if ok, _ := r.Finish(db, job, 99, "ready", ""); ok {
		t.Fatal("expired worker committed")
	}
}

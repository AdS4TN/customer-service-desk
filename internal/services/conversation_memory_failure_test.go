package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/repositories"
	"gorm.io/gorm"
)

func dueMemory(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	if err := db.Model(&models.ConversationMemory{}).Where("conversation_id = ?", id).Updates(map[string]any{"updated_at": time.Now().Add(-time.Minute), "next_retry_at": time.Now().Add(-time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestMemoryRetriesInvalidOutputThenSavesOnce(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	r := repositories.ConversationMemoryRepository
	if err := r.Queue(db, c.ID); err != nil {
		t.Fatal(err)
	}
	s := fakeMemoryService(t, m.ID)
	complete := s.complete
	calls := 0
	s.complete = func(ctx context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		calls++
		if cfg.MaxRetryCount != 0 {
			t.Fatal("nested model retries enabled")
		}
		if calls == 1 {
			return &ai.ChatCompletionResult{Content: "not-json"}, nil
		}
		if !strings.Contains(system, "previous attempt failed memory validation") {
			t.Fatal("missing retry correction")
		}
		return complete(ctx, cfg, system, input)
	}
	dueMemory(t, db, c.ID)
	s.ProcessPending()
	state, _ := r.Get(db, c.ID)
	if state.Status != "queued" || state.AttemptCount != 1 || state.ErrorCode != "memory_validation_failed" || state.ProcessedMessageID != 0 || state.NextRetryAt == nil {
		t.Fatalf("bad retry state: %+v", state)
	}
	if delay := time.Until(*state.NextRetryAt); delay < 8*time.Second || delay > 11*time.Second {
		t.Fatal("wrong first backoff")
	}
	if next, _ := r.Next(db); next != nil {
		t.Fatal("backoff ignored")
	}
	if ok, _ := r.Claim(db, state); ok {
		t.Fatal("early claim accepted")
	}
	s.ProcessPending()
	if calls != 1 {
		t.Fatal("busy retry")
	}
	dueMemory(t, db, c.ID)
	s.ProcessPending()
	s.ProcessPending()
	state, _ = r.Get(db, c.ID)
	entries, _ := r.Entries(db, c.ID)
	if calls != 2 || state.Status != "ready" || state.AttemptCount != 2 || state.ErrorCode != "" || state.NextRetryAt != nil || state.ProcessedMessageID != m.ID || len(entries) != 2 {
		t.Fatalf("retry did not recover: %+v", state)
	}
}

func TestMemoryRetryBudgetSurvivesRestartAndManualRefreshResets(t *testing.T) {
	db, c, _ := setupMemoryTest(t)
	r := repositories.ConversationMemoryRepository
	r.Queue(db, c.ID)
	calls := 0
	for attempt := 1; attempt <= MemoryMaxAttempts; attempt++ {
		s := &conversationMemoryService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
			calls++
			return nil, errors.New("private-provider-error")
		}}
		dueMemory(t, db, c.ID)
		s.ProcessPending()
		state, _ := r.Get(db, c.ID)
		if state.AttemptCount != attempt || state.ErrorCode != "model_request_failed" {
			t.Fatalf("lost durable budget: %+v", state)
		}
		if attempt == 2 && (state.NextRetryAt == nil || time.Until(*state.NextRetryAt) < 28*time.Second) {
			t.Fatal("wrong second backoff")
		}
	}
	state, _ := r.Get(db, c.ID)
	if state.Status != "failed" || state.NextRetryAt != nil || calls != 3 {
		t.Fatalf("retry budget not bounded: %+v", state)
	}
	if next, _ := r.Next(db); next != nil {
		t.Fatal("exhausted job rescheduled")
	}
	if err := ConversationMemoryService.Refresh(c.ID); err != nil {
		t.Fatal(err)
	}
	state, _ = r.Get(db, c.ID)
	if state.Status != "queued" || state.AttemptCount != 0 || state.ErrorCode != "" || state.NextRetryAt != nil {
		t.Fatal("refresh did not reset budget")
	}
}

func TestMemoryFailureStages(t *testing.T) {
	for _, tc := range []struct {
		name, output, want string
		err                error
	}{
		{"request", "", "model_request_failed", errors.New("private-provider-error")},
		{"timeout", "", "model_timeout", fmt.Errorf("provider: %w", context.DeadlineExceeded)},
		{"memory", `{"entries":[{"kind":"invalid"}],"leads":[]}`, "memory_validation_failed", nil},
		{"inquiry", `{"entries":[{"key":"a","kind":"inquiry","topic":"a","label":"a","value":"a","fieldKey":"unknown","sourceIds":[1]}],"leads":[]}`, "inquiry_validation_failed", nil},
		{"leads", `{"entries":[],"leads":null}`, "lead_validation_failed", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c, _ := setupMemoryTest(t)
			s := &conversationMemoryService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
				return &ai.ChatCompletionResult{Content: tc.output}, tc.err
			}}
			err := s.process(claimMemory(t, c))
			if err == nil || memoryFailureCode(err) != tc.want || strings.Contains(err.Error(), "private-provider") {
				t.Fatalf("incorrect safe category: %v", err)
			}
		})
	}
}

func TestMemoryConfigDoesNotRetryAndStaleFailureDoesNotOverwrite(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	r := repositories.ConversationMemoryRepository
	job := claimMemory(t, c)
	if err := r.Queue(db, c.ID); err != nil {
		t.Fatal(err)
	}
	s := fakeMemoryService(t, m.ID)
	s.finishFailure(job, &memoryStageError{code: "model_request_failed", cause: errors.New("private")}, time.Second)
	state, _ := r.Get(db, c.ID)
	if state.AttemptCount != 0 || state.ErrorCode != "" || state.NextRetryAt != nil {
		t.Fatal("stale worker overwrote new work")
	}
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("ai_config_id", 9999).Error; err != nil {
		t.Fatal(err)
	}
	dueMemory(t, db, c.ID)
	s.ProcessPending()
	state, _ = r.Get(db, c.ID)
	if state.Status != "failed" || state.ErrorCode != "model_unavailable" || state.NextRetryAt != nil {
		t.Fatal("bad config retried")
	}
}

func TestMemoryFailurePreservesSavedEntriesAndLeads(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	s := fakeMemoryService(t, m.ID)
	if err := s.process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	lead := models.SalesLead{ConversationID: c.ID, LeadKey: "existing", CustomerID: c.CustomerID, AIAgentID: c.AIAgentID, Data: `{"title":"Existing order","product":"Product"}`, Revision: 7, Confirmed: true}
	if err := db.Create(&lead).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(c.ID); err != nil {
		t.Fatal(err)
	}
	s.complete = func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: `{"entries":[],"leads":null}`}, nil
	}
	dueMemory(t, db, c.ID)
	s.ProcessPending()
	entries, _ := repositories.ConversationMemoryRepository.Entries(db, c.ID)
	var saved models.SalesLead
	db.First(&saved, lead.ID)
	if len(entries) != 2 || saved.Revision != 7 || saved.Data != lead.Data || !saved.Confirmed {
		t.Fatal("failure changed saved data")
	}
}

func TestMemoryRepeatedInterruptedLeasesStopModelCalls(t *testing.T) {
	db, c, _ := setupMemoryTest(t)
	job := claimMemory(t, c)
	db.Model(job).Updates(map[string]any{"attempt_count": MemoryMaxAttempts, "started_at": time.Now().Add(-4 * time.Minute)})
	s := &conversationMemoryService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		t.Fatal("retry budget exceeded after crash")
		return nil, nil
	}}
	s.ProcessPending()
	state, _ := repositories.ConversationMemoryRepository.Get(db, c.ID)
	if state.Status != "failed" || state.ErrorCode != "worker_interrupted" {
		t.Fatal("interrupted job never terminates")
	}
}

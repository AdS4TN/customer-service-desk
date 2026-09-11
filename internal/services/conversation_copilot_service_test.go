package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/ai/rag"
	"agent-desk/internal/ai/runtime/retrievers"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/enums"
	"gorm.io/gorm"
)

func setupCopilotTest(t *testing.T) (*gorm.DB, models.Conversation, models.Message) {
	t.Helper()
	db, c, m := setupMemoryTest(t)
	if err := db.AutoMigrate(&models.AgentRevision{}, &models.AIAgentWorkflowBinding{}, &models.KnowledgeBase{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(testReceptionPolicy())
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Updates(map[string]any{"status": enums.StatusOk, "name": "Synthetic reception", "system_prompt": "PUBLISHED_POLICY", "reception_policy": string(raw)}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AIAgentService.PublishAIAgent(c.AIAgentID, &dto.AuthPrincipal{UserID: 1, Username: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&c).Updates(map[string]any{"status": enums.IMConversationStatusActive, "last_message_id": m.ID, "current_assignee_id": 1}).Error; err != nil {
		t.Fatal(err)
	}
	return db, c, m
}

func TestCopilotUsesPublishedPolicyAndMemoryWithoutDispatch(t *testing.T) {
	db, c, m := setupCopilotTest(t)
	if err := fakeMemoryService(t, m.ID).process(claimMemory(t, c)); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Updates(map[string]any{"system_prompt": "UNPUBLISHED_POLICY", "knowledge_ids": "999"})
	s := &conversationCopilotService{complete: func(_ context.Context, cfg models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
		for _, part := range []string{"PUBLISHED_POLICY", "No tools are available", "ask at most ONE", "Collect purchase needs"} {
			if !strings.Contains(system, part) {
				t.Fatalf("missing policy %s", part)
			}
		}
		if strings.Contains(system, "UNPUBLISHED_POLICY") || !strings.Contains(input, "200 units") || !strings.Contains(input, "AI-extracted, unconfirmed") {
			t.Fatal("snapshot or memory boundary")
		}
		if cfg.MaxRetryCount != 0 {
			t.Fatal("unexpected retry")
		}
		return &ai.ChatCompletionResult{Content: "We can help. Which region is this purchase for?"}, nil
	}}
	r, err := s.Suggest(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.KnowledgeStatus != "not_configured" || r.LastMessageID != m.ID {
		t.Fatalf("unexpected result: %+v", r)
	}
	var count int64
	db.Model(&models.Message{}).Where("conversation_id = ?", c.ID).Count(&count)
	after := ConversationService.Get(c.ID)
	if count != 1 || after.Status != c.Status || after.CurrentAssigneeID != c.CurrentAssigneeID {
		t.Fatal("suggestion dispatched or changed conversation")
	}
}

func TestCopilotRejectsConcurrentChangesAndReleasesLock(t *testing.T) {
	for _, kind := range []string{"message", "assignment", "closed", "recall", "customer", "agent_disabled"} {
		t.Run(kind, func(t *testing.T) {
			db, c, m := setupCopilotTest(t)
			s := &conversationCopilotService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
				switch kind {
				case "message":
					db.Model(&c).Update("last_message_id", m.ID+1)
				case "assignment":
					db.Model(&c).Update("current_assignee_id", 99)
				case "closed":
					db.Model(&c).Update("status", enums.IMConversationStatusClosed)
				case "customer":
					db.Model(&c).Update("customer_id", 99)
				case "recall":
					db.Model(&m).Update("recalled_at", time.Now())
				case "agent_disabled":
					db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("status", enums.StatusDisabled)
				}
				return &ai.ChatCompletionResult{Content: "stale"}, nil
			}}
			if _, err := s.Suggest(context.Background(), c.ID); err == nil {
				t.Fatal("stale suggestion accepted")
			}
			if _, busy := s.active.Load(c.ID); busy {
				t.Fatal("lock leaked")
			}
		})
	}
}

func TestCopilotKnowledgeFailureNeverGeneratesAndReturnsUsedSources(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	agent := AIAgentService.Get(c.AIAgentID)
	agent.KnowledgeIDs = "1"
	cfg := AIConfigService.Get(agent.AIConfigID)
	snapshot, _ := AgentRevisionService.ResolvePublishedSnapshot(*agent, *cfg)
	var definition map[string]any
	json.Unmarshal([]byte(snapshot.Revision.Definition), &definition)
	definition["agent"].(map[string]any)["knowledgeIds"] = "1"
	raw, _ := json.Marshal(definition)
	db.Model(&models.AgentRevision{}).Where("id = ?", agent.PublishedRevisionID).Update("definition", string(raw))
	calls := 0
	fail := true
	s := &conversationCopilotService{retrieve: func(_ context.Context, a models.AIAgent, q string) (*retrievers.KnowledgeRetrieveResult, error) {
		if a.KnowledgeIDs != "1" || !strings.Contains(q, "200 units") {
			t.Fatal("retrieval scope lost")
		}
		if fail {
			return nil, errors.New("offline")
		}
		return &retrievers.KnowledgeRetrieveResult{ContextText: "Synthetic product specification", ContextResults: []rag.RetrieveResult{{ChunkID: 10, DocumentID: 1, DocumentTitle: "Synthetic catalog", Content: "Synthetic product specification"}}}, nil
	}, complete: func(_ context.Context, _ models.AIConfig, _, input string) (*ai.ChatCompletionResult, error) {
		calls++
		if !strings.Contains(input, "Synthetic product specification") {
			t.Fatal("missing knowledge")
		}
		return &ai.ChatCompletionResult{Content: "Draft"}, nil
	}}
	if _, err := s.Suggest(context.Background(), c.ID); err == nil || calls != 0 {
		t.Fatal("retrieval failure generated draft")
	}
	fail = false
	result, err := s.Suggest(context.Background(), c.ID)
	if err != nil || calls != 1 || len(result.Sources) != 1 || result.Sources[0].ChunkID != 10 {
		t.Fatalf("used evidence missing: %v", err)
	}
}

func TestCopilotBusyAndProviderFailure(t *testing.T) {
	_, c, _ := setupCopilotTest(t)
	s := &conversationCopilotService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return nil, errors.New("provider failure")
	}}
	s.active.Store(c.ID, true)
	if _, err := s.Suggest(context.Background(), c.ID); err == nil {
		t.Fatal("duplicate request accepted")
	}
	s.active.Delete(c.ID)
	if _, err := s.Suggest(context.Background(), c.ID); err == nil {
		t.Fatal("provider error ignored")
	}
	if _, busy := s.active.Load(c.ID); busy {
		t.Fatal("lock leaked")
	}
}

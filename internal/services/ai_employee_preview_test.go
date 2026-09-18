package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-desk/internal/ai"
	"agent-desk/internal/ai/rag"
	"agent-desk/internal/ai/runtime/retrievers"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
)

func TestEmployeePreviewUsesUnsavedCapabilitiesWithoutWrites(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	if err := db.AutoMigrate(&models.SkillDefinition{}, &models.ChannelMessageOutbox{}); err != nil {
		t.Fatal(err)
	}
	base := models.KnowledgeBase{Name: "Synthetic docs", Status: enums.StatusOk}
	skill := models.SkillDefinition{Name: "Negotiation", Instruction: "ASK_ABOUT_VOLUME", Status: enums.StatusOk}
	db.Create(&base)
	db.Create(&skill)
	a := AIAgentService.Get(c.AIAgentID)
	s := &aiEmployeePreviewService{
		retrieve: func(_ context.Context, agent models.AIAgent, query string) (*retrievers.KnowledgeRetrieveResult, error) {
			if !strings.Contains(query, "English please") || agent.SystemPrompt != "DRAFT_ONLY" {
				t.Fatal("draft or history lost")
			}
			return &retrievers.KnowledgeRetrieveResult{ContextText: "OUR_FACT", ContextResults: []rag.RetrieveResult{{DocumentID: 9, DocumentTitle: "Synthetic docs", Content: "OUR_FACT"}}}, nil
		},
		complete: func(_ context.Context, _ models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
			for _, expected := range []string{"DRAFT_ONLY", "ASK_ABOUT_VOLUME", "Reply language policy", "No business actions"} {
				if !strings.Contains(system, expected) {
					t.Fatalf("missing %s", expected)
				}
			}
			if strings.Contains(system, "PUBLISHED_POLICY") || !strings.Contains(input, "OUR_FACT") {
				t.Fatal("wrong prompt or missing evidence")
			}
			return &ai.ChatCompletionResult{Content: "How many units do you need?"}, nil
		},
	}
	r, err := s.Preview(context.Background(), request.AIEmployeePreviewRequest{Draft: request.CreateAIAgentRequest{AIConfigID: a.AIConfigID, SystemPrompt: "DRAFT_ONLY", KnowledgeBaseIDs: []int64{base.ID}, SkillIDs: []int64{skill.ID}}, Messages: []request.AIEmployeePreviewTurn{{Role: "user", Content: "English please"}, {Role: "assistant", Content: "Certainly"}, {Role: "user", Content: "Any discount?"}}})
	if err != nil || len(r.Sources) != 1 || len(r.MountedSkills) != 1 || r.KnowledgeStatus != "matched" {
		t.Fatalf("result=%+v err=%v", r, err)
	}
	for _, check := range []struct {
		model any
		want  int64
	}{{&models.Message{}, 1}, {&models.ChannelMessageOutbox{}, 0}, {&models.Conversation{}, 1}} {
		var count int64
		db.Model(check.model).Count(&count)
		if count != check.want {
			t.Fatalf("unexpected write to %T: %d", check.model, count)
		}
	}
	if AIAgentService.Get(a.ID).SystemPrompt != a.SystemPrompt {
		t.Fatal("draft was saved")
	}
}

func TestEmployeePreviewRejectsInvalidAndUnavailableInputs(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	a := AIAgentService.Get(c.AIAgentID)
	s := &aiEmployeePreviewService{complete: func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return nil, errors.New("provider failure")
	}}
	valid := request.AIEmployeePreviewRequest{Draft: request.CreateAIAgentRequest{AIConfigID: a.AIConfigID}, Messages: []request.AIEmployeePreviewTurn{{Role: "user", Content: "Hello"}}}
	for _, role := range []string{"system", "tool", "assistant"} {
		r := valid
		r.Messages = []request.AIEmployeePreviewTurn{{Role: role, Content: "Hello"}}
		if _, err := s.Preview(context.Background(), r); err == nil {
			t.Fatal("invalid role accepted")
		}
	}
	if _, err := s.Preview(context.Background(), valid); err == nil {
		t.Fatal("provider error lost")
	}
	db.Model(&models.AIConfig{}).Where("id = ?", a.AIConfigID).Update("status", enums.StatusDisabled)
	if _, err := s.Preview(context.Background(), valid); err == nil {
		t.Fatal("disabled model accepted")
	}
}

func TestEmployeeSkillCompareChangesOnlySelectedSkill(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	if err := db.AutoMigrate(&models.SkillDefinition{}, &models.ChannelMessageOutbox{}); err != nil {
		t.Fatal(err)
	}
	skill := models.SkillDefinition{Name: "Objection handling", Instruction: "DIAGNOSE_THE_REAL_RISK", Status: enums.StatusDisabled}
	if err := db.Create(&skill).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	s := &aiEmployeePreviewService{
		retrieve: func(context.Context, models.AIAgent, string) (*retrievers.KnowledgeRetrieveResult, error) {
			return nil, nil
		},
		complete: func(_ context.Context, _ models.AIConfig, system, input string) (*ai.ChatCompletionResult, error) {
			calls++
			if !strings.Contains(input, "Your price is high") {
				t.Fatal("customer message missing")
			}
			mounted := strings.Contains(system, "DIAGNOSE_THE_REAL_RISK")
			if calls == 1 && mounted {
				t.Fatal("baseline contains selected Skill")
			}
			if calls == 2 && !mounted {
				t.Fatal("mounted variant is missing selected Skill")
			}
			if mounted {
				return &ai.ChatCompletionResult{Content: "What risk matters most?"}, nil
			}
			return &ai.ChatCompletionResult{Content: "We can offer a discount."}, nil
		},
	}
	result, err := s.CompareSkill(context.Background(), request.CompareSalesExperienceSkill{
		AIAgentID: c.AIAgentID, SkillDefinitionID: skill.ID,
		Messages: []request.AIEmployeePreviewTurn{{Role: "user", Content: "Your price is high"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.WithoutSkill.Content == result.WithSkill.Content || len(result.WithSkill.MountedSkills) != 1 || len(result.WithoutSkill.MountedSkills) != 0 {
		t.Fatalf("unexpected comparison: %+v", result)
	}
	var messageCount, outboxCount int64
	db.Model(&models.Message{}).Count(&messageCount)
	db.Model(&models.ChannelMessageOutbox{}).Count(&outboxCount)
	if messageCount != 1 || outboxCount != 0 {
		t.Fatalf("comparison wrote live data: messages=%d outbox=%d", messageCount, outboxCount)
	}
}

func TestEmployeeCapabilitiesSavePreservesLiveReception(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	a := AIAgentService.Get(c.AIAgentID)
	db.Model(a).Updates(map[string]any{"service_mode": enums.IMConversationServiceModeHumanOnly, "rollout_percent": 12})
	err := AIAgentService.UpdateAIAgent(request.UpdateAIAgentRequest{ID: a.ID, CapabilitiesOnly: true, CreateAIAgentRequest: request.CreateAIAgentRequest{Name: a.Name, AIConfigID: a.AIConfigID, SystemPrompt: "changed", ServiceMode: enums.IMConversationServiceModeAIOnly, RolloutPercent: 100, HandoffMode: enums.AIAgentHandoffModeWaitPool}}, &dto.AuthPrincipal{UserID: 1, Username: "test"})
	if err != nil {
		t.Fatal(err)
	}
	current := AIAgentService.Get(a.ID)
	if current.ServiceMode != enums.IMConversationServiceModeHumanOnly || current.RolloutPercent != 12 || current.SystemPrompt != "changed" {
		t.Fatal("capabilities update changed live reception")
	}
}

func TestEmployeeExplicitEmptyKnowledgeRemainsUnbound(t *testing.T) {
	db, _, _ := setupCopilotTest(t)
	base := models.KnowledgeBase{Name: "Default documentation", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk}
	if err := db.Create(&base).Error; err != nil {
		t.Fatal(err)
	}
	empty, err := AIAgentService.normalizeKnowledgeBaseIDs([]int64{})
	if err != nil || len(empty) != 0 {
		t.Fatalf("explicit empty selection rebound: %v, %v", empty, err)
	}
	legacy, err := AIAgentService.normalizeKnowledgeBaseIDs(nil)
	if err != nil || len(legacy) != 1 {
		t.Fatalf("omitted selection lost legacy default: %v, %v", legacy, err)
	}
}

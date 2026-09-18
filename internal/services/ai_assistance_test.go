package services

import (
	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/reception"
	"context"
	"testing"
)

func TestPausedReceptionKeepsPrivateAssistance(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Updates(map[string]any{"status": enums.StatusDisabled, "service_mode": enums.IMConversationServiceModeHumanOnly}).Error; err != nil {
		t.Fatal(err)
	}
	a := AIAgentService.Get(c.AIAgentID)
	snapshot, err := resolveAssistanceSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Agent.ServiceMode != enums.IMConversationServiceModeHumanOnly || reception.AutomaticMessagesAllowed(&c, &snapshot.Agent) {
		t.Fatal("published snapshot re-enabled reception")
	}
	complete := func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error) {
		return &ai.ChatCompletionResult{Content: translationTestJSON}, nil
	}
	translator := &conversationTranslationService{complete: complete}
	if _, err := translator.Text(context.Background(), c.ID, request.TranslateConversationText{Text: "Hello", TargetLanguage: enums.TranslationLanguageChinese}); err != nil {
		t.Fatal(err)
	}
	copilot := &conversationCopilotService{complete: complete}
	if _, err := copilot.Suggest(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&models.Message{}).Where("conversation_id = ?", c.ID).Count(&count)
	if count != 1 {
		t.Fatal("assistance dispatched a message")
	}
	if err := db.Model(&models.AIConfig{}).Where("id = ?", snapshot.AIConfig.ID).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAssistanceSnapshot(a); err == nil {
		t.Fatal("disabled model accepted")
	}
}

func TestPublishedIdentityKeepsLiveReceptionMode(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("service_mode", enums.IMConversationServiceModeHumanOnly).Error; err != nil {
		t.Fatal(err)
	}
	a := AgentRevisionService.ResolvePublishedAgent(*AIAgentService.Get(c.AIAgentID))
	if a.ServiceMode != enums.IMConversationServiceModeHumanOnly {
		t.Fatal("public snapshot restored old service mode")
	}
}

func TestAutomaticMessageChecksCurrentAgentAtCommit(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	if err := db.AutoMigrate(&models.ConversationDelegation{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&c).Updates(map[string]any{"status": enums.IMConversationStatusAIServing, "current_assignee_id": 0}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("service_mode", enums.IMConversationServiceModeHumanOnly).Error; err != nil {
		t.Fatal(err)
	}
	if m, err := MessageService.SendAIServiceNotice(c.ID, c.AIAgentID, "must not send"); err != nil || m != nil {
		t.Fatal("paused notice was not skipped")
	}
	if _, err := MessageService.SendAIMessage(c.ID, c.AIAgentID, "paused-reply", enums.IMMessageTypeText, "must not send", "", &dto.AuthPrincipal{Username: "AI"}); err == nil {
		t.Fatal("paused AI reply accepted")
	}
}

func TestDisabledReceptionStillCreatesIncomingConversation(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	a := createWelcomeTestAIAgent(t, db, "must not send")
	if err := db.Model(a).Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	c, err := ConversationService.Create(welcomeTestExternalUser("paused-incoming"), 11, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c.ServiceMode != enums.IMConversationServiceModeHumanOnly {
		t.Fatal("new conversation not routed to humans")
	}
	var count int64
	db.Model(&models.Message{}).Where("conversation_id = ?", c.ID).Count(&count)
	if count != 0 {
		t.Fatal("paused reception generated outgoing messages")
	}
}

func TestReceptionStateSeparatesReceivingAndSending(t *testing.T) {
	db, c, _ := setupCopilotTest(t)
	if err := db.Model(&models.AIAgent{}).Where("id = ?", c.AIAgentID).Update("service_mode", enums.IMConversationServiceModeHumanOnly).Error; err != nil {
		t.Fatal(err)
	}
	for _, channel := range []models.Channel{
		{Name: "live", ChannelID: "synthetic-live", AIAgentID: c.AIAgentID, ChannelType: enums.ChannelTypeWhatsApp, Status: enums.StatusOk},
		{Name: "disabled", ChannelID: "synthetic-disabled", AIAgentID: c.AIAgentID, ChannelType: enums.ChannelTypeWeb, Status: enums.StatusDisabled},
		{Name: "deleted", ChannelID: "synthetic-deleted", AIAgentID: c.AIAgentID, ChannelType: enums.ChannelTypeWeb, Status: enums.StatusDeleted},
	} {
		if err := db.Create(&channel).Error; err != nil {
			t.Fatal(err)
		}
	}
	r, err := AIAgentService.ReceptionState(c.AIAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if r.ReceptionEnabled || !r.AssistanceAvailable || len(r.Channels) != 2 {
		t.Fatal("incorrect runtime availability")
	}
	if !r.Channels[0].ReceptionEnabled || r.Channels[0].OutboundBlocked || r.Channels[0].AutomaticMessagesAllowed || r.Channels[1].ReceptionEnabled {
		t.Fatal("channel switches coupled")
	}
}

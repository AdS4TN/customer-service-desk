package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"testing"
)

func TestHumanOnlyConversationDoesNotSendWelcome(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	agent := createWelcomeTestAIAgent(t, db, "Synthetic welcome that must not be sent")
	if err := db.Model(agent).Update("service_mode", enums.IMConversationServiceModeHumanOnly).Error; err != nil {
		t.Fatal(err)
	}
	conversation, err := ConversationService.Create(welcomeTestExternalUser("human-only-no-welcome"), 11, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ?", conversation.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("human-only conversation created %d outgoing messages", count)
	}
}

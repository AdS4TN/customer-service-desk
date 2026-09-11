package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"encoding/json"
	"testing"
	"time"
)

func TestReceptionInquirySurvivesHumanHandoff(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	if err := db.AutoMigrate(&models.ConversationMemory{}, &models.ConversationMemoryEntry{}); err != nil {
		t.Fatal(err)
	}
	WsService = newWsService()
	agent := createHumanDispatchRealtimeAIAgent(t, db, "1")
	createHumanDispatchRealtimeTeam(t, db, 1)
	createHumanDispatchRealtimeActiveSchedule(t, db, 1)
	createHumanDispatchRealtimeAgentProfile(t, db, 101, 1)
	c := createHumanDispatchRealtimeConversation(t, db, agent.ID)
	m := models.Message{ConversationID: c.ID, SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "Order A: 200 units for Canada. Please connect me to a human.", ClientMsgID: "acceptance-handoff"}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	ids, _ := json.Marshal([]int64{m.ID})
	entry := models.ConversationMemoryEntry{ConversationID: c.ID, EntryKey: "purchase-a", Kind: "inquiry", Topic: "Order A", Label: "Quantity", Value: "200 units", SourceMessageIDs: string(ids), Confirmed: true, UpdatedAt: time.Now()}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}
	reason := "Customer requested a human; Order A is 200 units for Canada."
	result, err := ConversationHumanDispatchService.HandoffByAI(c.ID, agent, reason)
	if err != nil || result.Decision != HandoffDecisionAssigned {
		t.Fatalf("handoff failed: %v", err)
	}
	view, err := ConversationMemoryService.View(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Conversation.HandoffReason != reason || view.Conversation.CurrentAssigneeID != 101 || len(view.Entries) != 1 || !view.Entries[0].Entry.Confirmed || len(view.Entries[0].Sources) != 1 || view.Entries[0].Sources[0].ID != m.ID {
		t.Fatal("handoff lost inquiry, confirmation or original evidence")
	}
}

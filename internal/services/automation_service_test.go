package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"testing"
	"time"
)

func setupAutomationTest(t *testing.T) (*gorm.DB, models.User, models.Conversation) {
	t.Helper()
	db := setupMessageWelcomeTestDB(t)
	if err := db.AutoMigrate(&models.AutomationRule{}, &models.AutomationRun{}, &models.SalesLead{}, &models.SalesLeadEvent{}, &models.ConversationMemory{}, &models.User{}, &models.Role{}, &models.UserRole{}, &models.Permission{}, &models.RolePermission{}, &models.UserPermission{}, &models.Ticket{}, &models.TicketProgress{}, &models.TicketNoSequence{}, &models.Notification{}); err != nil {
		t.Fatal(err)
	}
	u := models.User{Username: "automation-admin", Status: enums.StatusOk}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	role := models.Role{Code: constants.RoleCodeSuperAdmin, Status: enums.StatusOk}
	db.Create(&role)
	db.Create(&models.UserRole{UserID: u.ID, RoleID: role.ID})
	channel := models.Channel{ChannelID: "automation-test", ChannelType: "web"}
	db.Create(&channel)
	c := models.Conversation{ChannelID: channel.ID, AIAgentID: 1, CustomerName: "Synthetic", Status: enums.IMConversationStatusAIServing}
	db.Create(&c)
	return db, u, c
}
func automationMessage(t *testing.T, db *gorm.DB, c models.Conversation, text string, historical bool) models.Message {
	t.Helper()
	m := models.Message{ConversationID: c.ID, ClientMsgID: fmt.Sprint(time.Now().UnixNano()), Content: text, SenderType: enums.IMSenderTypeCustomer, SendStatus: enums.IMMessageStatusSent, IsHistorical: historical}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	return m
}
func automationRule(t *testing.T, u models.User, d request.AutomationDefinition) *models.AutomationRule {
	t.Helper()
	row, err := AutomationService.Save(request.SaveAutomation{Name: "Synthetic rule", Priority: 100, Definition: d}, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Enabled {
		t.Fatal("new rule enabled")
	}
	if err := AutomationService.Change(request.ChangeAutomation{ID: row.ID, Revision: row.Revision, Enabled: true}, u.ID, false); err != nil {
		t.Fatal(err)
	}
	return row
}
func ticketAutomation() request.AutomationDefinition {
	return request.AutomationDefinition{Trigger: "message_received", Match: "any", OncePerConversation: true, Conditions: []request.AutomationCondition{{Field: "text", Value: "broken"}}, Actions: []request.AutomationAction{{Type: "create_ticket", Value: "Support request"}}}
}

func TestAutomationConditionsAndValidation(t *testing.T) {
	d := ticketAutomation()
	d.Conditions = append(d.Conditions, request.AutomationCondition{Field: "channel", Value: "whatsapp"})
	if hit, _ := matchAutomation(d, "web", "BROKEN item", nil); !hit {
		t.Fatal("case insensitive any did not match")
	}
	d.Match = "all"
	if hit, _ := matchAutomation(d, "web", "broken", nil); hit {
		t.Fatal("all matched wrong channel")
	}
	d.Conditions = nil
	if hit, _ := matchAutomation(d, "web", "", nil); !hit {
		t.Fatal("empty conditions must match")
	}
	d.OncePerConversation = false
	if validateAutomation(d) == nil {
		t.Fatal("repeat ticket accepted")
	}
	d = ticketAutomation()
	d.Actions[0].Type = "send_message"
	if validateAutomation(d) == nil {
		t.Fatal("outbound action accepted")
	}
	d = ticketAutomation()
	d.Conditions = []request.AutomationCondition{{Field: "lead_tag", Value: "quote"}}
	if validateAutomation(d) == nil {
		t.Fatal("lead condition accepted on message")
	}
}
func TestAutomationOnlyNewLiveMessagesAndDeduplication(t *testing.T) {
	db, u, c := setupAutomationTest(t)
	automationMessage(t, db, c, "broken before enable", false)
	rule := automationRule(t, u, ticketAutomation())
	automationMessage(t, db, c, "broken historical", true)
	AutomationService.ProcessPending()
	var count int64
	db.Model(&models.Ticket{}).Count(&count)
	if count != 0 {
		t.Fatal("old or historical message triggered")
	}
	m := automationMessage(t, db, c, "broken new", false)
	AutomationService.ProcessPending()
	AutomationService.ProcessPending()
	automationMessage(t, db, c, "broken again", false)
	AutomationService.ProcessPending()
	db.Model(&models.Ticket{}).Count(&count)
	if count != 1 {
		t.Fatalf("tickets %d", count)
	}
	runs, _, err := AutomationService.Runs(rule.ID, 1, 20)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" || runs[0].EventID != m.ID {
		t.Fatalf("runs %#v %v", runs, err)
	}
	if AutomationService.Retry(runs[0].ID, u.ID) == nil {
		t.Fatal("retried success")
	}
}
func TestAutomationFailureRollsBackAndRetryIsIdempotent(t *testing.T) {
	db, u, c := setupAutomationTest(t)
	d := ticketAutomation()
	d.Actions = append([]request.AutomationAction{{Type: "extract_lead"}}, d.Actions...)
	rule := automationRule(t, u, d)
	if err := db.Callback().Create().Before("gorm:create").Register("automation_test_fail", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "t_ticket" {
			tx.AddError(errors.New("synthetic failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	automationMessage(t, db, c, "broken product", false)
	AutomationService.ProcessPending()
	var n int64
	db.Model(&models.ConversationMemory{}).Count(&n)
	if n != 0 {
		t.Fatal("partial extraction queue committed")
	}
	runs, _, _ := AutomationService.Runs(rule.ID, 1, 20)
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("failure not recorded %#v", runs)
	}
	db.Callback().Create().Remove("automation_test_fail")
	if err := AutomationService.Retry(runs[0].ID, u.ID); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.Ticket{}).Count(&n)
	if n != 1 {
		t.Fatal("retry did not create exactly one ticket")
	}
	run, _ := repositories.AutomationRepository.Run(db, runs[0].ID)
	if run.Status != "completed" || run.Attempts != 2 {
		t.Fatal("retry audit incorrect")
	}
	if AutomationService.Retry(run.ID, u.ID) == nil {
		t.Fatal("duplicate retry accepted")
	}
}
func TestAutomationLeadTagsAssignmentAndPriority(t *testing.T) {
	db, u, c := setupAutomationTest(t)
	d := request.AutomationDefinition{Trigger: "lead_created", Match: "all", Actions: []request.AutomationAction{{Type: "tag_lead", Value: "Sales follow-up"}, {Type: "assign_lead", OwnerID: u.ID}}}
	automationRule(t, u, d)
	other := models.User{Username: "other-owner", Status: enums.StatusOk}
	db.Create(&other)
	d.Actions[1].OwnerID = other.ID
	automationRule(t, u, d)
	m := automationMessage(t, db, c, "Please quote 200 units", false)
	data := request.LeadData{Title: "Purchase", Product: "Units", AutoTags: []enums.LeadTag{enums.LeadTagQuote}}
	if err := syncSalesLeads(db, c, []leadCandidate{{Key: "order", Data: data, SourceIDs: []int64{m.ID}}}); err != nil {
		t.Fatal(err)
	}
	AutomationService.ProcessPending()
	AutomationService.ProcessPending()
	rows, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	if len(rows) != 1 || rows[0].OwnerID != u.ID || rows[0].Confirmed {
		t.Fatalf("owner or evidence modified %#v", rows)
	}
	var tags []string
	_ = json.Unmarshal([]byte(rows[0].CustomTags), &tags)
	if len(tags) != 1 {
		t.Fatal("duplicate tag")
	}
	var n int64
	db.Model(&models.Notification{}).Count(&n)
	if n != 1 {
		t.Fatalf("notifications %d", n)
	}
}
func TestAutomationRevokedPermissionAndDraftConflict(t *testing.T) {
	db, u, c := setupAutomationTest(t)
	row := automationRule(t, u, ticketAutomation())
	if err := AutomationService.Change(request.ChangeAutomation{ID: row.ID, Revision: row.Revision, Enabled: false}, u.ID, false); err == nil {
		t.Fatal("stale revision accepted")
	}
	db.Model(&models.User{}).Where("id = ?", u.ID).Update("status", enums.StatusDisabled)
	automationMessage(t, db, c, "broken product", false)
	AutomationService.ProcessPending()
	runs, _, _ := AutomationService.Runs(row.ID, 1, 20)
	if len(runs) != 1 || runs[0].ErrorCode != "permission" {
		t.Fatal("revoked permissions ignored")
	}
	var n int64
	db.Model(&models.Ticket{}).Count(&n)
	if n != 0 {
		t.Fatal("unauthorized write")
	}
}

func TestAutomationQueueDoesNotInvalidateExistingAnalysis(t *testing.T) {
	db, u, c := setupAutomationTest(t)
	d := request.AutomationDefinition{Trigger: "message_received", Match: "all", Actions: []request.AutomationAction{{Type: "extract_lead"}}}
	automationRule(t, u, d)
	m := automationMessage(t, db, c, "Please quote", false)
	memory := models.ConversationMemory{ConversationID: c.ID, Revision: 7, Status: "processing"}
	if err := db.Create(&memory).Error; err != nil {
		t.Fatal(err)
	}
	AutomationService.ProcessPending()
	after, err := repositories.ConversationMemoryRepository.Get(db, c.ID)
	if err != nil || after.Revision != 7 || after.Status != "processing" {
		t.Fatal("existing extraction invalidated")
	}
	if err := db.Model(&models.ConversationMemory{}).Where("conversation_id = ?", c.ID).Updates(map[string]any{"status": "ready", "processed_message_id": m.ID}).Error; err != nil {
		t.Fatal(err)
	}
	AutomationService.ProcessPending()
	after, _ = repositories.ConversationMemoryRepository.Get(db, c.ID)
	if after.Status != "ready" {
		t.Fatal("completed extraction replayed")
	}
}

func TestAutomationPauseDeleteAndRecalledMessage(t *testing.T) {
	db, u, c := setupAutomationTest(t)
	rule := automationRule(t, u, ticketAutomation())
	m := automationMessage(t, db, c, "broken", false)
	db.Model(&models.Message{}).Where("id = ?", m.ID).Update("send_status", enums.IMMessageStatusRecalled)
	AutomationService.ProcessPending()
	row, _ := repositories.AutomationRepository.Get(db, rule.ID)
	if err := AutomationService.Change(request.ChangeAutomation{ID: row.ID, Revision: row.Revision}, u.ID, false); err != nil {
		t.Fatal(err)
	}
	automationMessage(t, db, c, "broken while paused", false)
	row, _ = repositories.AutomationRepository.Get(db, rule.ID)
	if err := AutomationService.Change(request.ChangeAutomation{ID: row.ID, Revision: row.Revision, Enabled: true}, u.ID, false); err != nil {
		t.Fatal(err)
	}
	AutomationService.ProcessPending()
	var count int64
	db.Model(&models.Ticket{}).Count(&count)
	if count != 0 {
		t.Fatal("pause history or recalled content triggered")
	}
	automationMessage(t, db, c, "broken after enable", false)
	AutomationService.ProcessPending()
	row, _ = repositories.AutomationRepository.Get(db, rule.ID)
	if err := AutomationService.Change(request.ChangeAutomation{ID: row.ID, Revision: row.Revision}, u.ID, true); err != nil {
		t.Fatal(err)
	}
	rules, _ := AutomationService.List()
	if len(rules) != 0 {
		t.Fatal("deleted rule visible")
	}
	runs, _, _ := AutomationService.Runs(rule.ID, 1, 20)
	if len(runs) != 1 {
		t.Fatal("delete lost audit")
	}
}

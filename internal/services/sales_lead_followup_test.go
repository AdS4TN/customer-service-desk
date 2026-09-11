package services

import (
	"encoding/json"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"gorm.io/gorm"
)

func setupLeadOwner(t *testing.T, db *gorm.DB) models.User {
	t.Helper()
	if err := db.AutoMigrate(&models.User{}, &models.Notification{}); err != nil {
		t.Fatal(err)
	}
	u := models.User{Username: "lead-owner", Status: enums.StatusOk}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	return u
}

func TestLeadFollowUpLifecycle(t *testing.T) {
	db, c, m := setupMemoryTest(t)
	u := setupLeadOwner(t, db)
	data := request.LeadData{Title: "Test purchase", Product: "Test product"}
	if err := syncSalesLeads(db, c, []leadCandidate{{Key: "test", Data: data, SourceIDs: []int64{m.ID}}}); err != nil {
		t.Fatal(err)
	}
	rows, _ := repositories.SalesLeadRepository.ForConversation(db, c.ID)
	id := rows[0].ID
	get := func() *models.SalesLead {
		row, err := repositories.SalesLeadRepository.Get(db, id)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	due := time.Now().Add(time.Hour)
	req := request.FollowUpSalesLead{Revision: 1, OwnerID: u.ID, Status: enums.LeadStatusFollowing, Result: "Confirmed requirements", NextAction: "Prepare quote", FollowUpAt: &due}
	for _, kind := range []string{"owner", "result", "next", "date", "status", "revision"} {
		bad := req
		switch kind {
		case "owner":
			bad.OwnerID = 999
		case "result":
			bad.Result = " "
		case "next":
			bad.NextAction = ""
		case "date":
			bad.FollowUpAt = nil
		case "status":
			bad.Status = "invalid"
		case "revision":
			bad.Revision = 999
		}
		if err := SalesLeadService.FollowUp(id, u.ID, bad); err == nil {
			t.Fatalf("accepted invalid %s", kind)
		}
	}
	if err := SalesLeadService.FollowUp(id, u.ID, req); err != nil {
		t.Fatal(err)
	}
	if err := SalesLeadService.FollowUp(id, u.ID, req); err == nil {
		t.Fatal("duplicate accepted")
	}
	row := get()
	if row.NextAction != req.NextAction || row.LastFollowUpAt == nil || row.Confirmed || row.Status != "following" {
		t.Fatal("follow-up state or evidence confirmation incorrect")
	}
	var notices []models.Notification
	db.Find(&notices)
	if len(notices) != 1 || notices[0].NotificationType != "lead_assigned" || notices[0].RecipientUserID != u.ID {
		t.Fatal("assignment notice missing")
	}
	for range 2 {
		if err := SalesLeadService.processLeadDue(id, due.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	db.Find(&notices)
	if len(notices) != 2 {
		t.Fatal("due reminder not deduplicated")
	}
	// An AI refresh must not reset the follow-up schedule or its notification claim.
	data.Quantity = "100"
	if err := syncSalesLeads(db, c, []leadCandidate{{Key: "test", Data: data, SourceIDs: []int64{m.ID}}}); err != nil {
		t.Fatal(err)
	}
	if err := SalesLeadService.processLeadDue(id, due.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	db.Find(&notices)
	if len(notices) != 2 || get().NextAction != req.NextAction {
		t.Fatal("AI reset sales handling")
	}
	// A newly scheduled follow-up gets exactly one new reminder.
	req.Revision = get().Revision
	if err := SalesLeadService.FollowUp(id, u.ID, req); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := SalesLeadService.processLeadDue(id, due.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	db.Find(&notices)
	if len(notices) != 3 {
		t.Fatal("rescheduled reminder missing or duplicated")
	}
	row = get()
	if err := SalesLeadService.Update(id, u.ID, request.UpdateSalesLead{Revision: row.Revision, Data: data, Status: enums.LeadStatusWon, OwnerID: u.ID, FollowUpAt: row.FollowUpAt}); err == nil {
		t.Fatal("profile endpoint bypassed follow-up")
	}
	for _, status := range []enums.LeadStatus{enums.LeadStatusWon, enums.LeadStatusFollowing, enums.LeadStatusLost, enums.LeadStatusFollowing, enums.LeadStatusArchived} {
		req.Revision = get().Revision
		req.Status = status
		if err := SalesLeadService.FollowUp(id, u.ID, req); err != nil {
			t.Fatal(err)
		}
		if leadClosed(string(status)) {
			if get().FollowUpAt != nil || get().NextAction != "" {
				t.Fatal("closed lead kept schedule")
			}
			before := len(notices)
			if err := SalesLeadService.processLeadDue(id, due.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			db.Find(&notices)
			if len(notices) != before {
				t.Fatal("closed lead notified")
			}
		}
	}
	events, _ := repositories.SalesLeadRepository.Events(db, id)
	var event request.FollowUpSalesLead
	if events[0].Kind != "follow_up" || json.Unmarshal([]byte(events[0].Data), &event) != nil || event.Result != req.Result {
		t.Fatal("audit result missing")
	}
}

func TestLeadReminderRollbackAndQueues(t *testing.T) {
	db, c, _ := setupMemoryTest(t)
	u := setupLeadOwner(t, db)
	past, future := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	rows := []models.SalesLead{
		{ConversationID: c.ID, LeadKey: "unassigned", Status: "new"},
		{ConversationID: c.ID, LeadKey: "unscheduled", Status: "following", OwnerID: u.ID},
		{ConversationID: c.ID, LeadKey: "due", Status: "following", OwnerID: u.ID, FollowUpAt: &past, NextAction: "Call"},
		{ConversationID: c.ID, LeadKey: "future", Status: "qualified", OwnerID: u.ID, FollowUpAt: &future, NextAction: "Quote"},
		{ConversationID: c.ID, LeadKey: "closed", Status: "won", OwnerID: u.ID, FollowUpAt: &past},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, queue := range []string{"unassigned", "unscheduled", "overdue", "upcoming"} {
		_, total, err := repositories.SalesLeadRepository.List(db, 0, 0, "", "", "", queue, 1, 20)
		if err != nil || total != 1 {
			t.Fatalf("queue %s total %d err %v", queue, total, err)
		}
	}
	_, total, err := repositories.SalesLeadRepository.List(db, 0, u.ID, "", "", "", "overdue", 1, 20)
	if err != nil || total != 1 {
		t.Fatal("owner queue filter failed")
	}
	// Failed notification insertion rolls back the claim, allowing retry.
	if err := db.Migrator().DropTable(&models.Notification{}); err != nil {
		t.Fatal(err)
	}
	if err := SalesLeadService.processLeadDue(rows[2].ID, time.Now()); err == nil {
		t.Fatal("missing notification table ignored")
	}
	row, _ := repositories.SalesLeadRepository.Get(db, rows[2].ID)
	if row.NotifiedRevision == row.ScheduleRevision {
		t.Fatal("failed notification consumed claim")
	}
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		t.Fatal(err)
	}
	if err := SalesLeadService.processLeadDue(row.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestLeadFollowUpNormalizesBrowserTimezone(t *testing.T) {
	now := time.Now()
	at := now.Add(time.Hour).UTC()
	req := request.FollowUpSalesLead{Revision: 1, OwnerID: 1, Status: enums.LeadStatusFollowing, Result: "Quote sent", NextAction: "Check response", FollowUpAt: &at}
	if err := validateLeadFollowUp(&req, now); err != nil {
		t.Fatal(err)
	}
	if !req.FollowUpAt.Equal(at) || req.FollowUpAt.Location() != time.Local {
		t.Fatal("schedule instant changed or storage timezone not normalized")
	}
}

func TestLeadFollowUpAssignmentRollsBackAndUTCDeadlineIsDue(t *testing.T) {
	db, c, _ := setupMemoryTest(t)
	u := setupLeadOwner(t, db)
	row := models.SalesLead{ConversationID: c.ID, LeadKey: "rollback", Status: "new"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(time.Minute).UTC()
	req := request.FollowUpSalesLead{Revision: row.Revision, OwnerID: u.ID, Status: enums.LeadStatusFollowing, Result: "Contacted", NextAction: "Quote", FollowUpAt: &at}
	if err := db.Migrator().DropTable(&models.Notification{}); err != nil {
		t.Fatal(err)
	}
	if err := SalesLeadService.FollowUp(row.ID, u.ID, req); err == nil {
		t.Fatal("assignment notification failure ignored")
	}
	current, _ := repositories.SalesLeadRepository.Get(db, row.ID)
	events, _ := repositories.SalesLeadRepository.Events(db, row.ID)
	if current.Revision != row.Revision || current.OwnerID != 0 || len(events) != 0 {
		t.Fatal("partial follow-up committed")
	}
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		t.Fatal(err)
	}
	if err := SalesLeadService.FollowUp(row.ID, u.ID, req); err != nil {
		t.Fatal(err)
	}
	due, err := repositories.SalesLeadRepository.Due(db, time.Now().Add(2*time.Minute))
	if err != nil || len(due) != 1 || due[0].ID != row.ID {
		t.Fatal("UTC browser schedule was not due at its actual instant")
	}
}

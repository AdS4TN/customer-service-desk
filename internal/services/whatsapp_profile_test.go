package services

import (
	"context"
	"reflect"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/whatsapp"
)

func TestContactProfileUpdatesOwnershipAndAvatarStates(t *testing.T) {
	customer := &models.Customer{Name: "Local name", Remark: "Local note", ManualTags: `["VIP"]`, AvatarAssetID: 9, AvatarHash: "old", AuditFields: models.AuditFields{UpdateUserID: 1}}
	for _, state := range []string{"failed", "hidden", "empty", "ready"} {
		updates := contactProfileUpdates(customer, whatsapp.ContactProfile{Name: "Channel name", AvatarState: state, Avatar: []byte{1}}, 10, "new")
		for _, field := range []string{"name", "remark", "manual_tags", "last_active_at", "update_user_id"} {
			if _, exists := updates[field]; exists {
				t.Fatalf("sync overwrote %s", field)
			}
		}
		if updates["channel_name"] != "Channel name" {
			t.Fatal("channel name missing")
		}
		if state == "failed" {
			if _, exists := updates["avatar_asset_id"]; exists {
				t.Fatal("transient failure erased avatar")
			}
		}
		if state == "hidden" || state == "empty" {
			if updates["avatar_asset_id"] != int64(0) {
				t.Fatal("unavailable avatar not cleared")
			}
		}
		if state == "ready" && updates["avatar_asset_id"] != int64(10) {
			t.Fatal("avatar not replaced")
		}
	}
	customer.UpdateUserID = 0
	if contactProfileUpdates(customer, whatsapp.ContactProfile{Name: "New name"}, 0, "")["name"] != "New name" {
		t.Fatal("unowned name not updated")
	}
	customer.AIProfileProjection = `{"name":"Confirmed"}`
	if _, exists := contactProfileUpdates(customer, whatsapp.ContactProfile{Name: "New name"}, 0, "")["name"]; exists {
		t.Fatal("AI name overwritten")
	}
	if len(contactProfileUpdates(customer, whatsapp.ContactProfile{}, 0, "")) != 0 {
		t.Fatal("empty profile mutated data")
	}
}

func TestWhatsAppProfileAliasSyncDoesNotChangeReception(t *testing.T) {
	db := setupTelegramTestDB(t)
	channel := &models.Channel{ChannelType: enums.ChannelTypeWhatsApp, Status: enums.StatusOk}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Hour).Truncate(time.Second)
	customer := &models.Customer{Name: "Old", Remark: "Note", ManualTags: `["VIP"]`, Status: enums.StatusOk, LastActiveAt: &now}
	if err := db.Create(customer).Error; err != nil {
		t.Fatal(err)
	}
	identity := &models.CustomerIdentity{CustomerID: customer.ID, ExternalSource: enums.ExternalSourceWhatsApp, ExternalID: whatsAppIdentity(channel.ID, "111@s.whatsapp.net", "222@lid"), Status: enums.StatusOk}
	if err := db.Create(identity).Error; err != nil {
		t.Fatal(err)
	}
	conversation := &models.Conversation{CustomerID: customer.ID, ChannelID: channel.ID, CustomerName: "Old", Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7, AgentUnreadCount: 4, WorkRevision: 8, LastMessageID: 99, LastActiveAt: now}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	fake := &fakeWhatsApp{state: whatsapp.Status{State: "connected", Account: "111@s.whatsapp.net"}}
	service := &linkedChatService{sessions: map[int64]whatsAppSession{channel.ID: fake}}
	profile := whatsapp.ContactProfile{Account: fake.state.Account, Chat: "222@s.whatsapp.net", Aliases: []string{"222@lid"}, Name: "Synced", AvatarState: "empty"}
	if err := service.saveContactProfile(channel.ID, profile); err != nil {
		t.Fatal(err)
	}
	updated := ConversationService.Get(conversation.ID)
	if updated.CustomerName != "Synced" || updated.AgentUnreadCount != 4 || updated.WorkRevision != 8 || updated.CurrentAssigneeID != 7 || updated.LastMessageID != 99 || !updated.LastActiveAt.Equal(now) {
		t.Fatal("sync changed reception state or missed name")
	}
	c := CustomerService.Get(customer.ID)
	if c.Remark != "Note" || c.ManualTags != `["VIP"]` || !c.LastActiveAt.Equal(now) {
		t.Fatal("customer local data changed")
	}
	for _, table := range []string{"t_message", "t_channel_message_outbox"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("sync created messages/outbox: %s", table)
		}
	}
	profile.Account = "other@s.whatsapp.net"
	if service.saveContactProfile(channel.ID, profile) == nil {
		t.Fatal("wrong account accepted")
	}
	if _, err := service.SyncContact(context.Background(), conversation.ID); err == nil {
		t.Fatal("unsupported session accepted")
	}
}

func TestCustomerManualTagsSavePreserveAndClear(t *testing.T) {
	db := setupTelegramTestDB(t)
	customer := &models.Customer{Name: "Customer", ManualTags: `["Old"]`, Status: enums.StatusOk}
	if err := db.Create(customer).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{UserID: 1, Username: "operator"}
	req := request.SaveCustomerProfileRequest{ID: &customer.ID, Name: customer.Name, Remark: "Local note"}
	if _, err := CustomerService.SaveCustomerProfile(req, operator); err != nil {
		t.Fatal(err)
	}
	if CustomerService.Get(customer.ID).ManualTags != `["Old"]` {
		t.Fatal("omitted tags not preserved")
	}
	tags := []string{" VIP ", "vip", "", "Needs quote"}
	req.ManualTags = &tags
	if _, err := CustomerService.SaveCustomerProfile(req, operator); err != nil {
		t.Fatal(err)
	}
	if CustomerService.Get(customer.ID).ManualTags != `["VIP","Needs quote"]` {
		t.Fatal("tags not normalized")
	}
	tags = []string{}
	if _, err := CustomerService.SaveCustomerProfile(req, operator); err != nil {
		t.Fatal(err)
	}
	if CustomerService.Get(customer.ID).ManualTags != `[]` {
		t.Fatal("explicit clear failed")
	}
	if !reflect.DeepEqual(normalizeCustomerTags([]string{" A", "a", " B "}), []string{"A", "B"}) {
		t.Fatal("normalization mismatch")
	}
}

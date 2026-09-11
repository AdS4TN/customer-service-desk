package whatsapp

import (
	"context"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"testing"
	"time"
)

func TestHistoryProtocolReferences(t *testing.T) {
	s, err := Open(t.TempDir(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	device, err := s.store.GetFirstDevice(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	device.ID = &types.JID{User: "111", Device: 1, Server: types.DefaultUserServer}
	device.Account = &waAdv.ADVSignedDeviceIdentity{Details: []byte{}, AccountSignature: make([]byte, 64), AccountSignatureKey: make([]byte, 32), DeviceSignature: make([]byte, 64)}
	if err := device.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	device.SetAllStores(sqlstore.NewSQLStore(s.store, *device.ID))
	ctx := context.Background()
	if err := device.MsgSecrets.PutMessageSecrets(ctx, []store.MessageSecretInsert{
		{Chat: types.NewJID("222", types.HiddenUserServer), Sender: types.NewJID("222", types.HiddenUserServer), ID: "missing", Secret: []byte("test-only")},
		{Chat: types.NewJID("333", types.HiddenUserServer), Sender: types.NewJID("333", types.HiddenUserServer), ID: "other-chat", Secret: []byte("test-only")},
	}); err != nil {
		t.Fatal(err)
	}
	refs, err := s.historyReferences(ctx, device.ID.String(), []string{"222@lid"})
	if err != nil || len(refs) != 1 || refs[0].id != "missing" {
		t.Fatalf("wrong reference selection: %v %v", refs, err)
	}
	refs, err = s.historyReferences(ctx, "another-account", []string{"222@lid"})
	if err != nil || len(refs) != 0 {
		t.Fatal("protocol references leaked across accounts")
	}
}

func TestHistoryRequestStatus(t *testing.T) {
	s := &Session{status: Status{State: "connected"}, history: historySync{
		HistoryStatus: HistoryStatus{ConversationID: 13, State: "waiting", RequestedAt: time.Now()},
		id:            "request-1", chat: "222@lid", updated: time.Now(),
	}}
	s.historyBatch("other", []string{"222@lid"}, 3, 0, true)
	if s.HistoryStatus().State != "waiting" {
		t.Fatal("unrelated response completed request")
	}
	s.historyBatch("", []string{"333@lid"}, 3, 0, true)
	if s.HistoryStatus().State != "waiting" {
		t.Fatal("unrelated chat completed request")
	}
	s.historyBatch("request-1", []string{"222@lid"}, 2, 0, false)
	if s.HistoryStatus().State != "receiving" {
		t.Fatal("chunk was not tracked")
	}
	s.historyBatch("request-1", []string{"222@lid"}, 1, 1, true)
	status := s.HistoryStatus()
	if status.State != "partial" || status.Processed != 3 || status.Failed != 1 {
		t.Fatalf("wrong status: %+v", status)
	}
	s.history.State, s.history.updated = "waiting", time.Now().Add(-3*time.Minute)
	if s.HistoryStatus().State != "timeout" {
		t.Fatal("request never timed out")
	}
	s.history.State, s.history.updated = "waiting", time.Now()
	s.status.State = "disconnected"
	if s.HistoryStatus().State != "disconnected" {
		t.Fatal("disconnect falsely completed request")
	}
}

func TestHistoryEmptyAndCancelled(t *testing.T) {
	s := &Session{history: historySync{id: "request-2", chat: "222@lid"}}
	s.historyBatch("request-2", nil, 0, 0, true)
	if s.HistoryStatus().State != "empty" {
		t.Fatal("empty result claimed success")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitHistory(ctx, time.Hour) {
		t.Fatal("cancelled session kept waiting")
	}
	if err := s.RequestHistory(ctx, HistoryRequest{ConversationID: 1}); err == nil {
		t.Fatal("offline request accepted")
	}
}

func TestHistoryReconnectCooldown(t *testing.T) {
	called := false
	s := &Session{lastHistoryRun: time.Now(), historyTargets: func(string) ([]HistoryRequest, error) { called = true; return nil, nil }}
	s.syncHistoryAutomatically(context.Background(), "111@s.whatsapp.net")
	if called {
		t.Fatal("reconnect storm started another history pass")
	}
}

package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestParseIncoming(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*events.Message)
		want   bool
	}{
		{"text", func(e *events.Message) {}, true},
		{"extended", func(e *events.Message) {
			e.Message = &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hello")}}
		}, true},
		{"lid", func(e *events.Message) { e.Info.Chat.Server = types.HiddenUserServer }, true},
		{"own", func(e *events.Message) { e.Info.IsFromMe = true }, true},
		{"group", func(e *events.Message) { e.Info.IsGroup = true }, false},
		{"history", func(e *events.Message) { e.SourceWebMsg = &waWeb.WebMessageInfo{} }, true},
		{"recovered", func(e *events.Message) { e.UnavailableRequestID = "request" }, true},
		{"status", func(e *events.Message) { e.Info.Chat = types.StatusBroadcastJID }, false},
		{"edit", func(e *events.Message) { e.IsEdit = true }, false},
		{"viewonce", func(e *events.Message) { e.IsViewOnce = true }, true},
		{"image", func(e *events.Message) { e.Message = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}} }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evt := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("123", types.DefaultUserServer)}, ID: "message-id"}, Message: &waE2E.Message{Conversation: proto.String("hello")}}
			tc.mutate(evt)
			msg, ok := ParseIncoming(evt, "owner@s.whatsapp.net")
			if ok != tc.want {
				t.Fatalf("accepted=%v want=%v", ok, tc.want)
			}
			if ok && msg.Message == nil && msg.Text != "hello" {
				t.Fatalf("unexpected text %q", msg.Text)
			}
			if ok && (msg.FromMe != evt.Info.IsFromMe || msg.History != (evt.SourceWebMsg != nil || evt.UnavailableRequestID != "")) {
				t.Fatal("history or direction flag lost")
			}
		})
	}
}

func TestWhatsAppHistoryEvents(t *testing.T) {
	var messages []Incoming
	s, err := Open(t.TempDir(), 1, func(msg Incoming) error { messages = append(messages, msg); return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	device, err := s.store.GetFirstDevice(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	device.ID = &types.JID{User: "111", Server: types.DefaultUserServer}
	device.SetAllStores(sqlstore.NewSQLStore(s.store, *device.ID))
	device.LIDs = s.store.LIDMap
	client := whatsmeow.NewClient(device, nil)
	makeMessage := func(id string, own bool) *waHistorySync.HistorySyncMsg {
		return &waHistorySync.HistorySyncMsg{Message: &waWeb.WebMessageInfo{
			Key:              &waCommon.MessageKey{ID: proto.String(id), FromMe: proto.Bool(own)},
			MessageTimestamp: proto.Uint64(1700000000), PushName: proto.String("Not the contact name"),
			Message: &waE2E.Message{Conversation: proto.String("test history")},
		}}
	}
	s.receiveHistory(context.Background(), client, &waHistorySync.HistorySync{Conversations: []*waHistorySync.Conversation{
		{ID: proto.String("222@s.whatsapp.net"), Name: proto.String("Saved customer"), Messages: []*waHistorySync.HistorySyncMsg{makeMessage("in", false), makeMessage("out", true)}},
		{ID: proto.String("group@g.us"), Messages: []*waHistorySync.HistorySyncMsg{makeMessage("group", false)}},
	}}, nil)
	if len(messages) != 2 {
		t.Fatalf("got %d messages", len(messages))
	}
	for i, msg := range messages {
		if !msg.History || msg.FromMe != (i == 1) || msg.Name != "Saved customer" || msg.SentAt.Unix() != 1700000000 {
			t.Fatalf("incorrect import flags: %+v", msg)
		}
	}
}

func TestSessionStoreIsolationAndExpiredQR(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []int64{1, 2} {
		s, err := Open(dir, id, nil)
		if err != nil {
			t.Fatal(err)
		}
		device, err := s.store.GetFirstDevice(context.Background())
		if err != nil || device.ID != nil {
			t.Fatalf("new device: %v", err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "channel-1.db"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("store permissions: %v", err)
	}
	s, err := Open(dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	past := time.Now().Add(-time.Second)
	s.status = Status{State: "qr", QR: "sensitive", QRExpiresAt: &past}
	if s.Status().QR != "" {
		t.Fatal("expired QR must be hidden")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.set(ctx, Status{State: "connected"})
	if s.Status().State != "qr" {
		t.Fatal("cancelled session changed state")
	}
}

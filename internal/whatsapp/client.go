// Package whatsapp connects linked devices using whatsmeow (MPL-2.0).
package whatsapp

import (
	"agent-desk/internal/pkg/linkedchat"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type Status = linkedchat.Status
type Incoming = linkedchat.Incoming

type Session struct {
	onProfile       func(ContactProfile) error
	profileRefresh  map[string]time.Time
	profileInflight map[string]bool
	op              sync.Mutex
	mu              sync.RWMutex
	status          Status
	history         historySync
	historyTargets  func(string) ([]HistoryRequest, error)
	historyMissing  func(Incoming) bool
	historyDB       *sql.DB
	historyRunning  bool
	lastHistoryRun  time.Time
	store           *sqlstore.Container
	client          *whatsmeow.Client
	cancel          context.CancelFunc
	onMessage       func(Incoming) error
	mediaJobs       chan func()
	mediaStop       chan struct{}
}

// Each channel owns one private database containing both credentials and Signal keys.
func Open(dir string, channelID int64, onMessage func(Incoming) error) (*Session, error) {
	if channelID <= 0 {
		return nil, errors.New("invalid channel id")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("channel-%d.db", channelID))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	if err := os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	container := sqlstore.NewWithDB(db, "sqlite", nil)
	if err := container.Upgrade(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Session{store: container, historyDB: db, status: Status{State: "disconnected"}, onMessage: onMessage,
		mediaJobs: make(chan func(), 128), mediaStop: make(chan struct{})}
	for range 2 {
		go func() {
			for {
				select {
				case <-s.mediaStop:
					return
				case job := <-s.mediaJobs:
					job()
				}
			}
		}()
	}
	return s, nil
}

func (s *Session) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := s.status
	if status.QRExpiresAt != nil && time.Now().After(*status.QRExpiresAt) {
		status.QR = ""
	}
	return status
}

func (s *Session) set(ctx context.Context, status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() == nil {
		s.status = status
	}
}

func (s *Session) Connect() error {
	s.op.Lock()
	defer s.op.Unlock()
	state := s.Status().State
	if state == "connecting" || state == "qr" || state == "connected" {
		return nil
	}
	s.stop()
	device, err := s.store.GetFirstDevice(context.Background())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := whatsmeow.NewClient(device, nil)
	client.BackgroundEventCtx = ctx
	client.InitialAutoReconnect = false
	// Request history when pairing without changing whatsmeow's global device props.
	client.GetClientPayload = func() *waWa6.ClientPayload {
		payload := device.GetClientPayload()
		if pairing := payload.DevicePairingData; pairing != nil {
			props := &waCompanionReg.DeviceProps{}
			if proto.Unmarshal(pairing.DeviceProps, props) == nil {
				props.RequireFullSync = proto.Bool(true)
				pairing.DeviceProps, _ = proto.Marshal(props)
			}
		}
		return payload
	}
	s.client, s.cancel = client, cancel
	s.set(ctx, Status{State: "connecting"})
	client.AddEventHandler(func(raw any) {
		if ctx.Err() != nil {
			return
		}
		switch evt := raw.(type) {
		case *events.Connected:
			account := ""
			if client.Store.ID != nil {
				account = client.Store.ID.ToNonAD().String()
			}
			s.set(ctx, Status{State: "connected", Account: account})
			go s.syncHistoryAutomatically(ctx, account)
			go s.syncContactProfiles(ctx, client, account)
		case *events.Disconnected:
			status := s.Status()
			if status.State == "connected" || status.State == "connecting" {
				s.set(ctx, Status{State: "disconnected", Account: status.Account})
			}
		case *events.LoggedOut:
			s.set(ctx, Status{State: "logged_out"})
		case *events.StreamReplaced:
			s.set(ctx, Status{State: "error", Error: "session_replaced"})
		case *events.ConnectFailure:
			s.set(ctx, Status{State: "error", Error: "connect_failed"})
		case *events.ClientOutdated:
			s.set(ctx, Status{State: "error", Error: "client_outdated"})
		case *events.Message:
			s.receiveEvent(ctx, client, evt, "")
		case *events.HistorySync:
			s.receiveHistory(ctx, client, evt.Data, evt.Notification)
		case *events.Contact:
			s.queueContactProfile(ctx, client, evt.JID, true)
		case *events.PushName:
			s.queueContactProfile(ctx, client, evt.JID, true)
		case *events.BusinessName:
			s.queueContactProfile(ctx, client, evt.JID, true)
		case *events.Picture:
			s.queueContactProfile(ctx, client, evt.JID, true)
		}
	})
	var qr <-chan whatsmeow.QRChannelItem
	if device.ID == nil {
		qr, err = client.GetQRChannel(ctx)
		if err != nil {
			cancel()
			return err
		}
	}
	go func() {
		// Cancel stuck handshakes, but keep the lifetime context alive after connection.
		timer := time.AfterFunc(30*time.Second, func() {
			if s.Status().State == "connecting" {
				s.set(ctx, Status{State: "error", Error: "connect_timeout"})
				cancel()
			}
		})
		defer timer.Stop()
		if err := client.ConnectContext(ctx); err != nil {
			slog.Warn("whatsapp connection failed", "error", err)
			s.set(ctx, Status{State: "error", Error: "connect_failed"})
			return
		}
		if qr == nil {
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-qr:
				if !ok {
					return
				}
				switch evt.Event {
				case "code":
					png, err := qrcode.Encode(evt.Code, qrcode.Medium, 280)
					if err != nil {
						s.set(ctx, Status{State: "error", Error: "qr_failed"})
						return
					}
					expires := time.Now().Add(evt.Timeout)
					s.set(ctx, Status{State: "qr", QR: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), QRExpiresAt: &expires})
				case "success":
					return // Connected, not just pairing, determines readiness.
				case "timeout":
					s.set(ctx, Status{State: "expired"})
					return
				default:
					s.set(ctx, Status{State: "error", Error: "pair_failed"})
					return
				}
			}
		}
	}()
	return nil
}

func (s *Session) stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.client != nil {
		s.client.Disconnect()
	}
	s.mu.Lock()
	s.status = Status{State: "disconnected", Account: s.status.Account}
	s.mu.Unlock()
}

func (s *Session) Disconnect() { s.op.Lock(); defer s.op.Unlock(); s.stop() }

func (s *Session) Logout(ctx context.Context) error {
	s.op.Lock()
	defer s.op.Unlock()
	if s.client != nil && s.client.Store.ID != nil {
		if err := s.client.Logout(ctx); err != nil {
			return err
		}
	}
	s.stop()
	s.mu.Lock()
	s.status = Status{State: "logged_out"}
	s.mu.Unlock()
	return nil
}

func (s *Session) Close() error {
	s.op.Lock()
	defer s.op.Unlock()
	s.stop()
	select {
	case <-s.mediaStop:
	default:
		close(s.mediaStop)
	}
	return s.store.Close()
}

func (s *Session) Send(ctx context.Context, account, chat, id, text string) error {
	if OutboundMessagesDisabled {
		return ErrOutboundDisabled
	}
	s.op.Lock()
	defer s.op.Unlock()
	status := s.Status()
	if s.client == nil || status.State != "connected" || !s.client.IsConnected() {
		return errors.New("whatsapp is disconnected")
	}
	if status.Account != account {
		return errors.New("whatsapp account changed")
	}
	jid, err := types.ParseJID(chat)
	if err != nil || !IsPrivateChat(jid) {
		return errors.New("invalid whatsapp recipient")
	}
	_, err = s.client.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(text)}, whatsmeow.SendRequestExtra{ID: id})
	return err
}

func IsPrivateChat(jid types.JID) bool {
	return jid.User != "" && (jid.Server == types.DefaultUserServer || jid.Server == types.HiddenUserServer)
}

func ParseIncoming(evt *events.Message, account string) (Incoming, bool) {
	if evt == nil || evt.Message == nil || account == "" || evt.Info.ID == "" || evt.Info.IsGroup ||
		!IsPrivateChat(evt.Info.Chat) {
		return Incoming{}, false
	}
	text, metadata, _ := parseContent(evt)
	if text == "" && metadata == nil {
		return Incoming{}, false
	}
	return Incoming{ID: evt.Info.ID, Account: account, Chat: evt.Info.Chat.ToNonAD().String(), Name: evt.Info.PushName, Text: text, SentAt: evt.Info.Timestamp,
		Message: metadata, History: evt.SourceWebMsg != nil || evt.UnavailableRequestID != "", FromMe: evt.Info.IsFromMe}, true
}

func (s *Session) receiveEvent(ctx context.Context, client *whatsmeow.Client, evt *events.Message, name string) int {
	if ctx.Err() != nil || client.Store.ID == nil {
		return 0
	}
	msg, ok := ParseIncoming(evt, client.Store.ID.ToNonAD().String())
	if !ok || s.onMessage == nil {
		return 0
	}
	text, metadata, media := decodeContent(ctx, client, evt)
	msg.Text, msg.Message = text, metadata
	// Prefer the phone-number identity across both PN and LID history/live events.
	chat := evt.Info.Chat.ToNonAD()
	if chat.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, chat); err == nil && !pn.IsEmpty() {
			chat = pn.ToNonAD()
		}
	}
	msg.Chat = chat.String()
	msg.ChatAliases = append(msg.ChatAliases, evt.Info.Chat.ToNonAD().String())
	if chat.Server == types.DefaultUserServer {
		if lid, err := client.Store.LIDs.GetLIDForPN(ctx, chat); err == nil && !lid.IsEmpty() {
			msg.ChatAliases = append(msg.ChatAliases, lid.ToNonAD().String())
		}
	}
	if msg.FromMe {
		msg.Name = "" // An outgoing push name belongs to us, not the customer.
	}
	if name != "" {
		msg.Name = name
	} else if contact, err := client.Store.Contacts.GetContact(ctx, chat); err == nil {
		if contact.FullName != "" {
			msg.Name = contact.FullName
		} else if contact.PushName != "" {
			msg.Name = contact.PushName
		}
	}
	if err := s.onMessage(msg); err != nil {
		s.mu.Lock()
		s.status.Error = "receive_failed"
		s.mu.Unlock()
		return -1
	}
	s.queueContactProfile(ctx, client, chat, false)
	if msg.Message != nil && msg.Message.State == "pending" {
		s.queueMedia(ctx, client, msg, media)
	}
	if evt.UnavailableRequestID != "" {
		slog.Info("whatsapp missing history received", "has_media", msg.Message != nil)
	}
	return 1
}

func (s *Session) receiveHistory(ctx context.Context, client *whatsmeow.Client, data *waHistorySync.HistorySync, notification *waE2E.HistorySyncNotification) {
	processed, failed := 0, 0
	var chats []string
	for _, conversation := range data.GetConversations() {
		chat, err := types.ParseJID(conversation.GetID())
		if err != nil || !IsPrivateChat(chat) {
			continue
		}
		chats = append(chats, chat.ToNonAD().String())
		if chat.Server == types.HiddenUserServer {
			if pn, err := client.Store.LIDs.GetPNForLID(ctx, chat); err == nil {
				chats = append(chats, pn.ToNonAD().String())
			}
		} else if lid, err := client.Store.LIDs.GetLIDForPN(ctx, chat); err == nil {
			chats = append(chats, lid.ToNonAD().String())
		}
		name := conversation.GetDisplayName()
		if name == "" {
			name = conversation.GetName()
		}
		for _, item := range conversation.GetMessages() {
			if ctx.Err() != nil {
				return
			}
			evt, err := client.ParseWebMessage(chat, item.GetMessage())
			if err == nil {
				// All on-demand records are history, regardless of how the library populated the event.
				evt.SourceWebMsg = item.GetMessage()
				switch s.receiveEvent(ctx, client, evt, name) {
				case 1:
					processed++
				case -1:
					failed++
				}
			} else {
				failed++
			}
		}
	}
	if data.GetSyncType() == waHistorySync.HistorySync_ON_DEMAND {
		complete := data.Progress == nil || data.GetProgress() >= 100
		requestID := notification.GetPeerDataRequestSessionID()
		if requestID == "" {
			requestID = notification.GetOriginalMessageID()
		}
		s.historyBatch(requestID, chats, processed, failed, complete)
	}
}

// Package messenger integrates mautrix-meta's AGPL-3.0 Messenger protocol client.
package messenger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent-desk/internal/pkg/linkedchat"
	_ "github.com/glebarez/go-sqlite"
	"github.com/rs/zerolog"
	"go.mau.fi/mautrix-meta/pkg/messagix"
	"go.mau.fi/mautrix-meta/pkg/messagix/cookies"
	"go.mau.fi/mautrix-meta/pkg/messagix/socket"
	"go.mau.fi/mautrix-meta/pkg/messagix/table"
	metaTypes "go.mau.fi/mautrix-meta/pkg/messagix/types"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waConsumerApplication"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func SessionDir() string {
	if dir := strings.TrimSpace(os.Getenv("AGENT_DESK_MESSENGER_SESSION_DIR")); dir != "" {
		return dir
	}
	return "data/messenger"
}

func credentialPath(id int64) string {
	return filepath.Join(SessionDir(), fmt.Sprintf("channel-%d.json", id))
}

// Accept a Cookie header, not a password. Never return its values through the API.
func ParseCookies(raw string) (*cookies.Cookies, error) {
	if len(raw) > 32768 || strings.ContainsAny(raw, "\r\n") {
		return nil, errors.New("invalid_cookie")
	}
	req := &http.Request{Header: http.Header{"Cookie": []string{strings.TrimSpace(raw)}}}
	values := make(map[cookies.MetaCookieName]string)
	for _, cookie := range req.Cookies() {
		values[cookies.MetaCookieName(cookie.Name)] = cookie.Value
	}
	c := &cookies.Cookies{Platform: metaTypes.Facebook}
	c.UpdateValues(values)
	if len(c.GetMissingCookieNames()) > 0 || c.GetUserID() <= 0 {
		return nil, errors.New("invalid_cookie")
	}
	return c, nil
}

func SaveCredentials(id int64, raw string) error {
	if id <= 0 {
		return errors.New("invalid_channel")
	}
	c, err := ParseCookies(raw)
	if err != nil {
		return err
	}
	if existing, err := loadCredentials(id); err == nil && existing.GetUserID() != c.GetUserID() {
		return errors.New("account_changed")
	}
	return persistCookies(id, c)
}

func persistCookies(id int64, c *cookies.Cookies) error {
	if err := os.MkdirAll(SessionDir(), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(SessionDir(), ".credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), credentialPath(id))
}

func loadCredentials(id int64) (*cookies.Cookies, error) {
	data, err := os.ReadFile(credentialPath(id))
	if err != nil {
		return nil, err
	}
	c := &cookies.Cookies{Platform: metaTypes.Facebook}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	if len(c.GetMissingCookieNames()) > 0 {
		return nil, errors.New("invalid_cookie")
	}
	return c, nil
}

func SavedStatus(id int64) linkedchat.Status {
	s := linkedchat.Status{State: "disconnected", EncryptedState: "disconnected"}
	if c, err := loadCredentials(id); err == nil {
		s.HasCredentials = true
		s.Account = strconv.FormatInt(c.GetUserID(), 10)
	}
	return s
}

func Forget(id int64) error {
	if id <= 0 {
		return errors.New("invalid_channel")
	}
	if err := os.Remove(credentialPath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type Session struct {
	op        sync.Mutex
	mu        sync.RWMutex
	id        int64
	status    linkedchat.Status
	client    *messagix.Client
	e2ee      *whatsmeow.Client
	store     *sqlstore.Container
	cancel    context.CancelFunc
	done      chan struct{}
	onMessage func(linkedchat.Incoming) error
	names     map[int64]string
	threads   map[int64]table.ThreadType
}

func Open(dir string, id int64, receive func(linkedchat.Incoming) error) (*Session, error) {
	if id <= 0 {
		return nil, errors.New("invalid_channel")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("channel-%d.db", id))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := sqlstore.NewWithDB(db, "sqlite", nil)
	if err := store.Upgrade(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return &Session{id: id, store: store, onMessage: receive, status: SavedStatus(id), names: make(map[int64]string), threads: make(map[int64]table.ThreadType)}, nil
}

func (s *Session) Status() linkedchat.Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := s.status
	if state.State == "connected" && (s.client == nil || !s.client.IsConnected() || s.e2ee == nil || !s.e2ee.IsConnected()) {
		state.State = "connecting"
	}
	return state
}

func (s *Session) state(ctx context.Context, state, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() == nil {
		s.status.State = state
		s.status.Error = reason
	}
}

func (s *Session) Connect() error {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.RLock()
	active, previousCancel, previousDone, failed := s.cancel != nil, s.cancel, s.done, s.status.State == "error"
	s.mu.RUnlock()
	if active && !failed {
		return nil
	}
	if active {
		previousCancel()
		<-previousDone
	}
	c, err := loadCredentials(s.id)
	if err != nil {
		return errors.New("credentials_required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.done = make(chan struct{})
	s.status = linkedchat.Status{State: "connecting", Account: strconv.FormatInt(c.GetUserID(), 10), HasCredentials: true, EncryptedState: "connecting"}
	done := s.done
	s.mu.Unlock()
	go s.run(ctx, done, c)
	return nil
}

func (s *Session) run(ctx context.Context, done chan struct{}, c *cookies.Cookies) {
	defer func() { s.mu.Lock(); s.cancel = nil; close(done); s.mu.Unlock() }()
	// Upstream trace/debug logs may include private protocol payloads.
	cli := messagix.NewClient(c, zerolog.Nop(), &messagix.Config{})
	s.mu.Lock()
	s.client = cli
	s.mu.Unlock()
	defer cli.Disconnect()
	type queuedMessage struct {
		message linkedchat.Incoming
		saved   chan struct{}
	}
	queue := make(chan queuedMessage, 128)
	workerDone := make(chan struct{})
	workerCtx, stopWorker := context.WithCancel(ctx)
	go func() {
		defer close(workerDone)
		for {
			select {
			case <-workerCtx.Done():
				return
			case item := <-queue:
				for s.onMessage(item.message) != nil {
					s.state(workerCtx, "error", "import_failed")
					select {
					case <-workerCtx.Done():
						return
					case <-time.After(time.Second):
					}
				}
				close(item.saved)
			}
		}
	}()
	defer func() { stopWorker(); <-workerDone }()
	started := time.Now()
	emit := func(msg linkedchat.Incoming) bool {
		if msg.ID == "" || strings.TrimSpace(msg.Text) == "" {
			return true
		}
		msg.Account = strconv.FormatInt(c.GetUserID(), 10)
		item := queuedMessage{message: msg, saved: make(chan struct{})}
		select {
		case queue <- item:
		case <-workerCtx.Done():
			return false
		}
		// Do not acknowledge an encrypted message until the inbox transaction commits.
		select {
		case <-item.saved:
			return true
		case <-workerCtx.Done():
			return false
		}
	}
	cli.SetEventHandler(func(eventCtx context.Context, event any) {
		switch evt := event.(type) {
		case *table.LSTable:
			s.handleTable(evt, started, c.GetUserID(), emit)
			cli.PostHandlePublishResponse(evt)
		case *messagix.PermanentErrorEvent:
			s.state(ctx, "error", "session_expired")
		case *messagix.TransientDisconnectEvent:
			s.state(ctx, "connecting", "")
		case *messagix.ConnectedEvent, *messagix.ReconnectedEvent:
			s.state(ctx, "connected", "")
		}
	})
	loadCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	user, initial, err := cli.LoadMessagesPage(loadCtx)
	cancel()
	if err != nil {
		s.state(ctx, "error", "login_failed")
		return
	}
	if user.GetFBID() != c.GetUserID() {
		s.state(ctx, "error", "account_changed")
		return
	}
	if err := persistCookies(s.id, cli.GetCookies()); err != nil {
		s.state(ctx, "error", "store_failed")
		return
	}
	s.handleTable(initial, time.Now(), c.GetUserID(), emit)
	if err = cli.Connect(ctx); err != nil {
		s.state(ctx, "error", "connect_failed")
		return
	}
	device, err := s.store.GetFirstDevice(ctx)
	if err != nil {
		s.state(ctx, "error", "store_failed")
		return
	}
	if device.ID != nil && device.ID.User != strconv.FormatInt(c.GetUserID(), 10) {
		s.state(ctx, "error", "account_changed")
		return
	}
	cli.SetDevice(device)
	if device.ID == nil {
		registerCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		err = cli.RegisterE2EE(registerCtx, c.GetUserID())
		cancel()
		if err != nil {
			s.state(ctx, "error", "encryption_failed")
			return
		}
		if err = device.Save(ctx); err != nil {
			s.state(ctx, "error", "store_failed")
			return
		}
	}
	e2ee, err := cli.PrepareE2EEClient()
	if err != nil {
		s.state(ctx, "error", "encryption_failed")
		return
	}
	s.mu.Lock()
	s.e2ee = e2ee
	s.mu.Unlock()
	defer e2ee.Disconnect()
	e2ee.AddEventHandlerWithSuccessStatus(func(event any) bool {
		switch evt := event.(type) {
		case *events.Connected:
			s.mu.Lock()
			s.status.EncryptedState = "connected"
			s.mu.Unlock()
			s.state(ctx, "connected", "")
		case *events.Disconnected:
			s.state(ctx, "connecting", "")
		case *events.LoggedOut:
			s.state(ctx, "error", "session_expired")
		case *events.FBMessage:
			if evt.Info.IsGroup {
				return true
			}
			msg, ok := evt.Message.(*waConsumerApplication.ConsumerApplication)
			if !ok {
				return true
			}
			content := msg.GetPayload().GetContent()
			text := content.GetMessageText().GetText()
			if text == "" {
				text = content.GetExtendedTextMessage().GetText().GetText()
			}
			return emit(linkedchat.Incoming{ID: evt.Info.ID, EchoID: evt.Info.ID, Chat: "e2ee:" + evt.Info.Chat.User, Text: text, SentAt: evt.Info.Timestamp, FromMe: evt.Info.IsFromMe, History: evt.Info.Timestamp.Before(started)})
		}
		return true
	})
	if err = e2ee.Connect(); err != nil {
		s.state(ctx, "error", "encryption_failed")
		return
	}
	<-ctx.Done()
}

func (s *Session) handleTable(tbl *table.LSTable, since time.Time, self int64, emit func(linkedchat.Incoming) bool) {
	if tbl == nil {
		return
	}
	s.mu.Lock()
	for _, thread := range tbl.LSDeleteThenInsertThread {
		s.names[thread.ThreadKey] = thread.ThreadName
		s.threads[thread.ThreadKey] = thread.ThreadType
	}
	for _, thread := range tbl.LSUpdateOrInsertThread {
		s.names[thread.ThreadKey] = thread.ThreadName
		s.threads[thread.ThreadKey] = thread.ThreadType
	}
	for _, thread := range tbl.LSVerifyThreadExists {
		s.threads[thread.ThreadKey] = thread.ThreadType
	}
	s.mu.Unlock()
	handle := func(msg *table.LSInsertMessage, history bool) {
		if msg.IsAdminMessage || msg.IsUnsent {
			return
		}
		s.mu.RLock()
		name, kind := s.names[msg.ThreadKey], s.threads[msg.ThreadKey]
		s.mu.RUnlock()
		// Never auto-reply to groups or treat encrypted-thread placeholders as plaintext.
		if kind != table.ONE_TO_ONE {
			return
		}
		emit(linkedchat.Incoming{ID: msg.MessageId, EchoID: msg.OfflineThreadingId, Chat: "plain:" + strconv.FormatInt(msg.ThreadKey, 10), Name: name, Text: msg.Text, SentAt: time.UnixMilli(msg.TimestampMs), FromMe: msg.SenderId == self, History: history || time.UnixMilli(msg.TimestampMs).Before(since)})
	}
	for _, msg := range tbl.LSUpsertMessage {
		handle(msg.ToInsert(), true)
	}
	for _, msg := range tbl.LSInsertMessage {
		handle(msg, false)
	}
}

func (s *Session) Disconnect() {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.RLock()
	cancel, done := s.cancel, s.done
	s.mu.RUnlock()
	if cancel != nil {
		cancel()
		<-done
	}
	s.mu.Lock()
	s.client = nil
	s.e2ee = nil
	s.status.State = "disconnected"
	s.status.EncryptedState = "disconnected"
	s.status.Error = ""
	s.mu.Unlock()
}

// Unlink locally. Device revocation in Meta account settings remains a user action.
func (s *Session) Logout(ctx context.Context) error {
	s.Disconnect()
	if err := Forget(s.id); err != nil {
		return err
	}
	device, err := s.store.GetFirstDevice(ctx)
	if err != nil {
		return err
	}
	if device.ID != nil {
		if err := device.Delete(ctx); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.status = SavedStatus(s.id)
	s.mu.Unlock()
	return nil
}

func (s *Session) Send(ctx context.Context, account, chat, id, text string) error {
	if s.Status().Account != account {
		return errors.New("account_changed")
	}
	s.mu.RLock()
	cli, e2ee := s.client, s.e2ee
	s.mu.RUnlock()
	kind, target, ok := strings.Cut(chat, ":")
	thread, err := strconv.ParseInt(target, 10, 64)
	if !ok || err != nil || thread <= 0 {
		return errors.New("invalid_chat")
	}
	if kind == "e2ee" {
		if e2ee == nil || !e2ee.IsConnected() {
			return errors.New("disconnected")
		}
		msg := &waConsumerApplication.ConsumerApplication{Payload: &waConsumerApplication.ConsumerApplication_Payload{Payload: &waConsumerApplication.ConsumerApplication_Payload_Content{Content: &waConsumerApplication.ConsumerApplication_Content{Content: &waConsumerApplication.ConsumerApplication_Content_MessageText{MessageText: &waCommon.MessageText{Text: proto.String(text)}}}}}}
		_, err := e2ee.SendFBMessage(ctx, types.NewJID(target, types.MessengerServer), msg, nil, whatsmeow.SendRequestExtra{ID: id})
		return err
	}
	if kind != "plain" || cli == nil {
		return errors.New("invalid_chat")
	}
	if err := cli.WaitUntilCanSendMessages(ctx, 15*time.Second); err != nil {
		return err
	}
	otid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	resp, err := cli.ExecuteTasks(ctx, &socket.SendMessageTask{ThreadId: thread, Otid: otid, Text: text, Source: table.MESSENGER_INBOX_IN_THREAD, InitiatingSource: table.FACEBOOK_INBOX, SendType: table.TEXT, SyncGroup: 1})
	if err != nil {
		return err
	}
	if resp != nil {
		for _, replaced := range resp.LSReplaceOptimsiticMessage {
			if replaced.OfflineThreadingId == id && replaced.MessageId != "" {
				return nil
			}
		}
	}
	return errors.New("send_not_confirmed")
}

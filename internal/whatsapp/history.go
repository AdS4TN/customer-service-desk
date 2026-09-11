package whatsapp

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var ErrHistoryBusy = errors.New("history request already in progress")

type HistoryRequest struct {
	ConversationID int64
	Account, Chat  string
	MessageID      string
	FromMe         bool
	Before         time.Time
}

type HistoryStatus struct {
	ConversationID int64     `json:"conversationId"`
	State          string    `json:"state"`
	Processed      int       `json:"processed"`
	Failed         int       `json:"failed"`
	RequestedAt    time.Time `json:"requestedAt"`
}

type historySync struct {
	HistoryStatus
	id, chat string
	updated  time.Time
}

func (s *Session) SetHistoryTargets(load func(string) ([]HistoryRequest, error), missing func(Incoming) bool) {
	s.mu.Lock()
	s.historyTargets = load
	s.historyMissing = missing
	s.mu.Unlock()
}

func (s *Session) syncHistoryAutomatically(ctx context.Context, account string) {
	s.mu.Lock()
	load := s.historyTargets
	if load == nil || s.historyRunning || time.Since(s.lastHistoryRun) < 5*time.Minute {
		s.mu.Unlock()
		return
	}
	s.historyRunning = true
	s.lastHistoryRun = time.Now()
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.historyRunning = false; s.mu.Unlock() }()
	// Let reconnect/offline notifications settle before making requests to the phone.
	if !waitHistory(ctx, 5*time.Second) {
		return
	}
	targets, err := load(account)
	if err != nil {
		slog.Warn("whatsapp history targets unavailable")
		return
	}
	for _, target := range targets {
		if ctx.Err() != nil || s.Status().Account != account || s.Status().State != "connected" {
			return
		}
		if err := s.recoverMissingHistory(ctx, target); err != nil {
			slog.Warn("whatsapp missing history request failed", "conversation_id", target.ConversationID)
		}
		target.Before = time.Now()
		if err := s.RequestHistory(ctx, target); err != nil {
			slog.Warn("whatsapp history request failed", "conversation_id", target.ConversationID)
			return // Do not repeatedly push requests to an unavailable phone.
		}
		slog.Info("whatsapp history requested", "conversation_id", target.ConversationID, "count", 50)
		for {
			if !waitHistory(ctx, time.Second) {
				return
			}
			state := s.HistoryStatus()
			if state.State == "waiting" || state.State == "receiving" {
				continue
			}
			slog.Info("whatsapp history request settled", "conversation_id", target.ConversationID,
				"state", state.State, "processed", state.Processed, "failed", state.Failed)
			if state.State == "timeout" || state.State == "disconnected" || state.State == "failed" {
				return
			}
			break
		}
		if !waitHistory(ctx, 10*time.Second) {
			return
		}
	}
}

func waitHistory(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Session) HistoryStatus() HistoryStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.history.State == "waiting" || s.history.State == "receiving" {
		if s.status.State != "connected" {
			s.history.State = "disconnected"
		} else if time.Since(s.history.updated) > 2*time.Minute {
			s.history.State = "timeout"
		}
	}
	return s.history.HistoryStatus
}

func (s *Session) RequestHistory(ctx context.Context, req HistoryRequest) error {
	s.op.Lock()
	defer s.op.Unlock()
	status := s.Status()
	if s.client == nil || status.State != "connected" || !s.client.IsConnected() {
		return errors.New("whatsapp is disconnected")
	}
	if req.Account != status.Account {
		return errors.New("whatsapp account changed")
	}
	chat, err := types.ParseJID(req.Chat)
	if err != nil || !IsPrivateChat(chat) || req.ConversationID <= 0 || req.Before.IsZero() {
		return errors.New("invalid history request")
	}
	previous := s.HistoryStatus()
	if previous.State == "waiting" || previous.State == "receiving" || time.Since(previous.RequestedAt) < 10*time.Second {
		return ErrHistoryBusy
	}
	id := s.client.GenerateMessageID()
	s.mu.Lock()
	s.history = historySync{HistoryStatus: HistoryStatus{ConversationID: req.ConversationID, State: "waiting", RequestedAt: time.Now()},
		id: id, chat: chat.ToNonAD().String(), updated: time.Now()}
	s.mu.Unlock()
	// An empty ID requests a timestamp-based recent window; never fabricate a message ID.
	info := &types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, IsFromMe: req.FromMe}, ID: req.MessageID, Timestamp: req.Before}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err = s.client.SendMessage(callCtx, s.client.Store.ID.ToNonAD(), s.client.BuildHistorySyncRequest(info, 50), whatsmeow.SendRequestExtra{Peer: true, ID: id})
	if err != nil {
		s.mu.Lock()
		if s.history.id == id && s.history.State == "waiting" {
			s.history.State = "failed"
		}
		s.mu.Unlock()
	}
	return err
}

// Notifications with an ID must match exactly. Older clients omit it, so match
// the requested chat, including the PN/LID aliases resolved by the caller.
func (s *Session) historyBatch(id string, chats []string, processed, failed int, complete bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := &s.history
	if h.id == "" || (id != "" && id != h.id) {
		return
	}
	matched := id != ""
	for _, chat := range chats {
		matched = matched || chat == h.chat
	}
	if !matched {
		return
	}
	h.Processed += processed
	h.Failed += failed
	h.updated = time.Now()
	h.State = "receiving"
	if complete {
		h.State = "received"
		if h.Failed > 0 {
			h.State = "partial"
		}
		if h.Processed == 0 && h.Failed == 0 {
			h.State = "empty"
		}
	}
}

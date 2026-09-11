package whatsapp

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

type historyReference struct{ chat, sender, id string }

// The transport store may retain IDs for messages discarded by an older parser.
// Read references only: never read/export message-secret keys or modify this table.
func (s *Session) historyReferences(ctx context.Context, own string, chats []string) ([]historyReference, error) {
	var result []historyReference
	for _, chat := range chats {
		rows, err := s.historyDB.QueryContext(ctx, "SELECT chat_jid, sender_jid, message_id FROM whatsmeow_message_secrets WHERE our_jid = ? AND chat_jid = ? ORDER BY message_id LIMIT 500", own, chat)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var ref historyReference
			if err := rows.Scan(&ref.chat, &ref.sender, &ref.id); err != nil {
				rows.Close()
				return nil, err
			}
			result = append(result, ref)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Session) recoverMissingHistory(ctx context.Context, target HistoryRequest) error {
	s.op.Lock()
	defer s.op.Unlock()
	client := s.client
	s.mu.RLock()
	missing := s.historyMissing
	s.mu.RUnlock()
	if missing == nil || s.historyDB == nil {
		return nil
	}
	if client == nil || client.Store.ID == nil || !client.IsConnected() || s.Status().Account != target.Account {
		return errors.New("whatsapp disconnected")
	}
	chat, err := types.ParseJID(target.Chat)
	if err != nil || !IsPrivateChat(chat) {
		return errors.New("invalid history chat")
	}
	chats := []string{chat.ToNonAD().String()}
	if chat.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, chat); err == nil && !pn.IsEmpty() {
			chats = append(chats, pn.ToNonAD().String())
		}
	} else if lid, err := client.Store.LIDs.GetLIDForPN(ctx, chat); err == nil && !lid.IsEmpty() {
		chats = append(chats, lid.ToNonAD().String())
	}
	refs, err := s.historyReferences(ctx, client.Store.ID.String(), chats)
	if err != nil {
		return err
	}
	var request *waE2E.Message
	seen := make(map[string]bool)
	count := 0
	for _, ref := range refs {
		if seen[ref.id] || ref.id == "" {
			continue
		}
		seen[ref.id] = true
		if !missing(Incoming{Account: target.Account, Chat: ref.chat, ChatAliases: chats, ID: ref.id}) {
			continue
		}
		sender, err := types.ParseJID(ref.sender)
		if err != nil || !IsPrivateChat(sender) {
			continue
		}
		jid, err := types.ParseJID(ref.chat)
		if err != nil || !IsPrivateChat(jid) {
			continue
		}
		part := client.BuildUnavailableMessageRequest(jid, sender, ref.id)
		if request == nil {
			request = part
		} else {
			pdo := request.ProtocolMessage.PeerDataOperationRequestMessage
			pdo.PlaceholderMessageResendRequest = append(pdo.PlaceholderMessageResendRequest, part.ProtocolMessage.PeerDataOperationRequestMessage.PlaceholderMessageResendRequest...)
		}
		count++
		if count >= 50 {
			break
		}
	}
	if request == nil {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err = client.SendPeerMessage(callCtx, request)
	if err == nil {
		slog.Info("whatsapp missing history requested", "conversation_id", target.ConversationID, "count", count)
	}
	return err
}

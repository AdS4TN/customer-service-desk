package services

import (
	"strings"

	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/repositories"
	"agent-desk/internal/whatsapp"
	"github.com/mlogclub/simple/sqls"
	"go.mau.fi/whatsmeow/types"
)

func (s *linkedChatService) historyTargets(channelID int64, account string) ([]whatsapp.HistoryRequest, error) {
	channel, err := s.channel(channelID)
	if err != nil {
		return nil, err
	}
	if s.kind == "messenger" || channel.Status != enums.StatusOk {
		return nil, nil
	}
	accountJID, err := types.ParseJID(account)
	if err != nil || !whatsapp.IsPrivateChat(accountJID) {
		return nil, errorsx.InvalidParamI18n("error.whatsapp.identityMismatch")
	}
	prefix := whatsAppIdentity(channelID, account, "")
	targets, err := repositories.FindWhatsAppHistoryTargets(sqls.DB(), channelID, prefix)
	if err != nil {
		return nil, err
	}
	result := make([]whatsapp.HistoryRequest, 0, len(targets))
	for _, target := range targets {
		if !strings.HasPrefix(target.ExternalID, prefix) {
			continue
		}
		chat := strings.TrimPrefix(target.ExternalID, prefix)
		jid, err := types.ParseJID(chat)
		if err != nil || !whatsapp.IsPrivateChat(jid) {
			continue
		}
		result = append(result, whatsapp.HistoryRequest{ConversationID: target.ConversationID, Account: account, Chat: chat})
	}
	return result, nil
}

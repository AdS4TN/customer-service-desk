package services

import (
	"agent-desk/internal/messenger"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
)

func (s *linkedChatService) SetMessengerCredentials(id int64, raw string) error {
	channel, err := s.channel(id)
	if err != nil {
		return err
	}
	if s.kind != "messenger" || channel.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.messenger.disabled")
	}
	if _, err := messenger.ParseCookies(raw); err != nil {
		return errorsx.InvalidParamI18n("error.messenger.invalidCookie")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[id]; session != nil {
		session.Disconnect()
	}
	if err := messenger.SaveCredentials(id, raw); err != nil {
		if err.Error() == "account_changed" {
			return errorsx.InvalidParamI18n("error.messenger.accountChanged")
		}
		return errorsx.BusinessErrorI18n(1, "error.messenger.storeFailed")
	}
	return nil
}

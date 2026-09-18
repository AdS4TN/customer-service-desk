package whatsapp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type ContactProfile struct {
	Account, Chat, Name, AvatarState string
	Aliases                          []string
	Avatar                           []byte
}

func (s *Session) SetProfileHandler(handler func(ContactProfile) error) {
	s.mu.Lock()
	s.onProfile = handler
	s.mu.Unlock()
}

func (s *Session) ContactProfile(ctx context.Context, account, chat string) (ContactProfile, error) {
	s.op.Lock()
	defer s.op.Unlock()
	if s.client == nil || s.Status().State != "connected" || s.Status().Account != account {
		return ContactProfile{}, errors.New("whatsapp is disconnected or account changed")
	}
	jid, err := types.ParseJID(chat)
	if err != nil || !IsPrivateChat(jid) {
		return ContactProfile{}, errors.New("invalid contact")
	}
	return readContactProfile(ctx, s.client, jid)
}

func readContactProfile(ctx context.Context, client *whatsmeow.Client, chat types.JID) (ContactProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if client.Store.ID == nil {
		return ContactProfile{}, errors.New("whatsapp account unavailable")
	}
	profile := ContactProfile{Account: client.Store.ID.ToNonAD().String(), Chat: chat.ToNonAD().String(), AvatarState: "failed"}
	if chat.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, chat); err == nil && !pn.IsEmpty() {
			profile.Aliases = append(profile.Aliases, pn.ToNonAD().String())
			chat = pn
		}
	} else if lid, err := client.Store.LIDs.GetLIDForPN(ctx, chat); err == nil && !lid.IsEmpty() {
		profile.Aliases = append(profile.Aliases, lid.ToNonAD().String())
	}
	for _, address := range append([]string{chat.String(), profile.Chat}, profile.Aliases...) {
		jid, _ := types.ParseJID(address)
		contact, err := client.Store.Contacts.GetContact(ctx, jid)
		if err != nil {
			continue
		}
		for _, name := range []string{contact.FullName, contact.FirstName, contact.BusinessName, contact.PushName} {
			if strings.TrimSpace(name) != "" {
				profile.Name = strings.TrimSpace(name)
				break
			}
		}
		if profile.Name != "" {
			break
		}
	}
	picture, err := client.GetProfilePictureInfo(ctx, chat, &whatsmeow.GetProfilePictureParams{Preview: true})
	if errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
		profile.AvatarState = "hidden"
		return profile, nil
	}
	if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) {
		profile.AvatarState = "empty"
		return profile, nil
	}
	if err != nil {
		return profile, err
	}
	if picture == nil {
		return profile, errors.New("missing profile picture response")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, picture.URL, nil)
	if err != nil {
		return profile, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return profile, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return profile, errors.New("profile picture download failed")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return profile, err
	}
	mime := http.DetectContentType(data)
	if len(data) > 8<<20 || (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" && mime != "image/gif") {
		return profile, errors.New("invalid profile picture")
	}
	profile.Avatar, profile.AvatarState = data, "ready"
	return profile, nil
}

func (s *Session) queueContactProfile(ctx context.Context, client *whatsmeow.Client, chat types.JID, force bool) {
	if !IsPrivateChat(chat) {
		return
	}
	if client.Store.ID == nil {
		return
	}
	key := client.Store.ID.ToNonAD().String() + "|" + chat.ToNonAD().String()
	s.mu.Lock()
	if s.onProfile == nil {
		s.mu.Unlock()
		return
	}
	if s.profileRefresh == nil {
		s.profileRefresh = map[string]time.Time{}
	}
	if s.profileInflight == nil {
		s.profileInflight = map[string]bool{}
	}
	if s.profileInflight[key] || (!force && time.Now().Before(s.profileRefresh[key])) {
		s.mu.Unlock()
		return
	}
	s.profileInflight[key] = true
	handler := s.onProfile
	s.mu.Unlock()
	job := func() {
		retryAfter := time.Minute
		defer func() {
			s.mu.Lock()
			delete(s.profileInflight, key)
			s.profileRefresh[key] = time.Now().Add(retryAfter)
			s.mu.Unlock()
		}()
		if ctx.Err() != nil {
			return
		}
		profile, err := readContactProfile(ctx, client, chat)
		if ctx.Err() != nil {
			return
		}
		if saveErr := handler(profile); saveErr == nil && err == nil {
			retryAfter = time.Hour
		}
	}
	select {
	case s.mediaJobs <- job:
	default:
		s.mu.Lock()
		delete(s.profileInflight, key)
		s.mu.Unlock()
	}
}

func (s *Session) syncContactProfiles(ctx context.Context, client *whatsmeow.Client, account string) {
	if !waitHistory(ctx, 5*time.Second) {
		return
	}
	s.mu.RLock()
	load := s.historyTargets
	s.mu.RUnlock()
	if load == nil {
		return
	}
	targets, err := load(account)
	if err != nil {
		return
	}
	for _, target := range targets {
		if ctx.Err() != nil {
			return
		}
		chat, err := types.ParseJID(target.Chat)
		if err == nil {
			s.queueContactProfile(ctx, client, chat, false)
		}
		if !waitHistory(ctx, 250*time.Millisecond) {
			return
		}
	}
}

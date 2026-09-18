package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/repositories"
	"agent-desk/internal/whatsapp"
	"github.com/mlogclub/simple/sqls"
)

func (s *linkedChatService) saveContactProfile(channelID int64, profile whatsapp.ContactProfile) error {
	channel, err := s.channel(channelID)
	if err != nil {
		return err
	}
	if channel.Status != enums.StatusOk || s.kind == "messenger" {
		return nil
	}
	s.mu.Lock()
	session := s.sessions[channelID]
	s.mu.Unlock()
	if session == nil || session.Status().Account != profile.Account || session.Status().State != "connected" {
		return errorsx.InvalidParamI18n("error.whatsapp.identityMismatch")
	}
	var customer *models.Customer
	for _, chat := range append([]string{profile.Chat}, profile.Aliases...) {
		identity := repositories.CustomerIdentityRepository.GetBy(sqls.DB(), s.externalSource(), whatsAppIdentity(channelID, profile.Account, chat))
		if identity != nil {
			customer = repositories.CustomerRepository.Get(sqls.DB(), identity.CustomerID)
			break
		}
	}
	if customer == nil {
		return errorsx.InvalidParamI18n("error.e0155")
	}
	assetID, hash := customer.AvatarAssetID, customer.AvatarHash
	if profile.AvatarState == "ready" && len(profile.Avatar) > 0 {
		sum := sha256.Sum256(profile.Avatar)
		hash = hex.EncodeToString(sum[:])
		if hash != customer.AvatarHash || assetID == 0 {
			extension := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif"}[http.DetectContentType(profile.Avatar)]
			if extension == "" {
				return errorsx.BusinessErrorI18n(1, "error.whatsapp.profileFailed")
			}
			asset, uploadErr := AssetService.UploadBytes(profile.Avatar, "customer-avatar", hash+"."+extension, nil)
			if uploadErr != nil {
				return uploadErr
			}
			assetID = asset.ID
		}
	}
	err = sqls.WithTransaction(func(tx *sqls.TxContext) error {
		if status := session.Status(); status.State != "connected" || status.Account != profile.Account {
			return errorsx.InvalidParamI18n("error.whatsapp.identityMismatch")
		}
		current := repositories.CustomerRepository.Get(tx.Tx, customer.ID)
		if current == nil || current.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0155")
		}
		updates := contactProfileUpdates(current, profile, assetID, hash)
		if len(updates) == 0 {
			return nil
		}
		if err := repositories.CustomerRepository.Updates(tx.Tx, current.ID, updates); err != nil {
			return err
		}
		if name, ok := updates["name"].(string); ok {
			return CustomerService.syncConversationCustomerName(tx.Tx, current.ID, name, nil, time.Now())
		}
		return nil
	})
	if err == nil {
		publishCustomerContactChanged(customer.ID)
	}
	return err
}

func contactProfileUpdates(customer *models.Customer, profile whatsapp.ContactProfile, assetID int64, hash string) map[string]any {
	updates := map[string]any{}
	if name := strings.TrimSpace(profile.Name); name != "" {
		updates["channel_name"] = name
		var projection customerProfileProjection
		_ = json.Unmarshal([]byte(customer.AIProfileProjection), &projection)
		if customer.UpdateUserID == 0 && projection.Name == "" {
			updates["name"] = name
		}
	}
	switch profile.AvatarState {
	case "ready":
		if len(profile.Avatar) > 0 {
			updates["avatar_asset_id"], updates["avatar_hash"], updates["avatar_state"] = assetID, hash, "ready"
		}
	case "empty", "hidden":
		updates["avatar_asset_id"], updates["avatar_hash"], updates["avatar_state"] = int64(0), "", profile.AvatarState
	case "failed":
		updates["avatar_state"] = "failed"
	}
	return updates
}

func (s *linkedChatService) SyncContact(ctx context.Context, conversationID int64) (*models.Customer, error) {
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	channel, err := s.channel(conversation.ChannelID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	session := s.sessions[channel.ID]
	s.mu.Unlock()
	syncer, ok := session.(interface {
		ContactProfile(context.Context, string, string) (whatsapp.ContactProfile, error)
	})
	if !ok || session.Status().State != "connected" {
		return nil, errorsx.BusinessErrorI18n(1, "error.whatsapp.profileDisconnected")
	}
	account := session.Status().Account
	prefix := whatsAppIdentity(channel.ID, account, "")
	for _, identity := range repositories.CustomerIdentityRepository.FindByCustomerID(sqls.DB(), conversation.CustomerID) {
		if identity.ExternalSource != s.externalSource() || !strings.HasPrefix(identity.ExternalID, prefix) {
			continue
		}
		profile, fetchErr := syncer.ContactProfile(ctx, account, strings.TrimPrefix(identity.ExternalID, prefix))
		if profile.Chat == "" {
			return nil, errorsx.BusinessErrorI18n(1, "error.whatsapp.profileFailed")
		}
		if err := s.saveContactProfile(channel.ID, profile); err != nil {
			return nil, err
		}
		if fetchErr != nil {
			return nil, errorsx.BusinessErrorI18n(1, "error.whatsapp.profileFailed")
		}
		return CustomerService.Get(conversation.CustomerID), nil
	}
	return nil, errorsx.InvalidParamI18n("error.whatsapp.identityMismatch")
}

func publishCustomerContactChanged(customerID int64) {
	customer := CustomerService.Get(customerID)
	if customer == nil {
		return
	}
	for _, conversation := range repositories.ConversationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("customer_id", customerID)) {
		WsService.PublishConversationChanged(&conversation, enums.IMRealtimeEventConversationUpdated, customer)
	}
}

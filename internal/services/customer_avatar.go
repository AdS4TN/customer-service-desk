package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/repositories"
	"agent-desk/internal/services/storage"
	"github.com/mlogclub/simple/sqls"
)

// Resolve URLs at read time: cloud storage URLs can expire.
func resolveCustomerAvatars(customers []*models.Customer) {
	ids := []int64{}
	for _, customer := range customers {
		if customer != nil && customer.AvatarAssetID > 0 {
			ids = append(ids, customer.AvatarAssetID)
		}
	}
	if len(ids) == 0 {
		return
	}
	urls := map[int64]string{}
	for _, asset := range repositories.AssetRepository.Find(sqls.DB(), sqls.NewCnd().In("id", ids)) {
		if provider, err := storage.NewProvider(asset.Provider); err == nil {
			urls[asset.ID] = provider.GetSignedURL(asset.StorageKey)
		}
	}
	for _, customer := range customers {
		if customer != nil {
			customer.Avatar = urls[customer.AvatarAssetID]
		}
	}
}

func enrichConversationAvatars(conversations []models.Conversation) {
	ids := []int64{}
	for _, conversation := range conversations {
		if conversation.CustomerID > 0 {
			ids = append(ids, conversation.CustomerID)
		}
	}
	if len(ids) == 0 {
		return
	}
	customers := repositories.CustomerRepository.Find(sqls.DB(), sqls.NewCnd().In("id", ids))
	pointers := make([]*models.Customer, len(customers))
	for i := range customers {
		pointers[i] = &customers[i]
	}
	resolveCustomerAvatars(pointers)
	avatars := map[int64]string{}
	for _, customer := range customers {
		avatars[customer.ID] = customer.Avatar
	}
	for i := range conversations {
		conversations[i].CustomerAvatar = avatars[conversations[i].CustomerID]
	}
}

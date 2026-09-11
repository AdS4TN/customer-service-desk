package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/linkedchat"
	"agent-desk/internal/services/storage"
	"context"
	"errors"
	"io"
)

type linkedMediaSender interface {
	SendMedia(context.Context, string, string, string, linkedchat.OutboundMedia) error
}

func sendLinkedMedia(ctx context.Context, session whatsAppSession, account, chat, id string, message *models.Message) error {
	sender, ok := session.(linkedMediaSender)
	if !ok {
		return errors.New("media_not_supported")
	}
	payload, err := parseIMMessageAssetPayload(message.Payload)
	if err != nil {
		return err
	}
	asset := AssetService.GetByAssetID(payload.AssetID)
	if err := validateConversationAsset(asset, message.ConversationID, message.MessageType); err != nil {
		return err
	}
	provider, err := storage.NewProvider(asset.Provider)
	if err != nil {
		return err
	}
	reader, err := provider.Read(asset.StorageKey)
	if err != nil {
		return err
	}
	defer reader.Close()
	const maxMediaBytes = 100 << 20
	data, err := io.ReadAll(io.LimitReader(reader, maxMediaBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxMediaBytes {
		return errors.New("media_too_large")
	}
	isImage := message.MessageType == enums.IMMessageTypeImage && (asset.MimeType == "image/jpeg" || asset.MimeType == "image/png")
	return sender.SendMedia(ctx, account, chat, id, linkedchat.OutboundMedia{Data: data, Filename: asset.Filename, MimeType: asset.MimeType, Image: isImage})
}

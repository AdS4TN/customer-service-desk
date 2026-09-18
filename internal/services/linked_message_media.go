package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/linkedchat"
	"agent-desk/internal/repositories"
	"agent-desk/internal/services/storage"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"
)

func (s *linkedChatService) updateIncomingMedia(existing *models.Message, msg linkedchat.Incoming) error {
	var payload map[string]any
	if strings.TrimSpace(existing.Payload) != "" && json.Unmarshal([]byte(existing.Payload), &payload) != nil {
		return nil
	}
	if payload == nil {
		payload = map[string]any{}
	}
	var previous linkedchat.Message
	encodedOld, _ := json.Marshal(payload["linkedMessage"])
	_ = json.Unmarshal(encodedOld, &previous)
	metadata := linkedchat.Message{Kind: "text"}
	if msg.Message != nil {
		metadata = *msg.Message
	}
	mutation := metadata.Update != ""
	upgrade := previous.Kind == "unsupported" || previous.State == "encrypted_unavailable" || previous.State == "target_unavailable"
	if previous.Kind == "revoked" {
		return nil
	}
	if mutation {
		if metadata.RevisionMS == 0 {
			metadata.RevisionMS = msg.SentAt.UnixMilli()
		}
		if previous.RevisionMS > metadata.RevisionMS || (previous.RevisionMS == metadata.RevisionMS && (previous.State == "ready" || metadata.State == "pending" || previous.State == "")) {
			return nil
		}
	} else if !upgrade && (previous.Kind == "" || previous.State == "ready" || previous.Kind != metadata.Kind || metadata.State == "pending" || previous.Update != "") {
		return nil
	}
	if upgrade && metadata.Kind == "unsupported" && metadata.RawType == "" && previous.State == metadata.State {
		return nil
	}
	if mutation {
		// Do not let a revoked message retain downloadable attachments or quote text.
		metadata.TargetPreview = ""
		for _, key := range []string{"assetId", "filename", "fileSize", "mimeType", "provider", "storageKey", "url"} {
			delete(payload, key)
		}
	}
	if msg.MediaPath != "" {
		metadata.State = "unavailable"
		file, err := os.Open(msg.MediaPath)
		if err == nil {
			defer file.Close()
			if stat, err := file.Stat(); err == nil {
				// Never serve an untrusted HTML/SVG filename as active same-origin content.
				header := make([]byte, 512)
				n, _ := file.Read(header)
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					return err
				}
				mime := http.DetectContentType(header[:n])
				filename := safeIncomingFilename(mime)
				asset, err := AssetService.Upload(file, storage.UploadInfo{Prefix: "whatsapp", Filename: filename,
					FileSize: stat.Size(), MimeType: mime})
				if err == nil {
					metadata.State = "ready"
					payload["assetId"], payload["filename"], payload["fileSize"], payload["mimeType"] = asset.AssetID, asset.Filename, asset.FileSize, asset.MimeType
					payload["provider"], payload["storageKey"] = asset.Provider, asset.StorageKey
				}
			}
		}
	}
	payload["linkedMessage"] = metadata
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	updates := map[string]any{"payload": string(encoded), "updated_at": time.Now()}
	if upgrade || mutation {
		existing.Content = msg.Text
		existing.MessageType = enums.IMMessageTypeAttachment
		if metadata.Kind == "text" || metadata.Kind == "reply" {
			existing.MessageType = enums.IMMessageTypeText
		}
		if metadata.Kind == "image" || metadata.Kind == "sticker" {
			existing.MessageType = enums.IMMessageTypeImage
		}
		if metadata.Kind == "revoked" {
			existing.Content = ""
		}
		updates["content"], updates["message_type"] = existing.Content, existing.MessageType
	}
	if string(encoded) == existing.Payload && !upgrade && !mutation {
		return nil
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.MessageRepository.Updates(ctx.Tx, existing.ID, updates); err != nil {
			return err
		}
		if upgrade || mutation {
			return repositories.UpdateLinkedMessageSummary(ctx.Tx, existing, limitText(buildMessageSummary(existing.MessageType, existing.Content), 255))
		}
		return nil
	}); err != nil {
		return err
	}
	existing.Payload = string(encoded)
	conversation := ConversationService.Get(existing.ConversationID)
	// The inbox merges repeated message IDs; this updates the placeholder in place.
	WsService.PublishMessageCreated(conversation, existing)
	WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationUpdated)
	return nil
}

func (s *linkedChatService) findIncomingTarget(channel *models.Channel, msg linkedchat.Incoming, id string) *models.Message {
	target := msg
	target.ID = id
	if original := MessageService.Take("client_msg_id = ?", s.messageID(channel.ID, target)); original != nil {
		return original
	}
	// Replies sent by this workbench have internal IDs; the outbox records their wire IDs.
	for _, candidate := range repositories.FindLinkedEchoCandidates(sqls.DB(), s.channelType(), channel.ID, s.outboxPayload(id)) {
		if s.protocolID(channel.ChannelID, candidate.MessageID) != id {
			continue
		}
		conversation := ConversationService.Get(candidate.ConversationID)
		identity := ConversationService.GetConversationExternalIdentity(conversation)
		if identity != nil && identity.ExternalID == whatsAppIdentity(channel.ID, msg.Account, msg.Chat) {
			return MessageService.Get(candidate.MessageID)
		}
	}
	return nil
}

func safeIncomingFilename(mime string) string {
	switch mime {
	case "image/jpeg":
		return "attachment.jpg"
	case "image/png":
		return "attachment.png"
	case "image/webp":
		return "attachment.webp"
	case "image/gif":
		return "attachment.gif"
	case "audio/ogg", "application/ogg":
		return "attachment.ogg"
	case "audio/mpeg":
		return "attachment.mp3"
	case "audio/wave", "audio/x-wav":
		return "attachment.wav"
	case "video/mp4":
		return "attachment.mp4"
	case "application/pdf":
		return "attachment.pdf"
	default:
		return "attachment.bin"
	}
}

// Temporary decryption jobs do not survive a restart; keep their messages visible.
func (s *linkedChatService) recoverPendingMedia() error {
	var cursor int64
	for {
		messages, err := repositories.FindPendingLinkedMedia(sqls.DB(), s.channelType(), cursor)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return nil
		}
		for _, message := range messages {
			cursor = message.ID
			var payload struct {
				LinkedMessage *linkedchat.Message `json:"linkedMessage"`
			}
			if json.Unmarshal([]byte(message.Payload), &payload) != nil || payload.LinkedMessage == nil || payload.LinkedMessage.State != "pending" {
				continue
			}
			payload.LinkedMessage.State = "unavailable"
			if err := s.updateIncomingMedia(&message, linkedchat.Incoming{Message: payload.LinkedMessage}); err != nil {
				return err
			}
		}
	}
}

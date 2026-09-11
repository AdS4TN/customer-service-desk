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
	"time"

	"github.com/mlogclub/simple/sqls"
)

func (s *linkedChatService) updateIncomingMedia(existing *models.Message, msg linkedchat.Incoming) error {
	if msg.Message == nil || msg.Message.State == "pending" {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal([]byte(existing.Payload), &payload) != nil {
		return nil
	}
	old, ok := payload["linkedMessage"].(map[string]any)
	if !ok || old["state"] == "ready" || old["kind"] != msg.Message.Kind {
		return nil
	}
	metadata := *msg.Message
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
	if err := repositories.MessageRepository.Updates(sqls.DB(), existing.ID, map[string]any{"payload": string(encoded), "updated_at": time.Now()}); err != nil {
		return err
	}
	existing.Payload = string(encoded)
	conversation := ConversationService.Get(existing.ConversationID)
	// The inbox merges repeated message IDs; this updates the placeholder in place.
	WsService.PublishMessageCreated(conversation, existing)
	WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationUpdated)
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

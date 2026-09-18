package whatsapp

import (
	"agent-desk/internal/pkg/linkedchat"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

const maxMediaBytes = 64 << 20

func parseContentBase(evt *events.Message) (string, *linkedchat.Message, whatsmeow.DownloadableMessage) {
	m := evt.Message
	if evt.IsViewOnce || evt.IsViewOnceV2 || evt.IsViewOnceV2Extension {
		return "", &linkedchat.Message{Kind: "view_once", State: "protected"}, nil
	}
	if text := strings.TrimSpace(m.GetConversation()); text != "" {
		return text, nil, nil
	}
	if text := strings.TrimSpace(m.GetExtendedTextMessage().GetText()); text != "" {
		return text, nil, nil
	}
	meta := &linkedchat.Message{State: "pending"}
	var media whatsmeow.DownloadableMessage
	var text string
	switch {
	case m.ImageMessage != nil:
		v := m.ImageMessage
		media = v
		meta.Kind, meta.MimeType, meta.Size, text = "image", v.GetMimetype(), v.GetFileLength(), v.GetCaption()
	case m.StickerMessage != nil:
		v := m.StickerMessage
		media = v
		meta.Kind, meta.MimeType, meta.Size = "sticker", v.GetMimetype(), v.GetFileLength()
	case m.AudioMessage != nil:
		v := m.AudioMessage
		media = v
		meta.Kind, meta.MimeType, meta.Size, meta.Seconds = "audio", v.GetMimetype(), v.GetFileLength(), v.GetSeconds()
	case m.VideoMessage != nil:
		v := m.VideoMessage
		media = v
		meta.Kind, meta.MimeType, meta.Size, meta.Seconds, text = "video", v.GetMimetype(), v.GetFileLength(), v.GetSeconds(), v.GetCaption()
	case m.PtvMessage != nil:
		v := m.PtvMessage
		media = v
		meta.Kind, meta.MimeType, meta.Size, meta.Seconds, text = "round_video", v.GetMimetype(), v.GetFileLength(), v.GetSeconds(), v.GetCaption()
	case m.DocumentMessage != nil:
		v := m.DocumentMessage
		media = v
		meta.Kind, meta.MimeType, meta.Size, meta.Filename, text = "document", v.GetMimetype(), v.GetFileLength(), v.GetFileName(), v.GetCaption()
	case m.ReactionMessage != nil:
		return m.ReactionMessage.GetText(), &linkedchat.Message{Kind: "reaction", TargetID: m.ReactionMessage.GetKey().GetID()}, nil
	case m.LocationMessage != nil:
		v := m.LocationMessage
		return strings.TrimSpace(v.GetName() + "\n" + v.GetAddress() + fmt.Sprintf("\n%.6f, %.6f", v.GetDegreesLatitude(), v.GetDegreesLongitude())), &linkedchat.Message{Kind: "location"}, nil
	case m.ContactMessage != nil:
		return m.ContactMessage.GetDisplayName() + "\n" + m.ContactMessage.GetVcard(), &linkedchat.Message{Kind: "contact"}, nil
	case m.ContactsArrayMessage != nil:
		var names []string
		for _, c := range m.ContactsArrayMessage.GetContacts() {
			names = append(names, c.GetDisplayName()+"\n"+c.GetVcard())
		}
		return strings.Join(names, "\n"), &linkedchat.Message{Kind: "contact"}, nil
	case m.ProtocolMessage != nil || m.SenderKeyDistributionMessage != nil || m.FastRatchetKeySenderKeyDistributionMessage != nil || m.GroupRootKeyShare != nil || m.MessageHistoryBundle != nil || m.MessageHistoryNotice != nil || m.StickerSyncRmrMessage != nil:
		return "", nil, nil
	default:
		return parseStructured(m)
	}
	meta.Filename = filepath.Base(strings.ReplaceAll(meta.Filename, "\\", "/"))
	if meta.Filename == "." || meta.Filename == "/" || meta.Filename == "" {
		meta.Filename = meta.Kind + mediaExtension(meta.Kind, meta.MimeType)
	}
	if meta.Size > maxMediaBytes {
		meta.State = "too_large"
	}
	return strings.TrimSpace(text), meta, media
}

func mediaExtension(kind, mime string) string {
	switch strings.Split(mime, ";")[0] {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/jpeg":
		return ".jpg"
	case "audio/ogg":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4":
		return ".m4a"
	case "video/mp4":
		return ".mp4"
	case "application/pdf":
		return ".pdf"
	}
	if kind == "sticker" {
		return ".webp"
	}
	return ".bin"
}

// Bound the actual bytes written, not only the untrusted declared file length.
type boundedMediaFile struct{ *os.File }

func (f boundedMediaFile) Write(p []byte) (int, error) {
	off, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return f.write(p, off, false)
}
func (f boundedMediaFile) WriteAt(p []byte, off int64) (int, error) { return f.write(p, off, true) }
func (f boundedMediaFile) write(p []byte, off int64, at bool) (int, error) {
	if off < 0 || off > maxMediaBytes+1024-int64(len(p)) {
		return 0, errors.New("media size limit")
	}
	if at {
		return f.File.WriteAt(p, off)
	}
	return f.File.Write(p)
}
func (f boundedMediaFile) Truncate(size int64) error {
	if size < 0 || size > maxMediaBytes+1024 {
		return errors.New("media size limit")
	}
	return f.File.Truncate(size)
}

func (s *Session) queueMedia(ctx context.Context, client *whatsmeow.Client, msg Incoming, media whatsmeow.DownloadableMessage) {
	meta := *msg.Message
	msg.Message = &meta
	job := func() {
		meta.State = "unavailable"
		if ctx.Err() == nil && media != nil {
			file, err := os.CreateTemp("", "agentdesk-wa-media-*")
			if err == nil {
				defer os.Remove(file.Name())
				defer file.Close()
				downloadCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				err = client.DownloadToFile(downloadCtx, media, boundedMediaFile{file})
				cancel()
				if err == nil {
					meta.State = "ready"
					msg.MediaPath = file.Name()
				}
			}
		}
		if err := s.onMessage(msg); err != nil {
			slog.Warn("whatsapp media update failed", "kind", meta.Kind)
		}
	}
	select {
	case s.mediaJobs <- job:
	default:
		meta.State = "unavailable"
		if err := s.onMessage(msg); err != nil {
			slog.Warn("whatsapp media queue full", "kind", meta.Kind)
		}
	}
}

package whatsapp

import (
	"agent-desk/internal/pkg/linkedchat"
	"context"
	"errors"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func (s *Session) SendMedia(ctx context.Context, account, chat, id string, media linkedchat.OutboundMedia) error {
	s.op.Lock()
	defer s.op.Unlock()
	if s.client == nil || !s.client.IsConnected() || s.Status().Account != account {
		return errors.New("disconnected_or_account_changed")
	}
	jid, err := types.ParseJID(chat)
	if err != nil || !IsPrivateChat(jid) {
		return errors.New("invalid_recipient")
	}
	kind := whatsmeow.MediaDocument
	if media.Image {
		kind = whatsmeow.MediaImage
	}
	upload, err := s.client.Upload(ctx, media.Data, kind)
	if err != nil {
		return err
	}
	message := &waE2E.Message{}
	if media.Image {
		message.ImageMessage = &waE2E.ImageMessage{URL: proto.String(upload.URL), DirectPath: proto.String(upload.DirectPath), MediaKey: upload.MediaKey, FileEncSHA256: upload.FileEncSHA256, FileSHA256: upload.FileSHA256, FileLength: proto.Uint64(uint64(len(media.Data))), Mimetype: proto.String(media.MimeType)}
	} else {
		message.DocumentMessage = &waE2E.DocumentMessage{URL: proto.String(upload.URL), DirectPath: proto.String(upload.DirectPath), MediaKey: upload.MediaKey, FileEncSHA256: upload.FileEncSHA256, FileSHA256: upload.FileSHA256, FileLength: proto.Uint64(uint64(len(media.Data))), Mimetype: proto.String(media.MimeType), FileName: proto.String(media.Filename)}
	}
	_, err = s.client.SendMessage(ctx, jid, message, whatsmeow.SendRequestExtra{ID: id})
	return err
}

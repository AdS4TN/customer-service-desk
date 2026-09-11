package messenger

import (
	"agent-desk/internal/pkg/linkedchat"
	"bytes"
	"context"
	"errors"
	"go.mau.fi/mautrix-meta/pkg/messagix/httpclient"
	"go.mau.fi/mautrix-meta/pkg/messagix/socket"
	"go.mau.fi/mautrix-meta/pkg/messagix/table"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waConsumerApplication"
	"go.mau.fi/whatsmeow/proto/waMediaTransport"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"strings"
	"time"
)

func (s *Session) SendMedia(ctx context.Context, account, chat, id string, media linkedchat.OutboundMedia) error {
	if s.Status().Account != account {
		return errors.New("account_changed")
	}
	s.mu.RLock()
	cli, e2ee := s.client, s.e2ee
	s.mu.RUnlock()
	kind, target, ok := strings.Cut(chat, ":")
	thread, err := strconv.ParseInt(target, 10, 64)
	if !ok || err != nil || thread <= 0 {
		return errors.New("invalid_chat")
	}
	if kind == "e2ee" {
		if e2ee == nil || !e2ee.IsConnected() {
			return errors.New("disconnected")
		}
		mediaType := whatsmeow.MediaDocument
		if media.Image {
			mediaType = whatsmeow.MediaImage
		}
		upload, err := e2ee.Upload(ctx, media.Data, mediaType)
		if err != nil {
			return err
		}
		cfg, _, _ := image.DecodeConfig(bytes.NewReader(media.Data))
		transport := &waMediaTransport.WAMediaTransport{
			Integral:  &waMediaTransport.WAMediaTransport_Integral{FileSHA256: upload.FileSHA256, MediaKey: upload.MediaKey, FileEncSHA256: upload.FileEncSHA256, DirectPath: proto.String(upload.DirectPath), MediaKeyTimestamp: proto.Int64(time.Now().Unix())},
			Ancillary: &waMediaTransport.WAMediaTransport_Ancillary{FileLength: proto.Uint64(uint64(len(media.Data))), Mimetype: proto.String(media.MimeType), ObjectID: proto.String(upload.ObjectID), Thumbnail: &waMediaTransport.WAMediaTransport_Ancillary_Thumbnail{ThumbnailWidth: proto.Uint32(uint32(cfg.Width)), ThumbnailHeight: proto.Uint32(uint32(cfg.Height))}},
		}
		content := &waConsumerApplication.ConsumerApplication_Content{}
		if media.Image {
			imageMessage := &waConsumerApplication.ConsumerApplication_ImageMessage{}
			err = imageMessage.Set(&waMediaTransport.ImageTransport{Integral: &waMediaTransport.ImageTransport_Integral{Transport: transport}, Ancillary: &waMediaTransport.ImageTransport_Ancillary{Width: proto.Uint32(uint32(cfg.Width)), Height: proto.Uint32(uint32(cfg.Height))}})
			content.Content = &waConsumerApplication.ConsumerApplication_Content_ImageMessage{ImageMessage: imageMessage}
		} else {
			document := &waConsumerApplication.ConsumerApplication_DocumentMessage{FileName: proto.String(media.Filename)}
			err = document.Set(&waMediaTransport.DocumentTransport{Integral: &waMediaTransport.DocumentTransport_Integral{Transport: transport}, Ancillary: &waMediaTransport.DocumentTransport_Ancillary{}})
			content.Content = &waConsumerApplication.ConsumerApplication_Content_DocumentMessage{DocumentMessage: document}
		}
		if err != nil {
			return err
		}
		message := &waConsumerApplication.ConsumerApplication{Payload: &waConsumerApplication.ConsumerApplication_Payload{Payload: &waConsumerApplication.ConsumerApplication_Payload_Content{Content: content}}}
		_, err = e2ee.SendFBMessage(ctx, types.NewJID(target, types.MessengerServer), message, nil, whatsmeow.SendRequestExtra{ID: id})
		return err
	}
	if kind != "plain" || cli == nil {
		return errors.New("invalid_chat")
	}
	if err := cli.WaitUntilCanSendMessages(ctx, 15*time.Second); err != nil {
		return err
	}
	upload, err := cli.GetHTTP().SendMercuryUploadRequest(ctx, thread, &httpclient.MercuryUploadMedia{Filename: media.Filename, MimeType: media.MimeType, MediaData: media.Data})
	if err != nil {
		return err
	}
	attachment := upload.Payload.RealMetadata.GetFbId()
	if attachment == 0 {
		return errors.New("upload_unconfirmed")
	}
	otid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	result, err := cli.ExecuteTasks(ctx, &socket.SendMessageTask{ThreadId: thread, Otid: otid, Source: table.MESSENGER_INBOX_IN_THREAD, InitiatingSource: table.FACEBOOK_INBOX, SendType: table.MEDIA, SyncGroup: 1, AttachmentFBIds: []int64{attachment}})
	if err != nil {
		return err
	}
	if result != nil {
		for _, sent := range result.LSReplaceOptimsiticMessage {
			if sent.OfflineThreadingId == id && sent.MessageId != "" {
				return nil
			}
		}
	}
	return errors.New("send_not_confirmed")
}

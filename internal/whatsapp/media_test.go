package whatsapp

import (
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
)

func TestMediaContentTypes(t *testing.T) {
	for _, tc := range []struct {
		kind         string
		msg          *waE2E.Message
		downloadable bool
	}{
		{"image", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("caption")}}, true},
		{"sticker", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}, true},
		{"audio", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true), Seconds: proto.Uint32(7)}}, true},
		{"video", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}, true},
		{"document", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("../../quote.pdf")}}, true},
		{"reaction", &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: &waCommon.MessageKey{ID: proto.String("target")}, Text: proto.String("👍")}}, false},
		{"reaction", &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Text: proto.String("")}}, false},
		{"location", &waE2E.Message{LocationMessage: &waE2E.LocationMessage{DegreesLatitude: proto.Float64(22.5)}}, false},
		{"contact", &waE2E.Message{ContactMessage: &waE2E.ContactMessage{DisplayName: proto.String("Customer")}}, false},
		{"unsupported", &waE2E.Message{PollCreationMessage: &waE2E.PollCreationMessage{Name: proto.String("poll")}}, false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			text, metadata, media := parseContent(&events.Message{Message: tc.msg})
			if metadata == nil || metadata.Kind != tc.kind || (media != nil) != tc.downloadable {
				t.Fatal("message type lost")
			}
			if tc.kind == "image" && text != "caption" {
				t.Fatal("caption lost")
			}
			if tc.kind == "document" && metadata.Filename != "quote.pdf" {
				t.Fatal("unsafe filename")
			}
		})
	}
	text, meta, _ := parseContent(&events.Message{Message: &waE2E.Message{Conversation: proto.String("👍")}})
	if text != "👍" || meta != nil {
		t.Fatal("emoji text changed")
	}
	_, meta, media := parseContent(&events.Message{IsViewOnce: true, Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}})
	if meta.State != "protected" || media != nil {
		t.Fatal("view once media exposed")
	}
	_, meta, _ = parseContent(&events.Message{Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{FileLength: proto.Uint64(maxMediaBytes + 1)}}})
	if meta.State != "too_large" {
		t.Fatal("size limit missing")
	}
	_, meta, _ = parseContent(&events.Message{Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{}}})
	if meta != nil {
		t.Fatal("protocol event became customer message")
	}
}

func TestBoundedMediaFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "media")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bounded := boundedMediaFile{f}
	if _, err := bounded.WriteAt([]byte("overflow"), maxMediaBytes+1024); err == nil {
		t.Fatal("size bypass")
	}
	if err := bounded.Truncate(maxMediaBytes + 2048); err == nil {
		t.Fatal("truncate bypass")
	}
	if _, err := bounded.Write([]byte("valid")); err != nil {
		t.Fatal(err)
	}
}

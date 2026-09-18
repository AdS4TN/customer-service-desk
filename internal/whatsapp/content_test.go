package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestStructuredContent(t *testing.T) {
	key := &waCommon.MessageKey{ID: proto.String("original")}
	poll := &waE2E.PollCreationMessage{Name: proto.String("Pick a finish"), EncKey: []byte("secret-do-not-export"), SelectableOptionsCount: proto.Uint32(1), Options: []*waE2E.PollCreationMessage_Option{{OptionName: proto.String("White")}, {OptionName: proto.String("Black")}}}
	for _, tc := range []struct {
		name, kind, text, title string
		m                       *waE2E.Message
		options                 int
		media                   bool
	}{
		{name: "ptv", kind: "round_video", m: &waE2E.Message{PtvMessage: &waE2E.VideoMessage{Mimetype: proto.String("video/mp4")}}, media: true},
		{name: "poll", kind: "poll", title: "Pick a finish", m: &waE2E.Message{PollCreationMessage: poll}, options: 2},
		{name: "poll_v2", kind: "poll", title: "Pick a finish", m: &waE2E.Message{PollCreationMessageV2: poll}, options: 2},
		{name: "poll_v3", kind: "poll", title: "Pick a finish", m: &waE2E.Message{PollCreationMessageV3: poll}, options: 2},
		{name: "poll_v4", kind: "poll", title: "Pick a finish", m: &waE2E.Message{PollCreationMessageV4: &waE2E.FutureProofMessage{Message: &waE2E.Message{PollCreationMessage: poll}}}, options: 2},
		{name: "poll_v5", kind: "poll", title: "Pick a finish", m: &waE2E.Message{PollCreationMessageV5: poll}, options: 2},
		{name: "poll_v6", kind: "poll", title: "Pick a finish", m: &waE2E.Message{PollCreationMessageV6: poll}, options: 2},
		{name: "poll_result", kind: "poll", title: "Result", m: &waE2E.Message{PollResultSnapshotMessage: &waE2E.PollResultSnapshotMessage{Name: proto.String("Result"), PollVotes: []*waE2E.PollResultSnapshotMessage_PollVote{{OptionName: proto.String("A"), OptionVoteCount: proto.Int64(3)}}}}, options: 1},
		{name: "poll_vote", kind: "poll_vote", m: &waE2E.Message{PollUpdateMessage: &waE2E.PollUpdateMessage{PollCreationMessageKey: key}}},
		{name: "poll_option", kind: "poll_update", m: &waE2E.Message{PollAddOptionMessage: &waE2E.PollAddOptionMessage{PollCreationMessageKey: key, AddOption: &waE2E.PollCreationMessage_Option{OptionName: proto.String("C")}}}, options: 1},
		{name: "buttons", kind: "buttons", text: "Select", m: &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{ContentText: proto.String("Select"), Buttons: []*waE2E.ButtonsMessage_Button{{ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("Quote")}}}}}, options: 1},
		{name: "button_reply", kind: "reply", text: "Quote", m: &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{Response: &waE2E.ButtonsResponseMessage_SelectedDisplayText{SelectedDisplayText: "Quote"}}}},
		{name: "template_reply", kind: "reply", text: "Quote", m: &waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{SelectedDisplayText: proto.String("Quote")}}},
		{name: "list", kind: "list", title: "Models", m: &waE2E.Message{ListMessage: &waE2E.ListMessage{Title: proto.String("Models"), Sections: []*waE2E.ListMessage_Section{{Title: proto.String("Standard"), Rows: []*waE2E.ListMessage_Row{{Title: proto.String("A"), Description: proto.String("Details")}}}}}}, options: 1},
		{name: "list_reply", kind: "reply", text: "A", m: &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{Title: proto.String("A")}}},
		{name: "template", kind: "template", text: "Welcome", m: &waE2E.Message{TemplateMessage: &waE2E.TemplateMessage{HydratedTemplate: &waE2E.TemplateMessage_HydratedFourRowTemplate{HydratedContentText: proto.String("Welcome")}}}},
		{name: "hsm", kind: "template", title: "Template ID", m: &waE2E.Message{HighlyStructuredMessage: &waE2E.HighlyStructuredMessage{ElementName: proto.String("Template ID")}}},
		{name: "interactive", kind: "interactive", text: "Pick", m: &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Pick")}}}},
		{name: "interactive_reply", kind: "reply", text: "A", m: &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{Body: &waE2E.InteractiveResponseMessage_Body{Text: proto.String("A")}}}},
		{name: "product", kind: "product", title: "Item", m: &waE2E.Message{ProductMessage: &waE2E.ProductMessage{Product: &waE2E.ProductMessage_ProductSnapshot{Title: proto.String("Item"), PriceAmount1000: proto.Int64(12345), CurrencyCode: proto.String("USD"), ProductImage: &waE2E.ImageMessage{}}}}, media: true},
		{name: "order", kind: "order", title: "Order", m: &waE2E.Message{OrderMessage: &waE2E.OrderMessage{OrderTitle: proto.String("Order"), Token: proto.String("secret-do-not-export")}}},
		{name: "event", kind: "event", title: "Meeting", m: &waE2E.Message{EventMessage: &waE2E.EventMessage{Name: proto.String("Meeting"), StartTime: proto.Int64(1800000000)}}},
		{name: "event_invite", kind: "event", title: "Meeting", m: &waE2E.Message{EventInviteMessage: &waE2E.EventInviteMessage{EventTitle: proto.String("Meeting")}}},
		{name: "event_response", kind: "event_response", m: &waE2E.Message{EncEventResponseMessage: &waE2E.EncEventResponseMessage{EventCreationMessageKey: key}}},
		{name: "album", kind: "album", m: &waE2E.Message{AlbumMessage: &waE2E.AlbumMessage{ExpectedImageCount: proto.Uint32(3)}}},
		{name: "sticker_pack", kind: "sticker_pack", title: "Stickers", m: &waE2E.Message{StickerPackMessage: &waE2E.StickerPackMessage{Name: proto.String("Stickers")}}},
		{name: "group_invite", kind: "invite", title: "Group", m: &waE2E.Message{GroupInviteMessage: &waE2E.GroupInviteMessage{GroupName: proto.String("Group")}}},
		{name: "newsletter_invite", kind: "invite", title: "News", m: &waE2E.Message{NewsletterAdminInviteMessage: &waE2E.NewsletterAdminInviteMessage{NewsletterName: proto.String("News")}}},
		{name: "call", kind: "call", m: &waE2E.Message{CallLogMesssage: &waE2E.CallLogMessage{DurationSecs: proto.Int64(12)}}},
		{name: "scheduled_call", kind: "call", title: "Tomorrow", m: &waE2E.Message{ScheduledCallCreationMessage: &waE2E.ScheduledCallCreationMessage{Title: proto.String("Tomorrow")}}},
		{name: "invoice", kind: "payment", text: "Invoice", m: &waE2E.Message{InvoiceMessage: &waE2E.InvoiceMessage{Note: proto.String("Invoice"), Token: proto.String("secret-do-not-export")}}},
		{name: "payment", kind: "payment", m: &waE2E.Message{RequestPaymentMessage: &waE2E.RequestPaymentMessage{Amount1000: proto.Uint64(1000)}}},
		{name: "pin", kind: "notice", m: &waE2E.Message{PinInChatMessage: &waE2E.PinInChatMessage{}}},
		{name: "comment", kind: "reply", text: "Comment", m: &waE2E.Message{CommentMessage: &waE2E.CommentMessage{Message: &waE2E.Message{Conversation: proto.String("Comment")}, TargetMessageKey: key}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, meta, media := parseContent(&events.Message{Message: tc.m})
			if meta == nil || meta.Kind != tc.kind || text != tc.text || meta.Title != tc.title || len(meta.Options) != tc.options || (media != nil) != tc.media {
				t.Fatalf("unexpected projection: text=%q meta=%+v media=%v", text, meta, media != nil)
			}
			encoded, _ := json.Marshal(meta)
			if strings.Contains(string(encoded), "secret-do-not-export") {
				t.Fatal("opaque upstream data exposed")
			}
			if meta.RawType == "" {
				t.Fatal("missing diagnostic type")
			}
		})
	}
}

func TestContentEnvelopesAndUpdates(t *testing.T) {
	inner := &waE2E.Message{PtvMessage: &waE2E.VideoMessage{}}
	wrapped := &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{AssociatedChildMessage: &waE2E.FutureProofMessage{Message: inner}}}}
	_, meta, media := parseContent(&events.Message{Message: wrapped})
	if meta.Kind != "round_video" || media == nil {
		t.Fatal("nested media envelope lost")
	}
	wrapped.EphemeralMessage.Message = &waE2E.Message{ViewOnceMessageV2: &waE2E.FutureProofMessage{Message: inner}}
	_, meta, media = parseContent(&events.Message{Message: wrapped})
	if meta.State != "protected" || media != nil {
		t.Fatal("protected media exposed")
	}
	for _, kind := range []waE2E.ProtocolMessage_Type{waE2E.ProtocolMessage_REVOKE, waE2E.ProtocolMessage_MESSAGE_EDIT} {
		m := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Key: &waCommon.MessageKey{ID: proto.String("original")}, Type: kind.Enum(), EditedMessage: &waE2E.Message{Conversation: proto.String("Corrected")}, TimestampMS: proto.Int64(1000)}}
		text, meta, _ := parseContent(&events.Message{Message: m})
		if meta.TargetID != "original" || meta.Update == "" || meta.RevisionMS != 1000 {
			t.Fatal("message update lost")
		}
		if kind == waE2E.ProtocolMessage_MESSAGE_EDIT && text != "Corrected" {
			t.Fatal("edit lost")
		}
	}
	quoted := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("Reply"), ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("original"), IsForwarded: proto.Bool(true), QuotedMessage: &waE2E.Message{Conversation: proto.String("Original")}}}}
	text, meta, _ := parseContent(&events.Message{Message: quoted})
	if text != "Reply" || meta.Kind != "text" || meta.TargetPreview != "Original" || !meta.Forwarded || !meta.Textual() {
		t.Fatal("quoted text lost or routed as attachment")
	}
	unknown := &waE2E.Message{}
	unknown.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 777, protowire.BytesType), []byte("opaque-secret")))
	_, meta, _ = parseContent(&events.Message{Message: unknown})
	if meta.RawType != "field_777" || meta.Kind != "unsupported" {
		t.Fatal("unknown field diagnostic lost")
	}
	for _, control := range []*waE2E.Message{{SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{}}, {GroupRootKeyShare: &waE2E.GroupRootKeyShare{}}, {MessageHistoryNotice: &waE2E.MessageHistoryNotice{}}, {MessageContextInfo: &waE2E.MessageContextInfo{}}} {
		text, meta, _ := parseContent(&events.Message{Message: control})
		if text != "" || meta != nil {
			t.Fatal("control event became user message")
		}
	}
}

type fakeContentDecryptor struct{ fail bool }

func (d fakeContentDecryptor) DecryptReaction(context.Context, *events.Message) (*waE2E.ReactionMessage, error) {
	if d.fail {
		return nil, errors.New("missing secret")
	}
	return &waE2E.ReactionMessage{Text: proto.String("OK")}, nil
}
func (d fakeContentDecryptor) DecryptPollVote(context.Context, *events.Message) (*waE2E.PollVoteMessage, error) {
	if d.fail {
		return nil, errors.New("missing secret")
	}
	return &waE2E.PollVoteMessage{SelectedOptions: [][]byte{{1, 2, 3}}}, nil
}
func (d fakeContentDecryptor) DecryptComment(context.Context, *events.Message) (*waE2E.Message, error) {
	if d.fail {
		return nil, errors.New("missing secret")
	}
	return &waE2E.Message{Conversation: proto.String("Reply")}, nil
}
func (d fakeContentDecryptor) DecryptSecretEncryptedMessage(context.Context, *events.Message) (*waE2E.Message, error) {
	if d.fail {
		return nil, errors.New("missing secret")
	}
	return &waE2E.Message{Conversation: proto.String("Edited")}, nil
}

func TestEncryptedContentUsesLibrary(t *testing.T) {
	key := &waCommon.MessageKey{ID: proto.String("original")}
	for _, m := range []*waE2E.Message{{EncReactionMessage: &waE2E.EncReactionMessage{TargetMessageKey: key}}, {PollUpdateMessage: &waE2E.PollUpdateMessage{PollCreationMessageKey: key}}, {EncCommentMessage: &waE2E.EncCommentMessage{TargetMessageKey: key}}, {SecretEncryptedMessage: &waE2E.SecretEncryptedMessage{TargetMessageKey: key, SecretEncType: waE2E.SecretEncryptedMessage_MESSAGE_EDIT.Enum()}}} {
		for _, fail := range []bool{true, false} {
			text, meta, _ := decodeContent(context.Background(), fakeContentDecryptor{fail}, &events.Message{Message: m})
			if meta.TargetID != "original" || (meta.State == "encrypted_unavailable") != fail {
				t.Fatal("decryption result not represented correctly")
			}
			if !fail && m.PollUpdateMessage != nil && meta.Options[0].ID != "010203" {
				t.Fatal("vote hash lost")
			}
			if !fail && m.EncReactionMessage != nil && text != "OK" {
				t.Fatal("reaction not decoded")
			}
		}
	}
	options := flowOptions(`{"display_text":"Quote","url":"javascript:alert(1)","token":"secret","sections":[{"title":"Sizes","rows":[{"title":"Large","description":"Details"}]}]}`, "")
	if len(options) != 2 || options[0].Description != "" || options[1].Description != "Sizes\nDetails" {
		t.Fatal("native flow projection failed")
	}
}

package whatsapp

import (
	"agent-desk/internal/pkg/linkedchat"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Normalize only message envelopes. Never traverse quoted messages or expose keys.
func normalizeContent(evt *events.Message) *events.Message {
	copy := *evt
	for depth := 0; copy.Message != nil && depth < 12; depth++ {
		m := copy.Message
		if m.ViewOnceMessage != nil || m.ViewOnceMessageV2 != nil || m.ViewOnceMessageV2Extension != nil || m.ConditionalRevealMessage != nil {
			copy.IsViewOnce = true
			return &copy
		}
		if m.DeviceSentMessage != nil {
			copy.Message = m.DeviceSentMessage.GetMessage()
			continue
		}
		var inner *waE2E.Message
		m.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			if fd.Kind() == protoreflect.MessageKind && !fd.IsList() {
				if wrapper, ok := v.Message().Interface().(*waE2E.FutureProofMessage); ok {
					inner = wrapper.GetMessage()
					if fd.Name() == "editedMessage" {
						copy.IsEdit = true
					}
					return false
				}
			}
			return true
		})
		if inner == nil {
			return &copy
		}
		copy.Message = inner
	}
	return &copy
}

func parseContent(evt *events.Message) (string, *linkedchat.Message, whatsmeow.DownloadableMessage) {
	if evt == nil || evt.Message == nil {
		return "", nil, nil
	}
	evt = normalizeContent(evt)
	if evt.Message == nil {
		return "", nil, nil
	}
	m := evt.Message
	if evt.IsViewOnce || evt.IsViewOnceV2 || evt.IsViewOnceV2Extension {
		return parseContentBase(evt)
	}
	if contentType(m) == "" {
		return "", nil, nil
	}
	// Edits arrive as protocol messages, often inside an editedMessage envelope.
	if p := m.GetProtocolMessage(); p != nil && p.GetKey().GetID() != "" {
		if p.GetType() == waE2E.ProtocolMessage_REVOKE {
			return "", &linkedchat.Message{Kind: "revoked", Update: "revoked", TargetID: p.GetKey().GetID(), RevisionMS: p.GetTimestampMS(), RawType: "protocolMessage.revoke"}, nil
		}
		if p.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT && p.GetEditedMessage() != nil {
			copy := *evt
			copy.Message, copy.IsEdit = p.GetEditedMessage(), false
			text, meta, media := parseContentBase(normalizeContent(&copy))
			if meta == nil {
				meta = &linkedchat.Message{Kind: "text"}
			}
			meta.Update, meta.TargetID, meta.RevisionMS, meta.RawType = "edited", p.GetKey().GetID(), p.GetTimestampMS(), "protocolMessage.edit"
			return text, meta, media
		}
	}
	text, meta, media := parseContentBase(evt)
	if text == "" && meta == nil {
		return text, meta, media
	}
	context := messageContext(m)
	if meta == nil && (context.GetStanzaID() != "" || context.GetIsForwarded() || evt.IsEdit) {
		meta = &linkedchat.Message{Kind: "text"}
	}
	if meta != nil {
		meta.RawType = contentType(m)
		meta.Forwarded = context.GetIsForwarded()
		if meta.TargetID == "" {
			meta.TargetID = context.GetStanzaID()
		}
		if q := context.GetQuotedMessage(); q != nil {
			q = normalizeContent(&events.Message{Message: q}).Message
			// Text-only quote preview; do not traverse or download quoted attachments.
			meta.TargetPreview = first(q.GetConversation(), q.GetExtendedTextMessage().GetText(), q.GetImageMessage().GetCaption(), q.GetVideoMessage().GetCaption())
			meta.TargetPreview = string([]rune(meta.TargetPreview)[:min(len([]rune(meta.TargetPreview)), 160)])
		}
		if evt.IsEdit && meta.Update == "" {
			meta.Update, meta.TargetID, meta.RevisionMS = "edited", evt.Info.ID, evt.Info.Timestamp.UnixMilli()
		}
	}
	return text, meta, media
}

func messageContext(m *waE2E.Message) *waE2E.ContextInfo {
	var context *waE2E.ContextInfo
	m.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.Kind() == protoreflect.MessageKind && !fd.IsList() {
			if c, ok := v.Message().Interface().(interface{ GetContextInfo() *waE2E.ContextInfo }); ok {
				context = c.GetContextInfo()
				if context != nil {
					return false
				}
			}
		}
		return true
	})
	return context
}

// Only field names and unknown tag numbers are retained, never opaque payloads.
func contentType(m *waE2E.Message) string {
	var names []string
	m.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		if fd.Name() != "messageContextInfo" {
			names = append(names, string(fd.Name()))
		}
		return true
	})
	unknown := m.ProtoReflect().GetUnknown()
	for len(unknown) > 0 && len(names) < 16 {
		num, _, n := protowire.ConsumeField(unknown)
		if n < 0 {
			break
		}
		names = append(names, fmt.Sprintf("field_%d", num))
		unknown = unknown[n:]
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func lines(values ...string) string {
	var nonempty []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			nonempty = append(nonempty, strings.TrimSpace(value))
		}
	}
	return strings.Join(nonempty, "\n")
}

func safeContentURL(value string) string {
	u, err := url.Parse(value)
	if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil {
		return value
	}
	return ""
}

func optionID(label string) string {
	hash := sha256.Sum256([]byte(label))
	return hex.EncodeToString(hash[:])
}

func money(amount int64, currency string) string {
	return strings.TrimSpace(fmt.Sprintf("%s %.3f", currency, float64(amount)/1000))
}

func parseStructured(m *waE2E.Message) (string, *linkedchat.Message, whatsmeow.DownloadableMessage) {
	meta := &linkedchat.Message{Kind: "unsupported"}
	var text string
	var attachment *waE2E.Message
	for _, poll := range []*waE2E.PollCreationMessage{m.PollCreationMessage, m.PollCreationMessageV2, m.PollCreationMessageV3, m.PollCreationMessageV5, m.PollCreationMessageV6} {
		if poll == nil {
			continue
		}
		meta.Kind, meta.Title, meta.Selectable = "poll", poll.GetName(), poll.GetSelectableOptionsCount()
		for _, o := range poll.GetOptions() {
			meta.Options = append(meta.Options, linkedchat.Option{ID: optionID(o.GetOptionName()), Label: o.GetOptionName()})
		}
		return "", meta, nil
	}
	switch {
	case m.PollResultSnapshotMessage != nil || m.PollResultSnapshotMessageV3 != nil:
		p := m.PollResultSnapshotMessage
		if p == nil {
			p = m.PollResultSnapshotMessageV3
		}
		meta.Kind, meta.Title = "poll", p.GetName()
		for _, o := range p.GetPollVotes() {
			count := o.GetOptionVoteCount()
			meta.Options = append(meta.Options, linkedchat.Option{Label: o.GetOptionName(), Count: &count})
		}
	case m.PollUpdateMessage != nil:
		meta.Kind, meta.State, meta.TargetID = "poll_vote", "encrypted_unavailable", m.PollUpdateMessage.GetPollCreationMessageKey().GetID()
	case m.EncReactionMessage != nil:
		meta.Kind, meta.State, meta.TargetID = "reaction", "encrypted_unavailable", m.EncReactionMessage.GetTargetMessageKey().GetID()
	case m.EncEventResponseMessage != nil:
		meta.Kind, meta.State, meta.TargetID = "event_response", "encrypted_unavailable", m.EncEventResponseMessage.GetEventCreationMessageKey().GetID()
	case m.EncCommentMessage != nil:
		meta.Kind, meta.State, meta.TargetID = "reply", "encrypted_unavailable", m.EncCommentMessage.GetTargetMessageKey().GetID()
	case m.SecretEncryptedMessage != nil:
		meta.Kind, meta.State, meta.TargetID = "notice", "encrypted_unavailable", m.SecretEncryptedMessage.GetTargetMessageKey().GetID()
	case m.PollAddOptionMessage != nil:
		p := m.PollAddOptionMessage
		meta.Kind, meta.TargetID = "poll_update", p.GetPollCreationMessageKey().GetID()
		meta.Options = []linkedchat.Option{{Label: p.GetAddOption().GetOptionName(), ID: optionID(p.GetAddOption().GetOptionName())}}
	case m.LiveLocationMessage != nil:
		v := m.LiveLocationMessage
		meta.Kind = "live_location"
		text = lines(v.GetCaption(), fmt.Sprintf("%.6f, %.6f", v.GetDegreesLatitude(), v.GetDegreesLongitude()))
		meta.URL = fmt.Sprintf("https://www.google.com/maps?q=%.6f,%.6f", v.GetDegreesLatitude(), v.GetDegreesLongitude())
	case m.CommentMessage != nil:
		v := m.CommentMessage
		inner := normalizeContent(&events.Message{Message: v.GetMessage()})
		if inner.Message != nil && inner.Message.CommentMessage == nil && inner.Message.TemplateMessage == nil && inner.Message.HighlyStructuredMessage == nil {
			text, result, media := parseContentBase(inner)
			if result == nil {
				result = &linkedchat.Message{Kind: "reply"}
			}
			result.TargetID = v.GetTargetMessageKey().GetID()
			return text, result, media
		}
	case m.ButtonsResponseMessage != nil:
		v := m.ButtonsResponseMessage
		meta.Kind, text = "reply", first(v.GetSelectedDisplayText(), v.GetSelectedButtonID())
	case m.ListResponseMessage != nil:
		v := m.ListResponseMessage
		meta.Kind, text = "reply", lines(first(v.GetTitle(), v.GetSingleSelectReply().GetSelectedRowID()), v.GetDescription())
	case m.TemplateButtonReplyMessage != nil:
		v := m.TemplateButtonReplyMessage
		meta.Kind, text = "reply", first(v.GetSelectedDisplayText(), v.GetSelectedID())
	case m.ListMessage != nil:
		v := m.ListMessage
		meta.Kind, meta.Title, meta.Footer, text = "list", v.GetTitle(), v.GetFooterText(), v.GetDescription()
		for _, section := range v.GetSections() {
			for _, row := range section.GetRows() {
				meta.Options = append(meta.Options, linkedchat.Option{ID: row.GetRowID(), Label: row.GetTitle(), Description: lines(section.GetTitle(), row.GetDescription())})
			}
		}
	case m.ButtonsMessage != nil:
		v := m.ButtonsMessage
		meta.Kind, meta.Title, meta.Footer, text = "buttons", v.GetText(), v.GetFooterText(), v.GetContentText()
		for _, b := range v.GetButtons() {
			meta.Options = append(meta.Options, linkedchat.Option{ID: b.GetButtonID(), Label: first(b.GetButtonText().GetDisplayText(), b.GetButtonID())})
		}
		attachment = &waE2E.Message{ImageMessage: v.GetImageMessage(), VideoMessage: v.GetVideoMessage(), DocumentMessage: v.GetDocumentMessage()}
	case m.TemplateMessage != nil:
		v := m.TemplateMessage
		meta.Kind = "template"
		if v.GetInteractiveMessageTemplate() != nil {
			return parseStructured(&waE2E.Message{InteractiveMessage: v.GetInteractiveMessageTemplate()})
		}
		h := v.GetHydratedTemplate()
		if h == nil {
			h = v.GetHydratedFourRowTemplate()
		}
		if h != nil {
			meta.Title, meta.Footer, text = h.GetHydratedTitleText(), h.GetHydratedFooterText(), h.GetHydratedContentText()
			for _, b := range h.GetHydratedButtons() {
				meta.Options = append(meta.Options, linkedchat.Option{Label: first(b.GetQuickReplyButton().GetDisplayText(), b.GetUrlButton().GetDisplayText(), b.GetCallButton().GetDisplayText()), Description: first(safeContentURL(b.GetUrlButton().GetURL()), b.GetCallButton().GetPhoneNumber())})
			}
			attachment = &waE2E.Message{ImageMessage: h.GetImageMessage(), VideoMessage: h.GetVideoMessage(), DocumentMessage: h.GetDocumentMessage()}
		} else {
			meta.State, meta.Title = "partial", v.GetTemplateID()
		}
	case m.HighlyStructuredMessage != nil:
		v := m.HighlyStructuredMessage
		meta.Kind, meta.State, meta.Title, text = "template", "partial", v.GetElementName(), lines(v.GetParams()...)
		// A hydrated HSM contains the actual rendered template; bound recursive variants.
		if h := v.GetHydratedHsm(); h != nil && (h.GetHydratedTemplate() != nil || h.GetHydratedFourRowTemplate() != nil) {
			return parseStructured(&waE2E.Message{TemplateMessage: h})
		}
	case m.InteractiveMessage != nil:
		v := m.InteractiveMessage
		meta.Kind, meta.Title, meta.Footer, text = "interactive", v.GetHeader().GetTitle(), v.GetFooter().GetText(), lines(v.GetHeader().GetSubtitle(), v.GetBody().GetText())
		for _, b := range v.GetNativeFlowMessage().GetButtons() {
			meta.Options = append(meta.Options, flowOptions(b.GetButtonParamsJSON(), b.GetName())...)
		}
		for _, card := range v.GetCarouselMessage().GetCards() {
			meta.Options = append(meta.Options, linkedchat.Option{Label: card.GetHeader().GetTitle(), Description: lines(card.GetBody().GetText(), card.GetFooter().GetText())})
		}
		if len(v.GetCarouselMessage().GetCards()) > 0 {
			meta.State = "partial"
		}
		attachment = &waE2E.Message{ImageMessage: v.GetHeader().GetImageMessage(), VideoMessage: v.GetHeader().GetVideoMessage(), DocumentMessage: v.GetHeader().GetDocumentMessage()}
	case m.InteractiveResponseMessage != nil:
		v := m.InteractiveResponseMessage
		meta.Kind, text = "reply", v.GetBody().GetText()
		meta.Options = flowOptions(v.GetNativeFlowResponseMessage().GetParamsJSON(), "")
		if text == "" && len(meta.Options) == 0 {
			meta.State = "partial"
		}
	case m.ProductMessage != nil:
		v := m.ProductMessage
		p := v.GetProduct()
		meta.Kind, meta.Title, meta.Footer, meta.URL, text = "product", p.GetTitle(), v.GetFooter(), safeContentURL(p.GetURL()), lines(p.GetDescription(), v.GetBody())
		meta.Fields = []linkedchat.Field{{Key: "productId", Value: p.GetProductID()}}
		if p != nil && p.PriceAmount1000 != nil {
			meta.Fields = append(meta.Fields, linkedchat.Field{Key: "price", Value: money(p.GetPriceAmount1000(), p.GetCurrencyCode())})
		}
		attachment = &waE2E.Message{ImageMessage: p.GetProductImage()}
	case m.OrderMessage != nil:
		v := m.OrderMessage
		meta.Kind, meta.Title, text = "order", v.GetOrderTitle(), v.GetMessage()
		meta.Fields = []linkedchat.Field{{Key: "orderId", Value: v.GetOrderID()}}
		if v.ItemCount != nil {
			meta.Fields = append(meta.Fields, linkedchat.Field{Key: "itemCount", Value: fmt.Sprint(v.GetItemCount())})
		}
		if v.TotalAmount1000 != nil {
			meta.Fields = append(meta.Fields, linkedchat.Field{Key: "total", Value: money(v.GetTotalAmount1000(), v.GetTotalCurrencyCode())})
		}
	case m.EventMessage != nil:
		v := m.EventMessage
		meta.Kind, meta.Title, meta.StartAt, meta.EndAt, meta.URL, text = "event", v.GetName(), v.GetStartTime(), v.GetEndTime(), safeContentURL(v.GetJoinLink()), lines(v.GetDescription(), v.GetLocation().GetName(), v.GetLocation().GetAddress())
		if v.GetIsCanceled() {
			meta.State = "canceled"
		}
	case m.EventInviteMessage != nil:
		v := m.EventInviteMessage
		meta.Kind, meta.Title, meta.StartAt, meta.EndAt, meta.URL, text = "event", v.GetEventTitle(), v.GetStartTime(), v.GetEndTime(), safeContentURL(v.GetCallLink()), v.GetCaption()
		if v.GetIsCanceled() {
			meta.State = "canceled"
		}
	case m.AlbumMessage != nil:
		v := m.AlbumMessage
		meta.Kind = "album"
		meta.Fields = []linkedchat.Field{{Key: "images", Value: fmt.Sprint(v.GetExpectedImageCount())}, {Key: "videos", Value: fmt.Sprint(v.GetExpectedVideoCount())}}
	case m.StickerPackMessage != nil:
		v := m.StickerPackMessage
		meta.Kind, meta.Title, meta.Footer, text = "sticker_pack", v.GetName(), v.GetPublisher(), lines(v.GetCaption(), v.GetPackDescription())
	case m.GroupInviteMessage != nil:
		v := m.GroupInviteMessage
		meta.Kind, meta.Title, text = "invite", v.GetGroupName(), v.GetCaption()
	case m.NewsletterAdminInviteMessage != nil:
		v := m.NewsletterAdminInviteMessage
		meta.Kind, meta.Title, text = "invite", v.GetNewsletterName(), v.GetCaption()
	case m.ScheduledCallCreationMessage != nil:
		v := m.ScheduledCallCreationMessage
		meta.Kind, meta.Title, meta.StartAt = "call", v.GetTitle(), v.GetScheduledTimestampMS()/1000
	case m.CallLogMesssage != nil:
		v := m.CallLogMesssage
		meta.Kind = "call"
		meta.Seconds = uint32(max(0, min(v.GetDurationSecs(), 1<<32-1)))
		meta.Fields = []linkedchat.Field{{Key: "callOutcome", Value: v.GetCallOutcome().String()}}
	case m.RequestPaymentMessage != nil:
		v := m.RequestPaymentMessage
		meta.Kind, meta.State, text = "payment", "partial", first(v.GetNoteMessage().GetConversation(), v.GetNoteMessage().GetExtendedTextMessage().GetText())
		meta.Fields = []linkedchat.Field{{Key: "total", Value: money(int64(v.GetAmount1000()), v.GetCurrencyCodeIso4217())}}
	case m.InvoiceMessage != nil:
		meta.Kind, meta.State, text = "payment", "partial", m.InvoiceMessage.GetNote()
	case m.SendPaymentMessage != nil || m.DeclinePaymentRequestMessage != nil || m.CancelPaymentRequestMessage != nil || m.PaymentInviteMessage != nil || m.PaymentReminderMessage != nil || m.SplitPaymentMessage != nil:
		meta.Kind, meta.State = "payment", "partial"
	case m.KeepInChatMessage != nil || m.PinInChatMessage != nil || m.RequestPhoneNumberMessage != nil || m.ScheduledCallEditMessage != nil:
		meta.Kind = "notice"
	case m.PlaceholderMessage != nil:
		meta.State = "unavailable"
	default:
		meta.State = "unsupported"
	}
	if attachment != nil && contentType(attachment) != "" {
		_, mediaMeta, media := parseContentBase(&events.Message{Message: attachment})
		if mediaMeta != nil && media != nil {
			meta.State, meta.MediaKind, meta.Filename, meta.MimeType, meta.Size, meta.Seconds = mediaMeta.State, mediaMeta.Kind, mediaMeta.Filename, mediaMeta.MimeType, mediaMeta.Size, mediaMeta.Seconds
			return text, meta, media
		}
	}
	return text, meta, nil
}

// Native-flow JSON is untrusted. Extract display fields, never tokens or raw JSON.
func flowOptions(raw, fallback string) []linkedchat.Option {
	var v struct {
		DisplayText string `json:"display_text"`
		ID          string `json:"id"`
		Label       string `json:"label"`
		Title       string `json:"title"`
		Text        string `json:"text"`
		URL         string `json:"url"`
		Sections    []struct {
			Title string `json:"title"`
			Rows  []struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			} `json:"rows"`
		} `json:"sections"`
	}
	if len(raw) > 256<<10 || json.Unmarshal([]byte(raw), &v) != nil {
		if fallback == "" {
			return nil
		}
		return []linkedchat.Option{{Label: fallback}}
	}
	var options []linkedchat.Option
	if label := first(v.DisplayText, v.Title, v.Text, v.Label, v.ID, fallback); label != "" {
		options = append(options, linkedchat.Option{Label: label, Description: safeContentURL(v.URL)})
	}
	for _, section := range v.Sections {
		for _, row := range section.Rows {
			options = append(options, linkedchat.Option{Label: row.Title, Description: lines(section.Title, row.Description)})
		}
	}
	return options
}

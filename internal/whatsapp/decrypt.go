package whatsapp

import (
	"agent-desk/internal/pkg/linkedchat"
	"context"
	"encoding/hex"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

type contentDecryptor interface {
	DecryptReaction(context.Context, *events.Message) (*waE2E.ReactionMessage, error)
	DecryptPollVote(context.Context, *events.Message) (*waE2E.PollVoteMessage, error)
	DecryptComment(context.Context, *events.Message) (*waE2E.Message, error)
	DecryptSecretEncryptedMessage(context.Context, *events.Message) (*waE2E.Message, error)
}

func decodeContent(ctx context.Context, client contentDecryptor, evt *events.Message) (string, *linkedchat.Message, whatsmeow.DownloadableMessage) {
	evt = normalizeContent(evt)
	text, meta, media := parseContent(evt)
	if meta == nil || meta.State != "encrypted_unavailable" || client == nil {
		return text, meta, media
	}
	m := evt.Message
	var decoded *waE2E.Message
	var err error
	switch {
	case m.EncReactionMessage != nil:
		var reaction *waE2E.ReactionMessage
		reaction, err = client.DecryptReaction(ctx, evt)
		if err == nil && reaction != nil {
			meta.State = ""
			return reaction.GetText(), meta, nil
		}
	case m.PollUpdateMessage != nil:
		var vote *waE2E.PollVoteMessage
		vote, err = client.DecryptPollVote(ctx, evt)
		if err == nil && vote != nil {
			meta.State = ""
			for _, hash := range vote.GetSelectedOptions() {
				meta.Options = append(meta.Options, linkedchat.Option{ID: hex.EncodeToString(hash)})
			}
			if len(meta.Options) == 0 {
				meta.State = "vote_removed"
			}
			return "", meta, nil
		}
	case m.EncCommentMessage != nil:
		decoded, err = client.DecryptComment(ctx, evt)
	case m.SecretEncryptedMessage != nil:
		decoded, err = client.DecryptSecretEncryptedMessage(ctx, evt)
	}
	if err == nil && decoded != nil {
		copy := *evt
		copy.Message, copy.IsEdit = decoded, false
		text, result, downloadable := parseContent(&copy)
		if result == nil {
			result = &linkedchat.Message{Kind: "text"}
		}
		result.TargetID, result.RawType = meta.TargetID, meta.RawType
		if m.SecretEncryptedMessage != nil && m.SecretEncryptedMessage.GetSecretEncType() != waE2E.SecretEncryptedMessage_POLL_ADD_OPTION {
			result.Update, result.RevisionMS = "edited", evt.Info.Timestamp.UnixMilli()
		}
		return text, result, downloadable
	}
	// Missing message secrets are common for history predating device linking.
	// Keep a retryable, typed placeholder, not the encrypted bytes or error payload.
	return text, meta, media
}

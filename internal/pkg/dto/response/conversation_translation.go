package response

import "agent-desk/internal/pkg/enums"

type ConversationTranslation struct {
	ConversationID int64                     `json:"conversationId"`
	MessageID      int64                     `json:"messageId,omitempty"`
	LastMessageID  int64                     `json:"lastMessageId"`
	SourceLanguage enums.TranslationLanguage `json:"sourceLanguage"`
	TargetLanguage enums.TranslationLanguage `json:"targetLanguage"`
	Content        string                    `json:"content"`
	Cached         bool                      `json:"cached"`
}

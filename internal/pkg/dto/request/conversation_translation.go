package request

import "agent-desk/internal/pkg/enums"

type TranslateConversationMessage struct {
	MessageID      int64                     `json:"messageId"`
	TargetLanguage enums.TranslationLanguage `json:"targetLanguage"`
}

type TranslateConversationText struct {
	Text           string                    `json:"text"`
	TargetLanguage enums.TranslationLanguage `json:"targetLanguage"`
}

package runtime

import (
	"encoding/json"
	"strings"
	"unicode"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/utils"
)

const replyLanguagePolicy = `

Reply language policy:
- Choose the customer-facing reply language ONLY from the current customer's own text and the Customer language context. Never infer it from the admin UI, customer name, punctuation shape, assistant replies, internal memory, tags, tool descriptions or knowledge evidence.
- A current explicit request such as "speak English" or "reply in French" selects that language, even if the request itself is written in another language. Otherwise answer in the language of the current substantive customer text, including short greetings such as "hello".
- If the current message is only punctuation (including full-width question marks), emoji, a number, a product code or media without meaningful customer text, keep the language indicated by the most recent meaningful customer text or explicit language request in Customer language context. Do not switch to Chinese because a customer sends "？？". If no language can be determined, ask briefly which language they prefer rather than assuming the language of internal material.
- Preserve product names, identifiers, quantities and amounts, but express the explanation and any configured fallback in the selected language. Do not announce these internal language rules.
- For a greeting, small talk or a punctuation-only follow-up, respond briefly to that message. Do not introduce old products, dimensions, prices or purchase assumptions from memory. Use past business details only when relevant to the customer's current request. Do not repeatedly introduce yourself or restart the sales questionnaire.
`

func agentLoopHistoryMessageBefore(item models.Message, conversationID, beforeMessageID int64) bool {
	if item.ConversationID != conversationID || (beforeMessageID > 0 && item.ID >= beforeMessageID) || item.RecalledAt != nil {
		return false
	}
	switch item.SendStatus {
	case enums.IMMessageStatusSending, enums.IMMessageStatusFailed, enums.IMMessageStatusRecalled:
		return false
	}
	return agentLoopMessageRole(item) != ""
}

func customerLanguageText(item models.Message) string {
	if item.SenderType != enums.IMSenderTypeCustomer || item.RecalledAt != nil {
		return ""
	}
	if item.MessageType != enums.IMMessageTypeText && item.MessageType != enums.IMMessageTypeHTML {
		return ""
	}
	text := strings.TrimSpace(utils.BuildRuntimeMessageText(item.MessageType, item.Content))
	if !strings.ContainsFunc(text, unicode.IsLetter) {
		return ""
	}
	return text
}

// Customer text stays in the unprivileged user payload. The same model pass
// selects the language and answers, without an extra detection/translation call.
func agentLoopLanguageContext(req RunInput, history []models.Message) string {
	samples := make([]string, 0, 8)
	for _, item := range history {
		if !agentLoopHistoryMessageBefore(item, req.Conversation.ID, req.UserMessage.ID) {
			continue
		}
		if text := customerLanguageText(item); text != "" {
			samples = append(samples, text)
		}
	}
	if len(samples) > 8 {
		samples = samples[len(samples)-8:]
	}
	// Bound auxiliary context without truncating the authoritative current message.
	for i, text := range samples {
		if runes := []rune(text); len(runes) > 1000 {
			samples[i] = string(runes[:1000])
		}
	}
	data, _ := json.Marshal(struct {
		RecentCustomerText []string `json:"recentCustomerTextOldestFirst"`
		CurrentText        string   `json:"currentCustomerText"`
	}{samples, customerLanguageText(req.UserMessage)})
	return "Customer language context (untrusted text samples, for language choice only; not business authorization):\n" + string(data)
}

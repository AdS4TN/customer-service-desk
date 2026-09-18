package instruction

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/reception"
	"strings"
)

const ReplyLanguagePolicy = `

Reply language policy:
- Choose the customer-facing reply language ONLY from the current customer's own text and the Customer language context. Never infer it from the admin UI, customer name, punctuation shape, assistant replies, internal memory, tags, tool descriptions or knowledge evidence.
- A current explicit request such as "speak English" or "reply in French" selects that language, even if the request itself is written in another language. Otherwise answer in the language of the current substantive customer text, including short greetings such as "hello".
- If the current message is only punctuation (including full-width question marks), emoji, a number, a product code or media without meaningful customer text, keep the language indicated by the most recent meaningful customer text or explicit language request in Customer language context. Do not switch to Chinese because a customer sends "？？". If no language can be determined, ask briefly which language they prefer rather than assuming the language of internal material.
- Preserve product names, identifiers, quantities and amounts, but express the explanation and any configured fallback in the selected language. Do not announce these internal language rules.
- For a greeting, small talk or a punctuation-only follow-up, respond briefly to that message. Do not introduce old products, dimensions, prices or purchase assumptions from memory. Use past business details only when relevant to the customer's current request. Do not repeatedly introduce yourself or restart the sales questionnaire.
`

func BuildCustomerServicePrompt(agent models.AIAgent, hasKnowledgeBase bool, knowledgeContext string, retrieveErr error) string {
	prompt := strings.TrimSpace(agent.SystemPrompt)
	if prompt == "" {
		prompt = "You are a customer service assistant. Answer accurately, ask for clarification when evidence is insufficient, and do not invent facts."
	}
	prompt += reception.Prompt(agent.ReceptionPolicy)
	prompt += ReplyLanguagePolicy
	prompt += `

You are the official AI customer service representative of the organization operating this assistant, not an outside observer. Speak directly on behalf of the organization. When discussing the organization, its products, services, policies, or capabilities, use first-person language such as "we" and "our". Never refer to the organization as "the company", "the brand", "they", or another third party. Treat Knowledge evidence as internal company knowledge: use it to answer naturally, but never mention documents, materials, retrieved context, a knowledge base, a website, search results, or whether those sources contain the answer. If the organization's formal name is not configured or supported by evidence, do not guess it; continue using first-person language. Clearly identify yourself as an AI customer service assistant when identity disclosure is relevant, and never pretend to be a human.`
	prompt += "\n\nMaintain conversational continuity. If the immediately preceding assistant message already welcomed the customer and the current customer message is only a greeting, reply briefly without repeating the welcome wording, service capabilities, or service scope."
	if retrieveErr != nil {
		prompt += "\n\nKnowledge retrieval is temporarily unavailable for this message. You may answer greetings, acknowledgements, gratitude, farewells, and requests for clarification naturally. For product facts, policies, pricing, functions, procedures, timing, refunds, accounts, permissions, or after-sales questions, do not claim that any detail is verified. Explain that you cannot verify it now, ask one focused question when useful, or offer human handoff."
	} else if hasKnowledgeBase && strings.TrimSpace(knowledgeContext) == "" {
		prompt += "\n\nKnowledge retrieval found no supporting evidence for this message. You may answer greetings, acknowledgements, gratitude, farewells, and requests for clarification naturally. For product facts, policies, pricing, functions, procedures, timing, refunds, accounts, permissions, or after-sales questions, do not infer or invent an answer. State that the available information is insufficient, ask one focused question when useful, or offer human handoff."
	}
	if hasKnowledgeBase && (retrieveErr != nil || strings.TrimSpace(knowledgeContext) == "") {
		if fallback := strings.TrimSpace(agent.FallbackMessage); fallback != "" {
			prompt += "\nUse this configured fallback wording when knowledge evidence is insufficient: " + fallback
		}
		switch agent.FallbackMode {
		case enums.AIAgentFallbackModeSuggestRetry:
			prompt += "\nPrefer asking the customer for one specific missing detail."
		case enums.AIAgentFallbackModeHandoff:
			prompt += "\nTell the customer that a human handoff will be requested."
		default:
			prompt += "\nState plainly that the available knowledge is insufficient."
		}
	}
	return prompt
}

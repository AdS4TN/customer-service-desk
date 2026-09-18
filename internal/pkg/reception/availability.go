package reception

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
)

// AutomaticMessagesAllowed is the live reception boundary, not a published
// capability. Channel transport and assignment checks remain with their owners.
func AutomaticMessagesAllowed(conversation *models.Conversation, agent *models.AIAgent) bool {
	return conversation != nil && agent != nil && agent.Status == enums.StatusOk &&
		agent.ServiceMode != enums.IMConversationServiceModeHumanOnly &&
		conversation.ServiceMode != enums.IMConversationServiceModeHumanOnly &&
		conversation.Status != enums.IMConversationStatusClosed
}

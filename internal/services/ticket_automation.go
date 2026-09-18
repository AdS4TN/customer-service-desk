package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

// No external messages or nested transaction: ticket creation shares the
// automation receipt's transaction and remains an internal pending work item.
func (s *ticketService) createAutomationTicket(db *gorm.DB, rule *models.AutomationRule, c *models.Conversation, m *models.Message, a request.AutomationAction) (*models.Ticket, error) {
	if m == nil || strings.TrimSpace(m.Content) == "" {
		return nil, automationError("source")
	}
	now := time.Now()
	number, err := TicketNoSequenceService.nextWithRetry(db, now)
	if err != nil {
		return nil, err
	}
	ticket := &models.Ticket{TicketNo: number, Title: strings.TrimSpace(a.Value), Description: m.Content, Source: enums.TicketSourceConversation, CustomerID: c.CustomerID, ConversationID: c.ID, Status: enums.TicketStatusPending, CurrentAssigneeID: a.OwnerID, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "automation", UpdateUserName: "automation"}}
	if channel := repositories.ChannelRepository.Get(db, c.ChannelID); channel != nil {
		ticket.Channel = channel.ChannelType
	}
	if err := repositories.TicketRepository.Create(db, ticket); err != nil {
		return nil, err
	}
	if err := repositories.TicketProgressRepository.Create(db, &models.TicketProgress{TicketID: ticket.ID, Content: fmt.Sprintf("Automation #%d; source message #%d", rule.ID, m.ID), CreatedAt: now}); err != nil {
		return nil, err
	}
	return ticket, nil
}

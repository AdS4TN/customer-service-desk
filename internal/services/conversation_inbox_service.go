package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

func (s *conversationService) InboxChannels() []response.InboxChannelResponse {
	list := ChannelService.Find(sqls.NewCnd().Asc("id"))
	result := make([]response.InboxChannelResponse, 0, len(list))
	for _, channel := range list {
		item := response.InboxChannelResponse{ID: channel.ID, Name: channel.Name, ChannelType: channel.ChannelType, Status: channel.Status}
		if channel.ChannelType == enums.ChannelTypeWhatsApp || channel.ChannelType == enums.ChannelTypeMessenger {
			service := WhatsAppService
			if channel.ChannelType == enums.ChannelTypeMessenger {
				service = MessengerService
			}
			state, err := service.Status(channel.ID)
			if err == nil {
				item.ConnectionState = state.State
			} else {
				item.ConnectionState = "disabled"
			}
		}
		result = append(result, item)
	}
	return result
}

func (s *conversationService) RetryInboxMessage(messageID int64, operator *dto.AuthPrincipal) (*models.Message, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	message := MessageService.Get(messageID)
	if message == nil || message.SenderType != enums.IMSenderTypeAgent || message.SenderID != operator.UserID || message.SendStatus != enums.IMMessageStatusFailed || message.RecalledAt != nil {
		return nil, errorsx.InvalidParamI18n("error.inbox.retryUnavailable")
	}
	conversation, err := MessageService.ValidateConversationSender(message.ConversationID, enums.IMSenderTypeAgent, operator, nil)
	if err != nil {
		return nil, err
	}
	channel := ChannelService.Get(conversation.ChannelID)
	if channel == nil || channel.Status != enums.StatusOk || (channel.ChannelType != enums.ChannelTypeWhatsApp && channel.ChannelType != enums.ChannelTypeMessenger) {
		return nil, errorsx.InvalidParamI18n("error.inbox.retryUnavailable")
	}
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ok, err := repositories.RetryLinkedOutbox(ctx.Tx, channel.ChannelType, message.ID, conversation.ID, operator.UserID)
		if err != nil {
			return errorsx.InvalidParamI18n("error.inbox.retryFailed")
		}
		if !ok {
			return errorsx.InvalidParamI18n("error.inbox.retryUnavailable")
		}
		return repositories.MessageRepository.UpdateColumn(ctx.Tx, message.ID, "send_status", enums.IMMessageStatusSending)
	})
	if err != nil {
		return nil, err
	}
	message.SendStatus = enums.IMMessageStatusSending
	WsService.PublishMessageCreated(conversation, message)
	return message, nil
}

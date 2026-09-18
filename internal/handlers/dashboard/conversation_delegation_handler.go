package dashboard

import (
	"errors"

	"agent-desk/internal/builders"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
)

func localizedDelegationError(err error) error {
	if err == nil {
		return nil
	}
	var localized *errorsx.I18nError
	if errors.As(err, &localized) {
		return localized
	}
	return errorsx.InvalidParamI18n("error.delegation.failed")
}

func ConversationGetDelegation(ctx *gin.Context) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	v, err := services.ConversationDelegationService.View(id, op)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildConversationDelegation(v.Conversation, v.State, v.OwnerID, v.Names, v.Agents, v.Events, v.CanManage))
}

func ConversationPostStartDelegation(ctx *gin.Context) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req request.StartConversationDelegation
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, localizedDelegationError(services.ConversationDelegationService.Start(id, req, op)))
}

func ConversationPostStopDelegation(ctx *gin.Context) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req request.ChangeConversationDelegation
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, localizedDelegationError(services.ConversationDelegationService.Stop(id, req.Revision, op)))
}

func ConversationPostPreviewDelegation(ctx *gin.Context) {
	op, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req request.ChangeConversationDelegation
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, localizedDelegationError(services.ConversationDelegationService.Preview(ctx.Request.Context(), id, req.Revision, op)))
}

package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
)

func ConversationPostWork(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req request.UpdateConversationWork
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ConversationWorkService.Update(id, req, operator))
}

func ConversationGetCollaboration(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var query struct {
		Before int64 `form:"before"`
	}
	if err := ctx.ShouldBindQuery(&query); err != nil {
		httpx.JsonErrorMsg(ctx, "error.reception.failed")
		return
	}
	result, err := services.ConversationWorkService.Collaboration(id, query.Before)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func ConversationGetColleagues(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.ConversationWorkService.Colleagues()
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func ConversationPostNote(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req request.CreateConversationNote
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ConversationWorkService.AddNote(id, req, operator))
}

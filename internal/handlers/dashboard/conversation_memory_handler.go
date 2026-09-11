package dashboard

import (
	"agent-desk/internal/builders"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
)

func ConversationGetMemory(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	view, err := services.ConversationMemoryService.View(id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildConversationMemory(view))
}
func ConversationPostMemoryRefresh(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	httpx.WriteJSON(ctx, services.ConversationMemoryService.Refresh(id))
}
func ConversationPostMemoryUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req request.UpdateConversationMemory
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ConversationMemoryService.Update(id, operator.UserID, req))
}

func ConversationPostProfileUpdate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionCustomerUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req request.UpdateConversationMemory
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.ConversationMemoryService.UpdateProfile(id, operator.UserID, req))
}

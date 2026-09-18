package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
)

func AIAgentGetReceptionState(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	state, err := services.AIAgentService.ReceptionState(id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, state)
}

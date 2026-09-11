package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
)

func ConversationPostSuggestReply(ctx *gin.Context) {
	for _, permission := range []constants.Permission{constants.PermissionConversationView, constants.PermissionConversationSend} {
		if _, err := services.AuthService.RequirePermission(ctx, permission); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	result, err := services.ConversationCopilotService.Suggest(ctx.Request.Context(), id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

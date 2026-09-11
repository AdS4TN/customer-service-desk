package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

func ConversationPostTranslateMessage(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 64<<10)
	var req request.TranslateConversationMessage
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.ConversationTranslationService.Message(ctx.Request.Context(), id, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func ConversationPostTranslateText(ctx *gin.Context) {
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
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 64<<10)
	var req request.TranslateConversationText
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.ConversationTranslationService.Text(ctx.Request.Context(), id, req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

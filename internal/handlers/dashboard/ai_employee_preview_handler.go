package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
)

func AIAgentPostPreview(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.AIEmployeePreviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JsonErrorMsg(ctx, "error.employeePreview.input")
		return
	}
	result, err := services.AIEmployeePreviewService.Preview(ctx.Request.Context(), req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

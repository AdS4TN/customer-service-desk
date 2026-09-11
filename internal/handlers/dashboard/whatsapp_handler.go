package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
)

func ChannelGetWhatsApp(ctx *gin.Context) {
	// QR codes grant device access, so viewing them requires channel-edit permission.
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionChannelUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")
	status, err := services.WhatsAppService.Status(id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

func ChannelPostWhatsApp(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionChannelUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")
	var err error
	switch ctx.Param("action") {
	case "connect":
		_, err = services.WhatsAppService.Connect(id)
	case "disconnect":
		err = services.WhatsAppService.Disconnect(id, false)
	case "logout":
		err = services.WhatsAppService.Disconnect(id, true)
	default:
		err = errorsx.InvalidParamI18n("error.whatsapp.invalidAction")
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	status, err := services.WhatsAppService.Status(id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

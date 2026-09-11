package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

func ChannelGetMessenger(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionChannelUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")
	status, err := services.MessengerService.Status(id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

func ChannelPostMessenger(ctx *gin.Context) {
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
	case "login":
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 40*1024)
		var req struct {
			Cookie string `json:"cookie"`
		}
		if ctx.ShouldBindJSON(&req) != nil {
			httpx.WriteJSON(ctx, errorsx.InvalidParamI18n("error.messenger.invalidCookie"))
			return
		}
		err = services.MessengerService.SetMessengerCredentials(id, req.Cookie)
		if err == nil {
			_, err = services.MessengerService.Connect(id)
		}
	case "connect":
		_, err = services.MessengerService.Connect(id)
	case "disconnect":
		err = services.MessengerService.Disconnect(id, false)
	case "logout":
		err = services.MessengerService.Disconnect(id, true)
	default:
		err = errorsx.InvalidParamI18n("error.messenger.invalidAction")
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	status, err := services.MessengerService.Status(id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

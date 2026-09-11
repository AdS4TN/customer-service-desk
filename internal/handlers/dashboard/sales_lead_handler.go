package dashboard

import (
	"agent-desk/internal/builders"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
	"strconv"
)

func SalesLeadGetList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	page, _ := strconv.Atoi(ctx.Query("page"))
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	conversationID, _ := strconv.ParseInt(ctx.Query("conversationId"), 10, 64)
	ownerID, _ := strconv.ParseInt(ctx.Query("ownerId"), 10, 64)
	if ctx.Query("mine") == "true" {
		ownerID = operator.UserID
	}
	rows, total, err := services.SalesLeadService.List(conversationID, ownerID, ctx.Query("status"), ctx.Query("keyword"), ctx.Query("tag"), ctx.Query("queue"), page, limit)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	out := response.SalesLeadList{Results: []response.SalesLead{}, Page: response.SalesLeadPage{Page: page, Limit: limit, Total: total}}
	for _, v := range rows {
		out.Results = append(out.Results, builders.BuildSalesLead(v))
	}
	httpx.WriteJSON(ctx, out)
}

func SalesLeadPostFollowUp(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "leadId")
	if !ok {
		return
	}
	var req request.FollowUpSalesLead
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.SalesLeadService.FollowUp(id, operator.UserID, req))
}
func SalesLeadGetBy(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "leadId")
	if !ok {
		return
	}
	v, err := services.SalesLeadService.Get(id)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSalesLead(*v))
}
func SalesLeadPostUpdate(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationSend)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "leadId")
	if !ok {
		return
	}
	var req request.UpdateSalesLead
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.SalesLeadService.Update(id, operator.UserID, req))
}

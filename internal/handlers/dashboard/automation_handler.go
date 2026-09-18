package dashboard

import (
	"agent-desk/internal/builders"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
	"strconv"
)

func automationAPIError(err error) error {
	if err == nil {
		return nil
	}
	var localized *errorsx.I18nError
	if errors.As(err, &localized) {
		return localized
	}
	return errorsx.InvalidParamI18n("error.automation.failed")
}

func AutomationGetList(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	rows, err := services.AutomationService.List()
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.automation.failed"))
		return
	}
	out := []response.AutomationRule{}
	for _, r := range rows {
		out = append(out, builders.BuildAutomation(r))
	}
	httpx.WriteJSON(ctx, out)
}
func AutomationPostSave(ctx *gin.Context) {
	u, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.SaveAutomation
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	row, err := services.AutomationService.Save(req, u.UserID)
	if err != nil {
		httpx.WriteJSON(ctx, automationAPIError(err))
		return
	}
	httpx.WriteJSON(ctx, builders.BuildAutomation(*row))
}
func AutomationPostChange(ctx *gin.Context) { automationChange(ctx, false) }
func AutomationPostDelete(ctx *gin.Context) { automationChange(ctx, true) }
func automationChange(ctx *gin.Context, remove bool) {
	u, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.ChangeAutomation
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, automationAPIError(services.AutomationService.Change(req, u.UserID, remove)))
}
func AutomationPostTest(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.TestAutomation
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	matched, conditions, err := services.AutomationService.Test(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, map[string]any{"matched": matched, "conditions": conditions})
}
func AutomationGetRuns(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	id, _ := strconv.ParseInt(ctx.Query("ruleId"), 10, 64)
	page, _ := strconv.Atoi(ctx.Query("page"))
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	rows, total, err := services.AutomationService.Runs(id, page, limit)
	if err != nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.automation.failed"))
		return
	}
	out := []response.AutomationRun{}
	for _, r := range rows {
		out = append(out, builders.BuildAutomationRun(r))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: out, Page: &sqls.Paging{Page: page, Limit: limit, Total: total}})
}
func AutomationPostRetry(ctx *gin.Context) {
	u, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, automationAPIError(services.AutomationService.Retry(req.ID, u.UserID)))
}

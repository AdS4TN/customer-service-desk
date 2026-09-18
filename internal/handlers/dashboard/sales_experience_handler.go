package dashboard

import (
	"agent-desk/internal/builders"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
	"strconv"
)

func experienceAuth(ctx *gin.Context, write bool) *dto.AuthPrincipal {
	u, err := services.AuthService.RequirePermission(ctx, constants.PermissionConversationView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return nil
	}
	permission := constants.PermissionSkillDefinitionView
	if write {
		permission = constants.PermissionSkillDefinitionUpdate
	}
	if _, err = services.AuthService.RequirePermission(ctx, permission); err != nil {
		httpx.WriteJSON(ctx, err)
		return nil
	}
	return u
}
func experiencePage(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.Query("page"))
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
func experienceID(ctx *gin.Context) int64 {
	id, _ := strconv.ParseInt(ctx.Query("id"), 10, 64)
	return id
}
func experienceWriteError(ctx *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	httpx.WriteJSON(ctx, localizedExperienceError(err))
	return true
}
func SalesExperienceGetOptions(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	skills, err := services.SalesExperienceService.Skills()
	if experienceWriteError(ctx, err) {
		return
	}
	ss := []response.SalesExperienceSkill{}
	for _, s := range skills {
		ss = append(ss, response.SalesExperienceSkill{ID: s.ID, ActiveRevisionID: s.ActiveRevisionID})
	}
	ms := []response.SalesExperienceModel{}
	for _, m := range services.SalesExperienceService.ModelOptions() {
		ms = append(ms, response.SalesExperienceModel{ID: m.ID, Name: m.Name, ModelName: m.ModelName})
	}
	httpx.WriteJSON(ctx, map[string]any{"skills": ss, "models": ms})
}
func SalesExperienceGetSources(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	page, limit := experiencePage(ctx)
	rows, total, err := services.SalesExperienceService.Sources(ctx.Query("keyword"), ctx.Query("channel"), page, limit)
	if experienceWriteError(ctx, err) {
		return
	}
	out := []response.SalesExperienceSource{}
	for _, r := range rows {
		out = append(out, builders.BuildSalesExperienceSource(r))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: out, Page: &sqls.Paging{Page: page, Limit: limit, Total: total}})
}
func SalesExperienceGetCases(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	page, limit := experiencePage(ctx)
	rows, total, err := services.SalesExperienceService.Cases(page, limit)
	if experienceWriteError(ctx, err) {
		return
	}
	out := []response.SalesExperienceCase{}
	for _, r := range rows {
		out = append(out, builders.BuildSalesExperienceCase(r))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: out, Page: &sqls.Paging{Page: page, Limit: limit, Total: total}})
}
func SalesExperienceGetCase(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	r, err := services.SalesExperienceService.Case(experienceID(ctx))
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSalesExperienceCase(*r))
}

func SalesExperiencePostDeleteCase(ctx *gin.Context) {
	if experienceAuth(ctx, true) == nil {
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	if !experienceWriteError(ctx, services.SalesExperienceService.DeleteCase(req.ID)) {
		httpx.WriteJSON(ctx, nil)
	}
}

func SalesExperiencePostSkillMinerDebug(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	var req struct {
		CaseID        int64 `json:"caseId"`
		ModelConfigID int64 `json:"modelConfigId"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.CaseID <= 0 {
		httpx.WriteJSON(ctx, map[string]any{"ok": false, "error": "caseId required"})
		return
	}
	result, err := services.SalesExperienceService.SkillMinerDebug(req.CaseID, req.ModelConfigID)
	if err != nil {
		if result != nil {
			httpx.WriteJSON(ctx, result)
			return
		}
		httpx.WriteJSON(ctx, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	httpx.WriteJSON(ctx, result)
}

func SalesExperiencePostSkillMinerReview(ctx *gin.Context) {
	operator := experienceAuth(ctx, true)
	if operator == nil {
		return
	}
	var req request.ReviewSalesExperienceSkillMiner
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.Action == "confirm" {
		if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionSkillDefinitionCreate); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
	}
	result, err := services.SalesExperienceService.ReviewSkillMiner(req, operator)
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, result)
}

func SalesExperiencePostSkillCompare(ctx *gin.Context) {
	if experienceAuth(ctx, true) == nil {
		return
	}
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionAIAgentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req request.CompareSalesExperienceSkill
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.AIEmployeePreviewService.CompareSkill(ctx.Request.Context(), req)
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, result)
}
func SalesExperienceGetRevisions(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	page, limit := experiencePage(ctx)
	rows, total, err := services.SalesExperienceService.Revisions(ctx.Query("skill"), page, limit)
	if experienceWriteError(ctx, err) {
		return
	}
	out := []response.SalesExperienceRevision{}
	for _, r := range rows {
		out = append(out, builders.BuildSalesExperienceRevision(r))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: out, Page: &sqls.Paging{Page: page, Limit: limit, Total: total}})
}
func SalesExperienceGetRevision(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	r, err := services.SalesExperienceService.Revision(experienceID(ctx))
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSalesExperienceRevision(*r))
}
func SalesExperienceGetJobs(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	page, limit := experiencePage(ctx)
	rows, total, err := services.SalesExperienceService.Jobs(page, limit)
	if experienceWriteError(ctx, err) {
		return
	}
	out := []response.SalesExperienceJob{}
	for _, r := range rows {
		out = append(out, builders.BuildSalesExperienceJob(r))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: out, Page: &sqls.Paging{Page: page, Limit: limit, Total: total}})
}
func SalesExperienceGetJob(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	r, err := services.SalesExperienceService.Job(experienceID(ctx))
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSalesExperienceJob(*r))
}
func SalesExperiencePostImport(ctx *gin.Context) {
	u := experienceAuth(ctx, true)
	if u == nil {
		return
	}
	var req request.ImportSalesExperience
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	rows, err := services.SalesExperienceService.Import(req, u.UserID)
	if experienceWriteError(ctx, err) {
		return
	}
	out := []response.SalesExperienceCase{}
	for _, r := range rows {
		out = append(out, builders.BuildSalesExperienceCase(r))
	}
	httpx.WriteJSON(ctx, out)
}
func SalesExperiencePostImportPreview(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	var req request.PreviewSalesExperienceImport
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	rows, err := services.SalesExperienceService.ImportPreview(req.ConversationIDs)
	if experienceWriteError(ctx, err) {
		return
	}
	out := make([]response.SalesExperienceImportPreview, 0, len(rows))
	for _, row := range rows {
		out = append(out, response.SalesExperienceImportPreview{
			ConversationID: row.ConversationIDs[0],
			CustomerID:     row.CustomerID,
			CustomerName:   row.CustomerName,
			Messages:       row.Messages,
		})
	}
	httpx.WriteJSON(ctx, out)
}
func SalesExperiencePostDistill(ctx *gin.Context) {
	u := experienceAuth(ctx, true)
	if u == nil {
		return
	}
	var req request.DistillSalesExperience
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	r, err := services.SalesExperienceService.Distill(req, u.UserID)
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSalesExperienceJob(*r))
}
func SalesExperiencePostEvaluate(ctx *gin.Context) {
	u := experienceAuth(ctx, true)
	if u == nil {
		return
	}
	var req request.EvaluateSalesExperience
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	r, err := services.SalesExperienceService.Evaluate(req, u.UserID)
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSalesExperienceJob(*r))
}
func SalesExperiencePostEdit(ctx *gin.Context) {
	u := experienceAuth(ctx, true)
	if u == nil {
		return
	}
	var req request.EditSalesExperience
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	r, err := services.SalesExperienceService.Edit(req, u.UserID)
	if experienceWriteError(ctx, err) {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSalesExperienceRevision(*r))
}
func SalesExperiencePostAnnotate(ctx *gin.Context) {
	if experienceAuth(ctx, true) == nil {
		return
	}
	var req request.AnnotateSalesExperience
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	if !experienceWriteError(ctx, services.SalesExperienceService.Annotate(req)) {
		httpx.WriteJSON(ctx, nil)
	}
}
func SalesExperiencePostRate(ctx *gin.Context) {
	if experienceAuth(ctx, true) == nil {
		return
	}
	var req request.RateSalesExperience
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	if !experienceWriteError(ctx, services.SalesExperienceService.Rate(req)) {
		httpx.WriteJSON(ctx, nil)
	}
}
func SalesExperiencePostActivate(ctx *gin.Context) { experienceIDAction(ctx, false) }
func SalesExperiencePostRetry(ctx *gin.Context)    { experienceIDAction(ctx, true) }

func SalesExperiencePostCancel(ctx *gin.Context) {
	if experienceAuth(ctx, true) == nil {
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	if !experienceWriteError(ctx, services.SalesExperienceService.Cancel(req.ID)) {
		httpx.WriteJSON(ctx, nil)
	}
}
func experienceIDAction(ctx *gin.Context, retry bool) {
	if experienceAuth(ctx, true) == nil {
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	var err error
	if retry {
		err = services.SalesExperienceService.Retry(req.ID)
	} else {
		err = services.SalesExperienceService.Activate(req.ID)
	}
	if !experienceWriteError(ctx, err) {
		httpx.WriteJSON(ctx, nil)
	}
}
func SalesExperienceGetExport(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	data, err := services.SalesExperienceService.Export(experienceID(ctx))
	if experienceWriteError(ctx, err) {
		return
	}
	ctx.Header("Content-Disposition", "attachment; filename=sales-skill-"+strconv.FormatInt(experienceID(ctx), 10)+".zip")
	ctx.Data(200, "application/zip", data)
}

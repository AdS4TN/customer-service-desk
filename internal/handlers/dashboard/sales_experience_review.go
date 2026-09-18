package dashboard

import (
	"agent-desk/internal/builders"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

func SalesExperienceGetSkills(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	skills, err := services.SalesExperienceService.Skills()
	if experienceWriteError(ctx, err) {
		return
	}
	rows := []response.SalesExperienceWorkspace{}
	for _, skill := range skills {
		w, err := services.SalesExperienceService.SkillWorkspace(skill.ID)
		if experienceWriteError(ctx, err) {
			return
		}
		rows = append(rows, builders.BuildSalesExperienceWorkspace(w))
	}
	httpx.WriteJSON(ctx, web.PageResult{Results: rows, Page: &sqls.Paging{Page: 1, Limit: len(rows), Total: int64(len(rows))}})
}
func SalesExperienceGetSkill(ctx *gin.Context) {
	if experienceAuth(ctx, false) == nil {
		return
	}
	w, err := services.SalesExperienceService.SkillWorkspace(ctx.Query("skill"))
	if !experienceWriteError(ctx, err) {
		httpx.WriteJSON(ctx, builders.BuildSalesExperienceWorkspace(w))
	}
}
func SalesExperiencePostReview(ctx *gin.Context) {
	u := experienceAuth(ctx, true)
	if u == nil {
		return
	}
	var req request.ReviewSalesExperience
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	if !experienceWriteError(ctx, services.SalesExperienceService.ReviewSkill(req, u.UserID)) {
		httpx.WriteJSON(ctx, nil)
	}
}
func SalesExperiencePostSaveSkill(ctx *gin.Context) {
	u := experienceAuth(ctx, true)
	if u == nil {
		return
	}
	var req request.SaveSalesExperienceSkill
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	if !experienceWriteError(ctx, services.SalesExperienceService.SaveCurrentSkill(req, u.UserID)) {
		httpx.WriteJSON(ctx, nil)
	}
}
func SalesExperiencePostReviewBatch(ctx *gin.Context) {
	u := experienceAuth(ctx, true)
	if u == nil {
		return
	}
	var req request.ReviewSalesExperienceBatch
	if err := params.ReadJSON(ctx, &req); experienceWriteError(ctx, err) {
		return
	}
	if !experienceWriteError(ctx, services.SalesExperienceService.ReviewSkills(req, u.UserID)) {
		httpx.WriteJSON(ctx, nil)
	}
}

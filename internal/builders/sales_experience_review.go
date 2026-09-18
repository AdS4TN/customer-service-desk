package builders

import (
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/services"
)

func BuildSalesExperienceWorkspace(w *services.ExperienceWorkspace) response.SalesExperienceWorkspace {
	r := response.SalesExperienceWorkspace{SkillID: w.SkillID, Current: BuildSalesExperienceRevision(w.Current), Suggestions: []response.SalesExperienceSuggestion{}}
	for _, s := range w.Suggestions {
		r.Suggestions = append(r.Suggestions, response.SalesExperienceSuggestion{ID: s.ID, JobID: s.JobID, CreatedAt: s.CreatedAt, Changes: s.Changes})
	}
	return r
}

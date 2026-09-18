package services

import (
	"encoding/json"
	"slices"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	sx "agent-desk/internal/pkg/salesexperience"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type ExperienceSuggestion struct {
	ID        int64
	JobID     int64
	CreatedAt time.Time
	Changes   []sx.RuleDelta
}
type ExperienceWorkspace struct {
	SkillID     string
	Current     models.SalesExperienceRevision
	Suggestions []ExperienceSuggestion
}

func experiencePayload(raw string) (sx.Payload, error) {
	var p sx.Payload
	err := json.Unmarshal([]byte(raw), &p)
	return p, err
}
func experienceDeltas(db *gorm.DB, row *models.SalesExperienceRevision) ([]sx.RuleDelta, error) {
	base, err := experienceRepo.Revision(db, row.ParentID)
	if err != nil {
		return nil, err
	}
	b, err := experiencePayload(base.Payload)
	if err != nil {
		return nil, err
	}
	raw := row.Payload
	if row.ReviewPayload != "" {
		raw = row.ReviewPayload
	}
	p, err := experiencePayload(raw)
	if err != nil {
		return nil, err
	}
	return sx.ReviewDiff(b, p), nil
}
func (s *salesExperienceService) SkillWorkspace(id string) (*ExperienceWorkspace, error) {
	if !sx.KnownSkill(id) {
		return nil, experienceError("notFound")
	}
	skill, err := experienceRepo.Skill(sqls.DB(), id)
	if err != nil {
		return nil, err
	}
	current, err := experienceRepo.Revision(sqls.DB(), skill.ActiveRevisionID)
	if err != nil {
		return nil, err
	}
	rows, err := experienceRepo.PendingReviews(sqls.DB(), id)
	if err != nil {
		return nil, err
	}
	w := &ExperienceWorkspace{SkillID: id, Current: *current, Suggestions: []ExperienceSuggestion{}}
	live, err := experiencePayload(current.Payload)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == skill.ActiveRevisionID {
			continue
		}
		deltas, err := experienceDeltas(sqls.DB(), &row)
		if err != nil {
			return nil, err
		}
		decisions := map[string]string{}
		if row.ReviewDecisions != "" {
			if err := json.Unmarshal([]byte(row.ReviewDecisions), &decisions); err != nil {
				return nil, err
			}
		}
		pending := []sx.RuleDelta{}
		for _, d := range deltas {
			if decisions[d.RuleID] == "" {
				for _, rule := range live.Rules {
					if rule.ID == d.RuleID {
						r := rule
						d.Current = &r
						break
					}
				}
				_, ok := sx.MergeReview(sx.Payload{Rules: slices.Clone(live.Rules)}, d, d.After)
				d.Conflict = !ok
				pending = append(pending, d)
			}
		}
		if len(pending) > 0 {
			w.Suggestions = append(w.Suggestions, ExperienceSuggestion{ID: row.ID, JobID: row.JobID, CreatedAt: row.CreatedAt, Changes: pending})
		}
	}
	return w, nil
}
func (s *salesExperienceService) ReviewSkill(req request.ReviewSalesExperience, userID int64) error {
	row, err := experienceRepo.Revision(sqls.DB(), req.ProposalID)
	if err != nil {
		return err
	}
	return s.ReviewSkills(request.ReviewSalesExperienceBatch{SkillID: row.SkillID, ExpectedID: req.ExpectedID, Items: []request.ReviewSalesExperience{req}}, userID)
}
func (s *salesExperienceService) ReviewSkills(req request.ReviewSalesExperienceBatch, userID int64) error {
	if len(req.Items) == 0 || req.ExpectedID <= 0 {
		return experienceError("state")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		skill, err := experienceRepo.Skill(ctx.Tx, req.SkillID)
		if err != nil {
			return err
		}
		if skill.ActiveRevisionID != req.ExpectedID {
			return experienceError("reviewConflict")
		}
		for _, item := range req.Items {
			if err := s.reviewSkill(ctx.Tx, req.SkillID, item, userID); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *salesExperienceService) reviewSkill(db *gorm.DB, skillID string, req request.ReviewSalesExperience, userID int64) error {
	if !slices.Contains([]string{"accept", "ignore"}, req.Decision) {
		return experienceError("state")
	}
	row, err := experienceRepo.ReviewRevision(db, req.ProposalID)
	if err != nil {
		return err
	}
	if row.SkillID != skillID {
		return experienceError("state")
	}
	decisions := map[string]string{}
	if row.ReviewDecisions != "" {
		if err := json.Unmarshal([]byte(row.ReviewDecisions), &decisions); err != nil {
			return err
		}
	}
	if prev := decisions[req.RuleID]; prev != "" {
		if prev == req.Decision {
			return nil
		}
		return experienceError("state")
	}
	if row.ReviewState != "pending" || row.JobID == 0 {
		return experienceError("state")
	}
	deltas, err := experienceDeltas(db, row)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(deltas, func(d sx.RuleDelta) bool { return d.RuleID == req.RuleID })
	if i < 0 {
		return experienceError("state")
	}
	d := deltas[i]
	if req.Decision == "accept" {
		rule := d.After
		if req.Rule != nil {
			rule = req.Rule
		}
		if d.Kind == "remove" && req.Rule != nil || rule != nil && (rule.ID != d.RuleID || sx.ValidateRules([]sx.Rule{*rule}) != nil) {
			return experienceError("rules")
		}
		skill, err := experienceRepo.Skill(db, row.SkillID)
		if err != nil {
			return err
		}
		current, err := experienceRepo.Revision(db, skill.ActiveRevisionID)
		if err != nil {
			return err
		}
		p, err := experiencePayload(current.Payload)
		if err != nil {
			return err
		}
		if req.Rule != nil {
			d.Before = nil
			for _, live := range p.Rules {
				if live.ID == d.RuleID {
					r := live
					d.Before = &r
					break
				}
			}
		}
		p, ok := sx.MergeReview(p, d, rule)
		if !ok {
			return experienceError("reviewConflict")
		}
		if rule != nil && d.After != nil && *rule != *d.After {
			p.Provenance = "manual"
		}
		next := models.SalesExperienceRevision{SkillID: row.SkillID, ParentID: current.ID, Payload: sx.Encode(p), Hash: sx.Hash(p), Note: "review", CreatedBy: userID, ReviewState: "reviewed"}
		if err := experienceRepo.CreateRevision(db, &next); err != nil {
			return err
		}
		if err := experienceRepo.Activate(db, row.SkillID, next.ID); err != nil {
			return err
		}
	}
	decisions[req.RuleID] = req.Decision
	state := "reviewed"
	for _, d := range deltas {
		if decisions[d.RuleID] == "" {
			state = "pending"
			break
		}
	}
	return experienceRepo.SaveReview(db, row.ID, state, sx.Encode(decisions))
}
func (s *salesExperienceService) SaveCurrentSkill(req request.SaveSalesExperienceSkill, userID int64) error {
	if sx.ValidateRules(req.Rules) != nil {
		return experienceError("rules")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		skill, err := experienceRepo.Skill(ctx.Tx, req.SkillID)
		if err != nil {
			return err
		}
		if skill.ActiveRevisionID != req.ExpectedID {
			return experienceError("reviewConflict")
		}
		current, err := experienceRepo.Revision(ctx.Tx, skill.ActiveRevisionID)
		if err != nil {
			return err
		}
		p, err := experiencePayload(current.Payload)
		if err != nil {
			return err
		}
		p.Rules = req.Rules
		p.Provenance = "manual"
		next := models.SalesExperienceRevision{SkillID: skill.ID, ParentID: current.ID, Payload: sx.Encode(p), Hash: sx.Hash(p), Note: "manual", CreatedBy: userID, ReviewState: "reviewed"}
		if err := experienceRepo.CreateRevision(ctx.Tx, &next); err != nil {
			return err
		}
		return experienceRepo.Activate(ctx.Tx, skill.ID, next.ID)
	})
}

package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"agent-desk/internal/models"
	sx "agent-desk/internal/pkg/salesexperience"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func distillScope(input sx.ExperimentInput) string {
	type caseScope struct {
		ID         int64  `json:"id"`
		SourceHash string `json:"sourceHash"`
	}
	type baseScope struct {
		SkillID    string `json:"skillId"`
		RevisionID int64  `json:"revisionId"`
	}
	cases := make([]caseScope, 0, len(input.Cases))
	for _, item := range input.Cases {
		cases = append(cases, caseScope{ID: item.ID, SourceHash: item.SourceHash})
	}
	bases := make([]baseScope, 0, len(input.Bases))
	for _, skillID := range sx.SkillIDs {
		base, ok := input.Bases[skillID]
		if ok {
			bases = append(bases, baseScope{SkillID: skillID, RevisionID: base.RevisionID})
		}
	}
	return sx.Hash(struct {
		Cases     []caseScope `json:"cases"`
		Bases     []baseScope `json:"bases"`
		IncludeAI bool        `json:"includeAI"`
	}{Cases: cases, Bases: bases, IncludeAI: input.IncludeAI})
}

func (s *salesExperienceService) supersedeDuplicateReviews(db *gorm.DB) error {
	for _, skillID := range sx.SkillIDs {
		rows, err := experienceRepo.PendingReviews(db, skillID)
		if err != nil {
			return err
		}
		latest := map[string]int64{}
		for _, row := range rows {
			if decisions := strings.TrimSpace(row.ReviewDecisions); decisions != "" && decisions != "{}" {
				continue
			}
			job, err := experienceRepo.Job(db, row.JobID)
			if err != nil {
				return err
			}
			var input sx.ExperimentInput
			if json.Unmarshal([]byte(job.Input), &input) != nil || len(input.Cases) == 0 || len(input.Bases) == 0 {
				continue
			}
			key := fmt.Sprintf("%d:%s", row.ParentID, distillScope(input))
			if previous := latest[key]; previous > 0 {
				if err := experienceRepo.SupersedeReview(db, previous); err != nil {
					return err
				}
			}
			latest[key] = row.ID
		}
	}
	return nil
}

func (s *salesExperienceService) ProcessPending() {
	if !s.mu.TryLock() {
		return
	}
	defer s.mu.Unlock()
	job, err := experienceRepo.Claim(sqls.DB())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		slog.Error("sales experience queue unavailable")
		return
	}
	if err = s.process(job); err != nil {
		code := err.Error()
		if !slices.Contains([]string{"format", "evidence", "timeout", "modelCall", "truncated", "modelChanged"}, code) {
			code = "failed"
		}
		if e := experienceRepo.UpdateRunningJob(sqls.DB(), job.ID, map[string]any{"state": "failed", "error_code": code}); e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			slog.Error("sales experience checkpoint failed", "jobId", job.ID)
		}
	}
}
func (s *salesExperienceService) process(job *models.SalesExperienceJob) error {
	var input sx.ExperimentInput
	var out sx.Output
	var cfg models.AIConfig
	if err := json.Unmarshal([]byte(job.Input), &input); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(job.Output), &out); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(job.ModelSnapshot), &cfg); err != nil {
		return err
	}
	current, err := experienceModel(job.ModelConfigID)
	if err != nil {
		return err
	}
	// Credentials may rotate, but a queued run must not silently switch models/providers.
	if experienceModelSnapshot(*current) != job.ModelSnapshot {
		return errors.New("modelChanged")
	}
	cfg.APIKey = current.APIKey
	// Do not carry old-language summaries or proposals into a new prompt version.
	if job.Kind == "distill" && input.DistillVersion != sx.DistillPromptVersion {
		input.DistillVersion = sx.DistillPromptVersion
		out = sx.Output{}
		if err := experienceRepo.UpdateRunningJob(sqls.DB(), job.ID, map[string]any{"input": sx.Encode(input), "output": sx.Encode(out), "stage": "queued"}); err != nil {
			return err
		}
	}
	if job.Kind == "evaluate" {
		if out.Replies == nil {
			out.Replies = map[string]string{}
		}
		for _, label := range []string{"A", "B"} {
			if out.Replies[label] != "" {
				continue
			}
			text, err := s.call(cfg, sx.ReplyPrompt, map[string]any{"conversation": input.Prefix, "business_context": input.BusinessContext, "skill_rules": input.Variants[label]})
			if err != nil {
				return err
			}
			out.Replies[label] = text
			if err := s.checkpoint(job, "evaluation:"+label, out); err != nil {
				return err
			}
		}
		return experienceRepo.UpdateRunningJob(sqls.DB(), job.ID, map[string]any{"state": "succeeded", "stage": "complete", "output": sx.Encode(out), "error_code": ""})
	}
	if out.Payloads == nil {
		out.Payloads = map[string]sx.Payload{}
		for id, base := range input.Bases {
			out.Payloads[id] = base.Payload
		}
	}
	for caseIndex, c := range input.Cases {
		windows := sx.Windows(sx.ExtractMessages(c.Snapshot, input.IncludeAI), 18000)
		if len(out.Cases) <= caseIndex {
			out.Cases = append(out.Cases, sx.CaseResult{CaseID: c.ID, Events: []sx.Event{}, WindowTotal: len(windows)})
		}
		state := &out.Cases[caseIndex]
		for wi := state.WindowDone; wi < len(windows); wi++ {
			if err := s.checkpoint(job, fmt.Sprintf("case:%d/%d:window:%d/%d", caseIndex+1, len(input.Cases), wi+1, len(windows)), out); err != nil {
				return err
			}
			text, err := s.callDistill(job, cfg, sx.ExtractPrompt, map[string]any{"customer": c.Snapshot.CustomerName, "conversationIds": c.Snapshot.ConversationIDs, "previous_summary": state.Summary, "messages": windows[wi], "outcome": c.Outcome, "outcome_note": c.OutcomeNote, "includeAI": input.IncludeAI})
			if err != nil {
				return err
			}
			var result sx.EventsOutput
			badWindow := false
			if err := sx.ParseJSON(text, &result); err != nil {
				if err.Error() != "format" {
					return err
				}
				badWindow = true
			}
			if err := sx.ValidateEvents(result, windows[wi]); err != nil {
				if err.Error() != "format" && err.Error() != "evidence" {
					return err
				}
				badWindow = true
			}
			if badWindow {
				result = sx.EventsOutput{Summary: state.Summary, Events: []sx.Event{}}
				state.Skipped++
			}
			signatures := map[string]bool{}
			for _, e := range state.Events {
				signatures[sx.EventSignature(e)] = true
			}
			for _, e := range result.Events {
				sig := sx.EventSignature(e)
				if signatures[sig] {
					continue
				}
				signatures[sig] = true
				e.ID = fmt.Sprintf("case-%d-event-%d", c.ID, len(state.Events)+1)
				state.Events = append(state.Events, e)
			}
			state.Summary = result.Summary
			state.WindowDone = wi + 1
			if err := s.checkpoint(job, fmt.Sprintf("case:%d/%d:window:%d/%d", caseIndex+1, len(input.Cases), wi+1, len(windows)), out); err != nil {
				return err
			}
		}
		if state.Proposed {
			continue
		}
		if len(state.Events) > 0 {
			rules := map[string][]sx.Rule{}
			for id, p := range out.Payloads {
				rules[id] = p.Rules
			}
			if err := s.checkpoint(job, fmt.Sprintf("case:%d/%d:proposals", caseIndex+1, len(input.Cases)), out); err != nil {
				return err
			}
			text, err := s.callDistill(job, cfg, sx.ChangePrompt, map[string]any{"skills": rules, "supplied_events": state.Events})
			if err != nil {
				return err
			}
			var result struct {
				Changes []sx.Change `json:"changes"`
			}
			if err := sx.ParseJSON(text, &result); err != nil {
				return err
			}
			if result.Changes == nil {
				return errors.New("format")
			}
			payloads, rejected, err := sx.ProposeGated(result.Changes, state.Events, out.Payloads, c)
			if err != nil {
				return err
			}
			out.Payloads = payloads
			state.Skipped += len(rejected)
		}
		state.Proposed = true
		if err := s.checkpoint(job, fmt.Sprintf("case:%d/%d:complete", caseIndex+1, len(input.Cases)), out); err != nil {
			return err
		}
	}
	// New immutable versions and completion are committed together, making retries idempotent.
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		out.RevisionIDs = []int64{}
		for _, id := range sx.SkillIDs {
			p := out.Payloads[id]
			base := input.Bases[id]
			if sx.Hash(p) == sx.Hash(base.Payload) {
				continue
			}
			rows, err := experienceRepo.PendingReviews(ctx.Tx, id)
			if err != nil {
				return err
			}
			for _, previous := range rows {
				if previous.ParentID != base.RevisionID || previous.JobID == job.ID {
					continue
				}
				if decisions := strings.TrimSpace(previous.ReviewDecisions); decisions != "" && decisions != "{}" {
					continue
				}
				previousJob, err := experienceRepo.Job(ctx.Tx, previous.JobID)
				if err != nil {
					return err
				}
				var previousInput sx.ExperimentInput
				if json.Unmarshal([]byte(previousJob.Input), &previousInput) == nil && distillScope(previousInput) == distillScope(input) {
					if err := experienceRepo.SupersedeReview(ctx.Tx, previous.ID); err != nil {
						return err
					}
				}
			}
			row := models.SalesExperienceRevision{SkillID: id, ParentID: base.RevisionID, JobID: job.ID, Payload: sx.Encode(p), Hash: sx.Hash(p), CreatedBy: job.CreatedBy, ReviewState: "pending"}
			if err := experienceRepo.CreateRevision(ctx.Tx, &row); err != nil {
				return err
			}
			out.RevisionIDs = append(out.RevisionIDs, row.ID)
		}
		return experienceRepo.UpdateRunningJob(ctx.Tx, job.ID, map[string]any{"state": "succeeded", "stage": "complete", "output": sx.Encode(out), "error_code": ""})
	})
}

package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	sx "agent-desk/internal/pkg/salesexperience"
	"testing"
)

func TestSalesExperienceReviewWorkflow(t *testing.T) {
	db, s := setupExperience(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	w, err := s.SkillWorkspace("negotiation")
	must(err)
	baseID := w.Current.ID
	base, err := experiencePayload(w.Current.Payload)
	must(err)
	a := sx.Rule{ID: "synthetic-a", Condition: "客户压价", Action: "先确认比较口径", Exceptions: "不承诺折扣"}
	b := sx.Rule{ID: "synthetic-b", Condition: "客户犹豫", Action: "确认阻碍"}
	proposed := base
	proposed.Rules = append(append([]sx.Rule{}, base.Rules...), a, b)
	row := models.SalesExperienceRevision{SkillID: w.SkillID, ParentID: baseID, JobID: 99, Payload: sx.Encode(proposed), ReviewState: "pending"}
	must(db.Create(&row).Error)
	w, err = s.SkillWorkspace(w.SkillID)
	must(err)
	if w.Current.ID != baseID || len(w.Suggestions) != 1 || len(w.Suggestions[0].Changes) != 2 {
		t.Fatal("proposal activated or missing")
	}
	if s.Activate(row.ID) == nil {
		t.Fatal("legacy activation bypassed review")
	}
	must(s.ReviewSkill(request.ReviewSalesExperience{ProposalID: row.ID, RuleID: a.ID, Decision: "accept", ExpectedID: baseID}, 1))
	w, err = s.SkillWorkspace(w.SkillID)
	must(err)
	p, err := experiencePayload(w.Current.Payload)
	must(err)
	if len(p.Rules) != len(base.Rules)+1 || len(w.Suggestions[0].Changes) != 1 {
		t.Fatal("review merged unselected changes")
	}
	if s.ReviewSkill(request.ReviewSalesExperience{ProposalID: row.ID, RuleID: b.ID, Decision: "accept", ExpectedID: baseID}, 1) == nil {
		t.Fatal("stale review accepted")
	}
	must(s.ReviewSkill(request.ReviewSalesExperience{ProposalID: row.ID, RuleID: b.ID, Decision: "ignore", ExpectedID: w.Current.ID}, 1))
	w, err = s.SkillWorkspace(w.SkillID)
	must(err)
	if len(w.Suggestions) != 0 {
		t.Fatal("ignored proposal still pending")
	}
	if s.SaveCurrentSkill(request.SaveSalesExperienceSkill{SkillID: w.SkillID, ExpectedID: baseID, Rules: []sx.Rule{}}, 1) == nil {
		t.Fatal("stale manual edit accepted")
	}
	must(s.SaveCurrentSkill(request.SaveSalesExperienceSkill{SkillID: w.SkillID, ExpectedID: w.Current.ID, Rules: []sx.Rule{}}, 1))
	var sends int64
	must(db.Model(&models.ChannelMessageOutbox{}).Count(&sends).Error)
	if sends != 0 {
		t.Fatal("review produced outbound messages")
	}
}

func TestSalesExperienceReviewBatchAtomicConflict(t *testing.T) {
	db, s := setupExperience(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	w, err := s.SkillWorkspace("negotiation")
	must(err)
	base, err := experiencePayload(w.Current.Payload)
	must(err)
	first := base.Rules[0]
	changed := first
	changed.Action = "本次修改"
	added := sx.Rule{ID: "new", Condition: "新场景", Action: "新动作"}
	proposed := base
	proposed.Rules = []sx.Rule{changed, added}
	row := models.SalesExperienceRevision{SkillID: w.SkillID, ParentID: w.Current.ID, JobID: 99, Payload: sx.Encode(proposed), ReviewState: "pending"}
	must(db.Create(&row).Error)
	manual := first
	manual.Action = "人工已修改"
	must(s.SaveCurrentSkill(request.SaveSalesExperienceSkill{SkillID: w.SkillID, ExpectedID: w.Current.ID, Rules: []sx.Rule{manual}}, 1))
	w, err = s.SkillWorkspace(w.SkillID)
	must(err)
	if !w.Suggestions[0].Changes[0].Conflict {
		t.Fatal("missing live conflict")
	}
	batch := request.ReviewSalesExperienceBatch{SkillID: w.SkillID, ExpectedID: w.Current.ID, Items: []request.ReviewSalesExperience{
		{ProposalID: row.ID, RuleID: added.ID, Decision: "accept"},
		{ProposalID: row.ID, RuleID: changed.ID, Decision: "accept"},
	}}
	if s.ReviewSkills(batch, 1) == nil {
		t.Fatal("conflict accepted")
	}
	after, err := s.SkillWorkspace(w.SkillID)
	must(err)
	if after.Current.ID != w.Current.ID || len(after.Suggestions[0].Changes) != 2 {
		t.Fatal("batch partially applied")
	}
	batch.Items[1].Rule = &changed
	must(s.ReviewSkills(batch, 1))
	after, err = s.SkillWorkspace(w.SkillID)
	must(err)
	if len(after.Suggestions) != 0 {
		t.Fatal("resolved batch still pending")
	}
	p, err := experiencePayload(after.Current.Payload)
	must(err)
	if len(p.Rules) != 2 {
		t.Fatal("batch did not preserve accepted addition")
	}
}

func TestSalesExperienceSupersedesOnlyDuplicateUnreviewedExtraction(t *testing.T) {
	db, s := setupExperience(t)
	skill, err := experienceRepo.Skill(db, "negotiation")
	if err != nil {
		t.Fatal(err)
	}
	input := sx.ExperimentInput{
		Cases: []sx.CaseInput{{ID: 10, SourceHash: "same-snapshot"}},
		Bases: map[string]sx.Base{
			"negotiation": {RevisionID: skill.ActiveRevisionID},
			"decision":    {RevisionID: 2},
			"closing":     {RevisionID: 3},
		},
	}
	firstJob := models.SalesExperienceJob{Kind: "distill", State: "succeeded", Input: sx.Encode(input)}
	secondJob := models.SalesExperienceJob{Kind: "distill", State: "succeeded", Input: sx.Encode(input)}
	if err := db.Create(&firstJob).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&secondJob).Error; err != nil {
		t.Fatal(err)
	}
	first := models.SalesExperienceRevision{SkillID: skill.ID, ParentID: skill.ActiveRevisionID, JobID: firstJob.ID, Payload: "{}", ReviewState: "pending"}
	second := models.SalesExperienceRevision{SkillID: skill.ID, ParentID: skill.ActiveRevisionID, JobID: secondJob.ID, Payload: "{}", ReviewState: "pending"}
	partial := models.SalesExperienceRevision{SkillID: skill.ID, ParentID: skill.ActiveRevisionID, JobID: firstJob.ID, Payload: "{}", ReviewState: "pending", ReviewDecisions: `{"existing":"accept"}`}
	for _, row := range []*models.SalesExperienceRevision{&first, &second, &partial} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&models.SalesExperienceRevision{}).Where("id = ?", first.ID).Update("review_decisions", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.supersedeDuplicateReviews(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&first, first.ID).Error; err != nil || first.ReviewState != "superseded" {
		t.Fatal("older unreviewed duplicate was not superseded")
	}
	if err := db.First(&second, second.ID).Error; err != nil || second.ReviewState != "pending" {
		t.Fatal("newest unreviewed extraction should remain pending")
	}
	if err := db.First(&partial, partial.ID).Error; err != nil || partial.ReviewState != "pending" {
		t.Fatal("partially reviewed extraction must not be superseded")
	}
}

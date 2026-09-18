package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	sx "agent-desk/internal/pkg/salesexperience"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupExperience(t *testing.T) (*gorm.DB, *salesExperienceService) {
	t.Helper()
	db := setupTelegramTestDB(t)
	if err := db.AutoMigrate(&models.SalesExperienceCase{}, &models.SalesExperienceSkill{}, &models.SalesExperienceRevision{}, &models.SalesExperienceJob{}, &models.AIConfig{}, &models.SkillDefinition{}); err != nil {
		t.Fatal(err)
	}
	s := &salesExperienceService{}
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.AIConfig{ID: 1, Name: "Synthetic", ModelType: enums.AIModelTypeLLM, ModelName: "mock", APIKey: "synthetic-secret", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	return db, s
}
func TestSalesExperienceImportFullHistory(t *testing.T) {
	db, s := setupExperience(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Create(&models.Channel{ID: 1, Name: "Synthetic WhatsApp", ChannelType: "whatsapp"}).Error)
	for i := 1; i <= 3; i++ {
		customer := int64(10)
		if i == 3 {
			customer = 20
		}
		must(db.Create(&models.Conversation{ID: int64(i), CustomerID: customer, CustomerName: fmt.Sprintf("Customer %d", customer), ChannelID: 1, Status: enums.IMConversationStatusClosed}).Error)
	}
	for i := 1; i <= 123; i++ {
		conv := int64(1)
		if i > 60 {
			conv = 2
		}
		if i > 120 {
			conv = 3
		}
		role := enums.IMSenderTypeCustomer
		if i%2 == 0 {
			role = enums.IMSenderTypeAgent
		}
		must(db.Create(&models.Message{ID: int64(i), ConversationID: conv, ClientMsgID: fmt.Sprint(i), SenderType: role, MessageType: enums.IMMessageTypeText, Content: fmt.Sprintf("Message %d", i), IsHistorical: true, AuditFields: models.AuditFields{CreatedAt: time.Unix(int64(i), 0)}}).Error)
	}
	rows, total, err := s.Sources("Customer", "whatsapp", 1, 2)
	must(err)
	if len(rows) != 2 || total != 3 {
		t.Fatalf("sources pagination/all statuses: %d %d", len(rows), total)
	}
	preview, err := s.ImportPreview([]int64{2, 1})
	must(err)
	if len(preview) != 2 || len(preview[0].Messages) != 60 || len(preview[1].Messages) != 60 {
		t.Fatalf("import preview did not preserve conversation messages: %+v", preview)
	}
	selected, err := s.Import(request.ImportSalesExperience{Selections: []request.SalesExperienceImportSelection{
		{ConversationID: 1, MessageIDs: []int64{1, 2}},
		{ConversationID: 2, MessageIDs: []int64{61, 62}},
	}}, 1)
	must(err)
	if len(selected) != 1 || selected[0].MessageCount != 4 {
		t.Fatalf("selected import included unselected messages: %+v", selected)
	}
	var selectedSnapshot sx.Snapshot
	must(json.Unmarshal([]byte(selected[0].Snapshot), &selectedSnapshot))
	got := []int64{selectedSnapshot.Messages[0].ID, selectedSnapshot.Messages[1].ID, selectedSnapshot.Messages[2].ID, selectedSnapshot.Messages[3].ID}
	if !slices.Equal(got, []int64{1, 2, 61, 62}) {
		t.Fatalf("selected import changed message set: %v", got)
	}
	if _, err = s.Import(request.ImportSalesExperience{Selections: []request.SalesExperienceImportSelection{{ConversationID: 1, MessageIDs: []int64{9999}}}}, 1); err == nil {
		t.Fatal("import accepted a message outside the selected conversation")
	}
	imported, err := s.Import(request.ImportSalesExperience{ConversationIDs: []int64{3, 2, 1, 1}}, 1)
	must(err)
	if len(imported) != 2 || imported[0].MessageCount != 120 {
		t.Fatalf("full customer history missing: %+v", imported)
	}
	again, err := s.Import(request.ImportSalesExperience{ConversationIDs: []int64{1, 2, 3}}, 1)
	must(err)
	if again[0].ID != imported[0].ID {
		t.Fatal("duplicate snapshot")
	}
	must(db.Create(&models.Message{ConversationID: 1, ClientMsgID: "new", SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText, Content: "New history"}).Error)
	changed, err := s.Import(request.ImportSalesExperience{ConversationIDs: []int64{1, 2}}, 1)
	must(err)
	if changed[0].ID == imported[0].ID || changed[0].MessageCount != 121 {
		t.Fatal("changed history did not create snapshot")
	}
	for _, m := range []models.Message{{SenderType: enums.IMSenderTypeSystem}, {SenderType: enums.IMSenderTypeAgent, SendStatus: enums.IMMessageStatusFailed}, {SenderType: enums.IMSenderTypeAgent, SendStatus: enums.IMMessageStatusSending}} {
		if _, ok := normalizeExperienceMessage(m); ok {
			t.Fatal("excluded message imported")
		}
	}
	nontext, ok := normalizeExperienceMessage(models.Message{SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeImage, Content: "Not OCR"})
	if !ok || nontext.Text != "" {
		t.Fatal("fabricated attachment text")
	}
	var n int64
	must(db.Model(&models.ChannelMessageOutbox{}).Count(&n).Error)
	if n != 0 {
		t.Fatal("import sent messages")
	}
}

func TestSalesExperienceDeleteCaseKeepsSourceAndSkills(t *testing.T) {
	db, s := setupExperience(t)
	conversation := models.Conversation{ID: 101, CustomerName: "Delete test", Status: enums.IMConversationStatusClosed}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	row := models.SalesExperienceCase{
		Name:             "Imported delete test",
		SourceHash:       "delete-case-source",
		Snapshot:         `{}`,
		Outcome:          "unknown",
		ExtractionStatus: "succeeded",
		ExtractionResult: `{"ok":true}`,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCase(row.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Case(row.ID); err == nil {
		t.Fatal("deleted case remains available")
	}
	var sourceCount, skillCount int64
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Count(&sourceCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.SalesExperienceSkill{}).Count(&skillCount).Error; err != nil {
		t.Fatal(err)
	}
	if sourceCount != 1 || skillCount != int64(len(sx.SkillIDs)) {
		t.Fatalf("delete changed source or Skills: source=%d skills=%d", sourceCount, skillCount)
	}

	running := models.SalesExperienceCase{Name: "Running", SourceHash: "running-case", Snapshot: `{}`, Outcome: "unknown", ExtractionStatus: "running"}
	if err := db.Create(&running).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCase(running.ID); err == nil {
		t.Fatal("running extraction case was deleted")
	}
	if _, err := s.Case(running.ID); err != nil {
		t.Fatal("running extraction case should remain")
	}
}
func TestSalesExperienceAblationAndVersions(t *testing.T) {
	db, s := setupExperience(t)
	snapshot := sx.Snapshot{Messages: []sx.Message{{ID: 1, Role: "buyer", Type: "text", Text: "Too expensive"}, {ID: 2, Role: "seller", Type: "text", Text: "FUTURE REPLY"}}}
	c := models.SalesExperienceCase{Snapshot: sx.Encode(snapshot), SourceHash: sx.Hash(snapshot), Outcome: "won", OutcomeNote: "FUTURE SALE"}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	skill, _ := experienceRepo.Skill(db, "negotiation")
	job, err := s.Evaluate(request.EvaluateSalesExperience{CaseID: c.ID, SkillID: skill.ID, CutoffID: 1, ModelConfigID: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(job.Input, "FUTURE") {
		t.Fatal("future context leaked")
	}
	var selected sx.ExperimentInput
	if err := json.Unmarshal([]byte(job.Input), &selected); err != nil || selected.RevisionID != skill.ActiveRevisionID {
		t.Fatal("evaluation did not select current confirmed Skill")
	}
	calls, mounted := 0, 0
	s.complete = func(_ context.Context, _ models.AIConfig, _ string, input string) (*ai.ChatCompletionResult, error) {
		calls++
		var v struct {
			Rules []sx.Rule `json:"skill_rules"`
		}
		_ = json.Unmarshal([]byte(input), &v)
		if len(v.Rules) > 0 {
			mounted++
			if v.Rules[0].ID != "price-reason" {
				t.Fatal("wrong Skill")
			}
		}
		return &ai.ChatCompletionResult{Content: "Synthetic response"}, nil
	}
	s.ProcessPending()
	completed, _ := s.Job(job.ID)
	if completed.State != "succeeded" || calls != 2 || mounted != 1 {
		t.Fatalf("ablation %s calls=%d mounted=%d", completed.State, calls, mounted)
	}
	if err = s.Rate(request.RateSalesExperience{ID: job.ID, Rating: "A", Note: "Synthetic assessment"}); err != nil {
		t.Fatal(err)
	}
	rated, _ := s.Job(job.ID)
	if rated.Rating != "A" {
		t.Fatal("rating not saved")
	}
	edit, err := s.Edit(request.EditSalesExperience{RevisionID: skill.ActiveRevisionID, Rules: []sx.Rule{}, Note: "Ablation"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := s.Revision(skill.ActiveRevisionID)
	if base.ID == edit.ID || !strings.Contains(base.Payload, "price-reason") {
		t.Fatal("mutated immutable baseline")
	}
	data, err := s.Export(edit.ID)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 5 {
		t.Fatal("incomplete export")
	}
	for _, file := range archive.File {
		r, _ := file.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		if strings.Contains(string(b), "synthetic-secret") {
			t.Fatal("export contains credential")
		}
	}
	var n int64
	db.Model(&models.ChannelMessageOutbox{}).Count(&n)
	if n != 0 {
		t.Fatal("evaluation sent messages")
	}
}
func TestSalesExperienceCheckpointRetry(t *testing.T) {
	db, s := setupExperience(t)
	for id := int64(1); id <= 2; id++ {
		snapshot := sx.Snapshot{CustomerID: id, Messages: []sx.Message{{ID: 1, Role: "buyer", Type: "text", Text: "Price?"}, {ID: 2, Role: "seller", Type: "text", Text: "Budget?"}, {ID: 3, Role: "buyer", Type: "text", Text: "My budget is 100."}}}
		if err := db.Create(&models.SalesExperienceCase{ID: id, Snapshot: sx.Encode(snapshot), SourceHash: sx.Hash(snapshot), Outcome: "unknown"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	job, err := s.Distill(request.DistillSalesExperience{CaseIDs: []int64{1, 2}, ModelConfigID: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.complete = func(_ context.Context, _ models.AIConfig, prompt, input string) (*ai.ChatCompletionResult, error) {
		calls++
		if calls == 3 {
			return nil, errors.New("synthetic failure")
		}
		if prompt == sx.ExtractPrompt {
			return &ai.ChatCompletionResult{Content: `{"summary":"Unknown sale","events":[{"id":"e1","title":"Budget","signal":"Price","sellerAction":"Ask","observedResult":"Buyer clarified the budget","skillIds":["negotiation"],"quotes":[{"messageId":1,"text":"Price?"},{"messageId":2,"text":"Budget?"},{"messageId":3,"text":"My budget is 100."}]}]}`}, nil
		}
		var req struct {
			Events []sx.Event           `json:"supplied_events"`
			Skills map[string][]sx.Rule `json:"skills"`
		}
		_ = json.Unmarshal([]byte(input), &req)
		r := sx.Rule{ID: "budget", Condition: "Price", Action: "Ask budget"}
		op := "add_rule"
		if len(req.Skills["negotiation"]) > 1 {
			op = "support"
		}
		assessment := &sx.LearningAssessment{NonRoutineMove: true, ObservedBuyerEffect: true, TransferableMechanism: true, AddsNewLearning: true, Reason: "The buyer clarified a decision input after the seller's diagnostic question."}
		return &ai.ChatCompletionResult{Content: sx.Encode(map[string]any{"changes": []sx.Change{{SkillID: "negotiation", Operation: op, Rule: &r, EventIDs: []string{req.Events[0].ID}, TransferTests: []string{"software", "retail", "services"}, LearningAssessment: assessment}}})}, nil
	}
	s.ProcessPending()
	failed, _ := s.Job(job.ID)
	if failed.State != "failed" {
		t.Fatal("expected failed checkpoint")
	}
	if err = s.Retry(job.ID); err != nil {
		t.Fatal(err)
	}
	s.ProcessPending()
	done, _ := s.Job(job.ID)
	if done.State != "succeeded" || calls != 5 {
		t.Fatalf("retry did not resume: %s %d", done.State, calls)
	}
	var out sx.Output
	_ = json.Unmarshal([]byte(done.Output), &out)
	if len(out.RevisionIDs) != 1 || len(out.Payloads["negotiation"].Evidence) != 2 {
		t.Fatal("batch accumulation failed")
	}
	if err = s.Retry(job.ID); err == nil {
		t.Fatal("completed task retry accepted")
	}
	var n int64
	sqls.DB().Model(&models.SalesExperienceRevision{}).Count(&n)
	if n != 4 {
		t.Fatal("duplicate revisions")
	}
}

func TestSalesExperienceNoLearnableTechniqueCompletesWithoutRevision(t *testing.T) {
	db, s := setupExperience(t)
	snapshot := sx.Snapshot{CustomerID: 1, Messages: []sx.Message{
		{ID: 1, Role: "buyer", Type: "text", Text: "Please send the quotation."},
		{ID: 2, Role: "seller", Type: "text", Text: "Quotation sent."},
		{ID: 3, Role: "buyer", Type: "text", Text: "Received, thank you."},
	}}
	row := models.SalesExperienceCase{Snapshot: sx.Encode(snapshot), SourceHash: sx.Hash(snapshot), Outcome: "unknown"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	job, err := s.Distill(request.DistillSalesExperience{CaseIDs: []int64{row.ID}, ModelConfigID: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.complete = func(_ context.Context, _ models.AIConfig, prompt, _ string) (*ai.ChatCompletionResult, error) {
		calls++
		if prompt == sx.ExtractPrompt {
			return &ai.ChatCompletionResult{Content: `{"summary":"客户索要并收到报价，未体现可学习的销售技巧。","events":[{"id":"e1","title":"发送报价","signal":"客户索要报价","sellerAction":"发送报价","observedResult":"客户确认收到","uncertainty":"未观察到决策状态变化","skillIds":["closing"],"quotes":[{"messageId":1,"text":"send the quotation"},{"messageId":2,"text":"Quotation sent"},{"messageId":3,"text":"Received"}]}]}`}, nil
		}
		return &ai.ChatCompletionResult{Content: `{"changes":[]}`}, nil
	}
	s.ProcessPending()
	done, err := s.Job(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.State != "succeeded" || calls != 2 {
		t.Fatalf("no-learning task did not complete normally: state=%s calls=%d", done.State, calls)
	}
	var out sx.Output
	if err := json.Unmarshal([]byte(done.Output), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.RevisionIDs) != 0 {
		t.Fatalf("routine conversation created Skill revisions: %v", out.RevisionIDs)
	}
	var revisions int64
	if err := db.Model(&models.SalesExperienceRevision{}).Count(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if revisions != int64(len(sx.SkillIDs)) {
		t.Fatalf("routine conversation persisted a pending revision: %d", revisions)
	}
}

func TestSalesExperienceModelChangeResetsRetry(t *testing.T) {
	db, s := setupExperience(t)
	job, err := s.enqueue("evaluate", sx.ExperimentInput{}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(job.ModelSnapshot, "synthetic-secret") {
		t.Fatal("credential in snapshot")
	}
	if err = experienceRepo.UpdateJob(db, job.ID, map[string]any{"state": "failed", "stage": "evaluation:A", "output": sx.Encode(sx.Output{Replies: map[string]string{"A": "old model"}})}); err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&models.AIConfig{}).Where("id = ?", 1).Update("model_name", "new-mock").Error; err != nil {
		t.Fatal(err)
	}
	if err = s.Retry(job.ID); err != nil {
		t.Fatal(err)
	}
	next, _ := s.Job(job.ID)
	if next.State != "queued" || next.Stage != "queued" || strings.Contains(next.Output, "old model") || !strings.Contains(next.ModelSnapshot, "new-mock") {
		t.Fatal("retry mixed model outputs")
	}
}

func TestSalesExperienceOldLanguageCheckpointRestarts(t *testing.T) {
	db, s := setupExperience(t)
	snapshot := sx.Snapshot{Messages: []sx.Message{{ID: 1, Role: "buyer", Type: "text", Text: "Price?"}, {ID: 2, Role: "seller", Type: "text", Text: "Budget?"}}}
	c := models.SalesExperienceCase{Snapshot: sx.Encode(snapshot), SourceHash: sx.Hash(snapshot), Outcome: "unknown"}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	job, err := s.Distill(request.DistillSalesExperience{CaseIDs: []int64{c.ID}, ModelConfigID: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	var input sx.ExperimentInput
	if err := json.Unmarshal([]byte(job.Input), &input); err != nil {
		t.Fatal(err)
	}
	if input.DistillVersion != sx.DistillPromptVersion {
		t.Fatal("new tasks missing prompt version")
	}
	input.DistillVersion = ""
	old := sx.Output{Cases: []sx.CaseResult{{CaseID: c.ID, Summary: "OLD ENGLISH SUMMARY", WindowDone: 1, WindowTotal: 1, Proposed: true}}}
	if err := experienceRepo.UpdateJob(db, job.ID, map[string]any{"state": "failed", "input": sx.Encode(input), "output": sx.Encode(old)}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.complete = func(_ context.Context, _ models.AIConfig, prompt, body string) (*ai.ChatCompletionResult, error) {
		calls++
		if prompt != sx.ExtractPrompt || strings.Contains(body, "OLD ENGLISH SUMMARY") {
			t.Fatal("reused old-language checkpoint")
		}
		return &ai.ChatCompletionResult{Content: `{"summary":"客户询价，销售询问预算，暂无足够证据。","events":[]}`}, nil
	}
	if err := s.Retry(job.ID); err != nil {
		t.Fatal(err)
	}
	s.ProcessPending()
	done, _ := s.Job(job.ID)
	if done.State != "succeeded" || calls != 1 || strings.Contains(done.Output, "OLD ENGLISH SUMMARY") {
		t.Fatalf("language restart failed: state=%s calls=%d", done.State, calls)
	}
	var updated sx.ExperimentInput
	_ = json.Unmarshal([]byte(done.Input), &updated)
	if updated.DistillVersion != sx.DistillPromptVersion {
		t.Fatal("new checkpoint version not saved")
	}
	original, _ := s.Case(c.ID)
	if original.Snapshot != c.Snapshot {
		t.Fatal("source history mutated")
	}
}

func TestSkillMinerEvidenceValidation(t *testing.T) {
	snapshot := sx.Snapshot{Messages: []sx.Message{
		{ID: 1, Role: "buyer", Type: "text", Text: "The price is higher, but I have not compared the warranty."},
		{ID: 2, Role: "seller", Type: "text", Text: "Which difference matters most to your decision?"},
		{ID: 3, Role: "buyer", Type: "text", Text: "The warranty matters most. Please compare that first."},
	}}
	evidence := skillMinerEvidence{
		CustomerBefore: []skillMinerEvidenceQuote{{MessageID: 1, Quote: "price is higher"}},
		SellerMove:     []skillMinerEvidenceQuote{{MessageID: 2, Quote: "Which difference matters most"}},
		CustomerAfter:  []skillMinerEvidenceQuote{{MessageID: 3, Quote: "warranty matters most"}},
	}
	audit := skillMinerEvidenceAudit{
		ProtocolVersion:      "sales-skill-evidence-audit-v1",
		HasCandidateEpisodes: true,
		Episodes: []skillMinerEvidenceEpisode{{
			ID:                    "episode-1",
			CustomerSituation:     "客户提出价格异议但比较标准不清楚",
			SellerMoveClaim:       "销售追问最重要的判断项",
			ClaimedCustomerChange: "客户说明保修最重要",
			Evidence:              evidence,
		}},
	}
	if err := validateSkillMinerEvidenceAudit(audit, snapshot); err != nil {
		t.Fatalf("valid audit rejected: %v", err)
	}

	wrongRole := audit
	wrongRole.Episodes = append([]skillMinerEvidenceEpisode(nil), audit.Episodes...)
	wrongRole.Episodes[0].Evidence.SellerMove = []skillMinerEvidenceQuote{{MessageID: 1, Quote: "price is higher"}}
	if err := validateSkillMinerEvidenceAudit(wrongRole, snapshot); err == nil {
		t.Fatal("buyer message accepted as seller evidence")
	}

	wrongOrder := audit
	wrongOrder.Episodes = append([]skillMinerEvidenceEpisode(nil), audit.Episodes...)
	wrongOrder.Episodes[0].Evidence.CustomerAfter = []skillMinerEvidenceQuote{{MessageID: 1, Quote: "price is higher"}}
	if err := validateSkillMinerEvidenceAudit(wrongOrder, snapshot); err == nil {
		t.Fatal("out-of-order evidence accepted")
	}

	output := skillMinerOutput{ProtocolVersion: "sales-skill-miner-v2", HasLearnableSkill: true}
	output.Candidates = append(output.Candidates, struct {
		SourceEpisodeID string             `json:"sourceEpisodeId"`
		Skill           json.RawMessage    `json:"skill"`
		Evidence        skillMinerEvidence `json:"evidence"`
	}{SourceEpisodeID: "episode-1", Skill: json.RawMessage(`{"name":"先确认比较标准"}`), Evidence: evidence})
	if err := validateSkillMinerOutput(output, audit); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}
	tampered := evidence
	tampered.CustomerAfter = []skillMinerEvidenceQuote{{MessageID: 3, Quote: "invented"}}
	output.Candidates[0].Evidence = tampered
	if err := validateSkillMinerOutput(output, audit); err == nil {
		t.Fatal("changed evidence accepted")
	}
}

func TestReviewSkillMinerConfirmsEditedSkillIntoDisabledLibraryEntry(t *testing.T) {
	db, s := setupExperience(t)
	stored := `{"ok":true,"model":"mock","result":{"protocolVersion":"sales-skill-miner-v2","hasLearnableSkill":true,"assessment":{"conversationNature":"sales_episode","reason":"buyer advanced"},"candidates":[{"sourceEpisodeId":"episode-1","skill":{"name":"old","description":"old","whenToUse":["old"],"objective":"old","steps":[{"instruction":"old","purpose":"old"}],"successSignals":["old"],"whenNotToUse":["old"]},"evidence":{"customerBefore":[],"sellerMove":[],"customerAfter":[]},"assessment":{"customerChange":"advanced","causalReason":"seller move","transferReason":"reusable","confidence":"high"}}]}}`
	row := models.SalesExperienceCase{Name: "Synthetic", SourceHash: "review-confirm", ExtractionStatus: "succeeded", ExtractionResult: stored}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	skill := request.SalesExperienceMinedSkill{
		Name:           "先诊断异议，再降低决策风险",
		Description:    "将表面价格异议转化为真实决策风险，再设计可逆的下一步。",
		WhenToUse:      []string{"客户以价格为由暂停决策"},
		Objective:      "识别阻力并推动低风险的下一步",
		Steps:          []request.SalesExperienceMinedSkillStep{{Instruction: "追问最严重的决策风险", Purpose: "区分表面异议和真实阻力"}},
		SuccessSignals: []string{"客户说出具体风险并接受下一步"},
		WhenNotToUse:   []string{"客户只是询价且没有表达异议"},
	}
	req := request.ReviewSalesExperienceSkillMiner{CaseID: row.ID, Action: "confirm", SourceEpisodeID: "episode-1", Skill: &skill}
	result, err := s.ReviewSkillMiner(req, &dto.AuthPrincipal{UserID: 7, Username: "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	if result["reviewStatus"] != "confirmed" {
		t.Fatalf("unexpected review result: %+v", result)
	}
	var skills []models.SkillDefinition
	if err := db.Find(&skills).Error; err != nil || len(skills) != 1 {
		t.Fatalf("expected one saved skill: %+v %v", skills, err)
	}
	if skills[0].Status != enums.StatusDisabled || skills[0].Name != skill.Name || !strings.Contains(skills[0].Instruction, "## 执行步骤") {
		t.Fatalf("unexpected saved skill: %+v", skills[0])
	}
	if _, err := s.ReviewSkillMiner(req, &dto.AuthPrincipal{UserID: 7, Username: "reviewer"}); err == nil {
		t.Fatal("confirmed result accepted twice")
	}
	var count int64
	db.Model(&models.SkillDefinition{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate skill created: %d", count)
	}
}

func TestReviewSkillMinerDismissesWithoutCreatingSkill(t *testing.T) {
	db, s := setupExperience(t)
	row := models.SalesExperienceCase{Name: "Synthetic", SourceHash: "review-dismiss", ExtractionStatus: "succeeded", ExtractionResult: `{"ok":true,"result":{"candidates":[{"sourceEpisodeId":"episode-1"}]}}`}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	result, err := s.ReviewSkillMiner(request.ReviewSalesExperienceSkillMiner{CaseID: row.ID, Action: "dismiss", SourceEpisodeID: "episode-1"}, &dto.AuthPrincipal{UserID: 7, Username: "reviewer"})
	if err != nil || result["reviewStatus"] != "dismissed" {
		t.Fatalf("dismiss failed: %+v %v", result, err)
	}
	var count int64
	db.Model(&models.SkillDefinition{}).Count(&count)
	if count != 0 {
		t.Fatalf("dismiss created skills: %d", count)
	}
}

func TestReviewSkillMinerReviewsCandidatesIndependently(t *testing.T) {
	db, s := setupExperience(t)
	stored := `{"ok":true,"result":{"candidates":[{"sourceEpisodeId":"episode-1","skill":{"name":"first"}},{"sourceEpisodeId":"episode-2","skill":{"name":"second"}}]}}`
	row := models.SalesExperienceCase{Name: "Synthetic", SourceHash: "review-independent", ExtractionStatus: "succeeded", ExtractionResult: stored}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	skill := request.SalesExperienceMinedSkill{
		Name: "可复用技巧", Description: "完整说明", WhenToUse: []string{"适用场景"}, Objective: "推进目标",
		Steps:          []request.SalesExperienceMinedSkillStep{{Instruction: "先诊断", Purpose: "找到阻力"}},
		SuccessSignals: []string{"客户明确阻力"}, WhenNotToUse: []string{"普通询价"},
	}
	first, err := s.ReviewSkillMiner(request.ReviewSalesExperienceSkillMiner{CaseID: row.ID, Action: "confirm", SourceEpisodeID: "episode-1", Skill: &skill}, &dto.AuthPrincipal{UserID: 7, Username: "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	if _, resolved := first["reviewStatus"]; resolved {
		t.Fatalf("batch resolved while second candidate is pending: %+v", first)
	}
	reviews, ok := first["candidateReviews"].(map[string]any)
	if !ok || len(reviews) != 1 {
		t.Fatalf("unexpected candidate reviews: %+v", first["candidateReviews"])
	}
	if _, err := s.ReviewSkillMiner(request.ReviewSalesExperienceSkillMiner{CaseID: row.ID, Action: "dismiss", SourceEpisodeID: "episode-2"}, &dto.AuthPrincipal{UserID: 7, Username: "reviewer"}); err != nil {
		t.Fatal(err)
	}
	final, err := s.Case(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal([]byte(final.ExtractionResult), &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted["reviewStatus"] != "mixed" {
		t.Fatalf("expected mixed result: %+v", persisted)
	}
	var count int64
	db.Model(&models.SkillDefinition{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one saved skill, got %d", count)
	}
}

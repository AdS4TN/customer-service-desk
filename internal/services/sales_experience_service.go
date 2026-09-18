package services

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	sx "agent-desk/internal/pkg/salesexperience"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var SalesExperienceService = &salesExperienceService{complete: ai.LLM.ChatWithConfig, stream: ai.LLM.ChatWithProgress}
var experienceRepo = repositories.SalesExperienceRepository

type salesExperienceService struct {
	mu       sync.Mutex
	complete func(context.Context, models.AIConfig, string, string) (*ai.ChatCompletionResult, error)
	stream   func(context.Context, models.AIConfig, string, string, func(ai.ChatProgress)) (*ai.ChatCompletionResult, error)
}

func experienceError(code string) error {
	return errorsx.InvalidParamI18n("error.salesExperience." + code)
}

func (s *salesExperienceService) Initialize() error {
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		for _, id := range sx.SkillIDs {
			if _, err := experienceRepo.Skill(ctx.Tx, id); err == nil {
				continue
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			p := sx.Seed(id)
			rev := models.SalesExperienceRevision{SkillID: id, Payload: sx.Encode(p), Hash: sx.Hash(p), Note: "baseline"}
			if err := experienceRepo.CreateRevision(ctx.Tx, &rev); err != nil {
				return err
			}
			if err := experienceRepo.CreateSkill(ctx.Tx, &models.SalesExperienceSkill{ID: id, ActiveRevisionID: rev.ID}); err != nil {
				return err
			}
		}
		if err := experienceRepo.PrepareReviews(ctx.Tx); err != nil {
			return err
		}
		if err := s.supersedeDuplicateReviews(ctx.Tx); err != nil {
			return err
		}
		return experienceRepo.Recover(ctx.Tx)
	})
}
func (s *salesExperienceService) Sources(keyword, channel string, page, limit int) ([]repositories.ExperienceSource, int64, error) {
	return experienceRepo.Sources(sqls.DB(), strings.TrimSpace(keyword), channel, page, limit)
}
func (s *salesExperienceService) Cases(page, limit int) ([]models.SalesExperienceCase, int64, error) {
	return experienceRepo.Cases(sqls.DB(), page, limit)
}
func (s *salesExperienceService) Case(id int64) (*models.SalesExperienceCase, error) {
	v, err := experienceRepo.Case(sqls.DB(), id)
	if err != nil {
		return nil, experienceError("notFound")
	}
	return v, nil
}
func (s *salesExperienceService) DeleteCase(id int64) error {
	if id <= 0 {
		return experienceError("notFound")
	}
	deleted, err := experienceRepo.DeleteCase(sqls.DB(), id)
	if err != nil {
		return err
	}
	if deleted {
		return nil
	}
	row, err := experienceRepo.Case(sqls.DB(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return experienceError("notFound")
	}
	if err != nil {
		return err
	}
	if row.ExtractionStatus == "running" {
		return experienceError("caseRunning")
	}
	return experienceError("failed")
}
func (s *salesExperienceService) Skills() ([]models.SalesExperienceSkill, error) {
	return experienceRepo.Skills(sqls.DB())
}
func (s *salesExperienceService) Revisions(skill string, page, limit int) ([]models.SalesExperienceRevision, int64, error) {
	return experienceRepo.Revisions(sqls.DB(), skill, page, limit)
}
func (s *salesExperienceService) Revision(id int64) (*models.SalesExperienceRevision, error) {
	v, err := experienceRepo.Revision(sqls.DB(), id)
	if err != nil {
		return nil, experienceError("notFound")
	}
	return v, nil
}
func (s *salesExperienceService) Jobs(page, limit int) ([]models.SalesExperienceJob, int64, error) {
	return experienceRepo.Jobs(sqls.DB(), page, limit)
}
func (s *salesExperienceService) Job(id int64) (*models.SalesExperienceJob, error) {
	v, err := experienceRepo.Job(sqls.DB(), id)
	if err != nil {
		return nil, experienceError("notFound")
	}
	return v, nil
}
func (s *salesExperienceService) ModelOptions() []models.AIConfig {
	return AIConfigService.Find(sqls.NewCnd().Eq("model_type", enums.AIModelTypeLLM).Eq("status", enums.StatusOk).Asc("sort_no").Asc("id"))
}
func uniqueExperienceIDs(ids []int64) []int64 {
	result := slices.Clone(ids)
	slices.Sort(result)
	result = slices.Compact(result)
	return slices.DeleteFunc(result, func(id int64) bool { return id <= 0 })
}
func (s *salesExperienceService) ImportPreview(conversationIDs []int64) ([]sx.Snapshot, error) {
	ids := uniqueExperienceIDs(conversationIDs)
	if len(ids) == 0 {
		return nil, experienceError("selection")
	}
	conversations, err := experienceRepo.Conversations(sqls.DB(), ids)
	if err != nil {
		return nil, err
	}
	if len(conversations) != len(ids) {
		return nil, experienceError("notFound")
	}
	messages, err := experienceRepo.Messages(sqls.DB(), ids)
	if err != nil {
		return nil, err
	}
	byConversation := make(map[int64]int, len(conversations))
	result := make([]sx.Snapshot, 0, len(conversations))
	for _, conversation := range conversations {
		byConversation[conversation.ID] = len(result)
		result = append(result, sx.Snapshot{
			SchemaVersion:   "1.0",
			CustomerID:      conversation.CustomerID,
			CustomerName:    conversation.CustomerName,
			ConversationIDs: []int64{conversation.ID},
			Messages:        []sx.Message{},
		})
	}
	for _, message := range messages {
		normalized, ok := normalizeExperienceMessage(message)
		if !ok {
			continue
		}
		index := byConversation[message.ConversationID]
		result[index].Messages = append(result[index].Messages, normalized)
	}
	return result, nil
}

type experienceImportSelection struct {
	includeAll bool
	messageIDs map[int64]struct{}
}

func normalizeExperienceImportSelections(req request.ImportSalesExperience) (map[int64]experienceImportSelection, []int64, error) {
	selections := req.Selections
	if len(selections) == 0 {
		for _, id := range uniqueExperienceIDs(req.ConversationIDs) {
			selections = append(selections, request.SalesExperienceImportSelection{ConversationID: id, IncludeAll: true})
		}
	}
	result := make(map[int64]experienceImportSelection, len(selections))
	for _, selection := range selections {
		if selection.ConversationID <= 0 {
			return nil, nil, experienceError("selection")
		}
		current := result[selection.ConversationID]
		if current.messageIDs == nil {
			current.messageIDs = map[int64]struct{}{}
		}
		if selection.IncludeAll {
			current.includeAll = true
			current.messageIDs = map[int64]struct{}{}
		} else if !current.includeAll {
			for _, messageID := range uniqueExperienceIDs(selection.MessageIDs) {
				current.messageIDs[messageID] = struct{}{}
			}
		}
		result[selection.ConversationID] = current
	}
	ids := make([]int64, 0, len(result))
	for id, selection := range result {
		if !selection.includeAll && len(selection.messageIDs) == 0 {
			delete(result, id)
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	if len(ids) == 0 {
		return nil, nil, experienceError("selection")
	}
	return result, ids, nil
}

func (s *salesExperienceService) Import(req request.ImportSalesExperience, userID int64) ([]models.SalesExperienceCase, error) {
	selections, ids, err := normalizeExperienceImportSelections(req)
	if err != nil {
		return nil, err
	}
	result := []models.SalesExperienceCase{}
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		conversations, err := experienceRepo.Conversations(ctx.Tx, ids)
		if err != nil {
			return err
		}
		if len(conversations) != len(ids) {
			return experienceError("notFound")
		}
		messages, err := experienceRepo.Messages(ctx.Tx, ids)
		if err != nil {
			return err
		}
		groups := map[int64]*sx.Snapshot{}
		conversationGroups := map[int64]int64{}
		selectedMessages := map[int64]map[int64]struct{}{}
		order := []int64{}
		for _, c := range conversations {
			key := c.CustomerID
			if key == 0 {
				key = -c.ID
			}
			if groups[key] == nil {
				groups[key] = &sx.Snapshot{SchemaVersion: "1.0", CustomerID: c.CustomerID, CustomerName: c.CustomerName, ConversationIDs: []int64{}, Messages: []sx.Message{}}
				order = append(order, key)
			}
			groups[key].ConversationIDs = append(groups[key].ConversationIDs, c.ID)
			conversationGroups[c.ID] = key
			selectedMessages[c.ID] = map[int64]struct{}{}
		}
		for _, m := range messages {
			selection := selections[m.ConversationID]
			if !selection.includeAll {
				if _, selected := selection.messageIDs[m.ID]; !selected {
					continue
				}
			}
			n, ok := normalizeExperienceMessage(m)
			if !ok {
				continue
			}
			g := groups[conversationGroups[m.ConversationID]]
			g.Messages = append(g.Messages, n)
			selectedMessages[m.ConversationID][m.ID] = struct{}{}
		}
		for conversationID, selection := range selections {
			if selection.includeAll {
				continue
			}
			if len(selectedMessages[conversationID]) != len(selection.messageIDs) {
				return experienceError("selection")
			}
		}
		for _, key := range order {
			v := groups[key]
			if len(v.Messages) == 0 {
				return experienceError("empty")
			}
			row := models.SalesExperienceCase{CustomerID: v.CustomerID, Name: v.CustomerName, Snapshot: sx.Encode(v), SourceHash: sx.Hash(v), MessageCount: len(v.Messages), Outcome: "unknown", CreatedBy: userID}
			if strings.TrimSpace(row.Name) == "" {
				row.Name = fmt.Sprintf("#%d", v.ConversationIDs[0])
			}
			if err := experienceRepo.Import(ctx.Tx, &row); err != nil {
				return err
			}
			result = append(result, row)
		}
		return nil
	})
	return result, err
}
func normalizeExperienceMessage(m models.Message) (sx.Message, bool) {
	role := ""
	switch m.SenderType {
	case enums.IMSenderTypeCustomer:
		role = "buyer"
	case enums.IMSenderTypeAgent:
		role = "seller"
	case enums.IMSenderTypeAI:
		role = "ai"
	default:
		return sx.Message{}, false
	}
	if m.SendStatus == enums.IMMessageStatusFailed || m.SendStatus == enums.IMMessageStatusSending {
		return sx.Message{}, false
	}
	at := m.CreatedAt
	if m.SentAt != nil {
		at = *m.SentAt
	}
	typ, text := string(m.MessageType), ""
	if m.RecalledAt != nil || m.SendStatus == enums.IMMessageStatusRecalled {
		typ = "recalled"
	} else if m.MessageType == enums.IMMessageTypeText || m.MessageType == enums.IMMessageTypeHTML {
		typ = "text"
		text = utils.BuildRuntimeMessageText(m.MessageType, m.Content)
	}
	return sx.Message{ID: m.ID, ConversationID: m.ConversationID, Role: role, Type: typ, Text: text, At: at}, true
}
func (s *salesExperienceService) Annotate(req request.AnnotateSalesExperience) error {
	if !slices.Contains([]string{"unknown", "won", "lost", "progress"}, req.Outcome) {
		return experienceError("format")
	}
	if req.Outcome != "unknown" && strings.TrimSpace(req.Note) == "" {
		return experienceError("outcome")
	}
	if _, err := s.Case(req.ID); err != nil {
		return err
	}
	return experienceRepo.Annotate(sqls.DB(), req.ID, req.Outcome, strings.TrimSpace(req.Note))
}
func experienceModel(id int64) (*models.AIConfig, error) {
	c := AIConfigService.Get(id)
	if c == nil || c.Status != enums.StatusOk || c.ModelType != enums.AIModelTypeLLM {
		return nil, experienceError("model")
	}
	return c, nil
}
func experienceModelSnapshot(c models.AIConfig) string {
	c.APIKey = ""
	c.Remark = ""
	c.AuditFields = models.AuditFields{}
	c.MaxRetryCount = 0
	return sx.Encode(c)
}
func (s *salesExperienceService) enqueue(kind string, input sx.ExperimentInput, modelID, userID int64) (*models.SalesExperienceJob, error) {
	if kind == "distill" {
		input.DistillVersion = sx.DistillPromptVersion
	}
	c, err := experienceModel(modelID)
	if err != nil {
		return nil, err
	}
	job := models.SalesExperienceJob{Kind: kind, State: "queued", Stage: "queued", Input: sx.Encode(input), Output: sx.Encode(sx.Output{}), ModelConfigID: modelID, ModelSnapshot: experienceModelSnapshot(*c), CreatedBy: userID}
	if err := experienceRepo.CreateJob(sqls.DB(), &job); err != nil {
		return nil, err
	}
	return &job, nil
}
func experienceCaseInput(c models.SalesExperienceCase) (sx.CaseInput, error) {
	var snapshot sx.Snapshot
	if err := json.Unmarshal([]byte(c.Snapshot), &snapshot); err != nil {
		return sx.CaseInput{}, err
	}
	return sx.CaseInput{ID: c.ID, SourceHash: c.SourceHash, Snapshot: snapshot, Outcome: c.Outcome, OutcomeNote: c.OutcomeNote}, nil
}
func (s *salesExperienceService) Distill(req request.DistillSalesExperience, userID int64) (*models.SalesExperienceJob, error) {
	ids := uniqueExperienceIDs(req.CaseIDs)
	if len(ids) == 0 {
		return nil, experienceError("selection")
	}
	input := sx.ExperimentInput{Cases: []sx.CaseInput{}, Bases: map[string]sx.Base{}, IncludeAI: req.IncludeAI}
	for _, id := range ids {
		c, err := s.Case(id)
		if err != nil {
			return nil, err
		}
		v, err := experienceCaseInput(*c)
		if err != nil {
			return nil, err
		}
		if !sx.HasSalesExchange(sx.ExtractMessages(v.Snapshot, req.IncludeAI)) {
			return nil, experienceError("exchange")
		}
		input.Cases = append(input.Cases, v)
	}
	skills, err := s.Skills()
	if err != nil {
		return nil, err
	}
	for _, skill := range skills {
		r, err := s.Revision(skill.ActiveRevisionID)
		if err != nil {
			return nil, err
		}
		var p sx.Payload
		if err := json.Unmarshal([]byte(r.Payload), &p); err != nil {
			return nil, err
		}
		input.Bases[skill.ID] = sx.Base{RevisionID: r.ID, Payload: p}
	}
	if len(input.Bases) != len(sx.SkillIDs) {
		return nil, experienceError("notFound")
	}
	return s.enqueue("distill", input, req.ModelConfigID, userID)
}
func (s *salesExperienceService) Evaluate(req request.EvaluateSalesExperience, userID int64) (*models.SalesExperienceJob, error) {
	if req.RevisionID == 0 {
		skill, err := experienceRepo.Skill(sqls.DB(), req.SkillID)
		if err != nil {
			return nil, experienceError("notFound")
		}
		req.RevisionID = skill.ActiveRevisionID
	}
	r, err := s.Revision(req.RevisionID)
	if err != nil {
		return nil, err
	}
	if req.SkillID != r.SkillID {
		return nil, experienceError("skill")
	}
	c, err := s.Case(req.CaseID)
	if err != nil {
		return nil, err
	}
	v, err := experienceCaseInput(*c)
	if err != nil {
		return nil, err
	}
	prefix, err := sx.Prefix(v.Snapshot, req.CutoffID)
	if err != nil {
		return nil, experienceError("cutoff")
	}
	var p sx.Payload
	if err := json.Unmarshal([]byte(r.Payload), &p); err != nil {
		return nil, err
	}
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	mounted, other := "A", "B"
	if b[0]%2 == 1 {
		mounted, other = other, mounted
	}
	// Only the prefix and selected rules enter the evaluation snapshot, never the actual reply or outcome.
	input := sx.ExperimentInput{Cases: []sx.CaseInput{{ID: c.ID, SourceHash: c.SourceHash}}, SkillID: r.SkillID, RevisionID: r.ID, CutoffID: req.CutoffID, Prefix: prefix, BusinessContext: strings.TrimSpace(req.BusinessContext), MountedLabel: mounted, Variants: map[string][]sx.Rule{mounted: p.Rules, other: {}}}
	return s.enqueue("evaluate", input, req.ModelConfigID, userID)
}
func (s *salesExperienceService) Edit(req request.EditSalesExperience, userID int64) (*models.SalesExperienceRevision, error) {
	if err := sx.ValidateRules(req.Rules); err != nil {
		return nil, experienceError("format")
	}
	base, err := s.Revision(req.RevisionID)
	if err != nil {
		return nil, err
	}
	var p sx.Payload
	if err := json.Unmarshal([]byte(base.Payload), &p); err != nil {
		return nil, err
	}
	p.Rules = req.Rules
	p.Provenance = "manual"
	row := models.SalesExperienceRevision{SkillID: base.SkillID, ParentID: base.ID, Payload: sx.Encode(p), Hash: sx.Hash(p), Note: strings.TrimSpace(req.Note), CreatedBy: userID}
	if err := experienceRepo.CreateRevision(sqls.DB(), &row); err != nil {
		return nil, err
	}
	return &row, nil
}
func (s *salesExperienceService) Activate(id int64) error {
	r, err := s.Revision(id)
	if err != nil {
		return err
	}
	if r.ReviewState != "reviewed" {
		return experienceError("state")
	}
	return experienceRepo.Activate(sqls.DB(), r.SkillID, r.ID)
}
func (s *salesExperienceService) Retry(id int64) error {
	j, err := s.Job(id)
	if err != nil {
		return err
	}
	c, err := experienceModel(j.ModelConfigID)
	if err != nil {
		return err
	}
	fields := map[string]any{"state": "queued", "error_code": "", "call_phase": "", "call_started_at": nil, "heartbeat_at": nil, "last_response_at": nil, "output_chars": 0}
	snapshot := experienceModelSnapshot(*c)
	if snapshot != j.ModelSnapshot {
		fields["model_snapshot"] = snapshot
		fields["output"] = sx.Encode(sx.Output{})
		fields["stage"] = "queued"
	}
	ok, err := experienceRepo.Retry(sqls.DB(), id, fields)
	if err != nil {
		return err
	}
	if !ok {
		return experienceError("state")
	}
	return nil
}
func (s *salesExperienceService) Rate(req request.RateSalesExperience) error {
	j, err := s.Job(req.ID)
	if err != nil {
		return err
	}
	if j.Kind != "evaluate" || j.State != "succeeded" || !slices.Contains([]string{"A", "B", "tie", "neither"}, req.Rating) {
		return experienceError("state")
	}
	return experienceRepo.UpdateJob(sqls.DB(), req.ID, map[string]any{"rating": req.Rating, "rating_note": strings.TrimSpace(req.Note)})
}

func (s *salesExperienceService) Cancel(id int64) error {
	ok, err := experienceRepo.Cancel(sqls.DB(), id)
	if err != nil {
		return err
	}
	if !ok {
		return experienceError("state")
	}
	return nil
}
func (s *salesExperienceService) Export(id int64) ([]byte, error) {
	r, err := s.Revision(id)
	if err != nil {
		return nil, err
	}
	var p sx.Payload
	if err := json.Unmarshal([]byte(r.Payload), &p); err != nil {
		return nil, err
	}
	manifest := map[string]any{"schemaVersion": "1.0", "skillId": r.SkillID, "revisionId": r.ID, "parentId": r.ParentID, "jobId": r.JobID, "hash": r.Hash, "createdAt": r.CreatedAt, "provenance": p.Provenance}
	if r.JobID > 0 {
		if j, e := s.Job(r.JobID); e == nil {
			var cfg models.AIConfig
			_ = json.Unmarshal([]byte(j.ModelSnapshot), &cfg)
			manifest["model"] = map[string]any{"name": cfg.ModelName, "provider": cfg.Provider, "configId": cfg.ID}
		}
	}
	buf := new(bytes.Buffer)
	writer := zip.NewWriter(buf)
	files := map[string]string{"SKILL.md": sx.Markdown(r.SkillID, p), "rules.json": sx.Encode(p.Rules), "evidence.json": sx.Encode(p.Evidence), "manual-edits.json": sx.Encode(map[string]any{"note": r.Note, "parentId": r.ParentID, "provenance": p.Provenance}), "manifest.json": sx.Encode(manifest)}
	for _, name := range []string{"SKILL.md", "rules.json", "evidence.json", "manual-edits.json", "manifest.json"} {
		f, e := writer.Create(name)
		if e != nil {
			return nil, e
		}
		if _, e = f.Write([]byte(files[name])); e != nil {
			return nil, e
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *salesExperienceService) checkpoint(job *models.SalesExperienceJob, stage string, out sx.Output) error {
	job.Stage = stage
	job.Output = sx.Encode(out)
	return experienceRepo.UpdateRunningJob(sqls.DB(), job.ID, map[string]any{"stage": stage, "output": job.Output})
}
func (s *salesExperienceService) call(cfg models.AIConfig, prompt string, input any) (string, error) {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cfg.MaxRetryCount = 0
	r, err := s.complete(ctx, cfg, prompt, sx.Encode(input))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", errors.New("timeout")
		}
		return "", errors.New("modelCall")
	}
	if r == nil || strings.TrimSpace(r.Content) == "" {
		return "", errors.New("format")
	}
	if r.FinishReason == "length" {
		return "", errors.New("truncated")
	}
	return r.Content, nil
}

func loadSkillMinerPrompt(name string) (string, error) {
	promptPath := filepath.Join(".codex", "skills", "sales-skill-miner", "references", name)
	promptBytes, err := os.ReadFile(promptPath)
	if err != nil {
		return "", err
	}
	prompt := string(promptBytes)
	start := strings.Index(prompt, "```text")
	if start < 0 {
		return "", errors.New("skill prompt template missing")
	}
	start += len("```text")
	end := strings.Index(prompt[start:], "```")
	if end < 0 {
		return "", errors.New("skill prompt template unterminated")
	}
	return strings.TrimSpace(prompt[start : start+end]), nil
}

func (s *salesExperienceService) callSkillMinerStage(cfg models.AIConfig, userPrompt string) (string, error) {
	cfg.MaxOutputTokens = 20000
	cfg.MaxRetryCount = 0
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	r, err := s.complete(ctx, cfg, "", userPrompt)
	if err != nil {
		return "", err
	}
	if r == nil || strings.TrimSpace(r.Content) == "" {
		return "", errors.New("empty response")
	}
	if r.FinishReason == "length" {
		return "", errors.New("truncated")
	}
	return r.Content, nil
}

type skillMinerEvidenceAudit struct {
	ProtocolVersion      string                      `json:"protocolVersion"`
	HasCandidateEpisodes bool                        `json:"hasCandidateEpisodes"`
	Assessment           map[string]any              `json:"assessment"`
	Episodes             []skillMinerEvidenceEpisode `json:"episodes"`
}

type skillMinerEvidenceQuote struct {
	MessageID int64  `json:"messageId"`
	Quote     string `json:"quote"`
}

type skillMinerEvidence struct {
	CustomerBefore []skillMinerEvidenceQuote `json:"customerBefore"`
	SellerMove     []skillMinerEvidenceQuote `json:"sellerMove"`
	CustomerAfter  []skillMinerEvidenceQuote `json:"customerAfter"`
}

type skillMinerEvidenceEpisode struct {
	ID                    string             `json:"id"`
	CustomerSituation     string             `json:"customerSituation"`
	SellerMoveClaim       string             `json:"sellerMoveClaim"`
	ClaimedCustomerChange string             `json:"claimedCustomerChange"`
	PriorStateChecks      map[string]any     `json:"priorStateChecks"`
	BusinessDependency    map[string]any     `json:"businessDependency"`
	Evidence              skillMinerEvidence `json:"evidence"`
}

type skillMinerOutput struct {
	ProtocolVersion   string `json:"protocolVersion"`
	HasLearnableSkill bool   `json:"hasLearnableSkill"`
	Candidates        []struct {
		SourceEpisodeID string             `json:"sourceEpisodeId"`
		Skill           json.RawMessage    `json:"skill"`
		Evidence        skillMinerEvidence `json:"evidence"`
	} `json:"candidates"`
}

func validateSkillMinerEvidenceAudit(audit skillMinerEvidenceAudit, snapshot sx.Snapshot) error {
	if audit.ProtocolVersion != "sales-skill-evidence-audit-v1" || audit.HasCandidateEpisodes != (len(audit.Episodes) > 0) || len(audit.Episodes) > 5 {
		return errors.New("invalid evidence audit")
	}
	messages := make(map[int64]sx.Message, len(snapshot.Messages))
	positions := make(map[int64]int, len(snapshot.Messages))
	for index, message := range snapshot.Messages {
		messages[message.ID] = message
		positions[message.ID] = index
	}
	seen := map[string]bool{}
	for _, episode := range audit.Episodes {
		if strings.TrimSpace(episode.ID) == "" || seen[episode.ID] || strings.TrimSpace(episode.CustomerSituation) == "" || strings.TrimSpace(episode.SellerMoveClaim) == "" || strings.TrimSpace(episode.ClaimedCustomerChange) == "" {
			return errors.New("invalid evidence episode")
		}
		seen[episode.ID] = true
		before, err := validateSkillMinerQuotes(episode.Evidence.CustomerBefore, "buyer", messages, positions)
		if err != nil {
			return err
		}
		move, err := validateSkillMinerQuotes(episode.Evidence.SellerMove, "seller", messages, positions)
		if err != nil {
			return err
		}
		after, err := validateSkillMinerQuotes(episode.Evidence.CustomerAfter, "buyer", messages, positions)
		if err != nil {
			return err
		}
		if slices.Max(before) >= slices.Min(move) || slices.Max(move) >= slices.Min(after) {
			return errors.New("invalid evidence chronology")
		}
	}
	return nil
}

func validateSkillMinerQuotes(quotes []skillMinerEvidenceQuote, role string, messages map[int64]sx.Message, positions map[int64]int) ([]int, error) {
	if len(quotes) == 0 {
		return nil, errors.New("missing evidence")
	}
	result := make([]int, 0, len(quotes))
	for _, quote := range quotes {
		message, ok := messages[quote.MessageID]
		position, hasPosition := positions[quote.MessageID]
		if !ok || !hasPosition || message.Role != role || message.Type != "text" || strings.TrimSpace(quote.Quote) == "" || !strings.Contains(message.Text, quote.Quote) {
			return nil, errors.New("invalid evidence quote")
		}
		result = append(result, position)
	}
	return result, nil
}

func validateSkillMinerOutput(output skillMinerOutput, audit skillMinerEvidenceAudit) error {
	if output.ProtocolVersion != "sales-skill-miner-v2" || output.HasLearnableSkill != (len(output.Candidates) > 0) || len(output.Candidates) > 2 {
		return errors.New("invalid skill miner output")
	}
	episodes := make(map[string]skillMinerEvidence, len(audit.Episodes))
	for _, episode := range audit.Episodes {
		episodes[episode.ID] = episode.Evidence
	}
	seen := map[string]bool{}
	for _, candidate := range output.Candidates {
		evidence, ok := episodes[candidate.SourceEpisodeID]
		if !ok || seen[candidate.SourceEpisodeID] || len(candidate.Skill) == 0 || sx.Encode(evidence) != sx.Encode(candidate.Evidence) {
			return errors.New("invalid skill miner candidate")
		}
		seen[candidate.SourceEpisodeID] = true
	}
	return nil
}

func (s *salesExperienceService) SkillMinerDebug(caseID int64, modelConfigID int64) (map[string]any, error) {
	if _, err := s.Case(caseID); err != nil {
		return nil, err
	}
	if err := experienceRepo.UpdateCaseExtraction(sqls.DB(), caseID, map[string]any{
		"extraction_status": "running",
		"extraction_model":  "",
		"extraction_result": "",
		"extracted_at":      nil,
	}); err != nil {
		return nil, err
	}
	result, err := s.runSkillMiner(caseID, modelConfigID)
	now := time.Now()
	status := "failed"
	model := ""
	stored := result
	if stored == nil {
		message := "skill extraction failed"
		if err != nil {
			message = err.Error()
		}
		stored = map[string]any{"ok": false, "error": message}
	}
	if value, ok := stored["model"].(string); ok {
		model = value
	}
	if err == nil {
		status = "succeeded"
		if payload, ok := stored["result"].(map[string]any); ok {
			if learnable, ok := payload["hasLearnableSkill"].(bool); ok && !learnable {
				status = "no_skill"
			}
		}
	}
	if persistErr := experienceRepo.UpdateCaseExtraction(sqls.DB(), caseID, map[string]any{
		"extraction_status": status,
		"extraction_model":  model,
		"extraction_result": sx.Encode(stored),
		"extracted_at":      &now,
	}); persistErr != nil {
		return nil, persistErr
	}
	return result, err
}

func (s *salesExperienceService) ReviewSkillMiner(req request.ReviewSalesExperienceSkillMiner, operator *dto.AuthPrincipal) (map[string]any, error) {
	episodeID := strings.TrimSpace(req.SourceEpisodeID)
	if req.CaseID <= 0 || episodeID == "" || (req.Action != "confirm" && req.Action != "dismiss") {
		return nil, experienceError("minerReview")
	}
	var saved map[string]any
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		row, err := experienceRepo.Case(ctx.Tx, req.CaseID)
		if err != nil || row.ExtractionStatus != "succeeded" || strings.TrimSpace(row.ExtractionResult) == "" {
			return experienceError("minerReview")
		}
		var stored map[string]any
		if json.Unmarshal([]byte(row.ExtractionResult), &stored) != nil || stored["ok"] != true {
			return experienceError("minerReview")
		}
		result, ok := stored["result"].(map[string]any)
		if !ok {
			return experienceError("minerReview")
		}
		candidates, ok := result["candidates"].([]any)
		if !ok || len(candidates) == 0 {
			return experienceError("minerReview")
		}

		reviews, _ := stored["candidateReviews"].(map[string]any)
		if reviews == nil {
			reviews = map[string]any{}
			// Results reviewed before per-candidate review existed remain resolved.
			if legacyStatus, _ := stored["reviewStatus"].(string); legacyStatus == "confirmed" || legacyStatus == "dismissed" {
				legacyIDs, _ := stored["skillDefinitionIds"].([]any)
				for index, value := range candidates {
					candidate, _ := value.(map[string]any)
					id, _ := candidate["sourceEpisodeId"].(string)
					if id == "" {
						continue
					}
					review := map[string]any{"status": legacyStatus, "reviewedAt": stored["reviewedAt"]}
					if legacyStatus == "confirmed" && index < len(legacyIDs) {
						review["skillDefinitionId"] = legacyIDs[index]
					}
					reviews[id] = review
				}
			}
		}
		if _, reviewed := reviews[episodeID]; reviewed {
			return experienceError("state")
		}

		var target map[string]any
		for _, value := range candidates {
			candidate, ok := value.(map[string]any)
			if !ok {
				return experienceError("minerReview")
			}
			candidateID, _ := candidate["sourceEpisodeId"].(string)
			if candidateID == episodeID {
				target = candidate
				break
			}
		}
		if target == nil {
			return experienceError("minerReview")
		}

		now := time.Now().Format(time.RFC3339)
		review := map[string]any{"status": req.Action + "ed", "reviewedAt": now}
		if req.Action == "confirm" {
			if req.Skill == nil {
				return experienceError("minerReview")
			}
			skill, err := normalizeMinedSkill(*req.Skill)
			if err != nil {
				return err
			}
			target["skill"] = skill
			item := &models.SkillDefinition{
				Name:        skill.Name,
				Description: skill.Description,
				Instruction: minedSkillMarkdown(skill),
				Examples:    mustMarshalSkillStringArray(skill.WhenToUse),
				Status:      enums.StatusDisabled,
				Remark:      fmt.Sprintf("由销售经验案例 #%d 确认入库；未自动挂载至任何 AI 客服。", req.CaseID),
				AuditFields: utils.BuildAuditFields(operator),
			}
			if err := repositories.SkillDefinitionRepository.Create(ctx.Tx, item); err != nil {
				return err
			}
			review["skillDefinitionId"] = item.ID
		}
		reviews[episodeID] = review
		stored["candidateReviews"] = reviews
		result["candidates"] = candidates
		stored["result"] = result

		confirmed, dismissed, resolved := 0, 0, 0
		for _, value := range candidates {
			candidate, _ := value.(map[string]any)
			id, _ := candidate["sourceEpisodeId"].(string)
			entry, _ := reviews[id].(map[string]any)
			status, _ := entry["status"].(string)
			switch status {
			case "confirmed":
				confirmed++
				resolved++
			case "dismissed":
				dismissed++
				resolved++
			}
		}
		delete(stored, "reviewStatus")
		delete(stored, "reviewedAt")
		delete(stored, "skillDefinitionIds")
		if resolved == len(candidates) {
			switch {
			case confirmed == len(candidates):
				stored["reviewStatus"] = "confirmed"
			case dismissed == len(candidates):
				stored["reviewStatus"] = "dismissed"
			default:
				stored["reviewStatus"] = "mixed"
			}
			stored["reviewedAt"] = now
		}
		if err := experienceRepo.UpdateCaseExtraction(ctx.Tx, req.CaseID, map[string]any{"extraction_result": sx.Encode(stored)}); err != nil {
			return err
		}
		saved = stored
		return nil
	})
	return saved, err
}

func normalizeMinedSkill(skill request.SalesExperienceMinedSkill) (request.SalesExperienceMinedSkill, error) {
	skill.Name = strings.TrimSpace(skill.Name)
	skill.Description = strings.TrimSpace(skill.Description)
	skill.Objective = strings.TrimSpace(skill.Objective)
	skill.WhenToUse = normalizeMinedSkillList(skill.WhenToUse)
	skill.SuccessSignals = normalizeMinedSkillList(skill.SuccessSignals)
	skill.WhenNotToUse = normalizeMinedSkillList(skill.WhenNotToUse)
	steps := make([]request.SalesExperienceMinedSkillStep, 0, len(skill.Steps))
	for _, step := range skill.Steps {
		step.Instruction = strings.TrimSpace(step.Instruction)
		step.Purpose = strings.TrimSpace(step.Purpose)
		if step.Instruction == "" || step.Purpose == "" {
			return skill, experienceError("minerReview")
		}
		steps = append(steps, step)
	}
	skill.Steps = steps
	if skill.Name == "" || len([]rune(skill.Name)) > 100 || skill.Description == "" || len([]rune(skill.Description)) > 255 || skill.Objective == "" || len(skill.WhenToUse) == 0 || len(skill.Steps) == 0 || len(skill.Steps) > 8 || len(skill.SuccessSignals) == 0 || len(skill.WhenNotToUse) == 0 {
		return skill, experienceError("minerReview")
	}
	return skill, nil
}

func normalizeMinedSkillList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func minedSkillMarkdown(skill request.SalesExperienceMinedSkill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n## 适用场景\n", skill.Name, skill.Description)
	for _, value := range skill.WhenToUse {
		fmt.Fprintf(&b, "- %s\n", value)
	}
	fmt.Fprintf(&b, "\n## 目标\n\n%s\n\n## 执行步骤\n", skill.Objective)
	for index, step := range skill.Steps {
		fmt.Fprintf(&b, "%d. **%s**\n   - 目的：%s\n", index+1, step.Instruction, step.Purpose)
	}
	b.WriteString("\n## 成功信号\n")
	for _, value := range skill.SuccessSignals {
		fmt.Fprintf(&b, "- %s\n", value)
	}
	b.WriteString("\n## 不适用场景\n")
	for _, value := range skill.WhenNotToUse {
		fmt.Fprintf(&b, "- %s\n", value)
	}
	return strings.TrimSpace(b.String())
}

func (s *salesExperienceService) runSkillMiner(caseID int64, modelConfigID int64) (map[string]any, error) {
	row, err := experienceRepo.Case(sqls.DB(), caseID)
	if err != nil {
		return nil, experienceError("notFound")
	}
	var snapshot sx.Snapshot
	if json.Unmarshal([]byte(row.Snapshot), &snapshot) != nil {
		return nil, experienceError("snapshot")
	}
	auditPrompt, err := loadSkillMinerPrompt("evidence-audit-prompt.md")
	if err != nil {
		return nil, err
	}
	extractionPrompt, err := loadSkillMinerPrompt("extraction-prompt.md")
	if err != nil {
		return nil, err
	}
	schemaPath := filepath.Join(".codex", "skills", "sales-skill-miner", "references", "output-schema.md")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}
	baseInput := map[string]any{
		"conversation":     snapshot,
		"business_context": map[string]any{"company": "Lolands", "productLine": "门窗/钢门等建材出口", "market": "美国与东南亚 B2B 出口", "language": "zh-CN"},
	}
	candidates := []models.AIConfig{}
	if selected := AIConfigService.Get(modelConfigID); selected != nil {
		candidates = append(candidates, *selected)
	}
	for _, cfg := range AIConfigService.Find(sqls.NewCnd().Eq("model_type", enums.AIModelTypeLLM).Asc("sort_no").Asc("id")) {
		if modelConfigID > 0 && cfg.ID == modelConfigID {
			continue
		}
		candidates = append(candidates, cfg)
	}
	if len(candidates) == 0 {
		return nil, errors.New("model config missing")
	}
	var failures []string
	for _, base := range candidates {
		auditContent, err := s.callSkillMinerStage(base, auditPrompt+"\n\nInput JSON:\n"+sx.Encode(baseInput))
		if err != nil {
			failures = append(failures, base.ModelName+" audit: "+err.Error())
			continue
		}
		var audit skillMinerEvidenceAudit
		if err := sx.ParseJSON(auditContent, &audit); err != nil || validateSkillMinerEvidenceAudit(audit, snapshot) != nil {
			failures = append(failures, base.ModelName+" audit: invalid JSON")
			continue
		}
		if !audit.HasCandidateEpisodes || len(audit.Episodes) == 0 {
			result := map[string]any{
				"protocolVersion":   "sales-skill-miner-v2",
				"hasLearnableSkill": false,
				"assessment":        audit.Assessment,
				"candidates":        []any{},
			}
			return map[string]any{"ok": true, "model": base.ModelName, "result": result}, nil
		}
		extractionInput := map[string]any{
			"conversation":     snapshot,
			"business_context": baseInput["business_context"],
			"evidence_audit":   audit,
		}
		extractionContent, err := s.callSkillMinerStage(base, extractionPrompt+"\n\n输出协议：\n"+string(schemaBytes)+"\n\nInput JSON:\n"+sx.Encode(extractionInput))
		if err != nil {
			failures = append(failures, base.ModelName+" extraction: "+err.Error())
			continue
		}
		var output skillMinerOutput
		if err := sx.ParseJSON(extractionContent, &output); err != nil || validateSkillMinerOutput(output, audit) != nil {
			failures = append(failures, base.ModelName+" extraction: invalid JSON")
			continue
		}
		var parsed any
		_ = sx.ParseJSON(extractionContent, &parsed)
		return map[string]any{"ok": true, "model": base.ModelName, "result": parsed}, nil
	}
	return map[string]any{"ok": false, "error": strings.Join(failures, "; "), "failures": failures}, errors.New("all model candidates failed")
}

package request

import sx "agent-desk/internal/pkg/salesexperience"

type ImportSalesExperience struct {
	ConversationIDs []int64                          `json:"conversationIds"`
	Selections      []SalesExperienceImportSelection `json:"selections"`
}
type SalesExperienceImportSelection struct {
	ConversationID int64   `json:"conversationId"`
	IncludeAll     bool    `json:"includeAll"`
	MessageIDs     []int64 `json:"messageIds"`
}
type PreviewSalesExperienceImport struct {
	ConversationIDs []int64 `json:"conversationIds"`
}
type AnnotateSalesExperience struct {
	ID      int64  `json:"id"`
	Outcome string `json:"outcome"`
	Note    string `json:"note"`
}
type DistillSalesExperience struct {
	CaseIDs       []int64 `json:"caseIds"`
	ModelConfigID int64   `json:"modelConfigId"`
	IncludeAI     bool    `json:"includeAI"`
}
type EvaluateSalesExperience struct {
	CaseID          int64  `json:"caseId"`
	SkillID         string `json:"skillId"`
	RevisionID      int64  `json:"revisionId"`
	CutoffID        int64  `json:"cutoffId"`
	ModelConfigID   int64  `json:"modelConfigId"`
	BusinessContext string `json:"businessContext"`
}
type EditSalesExperience struct {
	RevisionID int64     `json:"revisionId"`
	Rules      []sx.Rule `json:"rules"`
	Note       string    `json:"note"`
}
type RateSalesExperience struct {
	ID     int64  `json:"id"`
	Rating string `json:"rating"`
	Note   string `json:"note"`
}

type ReviewSalesExperience struct {
	ExpectedID int64    `json:"expectedId"`
	ProposalID int64    `json:"proposalId"`
	RuleID     string   `json:"ruleId"`
	Decision   string   `json:"decision"`
	Rule       *sx.Rule `json:"rule"`
}
type ReviewSalesExperienceBatch struct {
	SkillID    string                  `json:"skillId"`
	ExpectedID int64                   `json:"expectedId"`
	Items      []ReviewSalesExperience `json:"items"`
}
type SaveSalesExperienceSkill struct {
	SkillID    string    `json:"skillId"`
	ExpectedID int64     `json:"expectedId"`
	Rules      []sx.Rule `json:"rules"`
}

type SalesExperienceMinedSkillStep struct {
	Instruction string `json:"instruction"`
	Purpose     string `json:"purpose"`
}

type SalesExperienceMinedSkill struct {
	Name           string                          `json:"name"`
	Description    string                          `json:"description"`
	WhenToUse      []string                        `json:"whenToUse"`
	Objective      string                          `json:"objective"`
	Steps          []SalesExperienceMinedSkillStep `json:"steps"`
	SuccessSignals []string                        `json:"successSignals"`
	WhenNotToUse   []string                        `json:"whenNotToUse"`
}

type SalesExperienceMinedCandidate struct {
	SourceEpisodeID string                    `json:"sourceEpisodeId"`
	Skill           SalesExperienceMinedSkill `json:"skill"`
}

type ReviewSalesExperienceSkillMiner struct {
	CaseID          int64                      `json:"caseId"`
	Action          string                     `json:"action"`
	SourceEpisodeID string                     `json:"sourceEpisodeId"`
	Skill           *SalesExperienceMinedSkill `json:"skill"`
}

type CompareSalesExperienceSkill struct {
	AIAgentID         int64                   `json:"aiAgentId"`
	SkillDefinitionID int64                   `json:"skillDefinitionId"`
	Messages          []AIEmployeePreviewTurn `json:"messages"`
}

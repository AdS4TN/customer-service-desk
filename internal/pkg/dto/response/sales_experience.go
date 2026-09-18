package response

import (
	sx "agent-desk/internal/pkg/salesexperience"
	"encoding/json"
	"time"
)

type SalesExperienceSource struct {
	ID            int64     `json:"id"`
	CustomerID    int64     `json:"customerId"`
	CustomerName  string    `json:"customerName"`
	ChannelType   string    `json:"channelType"`
	ChannelName   string    `json:"channelName"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	MessageCount  int64     `json:"messageCount"`
	Status        int       `json:"status"`
}
type SalesExperienceCase struct {
	ID               int64           `json:"id"`
	Name             string          `json:"name"`
	CustomerID       int64           `json:"customerId"`
	SourceHash       string          `json:"sourceHash"`
	MessageCount     int             `json:"messageCount"`
	Outcome          string          `json:"outcome"`
	OutcomeNote      string          `json:"outcomeNote"`
	ExtractionStatus string          `json:"extractionStatus"`
	ExtractionModel  string          `json:"extractionModel"`
	ExtractionResult json.RawMessage `json:"extractionResult,omitempty"`
	ExtractedAt      *time.Time      `json:"extractedAt,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	Snapshot         *sx.Snapshot    `json:"snapshot,omitempty"`
}
type SalesExperienceImportPreview struct {
	ConversationID int64        `json:"conversationId"`
	CustomerID     int64        `json:"customerId"`
	CustomerName   string       `json:"customerName"`
	Messages       []sx.Message `json:"messages"`
}
type SalesExperienceSkill struct {
	ID               string `json:"id"`
	ActiveRevisionID int64  `json:"activeRevisionId"`
}
type SalesExperienceModel struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ModelName string `json:"modelName"`
}
type SalesExperienceRevision struct {
	ID        int64      `json:"id"`
	SkillID   string     `json:"skillId"`
	ParentID  int64      `json:"parentId"`
	JobID     int64      `json:"jobId"`
	Hash      string     `json:"hash"`
	Note      string     `json:"note"`
	Payload   sx.Payload `json:"payload"`
	CreatedAt time.Time  `json:"createdAt"`
}
type SalesExperienceJob struct {
	ID             int64               `json:"id"`
	Kind           string              `json:"kind"`
	State          string              `json:"state"`
	Stage          string              `json:"stage"`
	ErrorCode      string              `json:"errorCode"`
	Attempts       int                 `json:"attempts"`
	ModelConfigID  int64               `json:"modelConfigId"`
	ModelName      string              `json:"modelName,omitempty"`
	Rating         string              `json:"rating"`
	RatingNote     string              `json:"ratingNote"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
	CallStartedAt  *time.Time          `json:"callStartedAt,omitempty"`
	HeartbeatAt    *time.Time          `json:"heartbeatAt,omitempty"`
	LastResponseAt *time.Time          `json:"lastResponseAt,omitempty"`
	OutputChars    int                 `json:"outputChars"`
	CallPhase      string              `json:"callPhase"`
	Input          *sx.ExperimentInput `json:"input,omitempty"`
	Output         *sx.Output          `json:"output,omitempty"`
}

type SalesExperienceWorkspace struct {
	SkillID     string                      `json:"skillId"`
	Current     SalesExperienceRevision     `json:"current"`
	Suggestions []SalesExperienceSuggestion `json:"suggestions"`
}
type SalesExperienceSuggestion struct {
	ID        int64          `json:"id"`
	JobID     int64          `json:"jobId"`
	CreatedAt time.Time      `json:"createdAt"`
	Changes   []sx.RuleDelta `json:"changes"`
}

type SalesExperienceSkillCompareResponse struct {
	AIAgentID         int64                     `json:"aiAgentId"`
	AIAgentName       string                    `json:"aiAgentName"`
	SkillDefinitionID int64                     `json:"skillDefinitionId"`
	SkillName         string                    `json:"skillName"`
	WithoutSkill      AIEmployeePreviewResponse `json:"withoutSkill"`
	WithSkill         AIEmployeePreviewResponse `json:"withSkill"`
}

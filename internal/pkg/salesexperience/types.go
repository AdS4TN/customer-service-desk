package salesexperience

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Snapshot is the channel-independent input to extraction and replay.
type Snapshot struct {
	SchemaVersion   string    `json:"schemaVersion"`
	CustomerID      int64     `json:"customerId"`
	CustomerName    string    `json:"customerName"`
	ConversationIDs []int64   `json:"conversationIds"`
	Messages        []Message `json:"messages"`
}
type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversationId"`
	Role           string    `json:"role"`
	Type           string    `json:"type"`
	Text           string    `json:"text"`
	At             time.Time `json:"at"`
}
type Rule struct {
	ID         string `json:"id"`
	Condition  string `json:"condition"`
	Action     string `json:"action"`
	Exceptions string `json:"exceptions"`
}
type Quote struct {
	MessageID int64  `json:"messageId"`
	Text      string `json:"text"`
}
type Event struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Signal         string   `json:"signal"`
	SellerAction   string   `json:"sellerAction"`
	ObservedResult string   `json:"observedResult"`
	Uncertainty    string   `json:"uncertainty"`
	SkillIDs       []string `json:"skillIds"`
	Quotes         []Quote  `json:"quotes"`
}
type EventsOutput struct {
	Summary string  `json:"summary"`
	Events  []Event `json:"events"`
}
type Change struct {
	SkillID            string              `json:"skillId"`
	Operation          string              `json:"operation"`
	Rule               *Rule               `json:"rule"`
	EventIDs           []string            `json:"eventIds"`
	Rationale          string              `json:"rationale"`
	TransferTests      []string            `json:"transferTests,omitempty"`
	LearningAssessment *LearningAssessment `json:"learningAssessment,omitempty"`
}
type LearningAssessment struct {
	NonRoutineMove        bool   `json:"nonRoutineMove"`
	ObservedBuyerEffect   bool   `json:"observedBuyerEffect"`
	TransferableMechanism bool   `json:"transferableMechanism"`
	AddsNewLearning       bool   `json:"addsNewLearning"`
	Reason                string `json:"reason"`
}
type Evidence struct {
	CaseID     int64   `json:"caseId"`
	SourceHash string  `json:"sourceHash"`
	RuleID     string  `json:"ruleId"`
	Events     []Event `json:"events"`
}
type Payload struct {
	Rules      []Rule     `json:"rules"`
	Evidence   []Evidence `json:"evidence"`
	Changes    []Change   `json:"changes"`
	Provenance string     `json:"provenance"`
}
type CaseInput struct {
	ID          int64    `json:"id"`
	SourceHash  string   `json:"sourceHash"`
	Snapshot    Snapshot `json:"snapshot"`
	Outcome     string   `json:"outcome"`
	OutcomeNote string   `json:"outcomeNote"`
}
type Base struct {
	RevisionID int64   `json:"revisionId"`
	Payload    Payload `json:"payload"`
}
type ExperimentInput struct {
	DistillVersion  string            `json:"distillVersion,omitempty"`
	Cases           []CaseInput       `json:"cases"`
	Bases           map[string]Base   `json:"bases"`
	IncludeAI       bool              `json:"includeAI"`
	SkillID         string            `json:"skillId"`
	RevisionID      int64             `json:"revisionId"`
	CutoffID        int64             `json:"cutoffId"`
	Prefix          []Message         `json:"prefix"`
	BusinessContext string            `json:"businessContext"`
	Variants        map[string][]Rule `json:"variants"`
	MountedLabel    string            `json:"mountedLabel"`
}
type CaseResult struct {
	CaseID      int64   `json:"caseId"`
	Summary     string  `json:"summary"`
	Events      []Event `json:"events"`
	WindowDone  int     `json:"windowDone"`
	WindowTotal int     `json:"windowTotal"`
	Proposed    bool    `json:"proposed"`
	// Skipped counts model proposals that the learning gate discarded because
	// the cited evidence did not show a learnable, transferable technique.
	Skipped int `json:"skipped,omitempty"`
}
type Output struct {
	Cases       []CaseResult       `json:"cases"`
	Payloads    map[string]Payload `json:"payloads"`
	RevisionIDs []int64            `json:"revisionIds"`
	Replies     map[string]string  `json:"replies"`
}

var SkillIDs = []string{"negotiation", "decision", "closing"}

func KnownSkill(id string) bool {
	for _, s := range SkillIDs {
		if s == id {
			return true
		}
	}
	return false
}
func Encode(value any) string { b, _ := json.Marshal(value); return string(b) }
func Hash(value any) string {
	b := sha256.Sum256([]byte(Encode(value)))
	return hex.EncodeToString(b[:])
}

func Seed(id string) Payload {
	rules := map[string]Rule{
		"negotiation": {"price-reason", "客户提出价格异议或要求让步，但真实顾虑和交换条件尚不清楚", "先诊断异议属于预算、价值认知、比较基准还是风险顾虑；针对根因重构价值。确需让步时，用可授权让步交换客户的明确承诺。", "不擅自让价，不编造比较信息，不把未确认的未来意向当成承诺。"},
		"decision":    {"decision-barrier", "客户反复犹豫或推迟决定，但没有说明真正的决策阻碍", "用一个聚焦问题让客户指出唯一核心顾虑，确认它属于风险、信任、优先级还是内部决策；只处理这一阻碍，然后换取一个低风险的下一步承诺。", "不把沉默或礼貌回应当成拒绝，不编造稀缺性或时限压力。"},
		"closing":     {"next-step", "客户已认可核心价值并表达推进意愿，但尚未形成明确行动承诺", "先确认唯一剩余阻碍，再把意向收敛成一个可当场确认的下一步，同时锁定负责人、时间和完成标志。", "高意向不等于成交；关键条件未确认时，不宣称已达成交易或启动不可逆动作。"},
	}
	return Payload{Rules: []Rule{rules[id]}, Evidence: []Evidence{}, Changes: []Change{}, Provenance: "baseline"}
}

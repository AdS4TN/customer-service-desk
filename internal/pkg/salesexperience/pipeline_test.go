package salesexperience

import (
	"strings"
	"testing"
)

func fixture() ([]Message, Event) {
	messages := []Message{{ID: 1, Role: "buyer", Type: "text", Text: "Too expensive"}, {ID: 2, Role: "seller", Type: "text", Text: "What is your budget?"}, {ID: 3, Role: "buyer", Type: "text", Text: "Later outcome"}}
	event := Event{ID: "e1", Title: "Price question", Signal: "Price concern", SellerAction: "Clarify budget", ObservedResult: "Buyer clarified the next decision input", SkillIDs: []string{"negotiation"}, Quotes: []Quote{{1, "Too expensive"}, {2, "your budget"}, {3, "Later outcome"}}}
	return messages, event
}

func passingAssessment() *LearningAssessment {
	return &LearningAssessment{
		NonRoutineMove:        true,
		ObservedBuyerEffect:   true,
		TransferableMechanism: true,
		AddsNewLearning:       true,
		Reason:                "销售动作之后，客户给出了新的决策信息。",
	}
}
func TestEvidenceAndPrefix(t *testing.T) {
	messages, event := fixture()
	result := EventsOutput{Summary: "Unknown outcome", Events: []Event{event}}
	if err := ValidateEvents(result, messages); err != nil {
		t.Fatal(err)
	}
	result.Events[0].Quotes[1].Text = "invented quote"
	if ValidateEvents(result, messages) == nil {
		t.Fatal("accepted invented quote")
	}
	prefix, err := Prefix(Snapshot{Messages: messages}, 1)
	if err != nil || len(prefix) != 1 {
		t.Fatalf("future reply leaked: %v %v", prefix, err)
	}
	if _, err = Prefix(Snapshot{Messages: messages}, 2); err == nil {
		t.Fatal("seller cutoff accepted")
	}
	ai := Snapshot{Messages: []Message{messages[0], {ID: 4, Role: "ai", Type: "text", Text: "AI advice"}}}
	if HasSalesExchange(ExtractMessages(ai, false)) || !HasSalesExchange(ExtractMessages(ai, true)) {
		t.Fatal("AI opt-in broken")
	}
}
func TestWholeMessageWindows(t *testing.T) {
	messages, _ := fixture()
	messages[1].Text = strings.Repeat("long", 10000)
	windows := Windows(messages, 200)
	seen := map[int64]bool{}
	for _, w := range windows {
		for _, m := range w {
			seen[m.ID] = true
			if m.ID == 2 && m.Text != messages[1].Text {
				t.Fatal("truncated message")
			}
		}
	}
	if len(seen) != 3 {
		t.Fatal("missing messages")
	}
}
func TestProposalEvidenceAndOperations(t *testing.T) {
	_, event := fixture()
	bases := map[string]Payload{"negotiation": Seed("negotiation")}
	rule := Rule{ID: "budget", Condition: "Price concern", Action: "Clarify budget"}
	change := Change{SkillID: "negotiation", Operation: "add_rule", Rule: &rule, EventIDs: []string{"e1"}, TransferTests: []string{"software", "retail", "services"}, LearningAssessment: passingAssessment()}
	messages, _ := fixture()
	source := CaseInput{ID: 42, SourceHash: "snapshot", Snapshot: Snapshot{Messages: messages}}
	out, err := Propose([]Change{change}, []Event{event}, bases, source)
	if err != nil || len(out["negotiation"].Rules) != 2 || len(bases["negotiation"].Rules) != 1 {
		t.Fatalf("proposal failed or mutated base: %v", err)
	}
	change.Operation = "support"
	source.ID = 43
	if _, err = Propose([]Change{change}, []Event{event}, out, source); err != nil {
		t.Fatal(err)
	}
	change.Rule = &Rule{ID: "budget", Condition: "changed", Action: "changed"}
	if _, err = Propose([]Change{change}, []Event{event}, out, CaseInput{}); err == nil {
		t.Fatal("support changed rule")
	}
	change.Operation = "revise_rule"
	change.EventIDs = []string{"missing"}
	if _, err = Propose([]Change{change}, []Event{event}, out, CaseInput{}); err == nil {
		t.Fatal("missing evidence accepted")
	}
	change.EventIDs = []string{"e1"}
	change.SkillID = "closing"
	if _, err = Propose([]Change{change}, []Event{event}, out, CaseInput{}); err == nil {
		t.Fatal("cross-skill evidence accepted")
	}
	if ValidateRules([]Rule{}) != nil || ValidateRules(nil) == nil {
		t.Fatal("empty ablation rules contract")
	}
	var parsed map[string]any
	if ParseJSON("```json\n{\"ok\":true}\n```", &parsed) != nil {
		t.Fatal("fenced JSON rejected")
	}
}

func TestTechniqueChangesRequireTransferabilityAndBoundScope(t *testing.T) {
	rule := Rule{ID: "diagnose-objection", Condition: "客户表达异议，但真实顾虑尚不清楚", Action: "先诊断根因，再针对性重构价值，并换取一个明确承诺。"}
	change := func(id string) Change {
		r := rule
		r.ID = id
		return Change{SkillID: "negotiation", Operation: "add_rule", Rule: &r, EventIDs: []string{"e1"}, TransferTests: []string{"enterprise software", "consumer retail", "professional services"}, LearningAssessment: passingAssessment()}
	}
	if err := ValidateTechniqueChanges([]Change{change("one"), change("two")}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTechniqueChanges([]Change{change("one"), change("two"), change("three")}); err == nil {
		t.Fatal("accepted more than two substantive changes for one skill")
	}
	missing := change("missing")
	missing.TransferTests = nil
	if err := ValidateTechniqueChanges([]Change{missing}); err == nil {
		t.Fatal("accepted a rule without the three-industry transfer test")
	}
	long := change("long")
	long.Rule.Action = strings.Repeat("方", 161)
	if err := ValidateTechniqueChanges([]Change{long}); err == nil {
		t.Fatal("accepted an operational checklist-sized technique")
	}
	for field, mutate := range map[string]func(*LearningAssessment){
		"non-routine move":       func(a *LearningAssessment) { a.NonRoutineMove = false },
		"buyer effect":           func(a *LearningAssessment) { a.ObservedBuyerEffect = false },
		"transferable mechanism": func(a *LearningAssessment) { a.TransferableMechanism = false },
		"new learning":           func(a *LearningAssessment) { a.AddsNewLearning = false },
	} {
		candidate := change("gate-" + strings.ReplaceAll(field, " ", "-"))
		mutate(candidate.LearningAssessment)
		if err := ValidateTechniqueChanges([]Change{candidate}); err == nil {
			t.Fatalf("accepted change without %s", field)
		}
	}
	candidate := change("reason")
	candidate.LearningAssessment.Reason = ""
	if err := ValidateTechniqueChanges([]Change{candidate}); err == nil {
		t.Fatal("accepted learning assessment without an evidence reason")
	}
}

func TestProposalDefaultsToNoLearning(t *testing.T) {
	bases := map[string]Payload{"negotiation": Seed("negotiation")}
	out, err := Propose([]Change{}, nil, bases, CaseInput{})
	if err != nil {
		t.Fatal(err)
	}
	if Hash(out) != Hash(bases) {
		t.Fatal("empty proposal changed the Skill baseline")
	}
}

func TestProposalRequiresQuotedBuyerReactionAfterSellerMove(t *testing.T) {
	messages, event := fixture()
	event.Quotes = event.Quotes[:2]
	rule := Rule{ID: "budget", Condition: "客户提出异议但真实顾虑不明", Action: "诊断异议根因并据此重构价值"}
	change := Change{SkillID: "negotiation", Operation: "add_rule", Rule: &rule, EventIDs: []string{"e1"}, TransferTests: []string{"software", "retail", "services"}, LearningAssessment: passingAssessment()}
	bases := map[string]Payload{"negotiation": Seed("negotiation")}
	_, err := Propose([]Change{change}, []Event{event}, bases, CaseInput{Snapshot: Snapshot{Messages: messages}})
	if err == nil || err.Error() != "evidence" {
		t.Fatalf("accepted seller move without a cited later buyer reaction: %v", err)
	}
}

func TestProposeGatedDropsUnlearnableProposal(t *testing.T) {
	messages, event := fixture()
	event.Quotes = event.Quotes[:2]
	rule := Rule{ID: "budget", Condition: "客户提出异议但真实顾虑不明", Action: "诊断异议根因并据此重构价值"}
	change := Change{SkillID: "negotiation", Operation: "add_rule", Rule: &rule, EventIDs: []string{"e1"}, TransferTests: []string{"software", "retail", "services"}, LearningAssessment: passingAssessment()}
	bases := map[string]Payload{"negotiation": Seed("negotiation")}
	out, rejected, err := ProposeGated([]Change{change}, []Event{event}, bases, CaseInput{Snapshot: Snapshot{Messages: messages}})
	if err != nil {
		t.Fatalf("unlearnable proposal must not fail the task: %v", err)
	}
	if len(rejected) != 1 || rejected[0].SkillID != "negotiation" {
		t.Fatalf("expected one gate rejection, got %#v", rejected)
	}
	if Hash(out) != Hash(bases) {
		t.Fatal("gate-rejected proposal still changed the Skill baseline")
	}
}

func TestProposeGatedKeepsSupportedProposal(t *testing.T) {
	messages, event := fixture()
	rule := Rule{ID: "budget", Condition: "客户提出异议但真实顾虑不明", Action: "诊断异议根因并据此重构价值"}
	change := Change{SkillID: "negotiation", Operation: "add_rule", Rule: &rule, EventIDs: []string{"e1"}, TransferTests: []string{"software", "retail", "services"}, LearningAssessment: passingAssessment()}
	bases := map[string]Payload{"negotiation": Seed("negotiation")}
	out, rejected, err := ProposeGated([]Change{change}, []Event{event}, bases, CaseInput{Snapshot: Snapshot{Messages: messages}})
	if err != nil || len(rejected) != 0 {
		t.Fatalf("supported proposal was dropped: err=%v rejected=%#v", err, rejected)
	}
	if len(out["negotiation"].Rules) != 2 {
		t.Fatal("supported proposal was not applied")
	}
}

func TestChineseDistillationKeepsOriginalQuotesAndReplyLanguage(t *testing.T) {
	for _, prompt := range []string{ExtractPrompt, ChangePrompt} {
		if !strings.Contains(prompt, "Simplified Chinese") {
			t.Fatal("missing Chinese output requirement")
		}
	}
	if !strings.Contains(ExtractPrompt, "never translate or paraphrase quotations") {
		t.Fatal("quotes must remain original evidence")
	}
	for _, required := range []string{"Most conversations contain no technique worth learning", "strict learning gate", "nonRoutineMove", "observedBuyerEffect", "transferableMechanism", "addsNewLearning", "three-industry transfer test", "never summaries of the case", "observable customer signals", "Too vague", "verifiable evidence", "Cluster by sales mechanism", "not Skill vocabulary", "transferTests", "at most two substantive changes per skill"} {
		if !strings.Contains(ChangePrompt, required) {
			t.Fatalf("missing Skill abstraction requirement %q", required)
		}
	}
	if strings.Contains(ReplyPrompt, "Simplified Chinese") || !strings.Contains(ReplyPrompt, "customer's current language") {
		t.Fatal("Skill authoring language must not override customer reply language")
	}
	messages, event := fixture()
	event.Title = "澄清价格顾虑"
	event.Signal = "客户认为价格过高"
	event.SellerAction = "询问预算"
	event.ObservedResult = "未确认成交"
	if err := ValidateEvents(EventsOutput{Summary: "销售询问预算，尚无成交证据", Events: []Event{event}}, messages); err != nil {
		t.Fatal(err)
	}
	markdown := Markdown("negotiation", Seed("negotiation"))
	for _, expected := range []string{"name: sales-negotiation", "# 压价与议价", "客户状态与触发信号：", "可复用销售技巧：", "例外与边界："} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("missing export text %s", expected)
		}
	}
}

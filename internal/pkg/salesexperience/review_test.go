package salesexperience

import "testing"

func TestReviewPreservesQuotesAndUnrelatedRules(t *testing.T) {
	a := Rule{ID: "a", Condition: "价格", Action: "确认需求"}
	b := Rule{ID: "b", Condition: "时间", Action: "确认计划"}
	e := Evidence{RuleID: "a", Events: []Event{{ID: "event", Quotes: []Quote{{MessageID: 1, Text: "Original English quote."}}}}}
	d := RuleDelta{RuleID: "a", After: &a, Evidence: []Evidence{e}}
	p, ok := MergeReview(Payload{Rules: []Rule{b}}, d, &a)
	if !ok || len(p.Rules) != 2 || p.Evidence[0].Events[0].Quotes[0].Text != "Original English quote." {
		t.Fatal("unrelated data or source quote changed")
	}
	p, ok = MergeReview(p, d, &a)
	if !ok || len(p.Rules) != 2 || len(p.Evidence) != 1 {
		t.Fatal("repeated evidence/rule duplicated")
	}
}

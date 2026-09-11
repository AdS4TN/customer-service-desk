package reception

import (
	"strings"
	"testing"
)

func TestPolicyValidationAndDisabledCompatibility(t *testing.T) {
	for _, raw := range []string{"", "null", "not-json", `{"enabled":true}`, `{"fields":[{"key":"invalid key","label":"Name"}]}`} {
		if Prompt(raw) != "" {
			t.Fatalf("activated missing or invalid policy: %q", raw)
		}
	}
	for _, p := range []Policy{
		{Enabled: true},
		{Goal: strings.Repeat("a", 501)},
		{Fields: []Field{{Key: "a", Label: "Name"}, {Key: "b", Label: "name"}}},
		{Fields: []Field{{Key: "a", Label: "One"}, {Key: "a", Label: "Two"}}},
		{Fields: []Field{{Key: "a", Label: ""}}},
		{Fields: make([]Field, 17)},
	} {
		if _, err := Normalize(p); err == nil {
			t.Fatalf("accepted invalid policy: %+v", p)
		}
	}
	p, err := Normalize(Policy{Enabled: true, Goal: " Qualify inquiries ", Fields: []Field{{Key: "qty", Label: " Quantity "}}})
	if err != nil || p.Goal != "Qualify inquiries" || p.Fields[0].Label != "Quantity" {
		t.Fatalf("normalization: %+v %v", p, err)
	}
}

func TestPolicyPromptUsesGoalWithoutGrantingPermissions(t *testing.T) {
	prompt := Prompt(`{"enabled":true,"goal":"Collect delivery region","handoffConditions":"Customer requests a quote","fields":[{"key":"region","label":"Region","askWhen":"Purchase intent","required":true}]}`)
	for _, expected := range []string{"Collect delivery region", "Customer requests a quote", "at most ONE", "never repeat", "not new tool permissions", "conversation_decision", "not authorization", "customer declines"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q", expected)
		}
	}
}

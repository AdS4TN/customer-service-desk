package salesexperience

import "slices"

type RuleDelta struct {
	Conflict  bool       `json:"conflict"`
	Current   *Rule      `json:"current"`
	RuleID    string     `json:"ruleId"`
	Kind      string     `json:"kind"`
	Before    *Rule      `json:"before"`
	After     *Rule      `json:"after"`
	Evidence  []Evidence `json:"evidence"`
	Rationale []string   `json:"rationale"`
}

// Compare immutable extraction input/output, not the live Skill: otherwise a
// later approval would make unrelated current rules appear to be deletions.
func ReviewDiff(base, proposed Payload) []RuleDelta {
	old := map[string]Rule{}
	for _, r := range base.Rules {
		old[r.ID] = r
	}
	seenEvidence, seenChanges := map[string]bool{}, map[string]bool{}
	for _, e := range base.Evidence {
		seenEvidence[Hash(e)] = true
	}
	for _, c := range base.Changes {
		seenChanges[Hash(c)] = true
	}
	out := []RuleDelta{}
	for _, r := range proposed.Rules {
		d := RuleDelta{RuleID: r.ID, After: &r, Kind: "add", Evidence: []Evidence{}, Rationale: []string{}}
		if before, ok := old[r.ID]; ok {
			d.Before = &before
			d.Kind = "revise"
			if before == r {
				d.Kind = "support"
			}
			delete(old, r.ID)
		}
		for _, e := range proposed.Evidence {
			if e.RuleID == r.ID && !seenEvidence[Hash(e)] {
				d.Evidence = append(d.Evidence, e)
			}
		}
		for _, c := range proposed.Changes {
			if c.Rule != nil && c.Rule.ID == r.ID && !seenChanges[Hash(c)] && c.Rationale != "" && !slices.Contains(d.Rationale, c.Rationale) {
				d.Rationale = append(d.Rationale, c.Rationale)
			}
		}
		if d.Kind != "support" || len(d.Evidence) > 0 {
			out = append(out, d)
		}
	}
	for _, r := range base.Rules {
		if _, ok := old[r.ID]; ok {
			out = append(out, RuleDelta{RuleID: r.ID, Kind: "remove", Before: &r, Evidence: []Evidence{}, Rationale: []string{}})
		}
	}
	return out
}

// Merge only the reviewed rule and its new evidence, preserving newer unrelated
// rules. A changed target requires the reviewer to resolve it explicitly.
func MergeReview(current Payload, d RuleDelta, rule *Rule) (Payload, bool) {
	i := slices.IndexFunc(current.Rules, func(r Rule) bool { return r.ID == d.RuleID })
	var live *Rule
	if i >= 0 {
		r := current.Rules[i]
		live = &r
	}
	same := func(a, b *Rule) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
	if !same(live, d.Before) && !same(live, rule) {
		return current, false
	}
	if rule == nil {
		if i >= 0 {
			current.Rules = slices.Delete(current.Rules, i, i+1)
		}
	} else if i >= 0 {
		current.Rules[i] = *rule
	} else {
		current.Rules = append(current.Rules, *rule)
	}
	known := map[string]bool{}
	for _, e := range current.Evidence {
		known[Hash(e)] = true
	}
	for _, e := range d.Evidence {
		if !known[Hash(e)] {
			current.Evidence = append(current.Evidence, e)
			known[Hash(e)] = true
		}
	}
	current.Provenance = "reviewed"
	return current, true
}

package builders

import (
	"agent-desk/internal/models"
	sx "agent-desk/internal/pkg/salesexperience"
	"encoding/json"
	"testing"
	"time"
)

func TestSalesExperienceBlinding(t *testing.T) {
	j := models.SalesExperienceJob{Kind: "evaluate", Input: sx.Encode(sx.ExperimentInput{MountedLabel: "B", Variants: map[string][]sx.Rule{"A": {}, "B": sx.Seed("negotiation").Rules}})}
	v := BuildSalesExperienceJob(j)
	if v.Input.MountedLabel != "" || v.Input.Variants != nil {
		t.Fatal("assignment leaked before human rating")
	}
	j.Rating = "A"
	v = BuildSalesExperienceJob(j)
	if v.Input.MountedLabel != "B" {
		t.Fatal("assignment not revealed")
	}
}

func TestBuildSalesExperienceCaseIncludesPersistedExtraction(t *testing.T) {
	now := time.Now()
	row := models.SalesExperienceCase{
		ExtractionStatus: "succeeded",
		ExtractionModel:  "mock",
		ExtractionResult: `{"ok":true,"result":{"hasLearnableSkill":true}}`,
		ExtractedAt:      &now,
	}
	result := BuildSalesExperienceCase(row)
	if result.ExtractionStatus != "succeeded" || result.ExtractionModel != "mock" || result.ExtractedAt == nil {
		t.Fatalf("extraction metadata missing: %+v", result)
	}
	var persisted map[string]any
	if err := json.Unmarshal(result.ExtractionResult, &persisted); err != nil || persisted["ok"] != true {
		t.Fatalf("persisted extraction result missing: %s", result.ExtractionResult)
	}
}

package builders

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/response"
	sx "agent-desk/internal/pkg/salesexperience"
	"agent-desk/internal/repositories"
	"encoding/json"
)

func BuildSalesExperienceSource(v repositories.ExperienceSource) response.SalesExperienceSource {
	return response.SalesExperienceSource{ID: v.ID, CustomerID: v.CustomerID, CustomerName: v.CustomerName, ChannelType: v.ChannelType, ChannelName: v.ChannelName, LastMessageAt: v.LastMessageAt, MessageCount: v.MessageCount, Status: v.Status}
}
func BuildSalesExperienceCase(v models.SalesExperienceCase) response.SalesExperienceCase {
	r := response.SalesExperienceCase{ID: v.ID, Name: v.Name, CustomerID: v.CustomerID, SourceHash: v.SourceHash, MessageCount: v.MessageCount, Outcome: v.Outcome, OutcomeNote: v.OutcomeNote, ExtractionStatus: v.ExtractionStatus, ExtractionModel: v.ExtractionModel, ExtractedAt: v.ExtractedAt, CreatedAt: v.CreatedAt}
	if v.Snapshot != "" {
		var snapshot sx.Snapshot
		if json.Unmarshal([]byte(v.Snapshot), &snapshot) == nil {
			r.Snapshot = &snapshot
		}
	}
	if json.Valid([]byte(v.ExtractionResult)) {
		r.ExtractionResult = json.RawMessage(v.ExtractionResult)
	}
	return r
}
func BuildSalesExperienceRevision(v models.SalesExperienceRevision) response.SalesExperienceRevision {
	r := response.SalesExperienceRevision{ID: v.ID, SkillID: v.SkillID, ParentID: v.ParentID, JobID: v.JobID, Hash: v.Hash, Note: v.Note, CreatedAt: v.CreatedAt}
	_ = json.Unmarshal([]byte(v.Payload), &r.Payload)
	return r
}
func BuildSalesExperienceJob(v models.SalesExperienceJob) response.SalesExperienceJob {
	r := response.SalesExperienceJob{ID: v.ID, Kind: v.Kind, State: v.State, Stage: v.Stage, ErrorCode: v.ErrorCode, Attempts: v.Attempts, ModelConfigID: v.ModelConfigID, Rating: v.Rating, RatingNote: v.RatingNote, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
	r.CallStartedAt, r.HeartbeatAt, r.LastResponseAt = v.CallStartedAt, v.HeartbeatAt, v.LastResponseAt
	r.OutputChars, r.CallPhase = v.OutputChars, v.CallPhase
	if v.Output != "" {
		var out sx.Output
		if json.Unmarshal([]byte(v.Output), &out) == nil {
			r.Output = &out
		}
	}
	if v.Input != "" {
		var input sx.ExperimentInput
		if json.Unmarshal([]byte(v.Input), &input) == nil {
			// Blinded A/B assignment stays server-side until a human rating is saved.
			if v.Kind == "evaluate" && v.Rating == "" {
				input.MountedLabel = ""
				input.Variants = nil
			}
			r.Input = &input
		}
	}
	if v.ModelSnapshot != "" {
		var c models.AIConfig
		if json.Unmarshal([]byte(v.ModelSnapshot), &c) == nil {
			r.ModelName = c.ModelName
		}
	}
	return r
}

package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
)

// Human assistance reads the published capability, independently of whether
// automatic reception is paused. Disabling/deleting the model still takes effect.
func resolveAssistanceSnapshot(agent *models.AIAgent) (*AgentRevisionSnapshot, error) {
	if agent == nil || agent.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.translation.model")
	}
	config := AIConfigService.Get(agent.AIConfigID)
	if config == nil {
		config = &models.AIConfig{}
	}
	snapshot, err := AgentRevisionService.ResolvePublishedSnapshot(*agent, *config)
	if err != nil || snapshot.AIConfig.ID == 0 || snapshot.AIConfig.Status != enums.StatusOk {
		return nil, errorsx.InvalidParamI18n("error.translation.model")
	}
	return snapshot, nil
}

package builders

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/response"
	"encoding/json"
)

func BuildAutomation(row models.AutomationRule) response.AutomationRule {
	out := response.AutomationRule{ID: row.ID, Name: row.Name, Priority: row.Priority, Enabled: row.Enabled, Revision: row.Revision, UpdatedAt: row.UpdatedAt}
	_ = json.Unmarshal([]byte(row.Definition), &out.Definition)
	return out
}
func BuildAutomationRun(row models.AutomationRun) response.AutomationRun {
	out := response.AutomationRun{ID: row.ID, RuleID: row.RuleID, RuleName: row.RuleName, Revision: row.Revision, ConversationID: row.ConversationID, EventID: row.EventID, Status: row.Status, Results: []string{}, ErrorCode: row.ErrorCode, Attempts: row.Attempts, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	_ = json.Unmarshal([]byte(row.Definition), &out.Definition)
	_ = json.Unmarshal([]byte(row.Result), &out.Results)
	return out
}

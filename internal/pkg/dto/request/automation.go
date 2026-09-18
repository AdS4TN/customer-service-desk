package request

type AutomationCondition struct {
	Field string `json:"field"`
	Value string `json:"value"`
}
type AutomationAction struct {
	Type    string `json:"type"`
	Value   string `json:"value"`
	OwnerID int64  `json:"ownerId"`
}
type AutomationDefinition struct {
	Trigger             string                `json:"trigger"`
	Match               string                `json:"match"`
	OncePerConversation bool                  `json:"oncePerConversation"`
	Conditions          []AutomationCondition `json:"conditions"`
	Actions             []AutomationAction    `json:"actions"`
}
type SaveAutomation struct {
	ID         int64                `json:"id"`
	Revision   int64                `json:"revision"`
	Name       string               `json:"name"`
	Priority   int                  `json:"priority"`
	Definition AutomationDefinition `json:"definition"`
}
type ChangeAutomation struct {
	ID       int64 `json:"id"`
	Revision int64 `json:"revision"`
	Enabled  bool  `json:"enabled"`
}
type TestAutomation struct {
	Definition AutomationDefinition `json:"definition"`
	Channel    string               `json:"channel"`
	Text       string               `json:"text"`
	LeadTags   []string             `json:"leadTags"`
}

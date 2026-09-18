package response

import "agent-desk/internal/pkg/enums"

type AIReceptionState struct {
	AgentID             int64                           `json:"agentId"`
	ServiceMode         enums.IMConversationServiceMode `json:"serviceMode"`
	ReceptionEnabled    bool                            `json:"receptionEnabled"`
	AssistanceAvailable bool                            `json:"assistanceAvailable"`
	PublishedRevisionID int64                           `json:"publishedRevisionId"`
	ModelName           string                          `json:"modelName"`
	KnowledgeCount      int                             `json:"knowledgeCount"`
	SkillCount          int                             `json:"skillCount"`
	Channels            []AIReceptionChannel            `json:"channels"`
}

type AIReceptionChannel struct {
	ID                       int64  `json:"id"`
	Name                     string `json:"name"`
	ChannelType              string `json:"channelType"`
	ReceptionEnabled         bool   `json:"receptionEnabled"`
	AutomaticMessagesAllowed bool   `json:"automaticMessagesAllowed"`
	OutboundBlocked          bool   `json:"outboundBlocked"`
}

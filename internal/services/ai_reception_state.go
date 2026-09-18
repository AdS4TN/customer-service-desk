package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/reception"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/whatsapp"
	"github.com/mlogclub/simple/sqls"
)

// ReceptionState reports saved operational state and published capabilities,
// never draft model credentials or provider error text.
func (s *aIAgentService) ReceptionState(id int64) (*response.AIReceptionState, error) {
	a := s.Get(id)
	if a == nil || a.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0002")
	}
	r := &response.AIReceptionState{AgentID: id, ServiceMode: a.ServiceMode, PublishedRevisionID: a.PublishedRevisionID, Channels: []response.AIReceptionChannel{}}
	snapshot, err := resolveAssistanceSnapshot(a)
	if err == nil {
		r.AssistanceAvailable = true
		r.ModelName = snapshot.AIConfig.ModelName
		r.KnowledgeCount = len(utils.SplitInt64s(snapshot.Agent.KnowledgeIDs))
		r.SkillCount = len(utils.SplitInt64s(snapshot.Agent.SkillIDs))
	}
	c := &models.Conversation{ServiceMode: a.ServiceMode}
	r.ReceptionEnabled = reception.AutomaticMessagesAllowed(c, a)
	for _, channel := range ChannelService.Find(sqls.NewCnd().Eq("ai_agent_id", id).NotEq("status", enums.StatusDeleted).Asc("id")) {
		enabled := channel.Status == enums.StatusOk
		blocked := channel.ChannelType == enums.ChannelTypeWhatsApp && whatsapp.OutboundMessagesDisabled
		r.Channels = append(r.Channels, response.AIReceptionChannel{ID: channel.ID, Name: channel.Name, ChannelType: channel.ChannelType,
			ReceptionEnabled: enabled, OutboundBlocked: blocked, AutomaticMessagesAllowed: enabled && r.ReceptionEnabled && r.AssistanceAvailable && !blocked})
	}
	return r, nil
}

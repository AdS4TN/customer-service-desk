package services

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

var SalesLeadService = &salesLeadService{}

type salesLeadService struct{}
type SalesLeadView struct {
	Lead          models.SalesLead
	Sources       []models.Message
	ProposalValid bool
	SourceValid   bool
	OwnerName     string
	Events        []models.SalesLeadEvent
}

func leadError() error { return errorsx.InvalidParamI18n("error.lead.failed") }
func validLeadStatus(status enums.LeadStatus) bool {
	switch status {
	case enums.LeadStatusNew, enums.LeadStatusFollowing, enums.LeadStatusQualified, enums.LeadStatusWon, enums.LeadStatusLost, enums.LeadStatusArchived:
		return true
	}
	return false
}
func (s *salesLeadService) view(row models.SalesLead, events bool) (*SalesLeadView, error) {
	r := repositories.ConversationMemoryRepository
	var ids, proposalIDs []int64
	_ = json.Unmarshal([]byte(row.SourceMessageIDs), &ids)
	_ = json.Unmarshal([]byte(row.ProposedSourceIDs), &proposalIDs)
	sources, err := r.Sources(sqls.DB(), row.ConversationID, append(append([]int64{}, ids...), proposalIDs...))
	if err != nil {
		return nil, leadError()
	}
	available := map[int64]bool{}
	customerSources := []models.Message{}
	for _, m := range sources {
		if m.SenderType == enums.IMSenderTypeCustomer && m.SendStatus != enums.IMMessageStatusRecalled {
			available[m.ID] = true
			customerSources = append(customerSources, m)
		}
	}
	valid := func(ids []int64) bool {
		if len(ids) == 0 {
			return false
		}
		for _, id := range ids {
			if !available[id] {
				return false
			}
		}
		return true
	}
	v := &SalesLeadView{Lead: row, Sources: customerSources, SourceValid: valid(ids), ProposalValid: valid(proposalIDs)}
	c := ConversationService.Get(row.ConversationID)
	if c == nil || c.AIAgentID != row.AIAgentID || c.CustomerID != row.CustomerID {
		v.SourceValid = false
		v.ProposalValid = false
		v.Sources = nil
	}
	if row.OwnerID > 0 {
		if u := repositories.UserRepository.Get(sqls.DB(), row.OwnerID); u != nil {
			v.OwnerName = u.Nickname
			if v.OwnerName == "" {
				v.OwnerName = u.Username
			}
		}
	}
	if events {
		v.Events, err = repositories.SalesLeadRepository.Events(sqls.DB(), row.ID)
		if err != nil {
			return nil, leadError()
		}
	}
	return v, nil
}
func (s *salesLeadService) Get(id int64) (*SalesLeadView, error) {
	row, err := repositories.SalesLeadRepository.Get(sqls.DB(), id)
	if err != nil || row == nil {
		return nil, leadError()
	}
	return s.view(*row, true)
}
func (s *salesLeadService) List(conversationID, ownerID int64, status, keyword, tag, queue string, page, limit int) ([]SalesLeadView, int64, error) {
	if queue != "" && queue != "unassigned" && queue != "unscheduled" && queue != "overdue" && queue != "upcoming" {
		return nil, 0, leadError()
	}
	if status != "" && !validLeadStatus(enums.LeadStatus(status)) || tag != "" && !validLeadTag(enums.LeadTag(tag)) {
		return nil, 0, leadError()
	}
	rows, total, err := repositories.SalesLeadRepository.List(sqls.DB(), conversationID, ownerID, status, strings.TrimSpace(keyword), tag, queue, page, limit)
	if err != nil {
		return nil, 0, leadError()
	}
	views := []SalesLeadView{}
	for _, row := range rows {
		v, err := s.view(row, false)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, *v)
	}
	return views, total, nil
}
func (s *salesLeadService) Update(id, userID int64, req request.UpdateSalesLead) error {
	if req.Revision <= 0 || userID <= 0 || !validLeadStatus(req.Status) || req.OwnerID < 0 || utf8.RuneCountInString(req.Note) > 2000 || len(req.CustomTags) > 12 {
		return leadError()
	}
	if err := validateLeadData(&req.Data); err != nil {
		return errorsx.InvalidParamI18n("error.lead.invalid")
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range req.CustomTags {
		tag = strings.TrimSpace(tag)
		if utf8.RuneCountInString(tag) > 40 {
			return errorsx.InvalidParamI18n("error.lead.invalid")
		}
		if tag != "" && !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}
	return sqls.WithTransaction(func(tx *sqls.TxContext) error {
		r := repositories.SalesLeadRepository
		row, err := r.Get(tx.Tx, id)
		if err != nil || row == nil {
			return leadError()
		}
		if row.Revision != req.Revision {
			return errorsx.InvalidParamI18n("error.lead.conflict")
		}
		if row.Status != string(req.Status) || row.OwnerID != req.OwnerID || !sameFollowUpTime(row.FollowUpAt, req.FollowUpAt) {
			return errorsx.InvalidParamI18n("error.lead.followupRequired")
		}
		if req.OwnerID > 0 {
			u := repositories.UserRepository.Get(tx.Tx, req.OwnerID)
			if u == nil || u.Status != enums.StatusOk {
				return errorsx.InvalidParamI18n("error.lead.invalid")
			}
		}
		kind := "confirmed"
		idsText := row.SourceMessageIDs
		if req.AcceptProposal {
			if row.ProposedData == "" {
				return errorsx.InvalidParamI18n("error.lead.conflict")
			}
			idsText = row.ProposedSourceIDs
			kind = "accepted"
		}
		var ids []int64
		_ = json.Unmarshal([]byte(idsText), &ids)
		sources, err := repositories.ConversationMemoryRepository.Sources(tx.Tx, row.ConversationID, ids)
		if err != nil || len(sources) != len(ids) || len(ids) == 0 {
			return errorsx.InvalidParamI18n("error.lead.source")
		}
		for _, source := range sources {
			if source.SenderType != enums.IMSenderTypeCustomer || source.SendStatus == enums.IMMessageStatusRecalled {
				return errorsx.InvalidParamI18n("error.lead.source")
			}
		}
		c := repositories.ConversationRepository.Get(tx.Tx, row.ConversationID)
		if c == nil || c.AIAgentID != row.AIAgentID || c.CustomerID != row.CustomerID {
			return errorsx.InvalidParamI18n("error.lead.source")
		}
		data, _ := json.Marshal(req.Data)
		custom, _ := json.Marshal(tags)
		row.Data = string(data)
		row.SourceMessageIDs = idsText
		row.CustomTags = string(custom)
		row.Status = string(req.Status)
		row.OwnerID = req.OwnerID
		row.Note = strings.TrimSpace(req.Note)
		row.FollowUpAt = req.FollowUpAt
		row.Confirmed = true
		row.ProposedData = ""
		row.ProposedSourceIDs = ""
		row.Revision++
		row.UpdatedBy = userID
		row.UpdatedAt = time.Now()
		if req.AcceptProposal {
			row.Withdrawn = false
		}
		ok, err := r.Update(tx.Tx, row, req.Revision)
		if err != nil {
			return leadError()
		}
		if !ok {
			return errorsx.InvalidParamI18n("error.lead.conflict")
		}
		snapshot, _ := json.Marshal(row)
		if err := r.Event(tx.Tx, &models.SalesLeadEvent{LeadID: id, Kind: kind, Data: string(snapshot), ActorID: userID, CreatedAt: row.UpdatedAt}); err != nil {
			return leadError()
		}
		// Human changes invalidate the coalescing extraction's CAS revision.
		return repositories.ConversationMemoryRepository.Queue(tx.Tx, row.ConversationID)
	})
}

package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var AutomationService = &automationService{}

type automationService struct{ tick sync.Mutex }

func automationError(code string) error { return errorsx.InvalidParamI18n("error.automation." + code) }

func validateAutomation(d request.AutomationDefinition) error {
	if (d.Trigger != "message_received" && d.Trigger != "lead_created") || (d.Match != "all" && d.Match != "any") || len(d.Conditions) > 20 || len(d.Actions) == 0 || len(d.Actions) > 8 {
		return automationError("invalid")
	}
	for _, c := range d.Conditions {
		if strings.TrimSpace(c.Value) == "" || utf8.RuneCountInString(c.Value) > 200 {
			return automationError("invalid")
		}
		switch c.Field {
		case "channel":
			if !slices.Contains([]string{"web", "whatsapp", "messenger"}, c.Value) {
				return automationError("invalid")
			}
		case "text":
		case "lead_tag":
			if d.Trigger != "lead_created" || !validLeadTag(enums.LeadTag(c.Value)) {
				return automationError("invalid")
			}
		default:
			return automationError("invalid")
		}
	}
	seen := map[string]bool{}
	for _, a := range d.Actions {
		if seen[a.Type] {
			return automationError("invalid")
		}
		seen[a.Type] = true
		switch a.Type {
		case "extract_lead":
			if d.Trigger != "message_received" {
				return automationError("invalid")
			}
		case "create_ticket":
			if d.Trigger != "message_received" || !d.OncePerConversation || strings.TrimSpace(a.Value) == "" || utf8.RuneCountInString(a.Value) > 120 || a.OwnerID < 0 {
				return automationError("invalid")
			}
		case "tag_lead":
			if d.Trigger != "lead_created" || strings.TrimSpace(a.Value) == "" || utf8.RuneCountInString(a.Value) > 40 {
				return automationError("invalid")
			}
		case "assign_lead":
			if d.Trigger != "lead_created" || a.OwnerID <= 0 {
				return automationError("invalid")
			}
		default:
			return automationError("invalid")
		}
	}
	return nil
}

// Recheck the rule operator's current permissions at execution, not just save.
func authorizeAutomation(db *gorm.DB, userID int64, d request.AutomationDefinition) error {
	u := repositories.UserRepository.Get(db, userID)
	if u == nil || u.Status != enums.StatusOk {
		return automationError("permission")
	}
	roles, err := AuthService.loadUserRoleCodes(db, userID)
	if err != nil {
		return err
	}
	if slices.Contains(roles, constants.RoleCodeSuperAdmin) {
		return nil
	}
	perms, err := AuthService.loadUserPermissionCodes(db, userID)
	if err != nil {
		return err
	}
	needed := []string{constants.PermissionAIAgentUpdate.Code, constants.PermissionConversationView.Code, constants.PermissionConversationSend.Code}
	for _, a := range d.Actions {
		if a.Type == "create_ticket" {
			needed = append(needed, constants.PermissionTicketCreate.Code)
		}
	}
	for _, p := range needed {
		if !slices.Contains(perms, p) {
			return automationError("permission")
		}
	}
	return nil
}
func validateAutomationOwners(db *gorm.DB, d request.AutomationDefinition) error {
	for _, a := range d.Actions {
		if a.OwnerID > 0 {
			u := repositories.UserRepository.Get(db, a.OwnerID)
			if u == nil || u.Status != enums.StatusOk {
				return automationError("owner")
			}
		}
	}
	return nil
}
func (s *automationService) List() ([]models.AutomationRule, error) {
	return repositories.AutomationRepository.Rules(sqls.DB(), false)
}
func (s *automationService) Runs(id int64, page, limit int) ([]models.AutomationRun, int64, error) {
	return repositories.AutomationRepository.Runs(sqls.DB(), id, page, limit)
}
func (s *automationService) Save(req request.SaveAutomation, userID int64) (*models.AutomationRule, error) {
	if strings.TrimSpace(req.Name) == "" || utf8.RuneCountInString(req.Name) > 120 || req.Priority < 0 || req.Priority > 10000 {
		return nil, automationError("invalid")
	}
	if err := validateAutomation(req.Definition); err != nil {
		return nil, err
	}
	var row *models.AutomationRule
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		if err := authorizeAutomation(tx.Tx, userID, req.Definition); err != nil {
			return err
		}
		if err := validateAutomationOwners(tx.Tx, req.Definition); err != nil {
			return err
		}
		r := repositories.AutomationRepository
		row = &models.AutomationRule{CreatedAt: time.Now(), Revision: 0}
		if req.ID > 0 {
			var err error
			row, err = r.Get(tx.Tx, req.ID)
			if err != nil {
				return automationError("missing")
			}
			if row.Revision != req.Revision {
				return automationError("conflict")
			}
		}
		definition, _ := json.Marshal(req.Definition)
		row.Name, row.Priority, row.Definition = strings.TrimSpace(req.Name), req.Priority, string(definition)
		row.Enabled = false // Every edit is a draft until explicitly enabled.
		row.OperatorID = userID
		row.Revision++
		row.UpdatedAt = time.Now()
		return r.Save(tx.Tx, row)
	})
	return row, err
}
func (s *automationService) Change(req request.ChangeAutomation, userID int64, remove bool) error {
	return sqls.WithTransaction(func(tx *sqls.TxContext) error {
		r := repositories.AutomationRepository
		row, err := r.Get(tx.Tx, req.ID)
		if err != nil {
			return automationError("missing")
		}
		if row.Revision != req.Revision {
			return automationError("conflict")
		}
		var d request.AutomationDefinition
		if json.Unmarshal([]byte(row.Definition), &d) != nil {
			return automationError("invalid")
		}
		if err := authorizeAutomation(tx.Tx, userID, d); err != nil {
			return err
		}
		if req.Enabled && !remove {
			if err := validateAutomation(d); err != nil {
				return err
			}
			if err := validateAutomationOwners(tx.Tx, d); err != nil {
				return err
			}
			if !row.Enabled {
				row.Cursor, err = r.Head(tx.Tx, d.Trigger)
				if err != nil {
					return err
				}
			}
		}
		row.Enabled = req.Enabled && !remove
		row.Deleted = remove
		row.OperatorID = userID
		row.Revision++
		row.UpdatedAt = time.Now()
		return r.Save(tx.Tx, row)
	})
}

func matchAutomation(d request.AutomationDefinition, channel, text string, tags []string) (bool, []bool) {
	results := make([]bool, 0, len(d.Conditions))
	matched := d.Match == "all" || len(d.Conditions) == 0
	for _, c := range d.Conditions {
		hit := false
		switch c.Field {
		case "channel":
			hit = channel == c.Value
		case "text":
			hit = strings.Contains(strings.ToLower(text), strings.ToLower(strings.TrimSpace(c.Value)))
		case "lead_tag":
			hit = slices.Contains(tags, c.Value)
		}
		results = append(results, hit)
		if d.Match == "all" {
			matched = matched && hit
		} else {
			matched = matched || hit
		}
	}
	return matched, results
}
func (s *automationService) Test(req request.TestAutomation) (bool, []bool, error) {
	if err := validateAutomation(req.Definition); err != nil {
		return false, nil, err
	}
	if len(req.Text) > 20000 {
		return false, nil, automationError("invalid")
	}
	matched, conditions := matchAutomation(req.Definition, req.Channel, req.Text, req.LeadTags)
	return matched, conditions, nil
}

type automationEvent struct {
	conversation  *models.Conversation
	message       *models.Message
	lead          *models.SalesLead
	text, channel string
	tags          []string
}

func loadAutomationEvent(db *gorm.DB, trigger string, id int64) (*automationEvent, error) {
	e := &automationEvent{}
	if trigger == "message_received" {
		e.message = repositories.MessageRepository.Get(db, id)
		if e.message == nil || e.message.IsHistorical || e.message.SenderType != enums.IMSenderTypeCustomer || e.message.SendStatus == enums.IMMessageStatusRecalled {
			return nil, nil
		}
		e.text = e.message.Content
		e.conversation = repositories.ConversationRepository.Get(db, e.message.ConversationID)
	} else {
		var err error
		e.lead, err = repositories.SalesLeadRepository.Get(db, id)
		if err != nil {
			return nil, err
		}
		if e.lead == nil || e.lead.Withdrawn || leadClosed(e.lead.Status) {
			return nil, nil
		}
		e.conversation = repositories.ConversationRepository.Get(db, e.lead.ConversationID)
		var data request.LeadData
		if json.Unmarshal([]byte(e.lead.Data), &data) != nil {
			return nil, automationError("source")
		}
		var ids []int64
		_ = json.Unmarshal([]byte(e.lead.SourceMessageIDs), &ids)
		sources, err := repositories.ConversationMemoryRepository.Sources(db, e.lead.ConversationID, ids)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 || len(sources) != len(ids) {
			return nil, nil
		}
		live := false
		for _, m := range sources {
			if m.SenderType != enums.IMSenderTypeCustomer || m.SendStatus == enums.IMMessageStatusRecalled {
				return nil, nil
			}
			live = live || !m.IsHistorical
			e.text += m.Content + "\n"
		}
		if !live {
			return nil, nil
		}
		for _, tag := range data.AutoTags {
			e.tags = append(e.tags, string(tag))
		}
	}
	if e.conversation == nil || (e.lead != nil && (e.conversation.CustomerID != e.lead.CustomerID || e.conversation.AIAgentID != e.lead.AIAgentID)) {
		return nil, nil
	}
	if c := repositories.ChannelRepository.Get(db, e.conversation.ChannelID); c != nil {
		e.channel = c.ChannelType
	}
	return e, nil
}

func (s *automationService) ProcessPending() {
	if !s.tick.TryLock() {
		return
	}
	defer s.tick.Unlock()
	r := repositories.AutomationRepository
	rules, err := r.Rules(sqls.DB(), true)
	if err != nil {
		slog.Warn("automation scan failed")
		return
	}
	groups := map[string][]models.AutomationRule{}
	for _, rule := range rules {
		var d request.AutomationDefinition
		if json.Unmarshal([]byte(rule.Definition), &d) != nil {
			continue
		}
		groups[d.Trigger] = append(groups[d.Trigger], rule)
	}
	for _, trigger := range []string{"message_received", "lead_created"} {
		group := groups[trigger]
		if len(group) == 0 {
			continue
		}
		cursor := group[0].Cursor
		for _, rule := range group {
			cursor = min(cursor, rule.Cursor)
		}
		ids, err := r.Events(sqls.DB(), trigger, cursor)
		if err != nil {
			slog.Warn("automation events failed", "trigger", trigger)
			continue
		}
		// Match every rule for an event before moving forward. This preserves
		// priority even when enabling a new rule creates unequal scan cursors.
		for _, id := range ids {
			for _, rule := range group {
				if id <= rule.Cursor {
					continue
				}
				if err := s.process(rule.ID, id, 0, 0); err != nil {
					slog.Warn("automation process failed", "rule_id", rule.ID, "event_id", id)
					return
				}
			}
		}
	}
}
func (s *automationService) Retry(id, userID int64) error {
	run, err := repositories.AutomationRepository.Run(sqls.DB(), id)
	if err != nil {
		return automationError("missing")
	}
	return s.process(run.RuleID, run.EventID, id, userID)
}

func (s *automationService) process(ruleID, eventID, retryID, userID int64) error {
	var notices []*models.Notification
	err := sqls.WithTransaction(func(tx *sqls.TxContext) error {
		r := repositories.AutomationRepository
		rule, err := r.Get(tx.Tx, ruleID)
		if err != nil {
			return err
		}
		if !rule.Enabled {
			if retryID > 0 {
				return automationError("disabled")
			}
			return nil
		}
		var d request.AutomationDefinition
		if json.Unmarshal([]byte(rule.Definition), &d) != nil {
			return automationError("invalid")
		}
		if retryID == 0 && eventID <= rule.Cursor {
			return nil
		}
		advance := func() error {
			if retryID > 0 {
				return nil
			}
			rule.Cursor = eventID
			return r.Save(tx.Tx, rule)
		}
		e, err := loadAutomationEvent(tx.Tx, d.Trigger, eventID)
		if err != nil {
			return err
		}
		var previous *models.AutomationRun
		if retryID > 0 {
			if err := authorizeAutomation(tx.Tx, userID, d); err != nil {
				return err
			}
			previous, err = r.Run(tx.Tx, retryID)
			if err != nil {
				return err
			}
			if previous.RuleID != rule.ID || previous.Revision != rule.Revision || previous.Status != "failed" {
				return automationError("conflict")
			}
		}
		if e == nil {
			if previous != nil {
				previous.Status = "skipped"
				previous.ErrorCode = "source"
				previous.UpdatedAt = time.Now()
				return r.SaveRun(tx.Tx, previous)
			}
			return advance()
		}
		hit, _ := matchAutomation(d, e.channel, e.text, e.tags)
		if !hit {
			if previous != nil {
				previous.Status = "skipped"
				previous.ErrorCode = "notMatched"
				return r.SaveRun(tx.Tx, previous)
			}
			return advance()
		}
		key := fmt.Sprintf("%s/%d", d.Trigger, eventID)
		if d.Trigger == "message_received" && d.OncePerConversation {
			key = fmt.Sprintf("conversation/%d", e.conversation.ID)
		}
		if retryID == 0 {
			_, err := r.Receipt(tx.Tx, rule.ID, key)
			if err == nil {
				return advance()
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		run := previous
		if run == nil {
			run = &models.AutomationRun{RuleID: rule.ID, ScopeKey: key, RuleName: rule.Name, Revision: rule.Revision, Definition: rule.Definition, EventID: eventID, ConversationID: e.conversation.ID, Attempts: 0, CreatedAt: time.Now()}
		}
		run.Attempts++
		run.UpdatedAt = time.Now()
		run.Status = "completed"
		run.ErrorCode = ""
		// A failed step rolls back every action, while the outer transaction keeps
		// a failure receipt and advances the scan so later events are not blocked.
		if err := tx.Tx.SavePoint("automation_actions").Error; err != nil {
			return err
		}
		results := []string{}
		actionErr := authorizeAutomation(tx.Tx, rule.OperatorID, d)
		code := "permission"
		if actionErr == nil {
			actionErr = validateAutomationOwners(tx.Tx, d)
			code = "owner"
		}
		if actionErr == nil {
			code = "actionFailed"
			results, notices, actionErr = s.actions(tx.Tx, rule, d, e)
		}
		if actionErr != nil {
			if err := tx.Tx.RollbackTo("automation_actions").Error; err != nil {
				return err
			}
			notices = nil
			results = nil
			run.Status = "failed"
			run.ErrorCode = code
		}
		encoded, _ := json.Marshal(results)
		run.Result = string(encoded)
		if err := r.SaveRun(tx.Tx, run); err != nil {
			return err
		}
		return advance()
	})
	if err == nil {
		for _, n := range notices {
			NotificationService.Push(n)
		}
	}
	return err
}

func (s *automationService) actions(db *gorm.DB, rule *models.AutomationRule, d request.AutomationDefinition, e *automationEvent) ([]string, []*models.Notification, error) {
	results := []string{}
	notices := []*models.Notification{}
	for _, a := range d.Actions {
		switch a.Type {
		case "extract_lead":
			if e.conversation.AIAgentID <= 0 {
				return nil, nil, automationError("source")
			}
			memory, err := repositories.ConversationMemoryRepository.Get(db, e.conversation.ID)
			if err != nil {
				return nil, nil, err
			}
			// Incoming messages already queue analysis through the reception path.
			// Do not invalidate a running extraction or repeat a completed one.
			if memory != nil && (memory.Status == "queued" || memory.Status == "processing" || (e.message != nil && memory.ProcessedMessageID >= e.message.ID)) {
				results = append(results, "extraction_queued")
				continue
			}
			if err := repositories.ConversationMemoryRepository.Queue(db, e.conversation.ID); err != nil {
				return nil, nil, err
			}
			results = append(results, "extraction_queued")
		case "tag_lead", "assign_lead":
			row := e.lead
			if row == nil {
				return nil, nil, automationError("source")
			}
			revision := row.Revision
			if a.Type == "tag_lead" {
				var tags []string
				_ = json.Unmarshal([]byte(row.CustomTags), &tags)
				value := strings.TrimSpace(a.Value)
				if slices.Contains(tags, value) {
					results = append(results, "tag_exists")
					continue
				}
				if len(tags) >= 12 {
					return nil, nil, automationError("invalid")
				}
				tags = append(tags, value)
				encoded, _ := json.Marshal(tags)
				row.CustomTags = string(encoded)
			} else {
				if row.OwnerID > 0 {
					results = append(results, "owner_preserved")
					continue
				}
				row.OwnerID = a.OwnerID
				n := leadNotification(row, "lead_assigned")
				if err := repositories.NotificationRepository.Create(db, n); err != nil {
					return nil, nil, err
				}
				notices = append(notices, n)
			}
			row.Revision++
			row.UpdatedAt = time.Now()
			row.UpdatedBy = rule.OperatorID
			ok, err := repositories.SalesLeadRepository.Update(db, row, revision)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				return nil, nil, automationError("conflict")
			}
			eventData, _ := json.Marshal(map[string]any{"ruleId": rule.ID, "action": a.Type, "value": a.Value, "ownerId": a.OwnerID})
			if err := repositories.SalesLeadRepository.Event(db, &models.SalesLeadEvent{LeadID: row.ID, Kind: "automation", Data: string(eventData), ActorID: rule.OperatorID, CreatedAt: time.Now()}); err != nil {
				return nil, nil, err
			}
			results = append(results, fmt.Sprintf("%s:%d", a.Type, row.ID))
		case "create_ticket":
			ticket, err := TicketService.createAutomationTicket(db, rule, e.conversation, e.message, a)
			if err != nil {
				return nil, nil, err
			}
			results = append(results, fmt.Sprintf("ticket:%d", ticket.ID))
		}
	}
	return results, notices, nil
}

package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"
	"encoding/json"
	"fmt"
	"github.com/mlogclub/simple/sqls"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *conversationWorkService) Colleagues() ([]response.ConversationColleague, error) {
	result := []response.ConversationColleague{}
	for _, user := range UserService.Find(sqls.NewCnd().Eq("status", enums.StatusOk).Asc("id")) {
		permissions, err := AuthService.GetUserPermissions(user.ID)
		if err != nil {
			return nil, errorsx.InvalidParamI18n("error.reception.failed")
		}
		if slices.Contains(permissions, constants.PermissionConversationView.Code) {
			result = append(result, response.ConversationColleague{ID: user.ID, Name: collaborationName(user.ID)})
		}
	}
	return result, nil
}

func collaborationName(id int64) string {
	if id == 0 {
		return ""
	}
	if user := UserService.Get(id); user != nil {
		if user.Nickname != "" {
			return user.Nickname
		}
		return user.Username
	}
	return fmt.Sprintf("#%d", id)
}

func (s *conversationWorkService) Collaboration(id, before int64) (*response.ConversationCollaboration, error) {
	if ConversationService.Get(id) == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	notes, err := repositories.ListConversationNotes(sqls.DB(), id, before)
	if err != nil {
		return nil, errorsx.InvalidParamI18n("error.reception.failed")
	}
	handoffs, err := repositories.ConversationHandoffHistory(sqls.DB(), id)
	if err != nil {
		return nil, errorsx.InvalidParamI18n("error.reception.failed")
	}
	view := &response.ConversationCollaboration{Notes: []response.ConversationNote{}, Handoffs: []response.ConversationHandoff{}, HasMore: len(notes) == 50}
	for _, note := range notes {
		entry := response.ConversationNote{ID: note.ID, AuthorID: note.AuthorID, AuthorName: collaborationName(note.AuthorID), Content: note.Content, CreatedAt: utils.FormatTime(note.CreatedAt), Mentions: []response.ConversationColleague{}}
		var ids []int64
		_ = json.Unmarshal([]byte(note.MentionIDs), &ids)
		for _, id := range ids {
			entry.Mentions = append(entry.Mentions, response.ConversationColleague{ID: id, Name: collaborationName(id)})
		}
		view.Notes = append(view.Notes, entry)
	}
	for _, handoff := range handoffs {
		view.Handoffs = append(view.Handoffs, response.ConversationHandoff{ID: handoff.ID, From: collaborationName(handoff.FromUserID), To: collaborationName(handoff.ToUserID), Reason: handoff.Reason, CreatedAt: utils.FormatTime(handoff.CreatedAt)})
	}
	return view, nil
}

func (s *conversationWorkService) AddNote(id int64, req request.CreateConversationNote, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.ClientID == "" || len(req.ClientID) > 64 || req.Content == "" || utf8.RuneCountInString(req.Content) > 5000 || len(req.MentionIDs) > 20 {
		return errorsx.InvalidParamI18n("error.reception.invalidNote")
	}
	item := ConversationService.Get(id)
	if item == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	ids := []int64{}
	for _, id := range req.MentionIDs {
		user := UserService.Get(id)
		permissions, err := AuthService.GetUserPermissions(id)
		if err != nil || user == nil || user.Status != enums.StatusOk || !slices.Contains(permissions, constants.PermissionConversationView.Code) {
			return errorsx.InvalidParamI18n("error.reception.invalidMention")
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	data, _ := json.Marshal(ids)
	note := &models.ConversationNote{ConversationID: id, AuthorID: operator.UserID, ClientID: req.ClientID, Content: req.Content, MentionIDs: string(data), CreatedAt: time.Now()}
	var notices []*models.Notification
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		created, err := repositories.CreateConversationNote(ctx.Tx, note)
		if err != nil || !created {
			return err
		}
		for _, userID := range ids {
			if userID == operator.UserID {
				continue
			}
			notice := receptionNotification(item, userID, "conversation_mention", "notification.reception.mention")
			if err := repositories.NotificationRepository.Create(ctx.Tx, notice); err != nil {
				return err
			}
			notices = append(notices, notice)
		}
		return nil
	})
	if err != nil {
		return errorsx.InvalidParamI18n("error.reception.failed")
	}
	for _, notice := range notices {
		NotificationService.Push(notice)
	}
	return nil
}

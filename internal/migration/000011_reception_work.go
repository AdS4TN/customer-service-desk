package migration

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"errors"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"time"
)

func init() {
	register(11, "initialize reception handling states", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			var conversations []models.Conversation
			if err := ctx.Tx.Where("work_revision = 0").Find(&conversations).Error; err != nil {
				return err
			}
			for _, item := range conversations {
				var last, customer models.Message
				base := ctx.Tx.Where("conversation_id = ? AND is_historical = ? AND recalled_at IS NULL AND send_status NOT IN ?", item.ID, false, []enums.IMMessageStatus{enums.IMMessageStatusFailed, enums.IMMessageStatusRecalled}).Order("id DESC")
				if err := base.Session(&gorm.Session{}).Where("sender_type = ?", enums.IMSenderTypeCustomer).First(&customer).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if err := base.Session(&gorm.Session{}).Where("sender_type IN ?", []enums.IMSenderType{enums.IMSenderTypeCustomer, enums.IMSenderTypeAgent, enums.IMSenderTypeAI}).Where("send_status <> ?", enums.IMMessageStatusSending).First(&last).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				updates := map[string]any{"work_revision": 1, "last_customer_message_id": customer.ID}
				if item.Status != enums.IMConversationStatusClosed && last.SenderType == enums.IMSenderTypeCustomer {
					since := last.CreatedAt
					if last.SentAt != nil {
						since = *last.SentAt
					}
					if since.IsZero() {
						since = time.Now()
					}
					updates["work_status"] = enums.ConversationWorkStatusNeedsReply
					updates["pending_since"] = since
					updates["reply_due_at"] = since.Add(15 * time.Minute)
				}
				if err := ctx.Tx.Model(&models.Conversation{}).Where("id = ? AND work_revision = 0", item.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}

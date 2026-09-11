package migration

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(10, "normalize legacy chunk providers", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := ctx.Tx.Model(&models.KnowledgeBase{}).
				Where("chunk_provider = ?", "semantic").
				Update("chunk_provider", string(enums.KnowledgeChunkProviderStructured)).Error; err != nil {
				return err
			}
			if err := ctx.Tx.Model(&models.KnowledgeBase{}).
				Where("chunk_provider = ?", string(enums.KnowledgeChunkProviderFixed)).
				Update("chunk_provider", string(enums.KnowledgeChunkProviderRecursive)).Error; err != nil {
				return err
			}
			return ctx.Tx.Model(&models.KnowledgeDocument{}).
				Where("chunk_config_override = ? AND chunk_provider = ?", true, string(enums.KnowledgeChunkProviderFixed)).
				Update("chunk_provider", string(enums.KnowledgeChunkProviderRecursive)).Error
		})
	})
}

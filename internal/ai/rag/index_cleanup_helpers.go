package rag

import (
	"context"
	"fmt"
	"log/slog"

	"agent-desk/internal/ai/rag/vectordb"
	"agent-desk/internal/repositories"
	"github.com/mlogclub/simple/sqls"
)

func (s *index) deleteVectorsByFilter(ctx context.Context, filter *vectordb.SearchFilter) error {
	provider := vectordb.GetProvider()
	if provider == nil {
		return fmt.Errorf("vectordb provider not initialized")
	}
	return provider.DeleteVectorsByFilter(ctx, s.getCollectionName(), filter)
}

func (s *index) cleanupKnowledgeBaseChunks(ctx context.Context, knowledgeBaseID int64) error {
	if err := s.deleteVectorsByFilter(ctx, &vectordb.SearchFilter{KnowledgeBaseIDs: []int64{knowledgeBaseID}}); err != nil {
		return fmt.Errorf("failed to delete vectors for knowledge base %d before rebuild: %w", knowledgeBaseID, err)
	}
	if err := repositories.KnowledgeChunkRepository.DeleteByKnowledgeBaseID(sqls.DB(), knowledgeBaseID); err != nil {
		return fmt.Errorf("failed to clear chunks before rebuild: %w", err)
	}
	slog.Info("Knowledge base index storage reset",
		"knowledge_base_id", knowledgeBaseID,
		"collection", s.getCollectionName())
	return nil
}

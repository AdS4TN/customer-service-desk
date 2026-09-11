package rag

import (
	"context"
	"fmt"

	"agent-desk/internal/ai"
	"agent-desk/internal/ai/rag/vectordb"
	"agent-desk/internal/models"
	"agent-desk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"time"
)

type DocumentIndexStage string

const (
	DocumentIndexStageChunking  DocumentIndexStage = "chunking"
	DocumentIndexStageEmbedding DocumentIndexStage = "embedding"
	DocumentIndexStageIndexing  DocumentIndexStage = "indexing"
)

type DocumentIndexResult struct {
	ChunkCount  int
	VectorCount int
	ChunkMS     int64
	EmbeddingMS int64
	IndexMS     int64
}

func (s *index) runDocumentIndex(ctx context.Context, document models.KnowledgeDocument, knowledgeBase models.KnowledgeBase) ([]vectordb.Vector, int, error) {
	result, vectors, err := s.runDocumentIndexWithProgress(ctx, document, knowledgeBase, nil)
	if result == nil {
		return nil, 0, err
	}
	return vectors, result.ChunkCount, err
}

func (s *index) runDocumentIndexWithProgress(ctx context.Context, document models.KnowledgeDocument, knowledgeBase models.KnowledgeBase, progress func(DocumentIndexStage)) (*DocumentIndexResult, []vectordb.Vector, error) {
	result := &DocumentIndexResult{}
	if progress != nil {
		progress(DocumentIndexStageChunking)
	}
	startedAt := time.Now()
	chunks, err := s.buildDocumentChunks(ctx, document, knowledgeBase)
	result.ChunkMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		return result, nil, err
	}
	result.ChunkCount = len(chunks)

	collectionName := s.getCollectionName()
	provider := vectordb.GetProvider()
	if provider == nil {
		return result, nil, fmt.Errorf("vectordb provider not initialized")
	}
	if _, err := ai.Embedding.GetModel(ctx); err != nil {
		return result, nil, fmt.Errorf("failed to get embedding model: %w", err)
	}

	if progress != nil {
		progress(DocumentIndexStageEmbedding)
	}
	startedAt = time.Now()
	vectors, dimension, err := s.prepareDocumentVectors(ctx, knowledgeBase, document, chunks)
	result.EmbeddingMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		return result, nil, err
	}
	result.VectorCount = len(vectors)
	if progress != nil {
		progress(DocumentIndexStageIndexing)
	}
	startedAt = time.Now()
	if err := s.ensureCollection(ctx, provider, collectionName, dimension); err != nil {
		result.IndexMS = time.Since(startedAt).Milliseconds()
		return result, nil, err
	}
	if err := provider.DeleteVectorsByFilter(ctx, collectionName, &vectordb.SearchFilter{
		KnowledgeBaseIDs: []int64{knowledgeBase.ID},
		DocumentIDs:      []int64{document.ID},
	}); err != nil {
		result.IndexMS = time.Since(startedAt).Milliseconds()
		return result, nil, fmt.Errorf("failed to clear existing document vectors: %w", err)
	}
	if err := provider.UpsertVectors(ctx, collectionName, vectors); err != nil {
		result.IndexMS = time.Since(startedAt).Milliseconds()
		return result, nil, fmt.Errorf("failed to upsert vectors: %w", err)
	}
	if err := repositories.KnowledgeChunkRepository.DeleteByDocumentID(sqls.DB(), document.ID); err != nil {
		result.IndexMS = time.Since(startedAt).Milliseconds()
		return result, nil, fmt.Errorf("failed to clear legacy document chunks: %w", err)
	}
	result.IndexMS = time.Since(startedAt).Milliseconds()
	return result, vectors, nil
}

func (s *index) runFAQIndex(ctx context.Context, faq models.KnowledgeFAQ, knowledgeBase models.KnowledgeBase) error {
	content := buildFAQChunkContent(faq)
	if content == "" {
		return fmt.Errorf("faq content is empty")
	}

	provider := vectordb.GetProvider()
	if provider == nil {
		return fmt.Errorf("vectordb provider not initialized")
	}
	if _, err := ai.Embedding.GetModel(ctx); err != nil {
		return fmt.Errorf("failed to get embedding model: %w", err)
	}
	vector, dimension, err := s.prepareFAQVector(ctx, knowledgeBase, faq, content)
	if err != nil {
		return err
	}

	collectionName := s.getCollectionName()
	if err := s.ensureCollection(ctx, provider, collectionName, dimension); err != nil {
		return err
	}
	if err := provider.DeleteVectorsByFilter(ctx, collectionName, &vectordb.SearchFilter{
		KnowledgeBaseIDs: []int64{knowledgeBase.ID},
		FAQIDs:           []int64{faq.ID},
	}); err != nil {
		return fmt.Errorf("failed to clear existing faq vectors: %w", err)
	}

	if err := provider.UpsertVectors(ctx, collectionName, []vectordb.Vector{vector}); err != nil {
		return fmt.Errorf("failed to upsert vectors: %w", err)
	}
	if err := repositories.KnowledgeChunkRepository.DeleteByFaqID(sqls.DB(), faq.ID); err != nil {
		return fmt.Errorf("failed to clear legacy faq chunks: %w", err)
	}
	return nil
}

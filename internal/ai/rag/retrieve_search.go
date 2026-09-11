package rag

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/ai/rag/vectordb"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func (s *retrieve) searchKnowledgeBaseVectors(ctx context.Context, req RetrieveRequest, knowledgeBases []models.KnowledgeBase) ([]vectordb.SearchResult, *RetrieveTrace, error) {
	trace := &RetrieveTrace{}

	embeddingStartedAt := time.Now()
	embeddingResult, err := ai.Embedding.GenerateEmbedding(ctx, req.Query)
	trace.EmbeddingMs = time.Since(embeddingStartedAt).Milliseconds()
	if err != nil {
		return nil, trace, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	collectionName := knowledgeCollectionName
	provider := vectordb.GetProvider()
	if provider == nil {
		return nil, trace, fmt.Errorf("vectordb provider not initialized")
	}

	searchResults := make([]vectordb.SearchResult, 0)
	vectorSearchStartedAt := time.Now()
	for _, knowledgeBase := range knowledgeBases {
		topK, scoreThreshold := resolveKnowledgeBaseSearchOptions(req, &knowledgeBase)
		kbResults, searchErr := provider.Search(ctx, &vectordb.SearchRequest{
			CollectionName: collectionName,
			Query:          req.Query,
			Vector:         embeddingResult.Vector,
			TopK:           topK,
			ScoreThreshold: scoreThreshold,
			Filter: &vectordb.SearchFilter{
				KnowledgeBaseIDs: []int64{knowledgeBase.ID},
			},
		})
		if searchErr != nil {
			slog.Error("Failed to search vectors",
				"knowledge_base_id", knowledgeBase.ID,
				"error", searchErr)
			trace.VectorSearchMs = time.Since(vectorSearchStartedAt).Milliseconds()
			return nil, trace, fmt.Errorf("failed to search vectors: %w", searchErr)
		}
		if len(kbResults) == 0 && scoreThreshold > 0 {
			s.logEmptySearchDiagnostics(ctx, provider, collectionName, embeddingResult.Vector, topK, scoreThreshold, []int64{knowledgeBase.ID}, req)
		}
		searchResults = append(searchResults, kbResults...)
	}
	trace.VectorSearchMs = time.Since(vectorSearchStartedAt).Milliseconds()

	if len(searchResults) > 0 {
		sort.SliceStable(searchResults, func(i, j int) bool {
			if searchResults[i].Score == searchResults[j].Score {
				return searchResults[i].ID < searchResults[j].ID
			}
			return searchResults[i].Score > searchResults[j].Score
		})
	}

	return searchResults, trace, nil
}

func (s *retrieve) hydrateRetrieveResults(searchResults []vectordb.SearchResult) ([]RetrieveResult, int64) {
	if len(searchResults) == 0 {
		return nil, 0
	}

	hydrateStartedAt := time.Now()
	results := make([]RetrieveResult, 0, len(searchResults))
	documentIDs := make([]int64, 0)
	faqIDs := make([]int64, 0)
	documentSeen := make(map[int64]struct{})
	faqSeen := make(map[int64]struct{})
	for _, sr := range searchResults {
		payload := sr.Payload
		if payload.DocumentID > 0 {
			if _, ok := documentSeen[payload.DocumentID]; !ok {
				documentSeen[payload.DocumentID] = struct{}{}
				documentIDs = append(documentIDs, payload.DocumentID)
			}
		}
		if payload.FaqID > 0 {
			if _, ok := faqSeen[payload.FaqID]; !ok {
				faqSeen[payload.FaqID] = struct{}{}
				faqIDs = append(faqIDs, payload.FaqID)
			}
		}
	}
	documents := repositories.KnowledgeDocumentRepository.FindByIDs(sqls.DB(), documentIDs)
	documentByID := make(map[int64]*models.KnowledgeDocument, len(documents))
	for i := range documents {
		document := &documents[i]
		documentByID[document.ID] = document
	}
	faqs := repositories.KnowledgeFAQRepository.FindByIDs(sqls.DB(), faqIDs)
	faqByID := make(map[int64]*models.KnowledgeFAQ, len(faqs))
	for i := range faqs {
		faq := &faqs[i]
		faqByID[faq.ID] = faq
	}
	for _, sr := range searchResults {
		payload := sr.Payload
		if strings.TrimSpace(payload.Content) == "" {
			continue
		}

		documentTitle := payload.DocumentTitle
		faqQuestion := payload.FaqQuestion
		if payload.DocumentID > 0 {
			document := documentByID[payload.DocumentID]
			if document == nil || document.Status != enums.StatusOk {
				continue
			}
			documentTitle = document.Title
		}
		if payload.FaqID > 0 {
			faq := faqByID[payload.FaqID]
			if faq == nil || faq.Status != enums.StatusOk {
				continue
			}
			faqQuestion = faq.Question
		}

		contextContent := strings.TrimSpace(payload.ContextContent)
		if contextContent == "" {
			contextContent = payload.Content
		}
		results = append(results, RetrieveResult{
			KnowledgeBaseID: payload.KnowledgeBaseID,
			DocumentID:      payload.DocumentID,
			DocumentTitle:   documentTitle,
			FaqID:           payload.FaqID,
			FaqQuestion:     faqQuestion,
			ChunkNo:         payload.ChunkNo,
			Title:           payload.Title,
			SectionPath:     payload.SectionPath,
			Content:         contextContent,
			MatchedContent:  payload.Content,
			Score:           sr.Score,
			ChunkType:       extractChunkType(payload),
		})
	}

	return results, time.Since(hydrateStartedAt).Milliseconds()
}

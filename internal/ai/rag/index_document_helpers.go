package rag

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"agent-desk/internal/ai"
	ragchunk "agent-desk/internal/ai/rag/chunk"
	"agent-desk/internal/ai/rag/vectordb"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

type DocumentChunkPreview struct {
	Provider      string
	TargetTokens  int
	MaxTokens     int
	OverlapTokens int
	ParentTokens  int
	ChildTokens   int
	Chunks        []ragchunk.ChunkResult
}

type DocumentChunkOverrides struct {
	Enabled       bool
	Provider      string
	TargetTokens  int
	MaxTokens     int
	OverlapTokens int
	ParentTokens  int
	ChildTokens   int
}

func (s *index) buildDocumentChunkRequest(document models.KnowledgeDocument, knowledgeBase models.KnowledgeBase) *ragchunk.ChunkRequest {
	options := ragchunk.ChunkOptions{
		Provider:       firstNonEmptyString(knowledgeBase.ChunkProvider, s.chunkConfig.Provider),
		TargetTokens:   firstPositiveInt(knowledgeBase.ChunkTargetTokens, s.chunkConfig.TargetTokens),
		MaxTokens:      firstPositiveInt(knowledgeBase.ChunkMaxTokens, s.chunkConfig.MaxTokens),
		OverlapTokens:  knowledgeBase.ChunkOverlapTokens,
		ParentTokens:   firstPositiveInt(knowledgeBase.ParentChunkTokens, 900),
		ChildTokens:    firstPositiveInt(knowledgeBase.ChildChunkTokens, 200),
		EnableFallback: s.chunkConfig.EnableFallback,
	}
	if document.ChunkConfigOverride {
		options.Provider = firstNonEmptyString(document.ChunkProvider, options.Provider)
		options.TargetTokens = firstPositiveInt(document.ChunkTargetTokens, options.TargetTokens)
		options.MaxTokens = firstPositiveInt(document.ChunkMaxTokens, options.MaxTokens)
		options.OverlapTokens = document.ChunkOverlapTokens
		options.ParentTokens = firstPositiveInt(document.ParentChunkTokens, options.ParentTokens)
		options.ChildTokens = firstPositiveInt(document.ChildChunkTokens, options.ChildTokens)
	}
	options = ragchunk.NormalizeOptions(options)
	return &ragchunk.ChunkRequest{
		KnowledgeBaseID: document.KnowledgeBaseID,
		DocumentID:      document.ID,
		DocumentTitle:   document.Title,
		ContentType:     document.ContentType,
		Content:         document.Content,
		PlainText:       ExtractPlainText(document.Content, document.ContentType),
		Options:         options,
	}
}

func (s *index) buildDocumentChunks(ctx context.Context, document models.KnowledgeDocument, knowledgeBase models.KnowledgeBase) ([]ragchunk.ChunkResult, error) {
	chunks, err := s.registry.Chunk(ctx, s.buildDocumentChunkRequest(document, knowledgeBase))
	if err != nil {
		return nil, fmt.Errorf("failed to chunk document: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from document")
	}
	return chunks, nil
}

func (s *index) PreviewDocumentChunks(
	ctx context.Context,
	knowledgeBaseID int64,
	title string,
	contentType enums.KnowledgeDocumentContentType,
	content string,
	overrides DocumentChunkOverrides,
) (*DocumentChunkPreview, error) {
	if contentType != enums.KnowledgeDocumentContentTypeHTML && contentType != enums.KnowledgeDocumentContentTypeMarkdown {
		return nil, fmt.Errorf("unsupported document content type: %s", contentType)
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("document content is empty")
	}

	knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if knowledgeBase == nil {
		return nil, fmt.Errorf("knowledge base not found: %d", knowledgeBaseID)
	}
	if knowledgeBase.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return nil, fmt.Errorf("knowledge base %d is not a document knowledge base", knowledgeBaseID)
	}

	document := models.KnowledgeDocument{
		KnowledgeBaseID:     knowledgeBaseID,
		Title:               strings.TrimSpace(title),
		ContentType:         contentType,
		Content:             content,
		ChunkConfigOverride: overrides.Enabled,
		ChunkProvider:       overrides.Provider,
		ChunkTargetTokens:   overrides.TargetTokens,
		ChunkMaxTokens:      overrides.MaxTokens,
		ChunkOverlapTokens:  overrides.OverlapTokens,
		ParentChunkTokens:   overrides.ParentTokens,
		ChildChunkTokens:    overrides.ChildTokens,
	}
	chunkRequest := s.buildDocumentChunkRequest(document, *knowledgeBase)
	chunks, err := s.registry.Chunk(ctx, chunkRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to preview document chunks: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from document")
	}

	provider := chunkRequest.Options.Provider
	if actualProvider := documentChunkProvider(chunks[0].Metadata); actualProvider != "" {
		provider = actualProvider
	}
	return &DocumentChunkPreview{
		Provider:      provider,
		TargetTokens:  chunkRequest.Options.TargetTokens,
		MaxTokens:     chunkRequest.Options.MaxTokens,
		OverlapTokens: chunkRequest.Options.OverlapTokens,
		ParentTokens:  chunkRequest.Options.ParentTokens,
		ChildTokens:   chunkRequest.Options.ChildTokens,
		Chunks:        chunks,
	}, nil
}

func (s *index) prepareDocumentVectors(ctx context.Context, knowledgeBase models.KnowledgeBase, document models.KnowledgeDocument, chunks []ragchunk.ChunkResult) ([]vectordb.Vector, int, error) {
	vectors := make([]vectordb.Vector, 0, len(chunks))
	dimension := 0
	directoryPath := loadKnowledgeDirectoryPath(document.DirectoryID)
	contents := make([]string, len(chunks))
	for index, chunk := range chunks {
		contents[index] = chunk.Content
	}
	embeddings, err := ai.Embedding.GenerateBatchEmbeddings(ctx, contents)
	if err != nil {
		slog.Error("Failed to generate document embeddings", "document_id", document.ID, "chunk_count", len(chunks), "error", err)
		return nil, 0, fmt.Errorf("failed to generate document embeddings: %w", err)
	}

	for i, chunk := range chunks {
		embeddingResult := embeddings[i]
		if dimension == 0 {
			dimension = embeddingResult.Dimension
		}

		chunkID := buildKnowledgeChunkVectorID(knowledgeBase.ID, document.ID, chunk.ChunkNo)
		providerName := documentChunkProvider(chunk.Metadata)
		sectionPath := joinKnowledgeSectionPath(directoryPath, chunk.SectionPath)
		vectors = append(vectors, vectordb.Vector{
			ID:     chunkID,
			Vector: embeddingResult.Vector,
			Payload: vectordb.ChunkPayload{
				KnowledgeBaseID: knowledgeBase.ID,
				DocumentID:      document.ID,
				DocumentTitle:   document.Title,
				ChunkNo:         chunk.ChunkNo,
				ChunkType:       string(chunk.ChunkType),
				SectionPath:     sectionPath,
				Content:         chunk.Content,
				ContextContent:  chunk.ContextContent,
				Title:           chunk.Title,
				Provider:        providerName,
			},
		})
	}

	if len(vectors) == 0 {
		return nil, 0, fmt.Errorf("no vectors generated")
	}
	return vectors, dimension, nil
}

func documentChunkProvider(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	switch value := metadata["provider"].(type) {
	case string:
		return value
	case enums.KnowledgeChunkProvider:
		return string(value)
	default:
		return ""
	}
}

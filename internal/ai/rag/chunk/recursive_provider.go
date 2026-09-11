package chunk

import (
	"agent-desk/internal/pkg/enums"
	"context"
)

type recursiveProvider struct {
	name string
}

func NewRecursiveProvider() Provider {
	return &recursiveProvider{name: string(enums.KnowledgeChunkProviderRecursive)}
}

func newLegacyFixedProvider() Provider {
	return &recursiveProvider{name: string(enums.KnowledgeChunkProviderFixed)}
}

func (p *recursiveProvider) Name() string {
	return p.name
}

func (p *recursiveProvider) Supports(enums.KnowledgeDocumentContentType) bool {
	return true
}

func (p *recursiveProvider) Chunk(_ context.Context, req *ChunkRequest) ([]ChunkResult, error) {
	text := req.PlainText
	if text == "" {
		text = req.Content
	}
	parts := splitPlainText(text, req.Options)
	results := make([]ChunkResult, 0, len(parts))
	for i, part := range parts {
		results = append(results, ChunkResult{
			ChunkNo:     i,
			Title:       req.DocumentTitle,
			Content:     part,
			ChunkType:   enums.KnowledgeChunkTypeText,
			SectionPath: req.DocumentTitle,
			CharCount:   len([]rune(part)),
			TokenCount:  estimateTokenCount(part),
			Metadata: map[string]any{
				"provider": p.name,
			},
		})
	}
	return results, nil
}

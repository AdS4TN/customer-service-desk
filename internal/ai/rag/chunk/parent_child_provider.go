package chunk

import (
	"agent-desk/internal/pkg/enums"
	"context"
	"strings"
)

type parentChildProvider struct{}

func NewParentChildProvider() Provider {
	return &parentChildProvider{}
}

func (p *parentChildProvider) Name() string {
	return string(enums.KnowledgeChunkProviderParentChild)
}

func (p *parentChildProvider) Supports(enums.KnowledgeDocumentContentType) bool {
	return true
}

func (p *parentChildProvider) Chunk(_ context.Context, req *ChunkRequest) ([]ChunkResult, error) {
	opts := normalizeOptions(req.Options)
	parents := buildParentSegments(req, opts)
	childOpts := opts
	childOpts.TargetTokens = opts.ChildTokens
	childOpts.MaxTokens = maxInt(opts.ChildTokens, minInt(opts.MaxTokens, opts.ParentTokens))
	if childOpts.OverlapTokens >= childOpts.MaxTokens {
		childOpts.OverlapTokens = childOpts.MaxTokens / 4
	}

	results := make([]ChunkResult, 0)
	for _, parent := range parents {
		children := splitPlainText(parent.Text, childOpts)
		for _, child := range children {
			results = append(results, ChunkResult{
				ChunkNo:        len(results),
				Title:          parent.Title,
				Content:        child,
				ContextContent: parent.Text,
				ChunkType:      mapBlockType(parent.Type),
				SectionPath:    parent.SectionPath,
				CharCount:      len([]rune(child)),
				TokenCount:     estimateTokenCount(child),
				Metadata: map[string]any{
					"provider":    enums.KnowledgeChunkProviderParentChild,
					"parentIndex": parent.Index,
				},
			})
		}
	}
	return results, nil
}

type parentSegment struct {
	Index       int
	Type        string
	Title       string
	SectionPath string
	Text        string
}

func buildParentSegments(req *ChunkRequest, opts ChunkOptions) []parentSegment {
	blocks := structuredBlocksForRequest(req)

	parentOpts := opts
	parentOpts.TargetTokens = opts.ParentTokens
	parentOpts.MaxTokens = opts.ParentTokens
	parentOpts.OverlapTokens = 0
	groups := groupStructuredBlocks(blocks)
	parents := make([]parentSegment, 0)
	for _, group := range groups {
		for _, part := range splitPlainText(group.Text, parentOpts) {
			if strings.TrimSpace(part) == "" {
				continue
			}
			parents = append(parents, parentSegment{
				Index:       len(parents),
				Type:        group.Type,
				Title:       group.Title,
				SectionPath: group.SectionPath,
				Text:        part,
			})
		}
	}
	return parents
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

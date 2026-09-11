package chunk

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"agent-desk/internal/pkg/enums"
)

func TestSplitPlainTextUsesTargetTokensAndAllowsZeroOverlap(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 24; i++ {
		source.WriteString(fmt.Sprintf("第%d条唯一内容。", i))
	}
	text := source.String()
	chunks := splitPlainText(text, ChunkOptions{
		TargetTokens:  20,
		MaxTokens:     30,
		OverlapTokens: 0,
	})
	if len(chunks) < 4 {
		t.Fatalf("len(chunks) = %d, want at least 4", len(chunks))
	}
	for i, item := range chunks {
		if got := estimateTokenCount(item); got > 30 {
			t.Fatalf("chunk %d has %d tokens, want <= 30", i, got)
		}
	}
	if strings.HasSuffix(chunks[0], chunks[1]) || strings.HasPrefix(chunks[1], chunks[0]) {
		t.Fatalf("zero overlap unexpectedly duplicated a complete chunk")
	}
}

func TestParentChildProviderReturnsSearchChildAndParentContext(t *testing.T) {
	provider := NewParentChildProvider()
	chunks, err := provider.Chunk(context.Background(), &ChunkRequest{
		DocumentTitle: "产品手册",
		ContentType:   enums.KnowledgeDocumentContentTypeMarkdown,
		Content:       "# 质保\n\n" + strings.Repeat("本产品提供十五年质保。", 20),
		Options: ChunkOptions{
			TargetTokens:  80,
			MaxTokens:     100,
			OverlapTokens: 0,
			ParentTokens:  160,
			ChildTokens:   40,
		},
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want multiple child chunks", len(chunks))
	}
	for _, item := range chunks {
		if item.ContextContent == "" {
			t.Fatal("parent context is empty")
		}
		if estimateTokenCount(item.ContextContent) < estimateTokenCount(item.Content) {
			t.Fatal("parent context is shorter than matched child")
		}
		if documentProviderName(item) != string(enums.KnowledgeChunkProviderParentChild) {
			t.Fatalf("provider = %q, want parent_child", documentProviderName(item))
		}
	}
}

func TestDefaultRegistryDoesNotResolveSemanticProvider(t *testing.T) {
	registry := NewDefaultRegistry()
	provider := registry.Resolve("semantic", enums.KnowledgeDocumentContentTypeMarkdown)
	if provider == nil || provider.Name() != string(enums.KnowledgeChunkProviderStructured) {
		t.Fatalf("semantic resolves to %v, want structured compatibility fallback", provider)
	}
}

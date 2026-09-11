package chunk

import (
	"context"
	"testing"

	"agent-desk/internal/pkg/enums"
)

func TestStructuredProviderPreservesMarkdownSections(t *testing.T) {
	provider := NewStructuredProvider()
	chunks, err := provider.Chunk(context.Background(), &ChunkRequest{
		DocumentTitle: "产品手册",
		ContentType:   enums.KnowledgeDocumentContentTypeMarkdown,
		Content:       "# 安装\n\n按住按钮 8 秒。\n\n## 售后\n\n提供 18 个月质保。",
		PlainText:     "安装 按住按钮 8 秒。售后 提供 18 个月质保。",
		Options: ChunkOptions{
			Provider:       string(enums.KnowledgeChunkProviderStructured),
			TargetTokens:   300,
			MaxTokens:      400,
			OverlapTokens:  40,
			EnableFallback: true,
		},
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}
	if chunks[0].Title != "安装" || chunks[0].SectionPath != "安装" {
		t.Fatalf("first chunk section = %q / %q", chunks[0].Title, chunks[0].SectionPath)
	}
	if chunks[1].Title != "售后" || chunks[1].SectionPath != "安装 > 售后" {
		t.Fatalf("second chunk section = %q / %q", chunks[1].Title, chunks[1].SectionPath)
	}
	if providerName := documentProviderName(chunks[0]); providerName != string(enums.KnowledgeChunkProviderStructured) {
		t.Fatalf("provider = %q, want structured", providerName)
	}
}

func documentProviderName(chunk ChunkResult) string {
	switch value := chunk.Metadata["provider"].(type) {
	case enums.KnowledgeChunkProvider:
		return string(value)
	case string:
		return value
	default:
		return ""
	}
}

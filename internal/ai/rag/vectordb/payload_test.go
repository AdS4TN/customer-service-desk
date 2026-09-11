package vectordb

import "testing"

func TestChunkPayloadFromMapPreservesRetrievalContext(t *testing.T) {
	payload := ChunkPayloadFromMap(map[string]any{
		"knowledge_base_id": int64(1),
		"document_id":       int64(2),
		"document_title":    "产品手册",
		"chunk_no":          3,
		"chunk_type":        "child",
		"section_path":      "产品/参数",
		"title":             "技术参数",
		"content":           "命中的子块",
		"context_content":   "提供给模型的完整父块",
		"provider":          "parent_child",
	})

	if payload.KnowledgeBaseID != 1 || payload.DocumentID != 2 || payload.ChunkNo != 3 {
		t.Fatalf("unexpected payload identity: %#v", payload)
	}
	if payload.Content != "命中的子块" {
		t.Fatalf("Content = %q", payload.Content)
	}
	if payload.ContextContent != "提供给模型的完整父块" {
		t.Fatalf("ContextContent = %q", payload.ContextContent)
	}
}

func TestElasticsearchFilterIncludesEverySupportedScope(t *testing.T) {
	filters := elasticsearchFilter(&SearchFilter{
		KnowledgeBaseIDs: []int64{1},
		DocumentIDs:      []int64{2},
		FAQIDs:           []int64{3},
	})
	if len(filters) != 3 {
		t.Fatalf("len(filters) = %d, want 3", len(filters))
	}

	query := elasticsearchFilterQuery(nil)
	if _, ok := query["match_all"]; !ok {
		t.Fatalf("empty filter query = %#v, want match_all", query)
	}
}

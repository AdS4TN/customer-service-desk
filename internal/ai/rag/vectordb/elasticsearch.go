package vectordb

import (
	"agent-desk/internal/pkg/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ElasticsearchProvider struct {
	base, prefix, user, pass, apiKey string
	numberOfReplicas                 int
	client                           *http.Client
}

func NewElasticsearchProvider(cfg *config.ElasticsearchVectorDBConfig) (Provider, error) {
	if cfg == nil || strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("elasticsearch url is required")
	}
	p := &ElasticsearchProvider{
		base:             strings.TrimRight(cfg.URL, "/"),
		prefix:           cfg.IndexPrefix,
		user:             cfg.Username,
		pass:             cfg.Password,
		apiKey:           cfg.APIKey,
		numberOfReplicas: cfg.NumberOfReplicas,
		client:           &http.Client{},
	}
	if p.prefix == "" {
		p.prefix = "agentdesk_"
	}
	return p, nil
}
func (p *ElasticsearchProvider) index(name string) string {
	return p.prefix + strings.ToLower(strings.ReplaceAll(name, " ", "_"))
}
func (p *ElasticsearchProvider) do(ctx context.Context, method, path string, body any, out any) error {
	var r *http.Request
	var err error
	if body != nil {
		b, _ := json.Marshal(body)
		r, err = http.NewRequestWithContext(ctx, method, p.base+path, bytes.NewReader(b))
	} else {
		r, err = http.NewRequestWithContext(ctx, method, p.base+path, nil)
	}
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		r.Header.Set("Authorization", "ApiKey "+p.apiKey)
	} else if p.user != "" {
		r.SetBasicAuth(p.user, p.pass)
	}
	resp, err := p.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("elasticsearch %s: %s", resp.Status, path)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
func (p *ElasticsearchProvider) CreateCollection(ctx context.Context, name string, dim int) error {
	mapping := map[string]any{
		"settings": map[string]any{
			"number_of_replicas": p.numberOfReplicas,
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"vector":  map[string]any{"type": "dense_vector", "dims": dim, "index": true, "similarity": "cosine"},
				"content": map[string]any{"type": "text"},
				"payload": map[string]any{"type": "object", "enabled": true},
			},
		},
	}
	return p.do(ctx, http.MethodPut, "/"+p.index(name), mapping, nil)
}
func (p *ElasticsearchProvider) DeleteCollection(ctx context.Context, name string) error {
	return p.do(ctx, http.MethodDelete, "/"+p.index(name), nil, nil)
}
func (p *ElasticsearchProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	if err := p.do(ctx, http.MethodHead, "/"+p.index(name), nil, nil); err != nil {
		return nil, err
	}
	return &CollectionInfo{Name: name, Status: "available"}, nil
}
func (p *ElasticsearchProvider) ListCollections(ctx context.Context) ([]string, error) {
	var v []struct {
		Index string `json:"index"`
	}
	if err := p.do(ctx, http.MethodGet, "/_cat/indices/"+p.prefix+"*?format=json", nil, &v); err != nil {
		return nil, err
	}
	r := make([]string, 0, len(v))
	for _, x := range v {
		r = append(r, strings.TrimPrefix(x.Index, p.prefix))
	}
	return r, nil
}
func (p *ElasticsearchProvider) UpsertVectors(ctx context.Context, name string, vs []Vector) error {
	if len(vs) == 0 {
		return nil
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, v := range vs {
		if err := encoder.Encode(map[string]any{"index": map[string]any{"_id": v.ID}}); err != nil {
			return err
		}
		if err := encoder.Encode(map[string]any{"vector": v.Vector, "content": v.Payload.Content, "payload": v.Payload}); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/"+p.index(name)+"/_bulk?refresh=true", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+p.apiKey)
	} else if p.user != "" {
		req.SetBasicAuth(p.user, p.pass)
	}
	response, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("elasticsearch %s: bulk index", response.Status)
	}
	var result struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	if !result.Errors {
		return nil
	}
	for _, item := range result.Items {
		for _, operation := range item {
			if operation.Status >= 300 || len(operation.Error) > 0 {
				return fmt.Errorf("elasticsearch bulk item failed with status %d: %s", operation.Status, strings.TrimSpace(string(operation.Error)))
			}
		}
	}
	return fmt.Errorf("elasticsearch bulk request reported errors")
}
func (p *ElasticsearchProvider) DeleteVectors(ctx context.Context, name string, ids []string) error {
	for _, id := range ids {
		if err := p.do(ctx, http.MethodDelete, "/"+p.index(name)+"/_doc/"+id, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func elasticsearchFilter(filter *SearchFilter) []any {
	if filter == nil {
		return nil
	}
	filters := make([]any, 0, 3)
	if len(filter.KnowledgeBaseIDs) > 0 {
		filters = append(filters, map[string]any{"terms": map[string]any{"payload.knowledge_base_id": filter.KnowledgeBaseIDs}})
	}
	if len(filter.DocumentIDs) > 0 {
		filters = append(filters, map[string]any{"terms": map[string]any{"payload.document_id": filter.DocumentIDs}})
	}
	if len(filter.FAQIDs) > 0 {
		filters = append(filters, map[string]any{"terms": map[string]any{"payload.faq_id": filter.FAQIDs}})
	}
	return filters
}

func elasticsearchFilterQuery(filter *SearchFilter) map[string]any {
	filters := elasticsearchFilter(filter)
	if len(filters) == 0 {
		return map[string]any{"match_all": map[string]any{}}
	}
	return map[string]any{"bool": map[string]any{"filter": filters}}
}

func (p *ElasticsearchProvider) DeleteVectorsByFilter(ctx context.Context, name string, filter *SearchFilter) error {
	if len(elasticsearchFilter(filter)) == 0 {
		return fmt.Errorf("elasticsearch vector deletion requires a non-empty filter")
	}
	body := map[string]any{"query": elasticsearchFilterQuery(filter)}
	return p.do(ctx, http.MethodPost, "/"+p.index(name)+"/_delete_by_query?conflicts=proceed&refresh=true", body, nil)
}

func (p *ElasticsearchProvider) CountVectors(ctx context.Context, name string, filter *SearchFilter) (int64, error) {
	body := map[string]any{"query": elasticsearchFilterQuery(filter)}
	var out struct {
		Count int64 `json:"count"`
	}
	if err := p.do(ctx, http.MethodPost, "/"+p.index(name)+"/_count", body, &out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

func (p *ElasticsearchProvider) Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
	filter := elasticsearchFilter(req.Filter)
	// ES combines the lexical query and kNN candidates in one request. This
	// keeps retrieval latency to a single round trip while covering both exact
	// terms (BM25) and semantic matches (BGE-M3).
	query := map[string]any{"match": map[string]any{"content": req.Query}}
	if strings.TrimSpace(req.Query) == "" {
		query = map[string]any{"match_all": map[string]any{}}
	}
	if req.Filter != nil && len(filter) > 0 {
		query = map[string]any{"bool": map[string]any{"must": query, "filter": filter}}
	}
	body := map[string]any{
		"query": query,
		"knn":   map[string]any{"field": "vector", "query_vector": req.Vector, "k": req.TopK, "num_candidates": req.TopK * 5, "filter": filter},
		"size":  req.TopK,
	}
	var out struct {
		Hits struct {
			Hits []struct {
				ID     string  `json:"_id"`
				Score  float32 `json:"_score"`
				Source struct {
					Payload ChunkPayload `json:"payload"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := p.do(ctx, http.MethodPost, "/"+p.index(req.CollectionName)+"/_search", body, &out); err != nil {
		return nil, err
	}
	r := make([]SearchResult, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		if h.Score >= req.ScoreThreshold {
			r = append(r, SearchResult{ID: h.ID, Score: h.Score, Payload: h.Source.Payload})
		}
	}
	return r, nil
}
func (p *ElasticsearchProvider) Close() error { return nil }

package services

import (
	"agent-desk/internal/ai"
	"agent-desk/internal/ai/rag"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mlogclub/simple/sqls"
)

const (
	ingestionStatusQueued     = "queued"
	ingestionStatusProcessing = "processing"
	ingestionStatusCompleted  = "completed"
	ingestionStatusFailed     = "failed"
)

type KnowledgeIngestionSnapshot struct {
	Job      *models.KnowledgeIngestionJob
	Document *models.KnowledgeDocument
	Asset    *models.Asset
}

type parsedKnowledgeDocument struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
	Parser      string `json:"parser"`
}

type websiteIngestionSource struct {
	URL      string `json:"url"`
	MaxPages int    `json:"maxPages"`
	MaxDepth int    `json:"maxDepth"`
}

type knowledgeIngestionService struct {
	startOnce sync.Once
	client    *http.Client
}

var KnowledgeIngestionService = &knowledgeIngestionService{
	client: &http.Client{Timeout: 30 * time.Minute},
}

func (s *knowledgeIngestionService) QueueUpload(file *multipart.FileHeader, req request.UploadKnowledgeDocumentRequest, operator *dto.AuthPrincipal) (*KnowledgeIngestionSnapshot, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	base := KnowledgeBaseService.Get(req.KnowledgeBaseID)
	if base == nil {
		return nil, errorsx.InvalidParamI18n("error.e0283")
	}
	if base.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return nil, errorsx.InvalidParamI18n("error.e0026")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return nil, err
	}
	asset, err := AssetService.UploadFileWithoutSizeLimit(file, "knowledge", operator)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(file.Filename), filepath.Ext(file.Filename))
	}
	document, err := KnowledgeDocumentService.buildKnowledgeDocumentModel(request.CreateKnowledgeDocumentRequest{
		KnowledgeBaseID: req.KnowledgeBaseID, DirectoryID: req.DirectoryID, Title: title,
		ContentType:         enums.KnowledgeDocumentContentTypeMarkdown,
		ChunkConfigOverride: req.ChunkConfigOverride, ChunkProvider: req.ChunkProvider,
		ChunkTargetTokens: req.ChunkTargetTokens, ChunkMaxTokens: req.ChunkMaxTokens,
		ChunkOverlapTokens: req.ChunkOverlapTokens, ParentChunkTokens: req.ParentChunkTokens,
		ChildChunkTokens: req.ChildChunkTokens,
	})
	if err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	document.SourceAssetID = asset.ID
	document.Status = enums.StatusOk
	document.IndexStatus = enums.KnowledgeDocumentIndexStatusPending
	document.AuditFields = utils.BuildAuditFields(operator)
	job := &models.KnowledgeIngestionJob{
		KnowledgeBaseID: req.KnowledgeBaseID, AssetID: asset.ID,
		Status: ingestionStatusQueued, Stage: "queued", Progress: 0,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.KnowledgeDocumentRepository.Create(ctx.Tx, document); err != nil {
			return err
		}
		job.DocumentID = document.ID
		return repositories.KnowledgeIngestionJobRepository.Create(ctx.Tx, job)
	}); err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	return &KnowledgeIngestionSnapshot{Job: job, Document: document, Asset: asset}, nil
}

func (s *knowledgeIngestionService) QueueWebsite(req request.CrawlKnowledgeWebsiteRequest, operator *dto.AuthPrincipal) (*KnowledgeIngestionSnapshot, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	base := KnowledgeBaseService.Get(req.KnowledgeBaseID)
	if base == nil || base.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return nil, errorsx.InvalidParamI18n("error.knowledgeWebsite.invalidBase")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return nil, err
	}
	normalizedURL, host, err := validateWebsiteURL(req.URL)
	if err != nil {
		return nil, errorsx.InvalidParamI18n("error.knowledgeWebsite.invalidURL")
	}
	maxPages := req.MaxPages
	if maxPages <= 0 {
		maxPages = 30
	}
	if maxPages > 200 {
		maxPages = 200
	}
	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if maxDepth > 5 {
		maxDepth = 5
	}
	payload, _ := json.Marshal(websiteIngestionSource{URL: normalizedURL, MaxPages: maxPages, MaxDepth: maxDepth})
	asset, err := AssetService.UploadBytes(payload, "knowledge", host+".website.json", operator)
	if err != nil {
		return nil, err
	}
	document, err := KnowledgeDocumentService.buildKnowledgeDocumentModel(request.CreateKnowledgeDocumentRequest{
		KnowledgeBaseID: req.KnowledgeBaseID, DirectoryID: req.DirectoryID, Title: host,
		ContentType:         enums.KnowledgeDocumentContentTypeMarkdown,
		ChunkConfigOverride: req.ChunkConfigOverride, ChunkProvider: req.ChunkProvider,
		ChunkTargetTokens: req.ChunkTargetTokens, ChunkMaxTokens: req.ChunkMaxTokens,
		ChunkOverlapTokens: req.ChunkOverlapTokens, ParentChunkTokens: req.ParentChunkTokens,
		ChildChunkTokens: req.ChildChunkTokens,
	})
	if err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	document.SourceAssetID = asset.ID
	document.Status = enums.StatusOk
	document.IndexStatus = enums.KnowledgeDocumentIndexStatusPending
	document.AuditFields = utils.BuildAuditFields(operator)
	job := &models.KnowledgeIngestionJob{KnowledgeBaseID: req.KnowledgeBaseID, AssetID: asset.ID, Status: ingestionStatusQueued, Stage: "queued", AuditFields: utils.BuildAuditFields(operator)}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.KnowledgeDocumentRepository.Create(ctx.Tx, document); err != nil {
			return err
		}
		job.DocumentID = document.ID
		return repositories.KnowledgeIngestionJobRepository.Create(ctx.Tx, job)
	}); err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return nil, err
	}
	return &KnowledgeIngestionSnapshot{Job: job, Document: document, Asset: asset}, nil
}

func (s *knowledgeIngestionService) Get(id int64) *KnowledgeIngestionSnapshot {
	job := repositories.KnowledgeIngestionJobRepository.Get(sqls.DB(), id)
	if job == nil {
		return nil
	}
	return &KnowledgeIngestionSnapshot{
		Job:      job,
		Document: repositories.KnowledgeDocumentRepository.Get(sqls.DB(), job.DocumentID),
		Asset:    repositories.AssetRepository.Get(sqls.DB(), job.AssetID),
	}
}

func (s *knowledgeIngestionService) Retry(id int64, operator *dto.AuthPrincipal) (*KnowledgeIngestionSnapshot, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	job := repositories.KnowledgeIngestionJobRepository.Get(sqls.DB(), id)
	if job == nil {
		return nil, errorsx.InvalidParamI18n("error.knowledgeIngestion.notFound")
	}
	if job.Status == ingestionStatusProcessing {
		return s.Get(id), nil
	}
	if err := repositories.KnowledgeIngestionJobRepository.Updates(sqls.DB(), id, map[string]any{
		"status": ingestionStatusQueued, "stage": "queued", "progress": 0, "error": "",
		"finished_at": nil, "update_user_id": operator.UserID,
		"update_user_name": operator.Username, "updated_at": time.Now(),
	}); err != nil {
		return nil, err
	}
	_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), job.DocumentID, map[string]any{
		"index_status": enums.KnowledgeDocumentIndexStatusPending, "index_error": "", "indexed_at": nil,
	})
	return s.Get(id), nil
}

func (s *knowledgeIngestionService) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		if err := repositories.KnowledgeIngestionJobRepository.ResetInterrupted(sqls.DB()); err != nil {
			slog.Error("failed to reset interrupted knowledge ingestion jobs", "error", err)
		}
		go s.run(ctx)
	})
}

func (s *knowledgeIngestionService) run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := s.processNext(ctx); err != nil {
			slog.Error("knowledge ingestion worker failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *knowledgeIngestionService) processNext(ctx context.Context) error {
	job, err := repositories.KnowledgeIngestionJobRepository.ClaimNext(sqls.DB())
	if err != nil || job == nil {
		return err
	}
	fail := func(stage string, processErr error, result *rag.DocumentIndexResult) error {
		now := time.Now()
		updates := map[string]any{
			"status": ingestionStatusFailed, "stage": stage, "error": processErr.Error(),
			"finished_at": &now, "updated_at": now,
		}
		if result != nil {
			updates["chunk_count"] = result.ChunkCount
			updates["chunk_ms"] = result.ChunkMS
			updates["embedding_ms"] = result.EmbeddingMS
			updates["index_ms"] = result.IndexMS
		}
		_ = repositories.KnowledgeIngestionJobRepository.Updates(sqls.DB(), job.ID, updates)
		_ = repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), job.DocumentID, map[string]any{
			"index_status": enums.KnowledgeDocumentIndexStatusFailed,
			"index_error":  processErr.Error(), "updated_at": now,
		})
		return processErr
	}
	document := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), job.DocumentID)
	asset := repositories.AssetRepository.Get(sqls.DB(), job.AssetID)
	if document == nil || asset == nil {
		return fail("parsing", fmt.Errorf("ingestion source is unavailable"), nil)
	}
	parseStartedAt := time.Now()
	parsed, err := s.parse(ctx, asset)
	parseMS := time.Since(parseStartedAt).Milliseconds()
	if err != nil {
		_ = repositories.KnowledgeIngestionJobRepository.Updates(sqls.DB(), job.ID, map[string]any{"parse_ms": parseMS})
		return fail("parsing", err, nil)
	}
	plainText := rag.ExtractPlainText(parsed.Content, enums.KnowledgeDocumentContentType(parsed.ContentType))
	hash := sha256.Sum256([]byte(plainText))
	if err := repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), document.ID, map[string]any{
		"content": parsed.Content, "content_type": parsed.ContentType,
		"content_hash": hex.EncodeToString(hash[:]), "updated_at": time.Now(),
	}); err != nil {
		return fail("parsing", err, nil)
	}
	document.Content = parsed.Content
	document.ContentType = enums.KnowledgeDocumentContentType(parsed.ContentType)
	_ = repositories.KnowledgeIngestionJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"parser": parsed.Parser, "parse_ms": parseMS, "stage": "chunking", "progress": 35, "updated_at": time.Now(),
	})
	currentStage := "chunking"
	result, indexErr := rag.Index.IndexDocumentWithProgress(ctx, *document, func(stage rag.DocumentIndexStage) {
		currentStage = string(stage)
		progress := map[rag.DocumentIndexStage]int{
			rag.DocumentIndexStageChunking:  35,
			rag.DocumentIndexStageEmbedding: 55,
			rag.DocumentIndexStageIndexing:  80,
		}[stage]
		_ = repositories.KnowledgeIngestionJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
			"stage": string(stage), "progress": progress, "updated_at": time.Now(),
		})
	})
	if indexErr != nil {
		return fail(currentStage, indexErr, result)
	}
	now := time.Now()
	return repositories.KnowledgeIngestionJobRepository.Updates(sqls.DB(), job.ID, map[string]any{
		"status": ingestionStatusCompleted, "stage": "completed", "progress": 100,
		"chunk_count": result.ChunkCount, "chunk_ms": result.ChunkMS,
		"embedding_ms": result.EmbeddingMS, "index_ms": result.IndexMS,
		"finished_at": &now, "updated_at": now,
	})
}

func (s *knowledgeIngestionService) parse(ctx context.Context, asset *models.Asset) (*parsedKnowledgeDocument, error) {
	extension := strings.ToLower(filepath.Ext(asset.Filename))
	reader, err := AssetService.OpenReader(asset)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if strings.HasSuffix(strings.ToLower(asset.Filename), ".website.json") {
		var source websiteIngestionSource
		if err := json.NewDecoder(reader).Decode(&source); err != nil {
			return nil, err
		}
		content, err := crawlWebsite(ctx, source)
		if err != nil {
			return nil, err
		}
		return &parsedKnowledgeDocument{Filename: asset.Filename, ContentType: "markdown", Content: content, Parser: "website"}, nil
	}
	if extension == ".txt" || extension == ".md" || extension == ".markdown" || extension == ".html" || extension == ".htm" {
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(content)) == 0 {
			return nil, fmt.Errorf("document is empty")
		}
		contentType := "markdown"
		if extension == ".html" || extension == ".htm" {
			contentType = "html"
		}
		return &parsedKnowledgeDocument{Filename: asset.Filename, ContentType: contentType, Content: string(content), Parser: "plain-text"}, nil
	}
	endpoint, err := documentParserEndpoint()
	if err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	go func() {
		defer pipeWriter.Close()
		part, createErr := multipartWriter.CreateFormFile("file", asset.Filename)
		if createErr == nil {
			_, createErr = io.Copy(part, reader)
		}
		if createErr == nil {
			createErr = multipartWriter.Close()
		}
		if createErr != nil {
			_ = pipeWriter.CloseWithError(createErr)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, pipeReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("document parser request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		var payload struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(response.Body).Decode(&payload)
		if payload.Detail == "" {
			payload.Detail = response.Status
		}
		return nil, fmt.Errorf("document parser failed: %s", payload.Detail)
	}
	parsed := &parsedKnowledgeDocument{}
	if err := json.NewDecoder(response.Body).Decode(parsed); err != nil {
		return nil, err
	}
	parsed.Content = strings.TrimSpace(parsed.Content)
	if parsed.Content == "" {
		return nil, fmt.Errorf("document parser returned no content")
	}
	if parsed.ContentType != "html" {
		parsed.ContentType = "markdown"
	}
	return parsed, nil
}

func documentParserEndpoint() (string, error) {
	baseURL := strings.TrimSpace(os.Getenv("DOCUMENT_PARSER_BASE_URL"))
	if baseURL == "" {
		config, err := ai.GetEnabledAIConfig(enums.AIModelTypeEmbedding)
		if err != nil {
			return "", err
		}
		baseURL = config.BaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/parse-document", nil
	}
	return baseURL + "/v1/parse-document", nil
}

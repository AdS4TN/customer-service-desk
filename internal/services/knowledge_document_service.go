package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"agent-desk/internal/ai/rag"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"

	"agent-desk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/common/strs"
	"github.com/mlogclub/simple/sqls"
)

var KnowledgeDocumentService = newKnowledgeDocumentService()

func newKnowledgeDocumentService() *knowledgeDocumentService {
	return &knowledgeDocumentService{}
}

type knowledgeDocumentService struct {
}

func (s *knowledgeDocumentService) Get(id int64) *models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.Get(sqls.DB(), id)
}

func (s *knowledgeDocumentService) Take(where ...interface{}) *models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.Take(sqls.DB(), where...)
}

func (s *knowledgeDocumentService) Find(cnd *sqls.Cnd) []models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.Find(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) FindOne(cnd *sqls.Cnd) *models.KnowledgeDocument {
	return repositories.KnowledgeDocumentRepository.FindOne(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) FindPageByParams(params *params.QueryParams) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	return repositories.KnowledgeDocumentRepository.FindPageByParams(sqls.DB(), params)
}

func (s *knowledgeDocumentService) FindPageByCnd(cnd *sqls.Cnd) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	return repositories.KnowledgeDocumentRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) FindPageListByCnd(cnd *sqls.Cnd) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	return repositories.KnowledgeDocumentRepository.FindPageListByCnd(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) Count(cnd *sqls.Cnd) int64 {
	return repositories.KnowledgeDocumentRepository.Count(sqls.DB(), cnd)
}

func (s *knowledgeDocumentService) Create(t *models.KnowledgeDocument) error {
	return repositories.KnowledgeDocumentRepository.Create(sqls.DB(), t)
}

func (s *knowledgeDocumentService) Update(t *models.KnowledgeDocument) error {
	return repositories.KnowledgeDocumentRepository.Update(sqls.DB(), t)
}

func (s *knowledgeDocumentService) Updates(id int64, columns map[string]interface{}) error {
	return repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), id, columns)
}

func (s *knowledgeDocumentService) UpdateColumn(id int64, name string, value interface{}) error {
	return repositories.KnowledgeDocumentRepository.UpdateColumn(sqls.DB(), id, name, value)
}

func (s *knowledgeDocumentService) Delete(id int64) {
	repositories.KnowledgeDocumentRepository.Delete(sqls.DB(), id)
}

func (s *knowledgeDocumentService) CreateKnowledgeDocument(req request.CreateKnowledgeDocumentRequest, operator *dto.AuthPrincipal) (*models.KnowledgeDocument, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	kb := KnowledgeBaseService.Get(req.KnowledgeBaseID)
	if kb == nil {
		return nil, errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return nil, errorsx.InvalidParamI18n("error.e0026")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return nil, err
	}
	item, err := s.buildKnowledgeDocumentModel(req)
	if err != nil {
		return nil, err
	}
	item.Status = enums.StatusOk
	item.IndexStatus = enums.KnowledgeDocumentIndexStatusPending
	item.IndexError = ""
	item.IndexedAt = nil
	item.AuditFields = utils.BuildAuditFields(operator)
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := ctx.Tx.Create(item).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	documentID := item.ID
	go func() {
		if err := rag.Index.IndexDocumentByID(context.Background(), documentID); err != nil {
			slog.Error("failed to index created knowledge document", "document_id", documentID, "error", err)
		}
	}()
	return item, nil
}

func (s *knowledgeDocumentService) UpdateKnowledgeDocument(req request.UpdateKnowledgeDocumentRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	current := s.Get(req.ID)
	if current == nil {
		return errorsx.InvalidParamI18n("error.e0218")
	}
	kb := KnowledgeBaseService.Get(req.KnowledgeBaseID)
	if kb == nil {
		return errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return errorsx.InvalidParamI18n("error.e0026")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return err
	}
	item, err := s.buildKnowledgeDocumentModel(req.CreateKnowledgeDocumentRequest)
	if err != nil {
		return err
	}
	oldKnowledgeBaseID := current.KnowledgeBaseID
	if err := repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), req.ID, map[string]any{
		"knowledge_base_id":     item.KnowledgeBaseID,
		"directory_id":          item.DirectoryID,
		"title":                 item.Title,
		"content_type":          item.ContentType,
		"content_hash":          item.ContentHash,
		"content":               item.Content,
		"chunk_config_override": item.ChunkConfigOverride,
		"chunk_provider":        item.ChunkProvider,
		"chunk_target_tokens":   item.ChunkTargetTokens,
		"chunk_max_tokens":      item.ChunkMaxTokens,
		"chunk_overlap_tokens":  item.ChunkOverlapTokens,
		"parent_chunk_tokens":   item.ParentChunkTokens,
		"child_chunk_tokens":    item.ChildChunkTokens,
		"index_status":          enums.KnowledgeDocumentIndexStatusPending,
		"indexed_at":            nil,
		"index_error":           "",
		"update_user_id":        operator.UserID,
		"update_user_name":      operator.Username,
		"updated_at":            time.Now(),
	}); err != nil {
		return err
	}

	if oldKnowledgeBaseID != item.KnowledgeBaseID {
		if err := rag.Index.RemoveDocumentIndex(context.Background(), req.ID); err != nil {
			slog.Error("failed to remove old document index after knowledge base change", "document_id", req.ID, "knowledge_base_id", oldKnowledgeBaseID, "error", err)
		}
	}

	if err := rag.Index.IndexDocumentByID(context.Background(), req.ID); err != nil {
		slog.Error("failed to reindex updated knowledge document", "document_id", req.ID, "error", err)
	}
	return nil
}

func (s *knowledgeDocumentService) DeleteKnowledgeDocument(id int64) error {
	if err := repositories.KnowledgeDocumentRepository.Updates(sqls.DB(), id, map[string]any{
		"status":     enums.StatusDeleted,
		"updated_at": time.Now(),
	}); err != nil {
		return err
	}
	return rag.Index.RemoveDocumentIndex(context.Background(), id)
}

func (s *knowledgeDocumentService) BatchMoveKnowledgeDocuments(req request.BatchMoveKnowledgeDocumentRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0333")
	}
	kb := KnowledgeBaseService.Get(req.KnowledgeBaseID)
	if kb == nil {
		return errorsx.InvalidParamI18n("error.e0283")
	}
	if kb.KnowledgeType == string(enums.KnowledgeBaseTypeFAQ) {
		return errorsx.InvalidParamI18n("error.e0026")
	}
	if _, err := KnowledgeDirectoryService.RequireUsableDirectory(req.KnowledgeBaseID, req.DirectoryID); err != nil {
		return err
	}
	for _, id := range ids {
		current := s.Get(id)
		if current == nil || current.Status == enums.StatusDeleted {
			return errorsx.InvalidParamI18n("error.e0218")
		}
		if current.KnowledgeBaseID != req.KnowledgeBaseID {
			return errorsx.InvalidParamI18n("error.e0139")
		}
	}
	now := time.Now()
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		for _, id := range ids {
			if err := repositories.KnowledgeDocumentRepository.Updates(ctx.Tx, id, map[string]any{
				"directory_id":     req.DirectoryID,
				"index_status":     enums.KnowledgeDocumentIndexStatusPending,
				"indexed_at":       nil,
				"index_error":      "",
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
				"updated_at":       now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, id := range ids {
		if err := rag.Index.IndexDocumentByID(context.Background(), id); err != nil {
			slog.Error("failed to reindex moved knowledge document", "document_id", id, "error", err)
		}
	}
	return nil
}

func (s *knowledgeDocumentService) BatchDeleteKnowledgeDocuments(req request.BatchDeleteKnowledgeDocumentRequest) error {
	ids := uniquePositiveIDs(req.IDs)
	if len(ids) == 0 {
		return errorsx.InvalidParamI18n("error.e0331")
	}
	for _, id := range ids {
		if err := s.DeleteKnowledgeDocument(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *knowledgeDocumentService) buildKnowledgeDocumentModel(req request.CreateKnowledgeDocumentRequest) (*models.KnowledgeDocument, error) {
	if strs.IsBlank(string(req.ContentType)) {
		req.ContentType = enums.KnowledgeDocumentContentTypeHTML
	}
	if req.ContentType != enums.KnowledgeDocumentContentTypeHTML && req.ContentType != enums.KnowledgeDocumentContentTypeMarkdown {
		return nil, errorsx.InvalidParamI18n("error.e0129")
	}

	plainText := rag.ExtractPlainText(req.Content, req.ContentType)
	item := &models.KnowledgeDocument{
		KnowledgeBaseID:     req.KnowledgeBaseID,
		DirectoryID:         req.DirectoryID,
		Title:               req.Title,
		ContentType:         req.ContentType,
		Content:             req.Content,
		ChunkConfigOverride: req.ChunkConfigOverride,
		ChunkProvider:       req.ChunkProvider,
		ChunkTargetTokens:   req.ChunkTargetTokens,
		ChunkMaxTokens:      req.ChunkMaxTokens,
		ChunkOverlapTokens:  req.ChunkOverlapTokens,
		ParentChunkTokens:   req.ParentChunkTokens,
		ChildChunkTokens:    req.ChildChunkTokens,
	}
	if item.ChunkConfigOverride {
		if !isValidDocumentChunkProvider(item.ChunkProvider) || item.ChunkTargetTokens <= 0 || item.ChunkMaxTokens < item.ChunkTargetTokens || item.ChunkOverlapTokens < 0 || item.ChunkOverlapTokens >= item.ChunkMaxTokens {
			return nil, errorsx.InvalidParamI18n("error.e0130")
		}
		if item.ParentChunkTokens <= 0 {
			item.ParentChunkTokens = 900
		}
		if item.ChildChunkTokens <= 0 {
			item.ChildChunkTokens = 200
		}
		if item.ParentChunkTokens < item.ChildChunkTokens {
			return nil, errorsx.InvalidParamI18n("error.e0130")
		}
	} else {
		item.ChunkProvider = ""
		item.ChunkTargetTokens = 0
		item.ChunkMaxTokens = 0
		item.ChunkOverlapTokens = 0
		item.ParentChunkTokens = 0
		item.ChildChunkTokens = 0
	}
	if plainText != "" {
		hash := sha256.Sum256([]byte(plainText))
		item.ContentHash = hex.EncodeToString(hash[:])
	}
	return item, nil
}

func isValidDocumentChunkProvider(provider string) bool {
	switch provider {
	case string(enums.KnowledgeChunkProviderFixed),
		string(enums.KnowledgeChunkProviderStructured),
		string(enums.KnowledgeChunkProviderRecursive),
		string(enums.KnowledgeChunkProviderParentChild):
		return true
	default:
		return false
	}
}

func uniquePositiveIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

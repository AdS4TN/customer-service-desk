package dashboard

import (
	"agent-desk/internal/pkg/httpx"
	"context"

	"agent-desk/internal/ai/rag"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/services"

	"agent-desk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
)

func KnowledgeRetrievePostPreview(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.PreviewKnowledgeDocumentRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	preview, err := rag.Index.PreviewDocumentChunks(
		context.Background(),
		req.KnowledgeBaseID,
		req.Title,
		req.ContentType,
		req.Content,
		rag.DocumentChunkOverrides{
			Enabled:       req.ChunkConfigOverride,
			Provider:      req.ChunkProvider,
			TargetTokens:  req.ChunkTargetTokens,
			MaxTokens:     req.ChunkMaxTokens,
			OverlapTokens: req.ChunkOverlapTokens,
			ParentTokens:  req.ParentChunkTokens,
			ChildTokens:   req.ChildChunkTokens,
		},
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	chunks := make([]response.KnowledgeChunkPreviewResponse, 0, len(preview.Chunks))
	for _, chunk := range preview.Chunks {
		chunks = append(chunks, response.KnowledgeChunkPreviewResponse{
			ChunkNo:        chunk.ChunkNo,
			Title:          chunk.Title,
			Content:        chunk.Content,
			ContextContent: chunk.ContextContent,
			ChunkType:      chunk.ChunkType,
			SectionPath:    chunk.SectionPath,
			CharCount:      chunk.CharCount,
			TokenCount:     chunk.TokenCount,
		})
	}
	httpx.WriteJSON(ctx, response.KnowledgeDocumentChunkPreviewResponse{
		Provider:      preview.Provider,
		TargetTokens:  preview.TargetTokens,
		MaxTokens:     preview.MaxTokens,
		OverlapTokens: preview.OverlapTokens,
		ParentTokens:  preview.ParentTokens,
		ChildTokens:   preview.ChildTokens,
		ChunkCount:    len(chunks),
		Chunks:        chunks,
	})
}

func KnowledgeRetrievePostDebugSearch(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.KnowledgeSearchRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	resp, err := rag.Answer.DebugSearch(context.Background(), req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}

func KnowledgeRetrievePostDebugAnswer(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.KnowledgeAnswerRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	resp, err := rag.Answer.DebugAnswer(context.Background(), req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}

func KnowledgeRetrievePostBuild(ctx *gin.Context) {
	req := struct {
		DocumentID int64 `json:"documentId"`
		FAQID      int64 `json:"faqId"`
	}{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if req.DocumentID > 0 {
		if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeDocumentUpdate); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		if err := rag.Answer.BuildDocumentIndex(context.Background(), req.DocumentID); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		httpx.WriteJSON(ctx, nil)
		return
	}

	if req.FAQID > 0 {
		if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionKnowledgeFAQUpdate); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		if err := rag.Index.IndexFAQByID(context.Background(), req.FAQID); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		httpx.WriteJSON(ctx, nil)
		return
	}

	httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0066"))
}

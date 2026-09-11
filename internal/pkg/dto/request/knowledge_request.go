package request

import (
	"io"

	"agent-desk/internal/pkg/enums"
)

type CreateKnowledgeBaseRequest struct {
	Name                  string  `json:"name"`
	Description           string  `json:"description"`
	KnowledgeType         string  `json:"knowledgeType"`
	DefaultTopK           int     `json:"defaultTopK"`
	DefaultScoreThreshold float64 `json:"defaultScoreThreshold"`
	DefaultRerankLimit    int     `json:"defaultRerankLimit"`
	ChunkProvider         string  `json:"chunkProvider"`
	ChunkTargetTokens     int     `json:"chunkTargetTokens"`
	ChunkMaxTokens        int     `json:"chunkMaxTokens"`
	ChunkOverlapTokens    int     `json:"chunkOverlapTokens"`
	ParentChunkTokens     int     `json:"parentChunkTokens"`
	ChildChunkTokens      int     `json:"childChunkTokens"`
	AnswerMode            int     `json:"answerMode"`
	Remark                string  `json:"remark"`
}

type UpdateKnowledgeBaseRequest struct {
	ID int64 `json:"id"`
	CreateKnowledgeBaseRequest
}

type CreateKnowledgeDirectoryRequest struct {
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	ParentID        int64  `json:"parentId"`
	Name            string `json:"name"`
	Remark          string `json:"remark"`
}

type UpdateKnowledgeDirectoryRequest struct {
	ID int64 `json:"id"`
	CreateKnowledgeDirectoryRequest
}

type DeleteKnowledgeDirectoryRequest struct {
	ID int64 `json:"id"`
}

type CreateKnowledgeDocumentRequest struct {
	KnowledgeBaseID     int64                              `json:"knowledgeBaseId"`
	DirectoryID         int64                              `json:"directoryId"`
	Title               string                             `json:"title"`
	ContentType         enums.KnowledgeDocumentContentType `json:"contentType"`
	Content             string                             `json:"content"`
	ChunkConfigOverride bool                               `json:"chunkConfigOverride"`
	ChunkProvider       string                             `json:"chunkProvider"`
	ChunkTargetTokens   int                                `json:"chunkTargetTokens"`
	ChunkMaxTokens      int                                `json:"chunkMaxTokens"`
	ChunkOverlapTokens  int                                `json:"chunkOverlapTokens"`
	ParentChunkTokens   int                                `json:"parentChunkTokens"`
	ChildChunkTokens    int                                `json:"childChunkTokens"`
}

type UploadKnowledgeDocumentRequest struct {
	KnowledgeBaseID     int64  `form:"knowledgeBaseId" validate:"required"`
	DirectoryID         int64  `form:"directoryId"`
	Title               string `form:"title"`
	ChunkConfigOverride bool   `form:"chunkConfigOverride"`
	ChunkProvider       string `form:"chunkProvider"`
	ChunkTargetTokens   int    `form:"chunkTargetTokens"`
	ChunkMaxTokens      int    `form:"chunkMaxTokens"`
	ChunkOverlapTokens  int    `form:"chunkOverlapTokens"`
	ParentChunkTokens   int    `form:"parentChunkTokens"`
	ChildChunkTokens    int    `form:"childChunkTokens"`
}

type CrawlKnowledgeWebsiteRequest struct {
	KnowledgeBaseID     int64  `json:"knowledgeBaseId" validate:"required"`
	DirectoryID         int64  `json:"directoryId"`
	URL                 string `json:"url" validate:"required"`
	MaxPages            int    `json:"maxPages"`
	MaxDepth            int    `json:"maxDepth"`
	ChunkConfigOverride bool   `json:"chunkConfigOverride"`
	ChunkProvider       string `json:"chunkProvider"`
	ChunkTargetTokens   int    `json:"chunkTargetTokens"`
	ChunkMaxTokens      int    `json:"chunkMaxTokens"`
	ChunkOverlapTokens  int    `json:"chunkOverlapTokens"`
	ParentChunkTokens   int    `json:"parentChunkTokens"`
	ChildChunkTokens    int    `json:"childChunkTokens"`
}

type PreviewKnowledgeDocumentRequest struct {
	KnowledgeBaseID     int64                              `json:"knowledgeBaseId"`
	Title               string                             `json:"title"`
	ContentType         enums.KnowledgeDocumentContentType `json:"contentType"`
	Content             string                             `json:"content"`
	ChunkConfigOverride bool                               `json:"chunkConfigOverride"`
	ChunkProvider       string                             `json:"chunkProvider"`
	ChunkTargetTokens   int                                `json:"chunkTargetTokens"`
	ChunkMaxTokens      int                                `json:"chunkMaxTokens"`
	ChunkOverlapTokens  int                                `json:"chunkOverlapTokens"`
	ParentChunkTokens   int                                `json:"parentChunkTokens"`
	ChildChunkTokens    int                                `json:"childChunkTokens"`
}

type UpdateKnowledgeDocumentRequest struct {
	ID int64 `json:"id"`
	CreateKnowledgeDocumentRequest
}

type BatchMoveKnowledgeDocumentRequest struct {
	KnowledgeBaseID int64   `json:"knowledgeBaseId"`
	DirectoryID     int64   `json:"directoryId"`
	IDs             []int64 `json:"ids"`
}

type BatchDeleteKnowledgeDocumentRequest struct {
	IDs []int64 `json:"ids"`
}

type CreateKnowledgeFAQRequest struct {
	KnowledgeBaseID  int64    `json:"knowledgeBaseId"`
	DirectoryID      int64    `json:"directoryId"`
	Question         string   `json:"question"`
	Answer           string   `json:"answer"`
	SimilarQuestions []string `json:"similarQuestions"`
	Remark           string   `json:"remark"`
}

type UpdateKnowledgeFAQRequest struct {
	ID int64 `json:"id"`
	CreateKnowledgeFAQRequest
}

type BatchMoveKnowledgeFAQRequest struct {
	KnowledgeBaseID int64   `json:"knowledgeBaseId"`
	DirectoryID     int64   `json:"directoryId"`
	IDs             []int64 `json:"ids"`
}

type BatchDeleteKnowledgeFAQRequest struct {
	IDs []int64 `json:"ids"`
}

type KnowledgeFAQImportMode string

const (
	KnowledgeFAQImportModeAppend    KnowledgeFAQImportMode = "append"
	KnowledgeFAQImportModeOverwrite KnowledgeFAQImportMode = "overwrite"
)

type ImportKnowledgeFAQRequest struct {
	KnowledgeBaseID int64
	Mode            KnowledgeFAQImportMode
	Filename        string
	Reader          io.Reader
	Locale          string
}

type KnowledgeSearchRequest struct {
	KnowledgeBaseIDs []int64 `json:"knowledgeBaseIds"`
	Question         string  `json:"question"`
	TopK             int     `json:"topK"`
	ScoreThreshold   float64 `json:"scoreThreshold"`
	RerankLimit      int     `json:"rerankLimit"`
	Channel          string  `json:"channel"`
	Scene            string  `json:"scene"`
	SessionID        string  `json:"sessionId"`
	ConversationID   int64   `json:"conversationId"`
}

type KnowledgeAnswerRequest struct {
	KnowledgeBaseIDs []int64 `json:"knowledgeBaseIds"`
	Question         string  `json:"question"`
	TopK             int     `json:"topK"`
	ScoreThreshold   float64 `json:"scoreThreshold"`
	RerankLimit      int     `json:"rerankLimit"`
	Channel          string  `json:"channel"`
	Scene            string  `json:"scene"`
	SessionID        string  `json:"sessionId"`
	ConversationID   int64   `json:"conversationId"`
	AnswerMode       int     `json:"answerMode"`
}

type CreateKnowledgeFeedbackRequest struct {
	RetrieveLogID  int64  `json:"retrieveLogId"`
	FeedbackType   int    `json:"feedbackType"`
	FeedbackReason string `json:"feedbackReason"`
	Remark         string `json:"remark"`
}

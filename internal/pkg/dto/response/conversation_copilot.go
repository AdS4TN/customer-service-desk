package response

type CopilotKnowledgeSource struct {
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	DocumentID      int64  `json:"documentId"`
	ChunkID         int64  `json:"chunkId"`
	Title           string `json:"title"`
	Content         string `json:"content"`
}

type ConversationReplySuggestion struct {
	ConversationID  int64                    `json:"conversationId"`
	LastMessageID   int64                    `json:"lastMessageId"`
	Content         string                   `json:"content"`
	KnowledgeStatus string                   `json:"knowledgeStatus"`
	Sources         []CopilotKnowledgeSource `json:"sources"`
	RetrievalMs     int64                    `json:"retrievalMs"`
	GenerationMs    int64                    `json:"generationMs"`
	DurationMs      int64                    `json:"durationMs"`
}

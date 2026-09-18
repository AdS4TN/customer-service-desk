package response

type CopilotKnowledgeSource struct {
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	DocumentID      int64  `json:"documentId"`
	ChunkID         int64  `json:"chunkId"`
	Title           string `json:"title"`
	Content         string `json:"content"`
}

type CopilotSkillTrace struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type CopilotToolTrace struct {
	Code   string `json:"code"`
	Status string `json:"status"`
}

type ConversationReplySuggestion struct {
	HandoffRequested bool                     `json:"handoffRequested,omitempty"`
	HandoffReason    string                   `json:"handoffReason,omitempty"`
	ConversationID   int64                    `json:"conversationId"`
	LastMessageID    int64                    `json:"lastMessageId"`
	AgentName        string                   `json:"agentName"`
	ModelName        string                   `json:"modelName"`
	Content          string                   `json:"content"`
	SkillStatus      string                   `json:"skillStatus"`
	Skills           []CopilotSkillTrace      `json:"skills"`
	Tools            []CopilotToolTrace       `json:"tools"`
	KnowledgeStatus  string                   `json:"knowledgeStatus"`
	Sources          []CopilotKnowledgeSource `json:"sources"`
	RoutingMs        int64                    `json:"routingMs"`
	RetrievalMs      int64                    `json:"retrievalMs"`
	GenerationMs     int64                    `json:"generationMs"`
	DurationMs       int64                    `json:"durationMs"`
}

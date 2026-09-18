package request

type AIEmployeePreviewTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIEmployeePreviewRequest struct {
	Draft    CreateAIAgentRequest    `json:"draft"`
	Messages []AIEmployeePreviewTurn `json:"messages"`
}

package request

type StartConversationDelegation struct {
	Revision        int64  `json:"revision"`
	AIAgentID       int64  `json:"aiAgentId"`
	PreviewOnly     bool   `json:"previewOnly"`
	DurationMinutes int    `json:"durationMinutes"`
	Instructions    string `json:"instructions"`
}

type ChangeConversationDelegation struct {
	Revision int64 `json:"revision"`
}

package request

type UpdateConversationMemory struct {
	ID       int64  `json:"id"`
	Revision int64  `json:"revision"`
	Value    string `json:"value"`
	Delete   bool   `json:"delete"`
}

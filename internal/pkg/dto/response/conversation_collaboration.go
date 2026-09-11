package response

type ConversationColleague struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type ConversationNote struct {
	ID         int64                   `json:"id"`
	AuthorID   int64                   `json:"authorId"`
	AuthorName string                  `json:"authorName"`
	Content    string                  `json:"content"`
	Mentions   []ConversationColleague `json:"mentions"`
	CreatedAt  string                  `json:"createdAt"`
}
type ConversationHandoff struct {
	ID        int64  `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"createdAt"`
}
type ConversationCollaboration struct {
	Notes    []ConversationNote    `json:"notes"`
	HasMore  bool                  `json:"hasMore"`
	Handoffs []ConversationHandoff `json:"handoffs"`
}

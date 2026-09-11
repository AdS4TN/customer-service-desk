package response

type WidgetConfigResponse struct {
	ChannelID   string `json:"channelId"`
	ChannelType string `json:"channelType"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	AgentName   string `json:"agentName"`
	AgentAvatar string `json:"agentAvatar"`
	AgentStatus string `json:"agentStatus"`
	ThemeColor  string `json:"themeColor"`
	Position    string `json:"position"`
	Width       string `json:"width"`
}

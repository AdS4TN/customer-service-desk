package response

type PreviewMountedSkill struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type AIEmployeePreviewResponse struct {
	Content         string                   `json:"content"`
	ModelName       string                   `json:"modelName"`
	KnowledgeStatus string                   `json:"knowledgeStatus"`
	Sources         []CopilotKnowledgeSource `json:"sources"`
	MountedSkills   []PreviewMountedSkill    `json:"mountedSkills"`
	DurationMs      int64                    `json:"durationMs"`
}

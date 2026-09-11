package enums

type MemoryKind string

const (
	MemoryKindSummary         MemoryKind = "summary"
	MemoryKindInquiry         MemoryKind = "inquiry"
	MemoryKindCustomer        MemoryKind = "customer"
	MemoryKindCustomerTag     MemoryKind = "customer_tag"
	MemoryKindCustomerProfile MemoryKind = "customer_profile"
	MemoryKindOpenQuestion    MemoryKind = "open_question"
	MemoryKindNextStep        MemoryKind = "next_step"
)

type MemoryStatus string

var memoryKindLabelMap = map[MemoryKind]string{
	MemoryKindSummary: "交接概况", MemoryKindInquiry: "当前需求", MemoryKindCustomer: "客户长期信息",
	MemoryKindOpenQuestion: "待确认问题", MemoryKindNextStep: "建议下一步",
	MemoryKindCustomerTag:     "AI 客户标签",
	MemoryKindCustomerProfile: "AI 客户档案",
}

const (
	MemoryStatusEmpty      MemoryStatus = "empty"
	MemoryStatusQueued     MemoryStatus = "queued"
	MemoryStatusProcessing MemoryStatus = "processing"
	MemoryStatusReady      MemoryStatus = "ready"
	MemoryStatusFailed     MemoryStatus = "failed"
)

var memoryStatusLabelMap = map[MemoryStatus]string{
	MemoryStatusEmpty: "尚未整理", MemoryStatusQueued: "等待整理", MemoryStatusProcessing: "正在分析对话",
	MemoryStatusReady: "已整理", MemoryStatusFailed: "整理失败",
}

package enums

type LeadStatus string

const (
	LeadStatusNew       LeadStatus = "new"
	LeadStatusFollowing LeadStatus = "following"
	LeadStatusQualified LeadStatus = "qualified"
	LeadStatusWon       LeadStatus = "won"
	LeadStatusLost      LeadStatus = "lost"
	LeadStatusArchived  LeadStatus = "archived"
)

var leadStatusLabelMap = map[LeadStatus]string{
	LeadStatusNew: "新线索", LeadStatusFollowing: "跟进中", LeadStatusQualified: "有效询盘",
	LeadStatusWon: "已成交", LeadStatusLost: "已流失", LeadStatusArchived: "已归档",
}

type LeadTag string

const (
	LeadTagQuote   LeadTag = "quote"
	LeadTagSample  LeadTag = "sample"
	LeadTagBulk    LeadTag = "bulk"
	LeadTagUrgent  LeadTag = "urgent"
	LeadTagContact LeadTag = "contact"
)

var leadTagLabelMap = map[LeadTag]string{
	LeadTagQuote: "询价", LeadTagSample: "样品需求", LeadTagBulk: "批量采购", LeadTagUrgent: "交期紧急", LeadTagContact: "已留联系方式",
}

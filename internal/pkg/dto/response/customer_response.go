package response

import "agent-desk/internal/pkg/enums"

type CustomerResponse struct {
	Avatar        string           `json:"avatar"`
	AvatarState   string           `json:"avatarState"`
	ChannelName   string           `json:"channelName"`
	ManualTags    []string         `json:"manualTags"`
	ID            int64            `json:"id"`
	Name          string           `json:"name"`
	Gender        enums.Gender     `json:"gender"`
	CompanyID     int64            `json:"companyId"`
	Company       *CompanyResponse `json:"company"`
	LastActiveAt  string           `json:"lastActiveAt"`
	PrimaryMobile string           `json:"primaryMobile"`
	PrimaryEmail  string           `json:"primaryEmail"`
	Status        enums.Status     `json:"status"`
	Remark        string           `json:"remark"`
	CreatedAt     string           `json:"createdAt"`
	UpdatedAt     string           `json:"updatedAt"`
}

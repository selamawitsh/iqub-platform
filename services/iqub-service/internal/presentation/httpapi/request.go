package httpapi

type createIqubRequest struct {
	Name               string `json:"name"`
	ContributionAmount int64  `json:"contribution_amount"`
	Frequency          string `json:"frequency"`
	MaxMembers         int    `json:"max_members"`
}

type joinIqubRequest struct {
	InviteCode string `json:"invite_code"`
}

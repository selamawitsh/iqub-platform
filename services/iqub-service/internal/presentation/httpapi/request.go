package httpapi

// createIqubRequest is the JSON the client sends. It is separate from the
// domain model on purpose: the API shape can change without touching the domain.
type createIqubRequest struct {
	Name               string `json:"name"`
	ContributionAmount int64  `json:"contribution_amount"` // minor units (santim)
	Frequency          string `json:"frequency"`
	MaxMembers         int    `json:"max_members"`
}

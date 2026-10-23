package model

import "time"

type ContributionStatus string

const (
	ContributionRecorded  ContributionStatus = "RECORDED"
	ContributionConfirmed ContributionStatus = "CONFIRMED"
)

// Contribution is one member's payment for one round. Amount is in minor
// units and always comes from the group, never from the client (SRS BR-12).
type Contribution struct {
	ID          string
	IqubID      string
	RoundID     string
	MemberID    string
	Amount      int64
	Status      ContributionStatus
	RecordedAt  time.Time
	ConfirmedAt time.Time
}

func NewContribution(id, iqubID, roundID, memberID string, amount int64, now time.Time) (Contribution, error) {
	if id == "" || iqubID == "" || roundID == "" || memberID == "" {
		return Contribution{}, validation("contribution ids must not be empty")
	}
	if amount <= 0 {
		return Contribution{}, validation("amount must be greater than zero")
	}
	return Contribution{
		ID:         id,
		IqubID:     iqubID,
		RoundID:    roundID,
		MemberID:   memberID,
		Amount:     amount,
		Status:     ContributionRecorded,
		RecordedAt: now,
	}, nil
}

// Confirm moves RECORDED to CONFIRMED.
func (c *Contribution) Confirm(now time.Time) error {
	if c.Status != ContributionRecorded {
		return conflict("contribution is already confirmed")
	}
	c.Status = ContributionConfirmed
	c.ConfirmedAt = now
	return nil
}

// HasContributed reports whether the member already has a contribution in
// the given list (SRS FR-I7).
func HasContributed(contributions []Contribution, memberID string) bool {
	for _, c := range contributions {
		if c.MemberID == memberID {
			return true
		}
	}
	return false
}

// AllConfirmed reports whether every member has a CONFIRMED contribution
// (SRS BR-6). This is the rule that lets a round complete.
func AllConfirmed(memberIDs []string, contributions []Contribution) bool {
	confirmed := make(map[string]bool, len(contributions))
	for _, c := range contributions {
		if c.Status == ContributionConfirmed {
			confirmed[c.MemberID] = true
		}
	}
	for _, id := range memberIDs {
		if !confirmed[id] {
			return false
		}
	}
	return true
}

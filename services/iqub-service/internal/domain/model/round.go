package model

import "time"

type RoundStatus string

const (
	RoundPending  RoundStatus = "PENDING"
	RoundCurrent  RoundStatus = "CURRENT"
	RoundComplete RoundStatus = "COMPLETE"
)

// Round is one period of the cycle. Exactly one member receives the pool.
type Round struct {
	ID          string
	IqubID      string
	Number      int
	DueDate     time.Time
	Status      RoundStatus
	RecipientID string
}

// Activate moves PENDING to CURRENT.
func (r *Round) Activate() error {
	if r.Status != RoundPending {
		return conflict("only a pending round can become current")
	}
	r.Status = RoundCurrent
	return nil
}

// Complete moves CURRENT to COMPLETE.
func (r *Round) Complete() error {
	if r.Status != RoundCurrent {
		return conflict("only the current round can be completed")
	}
	r.Status = RoundComplete
	return nil
}

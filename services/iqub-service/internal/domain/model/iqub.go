package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Frequency string

const (
	Weekly   Frequency = "WEEKLY"
	Biweekly Frequency = "BIWEEKLY"
	Monthly  Frequency = "MONTHLY"
)

type Status string

const (
	StatusOpen      Status = "OPEN"
	StatusActive    Status = "ACTIVE"
	StatusCompleted Status = "COMPLETED"
)

const (
	MinMembers      = 2
	MaxMembersLimit = 50
)

// ErrValidation marks errors caused by invalid input, so outer layers can
// tell them apart from internal failures.
var ErrValidation = errors.New("validation error")

func validation(msg string) error {
	return fmt.Errorf("%w: %s", ErrValidation, msg)
}

// Iqub is a savings group. ContributionAmount is in minor units (santim),
// never a float (SRS BR-1).
type Iqub struct {
	ID                 string
	Name               string
	ContributionAmount int64
	Frequency          Frequency
	MaxMembers         int
	Status             Status
	AdminID            string
	MemberIDs          []string
	CreatedAt          time.Time
}

// NewIqub creates a group in OPEN status. The admin is its first member.
// The time is passed in, so the function is deterministic and easy to test.
func NewIqub(id, name string, amount int64, freq Frequency, maxMembers int, adminID string, now time.Time) (Iqub, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Iqub{}, validation("name must not be empty")
	}
	if amount <= 0 {
		return Iqub{}, validation("contribution amount must be greater than zero")
	}
	switch freq {
	case Weekly, Biweekly, Monthly:
	default:
		return Iqub{}, validation("frequency must be WEEKLY, BIWEEKLY or MONTHLY")
	}
	if maxMembers < MinMembers || maxMembers > MaxMembersLimit {
		return Iqub{}, validation(fmt.Sprintf("max members must be between %d and %d", MinMembers, MaxMembersLimit))
	}
	if strings.TrimSpace(adminID) == "" {
		return Iqub{}, validation("admin id must not be empty")
	}

	return Iqub{
		ID:                 id,
		Name:               name,
		ContributionAmount: amount,
		Frequency:          freq,
		MaxMembers:         maxMembers,
		Status:             StatusOpen,
		AdminID:            adminID,
		MemberIDs:          []string{adminID},
		CreatedAt:          now,
	}, nil
}

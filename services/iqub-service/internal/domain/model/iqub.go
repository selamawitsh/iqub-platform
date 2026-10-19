package model

import (
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
	InviteCode         string
	CreatedAt          time.Time
	StartedAt          time.Time
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

// SetInviteCode stores the code members use to join. Codes are uppercase.
// The code is generated outside the domain, which keeps the domain deterministic.
func (i *Iqub) SetInviteCode(code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return validation("invite code must not be empty")
	}
	i.InviteCode = code
	return nil
}

// IsMember reports whether the member belongs to the group.
func (i *Iqub) IsMember(memberID string) bool {
	for _, id := range i.MemberIDs {
		if id == memberID {
			return true
		}
	}
	return false
}

// EnsureAdmin returns ErrForbidden unless the member is the group admin.
func (i *Iqub) EnsureAdmin(memberID string) error {
	if memberID != i.AdminID {
		return forbidden("only the group admin can do this")
	}
	return nil
}

// EnsureActive returns ErrConflict unless the group is ACTIVE.
func (i *Iqub) EnsureActive() error {
	if i.Status != StatusActive {
		return conflict("group is not active")
	}
	return nil
}

// AddMember adds a member to the group. A member can only join while the
// group is OPEN, only once (SRS BR-4), and only if there is room.
func (i *Iqub) AddMember(memberID string) error {
	memberID = strings.TrimSpace(memberID)
	if memberID == "" {
		return validation("member id must not be empty")
	}
	if i.Status != StatusOpen {
		return conflict("group is not open for joining")
	}
	if i.IsMember(memberID) {
		return conflict("already a member of this group")
	}
	if len(i.MemberIDs) >= i.MaxMembers {
		return conflict("group is full")
	}
	i.MemberIDs = append(i.MemberIDs, memberID)
	return nil
}

// PayoutAmount is what each round's recipient receives (SRS BR-2, BR-9).
// Nobody can join after the group starts, so the current member count is
// the count at the moment the group started.
func (i *Iqub) PayoutAmount() int64 {
	return i.ContributionAmount * int64(len(i.MemberIDs))
}

// Start moves the group from OPEN to ACTIVE and returns one round per
// member. order is the payout order and must contain every member exactly
// once. The caller draws it randomly; the domain only checks it.
func (i *Iqub) Start(callerID string, order []string, now time.Time) ([]Round, error) {
	if err := i.EnsureAdmin(callerID); err != nil {
		return nil, err
	}
	if i.Status != StatusOpen {
		return nil, conflict("group has already started")
	}
	if len(i.MemberIDs) < MinMembers {
		return nil, conflict(fmt.Sprintf("group needs at least %d members to start", MinMembers))
	}
	if !isPermutation(order, i.MemberIDs) {
		return nil, validation("payout order must contain every member exactly once")
	}

	i.Status = StatusActive
	i.StartedAt = now

	rounds := make([]Round, len(order))
	for n, recipient := range order {
		number := n + 1
		status := RoundPending
		if number == 1 {
			status = RoundCurrent
		}
		rounds[n] = Round{
			ID:          fmt.Sprintf("%s-r%d", i.ID, number),
			IqubID:      i.ID,
			Number:      number,
			DueDate:     i.dueDate(number),
			Status:      status,
			RecipientID: recipient,
		}
	}
	return rounds, nil
}

// Complete marks an ACTIVE group as COMPLETED (after the last round).
func (i *Iqub) Complete() error {
	if i.Status != StatusActive {
		return conflict("only an active group can be completed")
	}
	i.Status = StatusCompleted
	return nil
}

// dueDate implements SRS BR-10: start date plus n periods.
func (i *Iqub) dueDate(n int) time.Time {
	switch i.Frequency {
	case Weekly:
		return i.StartedAt.AddDate(0, 0, 7*n)
	case Biweekly:
		return i.StartedAt.AddDate(0, 0, 14*n)
	default:
		return i.StartedAt.AddDate(0, n, 0)
	}
}

func isPermutation(order, members []string) bool {
	if len(order) != len(members) {
		return false
	}
	counts := make(map[string]int, len(members))
	for _, m := range members {
		counts[m]++
	}
	for _, o := range order {
		if counts[o] == 0 {
			return false
		}
		counts[o]--
	}
	return true
}

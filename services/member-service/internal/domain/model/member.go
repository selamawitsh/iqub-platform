package model

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MinFullNameLen = 2
	MaxFullNameLen = 100
)

// Member is a registered person who can belong to iqub groups.
type Member struct {
	id           string
	fullName     string
	phone        Phone
	passwordHash string
	createdAt    time.Time
}

// NewMember creates a brand-new member. The id and time come from the caller
// so the domain stays free of randomness and clocks.
func NewMember(id, fullName string, phone Phone, passwordHash string, now time.Time) (*Member, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrInvalidID
	}
	name, err := normalizeFullName(fullName)
	if err != nil {
		return nil, err
	}
	if phone == "" {
		return nil, ErrInvalidPhone
	}
	if passwordHash == "" {
		return nil, ErrInvalidPasswordHash
	}

	return &Member{
		id:           id,
		fullName:     name,
		phone:        phone,
		passwordHash: passwordHash,
		createdAt:    now.UTC(),
	}, nil
}

// Restore rebuilds a member from stored data (used by repositories).
// It trusts the data because it was validated when first created.
func Restore(id, fullName string, phone Phone, passwordHash string, createdAt time.Time) *Member {
	return &Member{
		id:           id,
		fullName:     fullName,
		phone:        phone,
		passwordHash: passwordHash,
		createdAt:    createdAt,
	}
}

// ChangeFullName updates the name (FR-M5).
func (m *Member) ChangeFullName(fullName string) error {
	name, err := normalizeFullName(fullName)
	if err != nil {
		return err
	}
	m.fullName = name
	return nil
}

// ChangePasswordHash replaces the stored hash (FR-M5).
func (m *Member) ChangePasswordHash(passwordHash string) error {
	if passwordHash == "" {
		return ErrInvalidPasswordHash
	}
	m.passwordHash = passwordHash
	return nil
}

func (m *Member) ID() string           { return m.id }
func (m *Member) FullName() string     { return m.fullName }
func (m *Member) Phone() Phone         { return m.phone }
func (m *Member) PasswordHash() string { return m.passwordHash }
func (m *Member) CreatedAt() time.Time { return m.createdAt }

// normalizeFullName collapses repeated whitespace and checks the length in
// characters (not bytes), so Amharic names are counted correctly.
func normalizeFullName(s string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	n := utf8.RuneCountInString(s)
	if n < MinFullNameLen || n > MaxFullNameLen {
		return "", ErrInvalidFullName
	}
	return s, nil
}

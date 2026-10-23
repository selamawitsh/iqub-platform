package model

import "time"

type LedgerType string

const (
	LedgerContribution LedgerType = "CONTRIBUTION"
	LedgerPayout       LedgerType = "PAYOUT"
	LedgerReversal     LedgerType = "REVERSAL"
)

// LedgerEntry is an append-only record of money moving (SRS BR-7).
// Entries are never updated or deleted. Corrections are new REVERSAL entries.
type LedgerEntry struct {
	ID        string
	IqubID    string
	RoundID   string
	MemberID  string
	Type      LedgerType
	Amount    int64
	CreatedAt time.Time
}

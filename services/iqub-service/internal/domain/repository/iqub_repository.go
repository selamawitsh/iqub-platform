package repository

import (
	"context"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
)

// ConfirmationResult is everything that changes when a contribution is
// confirmed. It is saved as one unit (see SaveConfirmation).
type ConfirmationResult struct {
	Contribution   model.Contribution
	CompletedRound *model.Round // set when this confirmation completed the round
	NextRound      *model.Round // set when another round becomes current
	Iqub           *model.Iqub  // set when the last round completed the group
	LedgerEntries  []model.LedgerEntry
}

// IqubRepository is the storage contract the domain needs. It is owned by
// the domain layer. Outer layers (memory, postgres) implement it, so the
// dependency points inward, never outward.
//
// Find methods return model.ErrNotFound when nothing matches.
type IqubRepository interface {
	// Save inserts or updates a group.
	Save(ctx context.Context, iqub model.Iqub) error
	FindByID(ctx context.Context, id string) (model.Iqub, error)
	FindByInviteCode(ctx context.Context, code string) (model.Iqub, error)
	ListByMember(ctx context.Context, memberID string) ([]model.Iqub, error)

	// SaveStart stores the started group and its rounds as one unit.
	SaveStart(ctx context.Context, iqub model.Iqub, rounds []model.Round) error
	// ListRounds returns the group's rounds ordered by Number.
	ListRounds(ctx context.Context, iqubID string) ([]model.Round, error)

	// SaveContribution inserts or updates a contribution. It must return
	// model.ErrConflict if the member already has a different contribution
	// in the same round (SRS FR-I7, NFR-R2).
	SaveContribution(ctx context.Context, c model.Contribution) error
	FindContribution(ctx context.Context, id string) (model.Contribution, error)
	ListContributions(ctx context.Context, roundID string) ([]model.Contribution, error)

	// SaveConfirmation stores a confirmation and all its effects as one
	// unit: everything is saved or nothing is (SRS NFR-R1).
	SaveConfirmation(ctx context.Context, r ConfirmationResult) error

	// ListLedger returns the group's ledger entries, oldest first.
	ListLedger(ctx context.Context, iqubID string) ([]model.LedgerEntry, error)
}

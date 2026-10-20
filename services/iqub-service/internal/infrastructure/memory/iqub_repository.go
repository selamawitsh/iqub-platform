package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/repository"
)

// IqubRepository keeps everything in memory. It is used for milestone M1
// and in tests. One lock protects all data, so each method is all-or-nothing.
type IqubRepository struct {
	mu            sync.RWMutex
	order         []string // group IDs in creation order
	iqubs         map[string]model.Iqub
	rounds        map[string][]model.Round // by group ID, ordered by number
	contributions map[string]model.Contribution
	ledger        map[string][]model.LedgerEntry // by group ID
}

// Compile-time check that this type satisfies the domain's contract.
var _ repository.IqubRepository = (*IqubRepository)(nil)

func NewIqubRepository() *IqubRepository {
	return &IqubRepository{
		iqubs:         make(map[string]model.Iqub),
		rounds:        make(map[string][]model.Round),
		contributions: make(map[string]model.Contribution),
		ledger:        make(map[string][]model.LedgerEntry),
	}
}

func (r *IqubRepository) Save(_ context.Context, iqub model.Iqub) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store(iqub)
	return nil
}

func (r *IqubRepository) FindByID(_ context.Context, id string) (model.Iqub, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	iqub, ok := r.iqubs[id]
	if !ok {
		return model.Iqub{}, model.ErrNotFound
	}
	return clone(iqub), nil
}

func (r *IqubRepository) FindByInviteCode(_ context.Context, code string) (model.Iqub, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, id := range r.order {
		if r.iqubs[id].InviteCode == code {
			return clone(r.iqubs[id]), nil
		}
	}
	return model.Iqub{}, model.ErrNotFound
}

func (r *IqubRepository) ListByMember(_ context.Context, memberID string) ([]model.Iqub, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []model.Iqub
	for _, id := range r.order {
		iqub := r.iqubs[id]
		if iqub.IsMember(memberID) {
			out = append(out, clone(iqub))
		}
	}
	return out, nil
}

func (r *IqubRepository) SaveStart(_ context.Context, iqub model.Iqub, rounds []model.Round) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store(iqub)
	stored := append([]model.Round(nil), rounds...)
	sort.Slice(stored, func(a, b int) bool { return stored[a].Number < stored[b].Number })
	r.rounds[iqub.ID] = stored
	return nil
}

func (r *IqubRepository) ListRounds(_ context.Context, iqubID string) ([]model.Round, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.Round(nil), r.rounds[iqubID]...), nil
}

func (r *IqubRepository) SaveContribution(_ context.Context, c model.Contribution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.contributions {
		if existing.RoundID == c.RoundID && existing.MemberID == c.MemberID && existing.ID != c.ID {
			return fmt.Errorf("%w: contribution already recorded for this round", model.ErrConflict)
		}
	}
	r.contributions[c.ID] = c
	return nil
}

func (r *IqubRepository) FindContribution(_ context.Context, id string) (model.Contribution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.contributions[id]
	if !ok {
		return model.Contribution{}, model.ErrNotFound
	}
	return c, nil
}

func (r *IqubRepository) ListContributions(_ context.Context, roundID string) ([]model.Contribution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []model.Contribution
	for _, c := range r.contributions {
		if c.RoundID == roundID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].RecordedAt.Equal(out[b].RecordedAt) {
			return out[a].ID < out[b].ID
		}
		return out[a].RecordedAt.Before(out[b].RecordedAt)
	})
	return out, nil
}

// SaveConfirmation applies every effect of a confirmation under one lock.
func (r *IqubRepository) SaveConfirmation(_ context.Context, res repository.ConfirmationResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.contributions[res.Contribution.ID] = res.Contribution
	if res.CompletedRound != nil {
		replaceRound(r.rounds[res.CompletedRound.IqubID], *res.CompletedRound)
	}
	if res.NextRound != nil {
		replaceRound(r.rounds[res.NextRound.IqubID], *res.NextRound)
	}
	if res.Iqub != nil {
		r.store(*res.Iqub)
	}
	for _, entry := range res.LedgerEntries {
		r.ledger[entry.IqubID] = append(r.ledger[entry.IqubID], entry)
	}
	return nil
}

func (r *IqubRepository) ListLedger(_ context.Context, iqubID string) ([]model.LedgerEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]model.LedgerEntry(nil), r.ledger[iqubID]...), nil
}

// store inserts or updates a group. The caller must hold the write lock.
func (r *IqubRepository) store(iqub model.Iqub) {
	if _, exists := r.iqubs[iqub.ID]; !exists {
		r.order = append(r.order, iqub.ID)
	}
	r.iqubs[iqub.ID] = clone(iqub)
}

// clone copies the member list so callers can never change stored data.
func clone(iqub model.Iqub) model.Iqub {
	iqub.MemberIDs = append([]string(nil), iqub.MemberIDs...)
	return iqub
}

func replaceRound(rounds []model.Round, updated model.Round) {
	for k := range rounds {
		if rounds[k].ID == updated.ID {
			rounds[k] = updated
			return
		}
	}
}

package memory

import (
	"context"
	"sync"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/repository"
)

// Compile-time check: this type must satisfy the domain's contract.
var _ repository.IqubRepository = (*IqubRepository)(nil)

// IqubRepository keeps iqubs in memory. Data is lost on restart.
// The mutex is needed because HTTP requests run concurrently.
type IqubRepository struct {
	mu    sync.RWMutex
	items map[string]model.Iqub
	order []string // insertion order, so listings are stable
}

func NewIqubRepository() *IqubRepository {
	return &IqubRepository{items: make(map[string]model.Iqub)}
}

func (r *IqubRepository) Save(_ context.Context, iqub model.Iqub) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[iqub.ID]; !exists {
		r.order = append(r.order, iqub.ID)
	}
	r.items[iqub.ID] = clone(iqub)
	return nil
}

func (r *IqubRepository) ListByMember(_ context.Context, memberID string) ([]model.Iqub, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.Iqub, 0)
	for _, id := range r.order {
		iqub := r.items[id]
		for _, m := range iqub.MemberIDs {
			if m == memberID {
				out = append(out, clone(iqub))
				break
			}
		}
	}
	return out, nil
}

// clone copies the slice inside, so callers can never change stored data
// by modifying what they received.
func clone(i model.Iqub) model.Iqub {
	i.MemberIDs = append([]string(nil), i.MemberIDs...)
	return i
}

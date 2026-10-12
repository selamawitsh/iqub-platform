package repository

import (
	"context"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
)

// IqubRepository is the storage contract the domain needs. It is owned by
// the domain layer. Outer layers (memory, postgres) implement it, so the
// dependency points inward, never outward.
type IqubRepository interface {
	Save(ctx context.Context, iqub model.Iqub) error
	ListByMember(ctx context.Context, memberID string) ([]model.Iqub, error)
}

package repository

import (
	"context"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/domain/model"
)

// MemberRepository is the storage port for members.
//
// Implementations must return:
//   - model.ErrPhoneTaken from Save when the phone is already registered
//   - model.ErrMemberNotFound from the Find methods when nothing matches
type MemberRepository interface {
	Save(ctx context.Context, m *model.Member) error
	FindByID(ctx context.Context, id string) (*model.Member, error)
	FindByPhone(ctx context.Context, phone model.Phone) (*model.Member, error)
}

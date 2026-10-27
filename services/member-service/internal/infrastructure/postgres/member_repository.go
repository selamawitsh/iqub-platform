package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/domain/repository"
)

const (
	pgUniqueViolation = "23505"
	phoneConstraint   = "members_phone_key"
)

// MemberRepository stores members in PostgreSQL.
type MemberRepository struct {
	pool *pgxpool.Pool
}

var _ repository.MemberRepository = (*MemberRepository)(nil)

func NewMemberRepository(pool *pgxpool.Pool) *MemberRepository {
	return &MemberRepository{pool: pool}
}

// Save inserts a new member. A duplicate phone is reported as
// model.ErrPhoneTaken; the unique constraint is the source of truth.
func (r *MemberRepository) Save(ctx context.Context, m *model.Member) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO members (id, full_name, phone, password_hash, created_at)
		VALUES ($1::uuid, $2, $3, $4, $5)`,
		m.ID(), m.FullName(), m.Phone().String(), m.PasswordHash(), m.CreatedAt(),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == phoneConstraint {
			return model.ErrPhoneTaken
		}
		return err
	}
	return nil
}

func (r *MemberRepository) FindByID(ctx context.Context, id string) (*model.Member, error) {
	// A malformed id cannot exist in a UUID column, so it is simply "not found".
	if _, err := uuid.Parse(id); err != nil {
		return nil, model.ErrMemberNotFound
	}
	return r.findOne(ctx, `WHERE id = $1::uuid`, id)
}

func (r *MemberRepository) FindByPhone(ctx context.Context, phone model.Phone) (*model.Member, error) {
	return r.findOne(ctx, `WHERE phone = $1`, phone.String())
}

func (r *MemberRepository) findOne(ctx context.Context, where string, arg any) (*model.Member, error) {
	var (
		id, fullName, phone, hash string
		createdAt                 time.Time
	)
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, full_name, phone, password_hash, created_at FROM members `+where, arg,
	).Scan(&id, &fullName, &phone, &hash, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrMemberNotFound
		}
		return nil, err
	}
	return model.Restore(id, fullName, model.Phone(phone), hash, createdAt.UTC()), nil
}

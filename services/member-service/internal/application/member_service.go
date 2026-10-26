package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/member-service/internal/domain/repository"
)

type RegisterInput struct {
	FullName string
	Phone    string
	Password string
}

type LoginInput struct {
	Phone    string
	Password string
}

type LoginResult struct {
	AccessToken string
	ExpiresAt   time.Time
}

// MemberService holds the member use cases: register, login, view profile.
type MemberService struct {
	repo      repository.MemberRepository
	hasher    PasswordHasher
	tokens    TokenIssuer
	ids       IDGenerator
	clock     Clock
	publisher EventPublisher
	logger    *slog.Logger
}

func NewMemberService(
	repo repository.MemberRepository,
	hasher PasswordHasher,
	tokens TokenIssuer,
	ids IDGenerator,
	clock Clock,
	publisher EventPublisher,
	logger *slog.Logger,
) *MemberService {
	return &MemberService{
		repo:      repo,
		hasher:    hasher,
		tokens:    tokens,
		ids:       ids,
		clock:     clock,
		publisher: publisher,
		logger:    logger,
	}
}

// Register creates a member (FR-M1) and publishes member.registered (FR-M4).
func (s *MemberService) Register(ctx context.Context, in RegisterInput) (*model.Member, error) {
	phone, err := model.NewPhone(in.Phone)
	if err != nil {
		return nil, err
	}
	if err := model.ValidatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}

	m, err := model.NewMember(s.ids.NewID(), in.FullName, phone, hash, s.clock.Now())
	if err != nil {
		return nil, err
	}

	// The database unique constraint is the source of truth for phone
	// uniqueness, so there is no "exists?" check before saving.
	if err := s.repo.Save(ctx, m); err != nil {
		return nil, err
	}

	s.publishRegistered(ctx, m)
	return m, nil
}

// Login verifies the phone and password and returns an access token (FR-M2).
// Every credential failure returns ErrInvalidCredentials so the API does not
// reveal which phone numbers are registered.
func (s *MemberService) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	phone, err := model.NewPhone(in.Phone)
	if err != nil {
		return nil, model.ErrInvalidCredentials
	}

	m, err := s.repo.FindByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, model.ErrMemberNotFound) {
			return nil, model.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := s.hasher.Compare(m.PasswordHash(), in.Password); err != nil {
		return nil, model.ErrInvalidCredentials
	}

	token, expiresAt, err := s.tokens.Issue(m.ID(), s.clock.Now())
	if err != nil {
		return nil, err
	}

	return &LoginResult{AccessToken: token, ExpiresAt: expiresAt}, nil
}

// GetProfile returns the member with the given id (FR-M3).
func (s *MemberService) GetProfile(ctx context.Context, id string) (*model.Member, error) {
	return s.repo.FindByID(ctx, id)
}

// publishRegistered publishes after the save has committed. A failure is
// logged but does not fail the registration (ADR-005, known v1 limitation).
func (s *MemberService) publishRegistered(ctx context.Context, m *model.Member) {
	now := s.clock.Now()
	event := Event{
		EventID:    s.ids.NewID(),
		EventType:  EventMemberRegistered,
		OccurredAt: now.UTC(),
		Version:    EventVersion,
		Data: MemberRegisteredData{
			MemberID: m.ID(),
			FullName: m.FullName(),
			Phone:    m.Phone().String(),
		},
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		s.logger.Error("failed to publish event",
			slog.String("event_type", event.EventType),
			slog.String("event_id", event.EventID),
			slog.String("member_id", m.ID()),
			slog.String("error", err.Error()),
		)
	}
}

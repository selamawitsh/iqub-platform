package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/repository"
)

type CreateIqubCommand struct {
	AdminID            string
	Name               string
	ContributionAmount int64
	Frequency          model.Frequency
	MaxMembers         int
}

type JoinIqubCommand struct {
	MemberID   string
	InviteCode string
}

type StartIqubCommand struct {
	IqubID   string
	CallerID string
}

// IqubService holds the group use cases. It depends on the repository
// contract from the domain layer, never on a concrete database.
type IqubService struct {
	repo          repository.IqubRepository
	now           func() time.Time
	newID         func() string
	newInviteCode func() string
	shuffle       func([]string) []string
}

func NewIqubService(repo repository.IqubRepository) *IqubService {
	return &IqubService{
		repo:          repo,
		now:           time.Now,
		newID:         randomID,
		newInviteCode: randomInviteCode,
		shuffle:       shuffled,
	}
}

// 1. CreateIqub (FR-I1)
func (s *IqubService) CreateIqub(ctx context.Context, cmd CreateIqubCommand) (model.Iqub, error) {
	iqub, err := model.NewIqub(
		s.newID(), cmd.Name, cmd.ContributionAmount,
		cmd.Frequency, cmd.MaxMembers, cmd.AdminID, s.now(),
	)
	if err != nil {
		return model.Iqub{}, err
	}
	code, err := s.uniqueInviteCode(ctx)
	if err != nil {
		return model.Iqub{}, err
	}
	if err := iqub.SetInviteCode(code); err != nil {
		return model.Iqub{}, err
	}
	if err := s.repo.Save(ctx, iqub); err != nil {
		return model.Iqub{}, err
	}
	return iqub, nil
}

// 2. ListMyIqubs (FR-I2)
func (s *IqubService) ListMyIqubs(ctx context.Context, memberID string) ([]model.Iqub, error) {
	return s.repo.ListByMember(ctx, memberID)
}

// 3. GetIqub (FR-I2, FR-I10). A non-member gets ErrNotFound, so the
// existence of groups is not leaked.
func (s *IqubService) GetIqub(ctx context.Context, iqubID, callerID string) (model.Iqub, error) {
	return loadForMember(ctx, s.repo, iqubID, callerID)
}

// 4. JoinIqub (FR-I3)
func (s *IqubService) JoinIqub(ctx context.Context, cmd JoinIqubCommand) (model.Iqub, error) {
	code := strings.ToUpper(strings.TrimSpace(cmd.InviteCode))
	iqub, err := s.repo.FindByInviteCode(ctx, code)
	if err != nil {
		return model.Iqub{}, err
	}
	if err := iqub.AddMember(cmd.MemberID); err != nil {
		return model.Iqub{}, err
	}
	if err := s.repo.Save(ctx, iqub); err != nil {
		return model.Iqub{}, err
	}
	return iqub, nil
}

// 5. StartIqub (FR-I4). The payout order is drawn here, at random, and
// the domain checks and applies it.
func (s *IqubService) StartIqub(ctx context.Context, cmd StartIqubCommand) (model.Iqub, []model.Round, error) {
	iqub, err := loadForMember(ctx, s.repo, cmd.IqubID, cmd.CallerID)
	if err != nil {
		return model.Iqub{}, nil, err
	}
	order := s.shuffle(iqub.MemberIDs)
	rounds, err := iqub.Start(cmd.CallerID, order, s.now())
	if err != nil {
		return model.Iqub{}, nil, err
	}
	if err := s.repo.SaveStart(ctx, iqub, rounds); err != nil {
		return model.Iqub{}, nil, err
	}
	return iqub, rounds, nil
}

// 6. GetSchedule (FR-I5)
func (s *IqubService) GetSchedule(ctx context.Context, iqubID, callerID string) ([]model.Round, error) {
	if _, err := loadForMember(ctx, s.repo, iqubID, callerID); err != nil {
		return nil, err
	}
	return s.repo.ListRounds(ctx, iqubID)
}

// GetLedger (FR-I11, SHOULD)
func (s *IqubService) GetLedger(ctx context.Context, iqubID, callerID string) ([]model.LedgerEntry, error) {
	if _, err := loadForMember(ctx, s.repo, iqubID, callerID); err != nil {
		return nil, err
	}
	return s.repo.ListLedger(ctx, iqubID)
}

// uniqueInviteCode retries a few times if a generated code is already taken.
func (s *IqubService) uniqueInviteCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		code := s.newInviteCode()
		_, err := s.repo.FindByInviteCode(ctx, code)
		if errors.Is(err, model.ErrNotFound) {
			return code, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not generate a unique invite code")
}

// loadForMember loads a group and hides it from non-members (FR-I10).
func loadForMember(ctx context.Context, repo repository.IqubRepository, iqubID, callerID string) (model.Iqub, error) {
	iqub, err := repo.FindByID(ctx, iqubID)
	if err != nil {
		return model.Iqub{}, err
	}
	if !iqub.IsMember(callerID) {
		return model.Iqub{}, model.ErrNotFound
	}
	return iqub, nil
}

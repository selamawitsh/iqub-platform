package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// IqubService holds the use cases. It depends on the repository contract
// from the domain layer, never on a concrete database.
type IqubService struct {
	repo  repository.IqubRepository
	now   func() time.Time
	newID func() string
}

func NewIqubService(repo repository.IqubRepository) *IqubService {
	return &IqubService{repo: repo, now: time.Now, newID: randomID}
}

func (s *IqubService) CreateIqub(ctx context.Context, cmd CreateIqubCommand) (model.Iqub, error) {
	iqub, err := model.NewIqub(
		s.newID(), cmd.Name, cmd.ContributionAmount,
		cmd.Frequency, cmd.MaxMembers, cmd.AdminID, s.now(),
	)
	if err != nil {
		return model.Iqub{}, err
	}
	if err := s.repo.Save(ctx, iqub); err != nil {
		return model.Iqub{}, err
	}
	return iqub, nil
}

func (s *IqubService) ListMyIqubs(ctx context.Context, memberID string) ([]model.Iqub, error) {
	return s.repo.ListByMember(ctx, memberID)
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// The system random source failing is not recoverable.
		panic("cannot generate id: " + err.Error())
	}
	return hex.EncodeToString(b)
}

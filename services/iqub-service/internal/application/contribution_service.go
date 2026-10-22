package application

import (
	"context"
	"fmt"
	"time"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/repository"
)

type RecordContributionCommand struct {
	IqubID   string
	MemberID string
}

type ConfirmContributionCommand struct {
	IqubID         string
	ContributionID string
	CallerID       string
}

// ConfirmResult tells the caller what happened. A later milestone uses it
// to publish contribution.confirmed and round.completed events.
type ConfirmResult struct {
	Contribution   model.Contribution
	RoundCompleted bool
	RecipientID    string
	PayoutAmount   int64
	GroupCompleted bool
}

// ContributionService holds the use cases about paying into a round.
type ContributionService struct {
	repo  repository.IqubRepository
	now   func() time.Time
	newID func() string
}

func NewContributionService(repo repository.IqubRepository) *ContributionService {
	return &ContributionService{repo: repo, now: time.Now, newID: randomID}
}

// 7. RecordContribution (FR-I6, FR-I7). The amount is the group's amount,
// never taken from the request (BR-12).
func (s *ContributionService) RecordContribution(ctx context.Context, cmd RecordContributionCommand) (model.Contribution, error) {
	iqub, err := loadForMember(ctx, s.repo, cmd.IqubID, cmd.MemberID)
	if err != nil {
		return model.Contribution{}, err
	}
	if err := iqub.EnsureActive(); err != nil {
		return model.Contribution{}, err
	}

	rounds, err := s.repo.ListRounds(ctx, iqub.ID)
	if err != nil {
		return model.Contribution{}, err
	}
	cur := currentIndex(rounds)
	if cur < 0 {
		return model.Contribution{}, fmt.Errorf("%w: no current round", model.ErrConflict)
	}
	round := rounds[cur]

	existing, err := s.repo.ListContributions(ctx, round.ID)
	if err != nil {
		return model.Contribution{}, err
	}
	if model.HasContributed(existing, cmd.MemberID) {
		return model.Contribution{}, fmt.Errorf("%w: contribution already recorded for this round", model.ErrConflict)
	}

	contribution, err := model.NewContribution(
		s.newID(), iqub.ID, round.ID, cmd.MemberID, iqub.ContributionAmount, s.now(),
	)
	if err != nil {
		return model.Contribution{}, err
	}
	// The repository also rejects a duplicate, which protects against two
	// requests arriving at the same moment.
	if err := s.repo.SaveContribution(ctx, contribution); err != nil {
		return model.Contribution{}, err
	}
	return contribution, nil
}

// 8. ConfirmContribution (FR-I6, FR-I8, FR-I9). The admin confirms. If that
// was the last missing contribution, the round completes, the payout is
// recorded, and the next round starts (or the group completes). All changes
// are saved together by SaveConfirmation.
func (s *ContributionService) ConfirmContribution(ctx context.Context, cmd ConfirmContributionCommand) (ConfirmResult, error) {
	iqub, err := loadForMember(ctx, s.repo, cmd.IqubID, cmd.CallerID)
	if err != nil {
		return ConfirmResult{}, err
	}
	if err := iqub.EnsureAdmin(cmd.CallerID); err != nil {
		return ConfirmResult{}, err
	}
	if err := iqub.EnsureActive(); err != nil {
		return ConfirmResult{}, err
	}

	rounds, err := s.repo.ListRounds(ctx, iqub.ID)
	if err != nil {
		return ConfirmResult{}, err
	}
	cur := currentIndex(rounds)
	if cur < 0 {
		return ConfirmResult{}, fmt.Errorf("%w: no current round", model.ErrConflict)
	}
	round := rounds[cur]

	contribution, err := s.repo.FindContribution(ctx, cmd.ContributionID)
	if err != nil {
		return ConfirmResult{}, err
	}
	if contribution.IqubID != iqub.ID {
		return ConfirmResult{}, model.ErrNotFound
	}
	if contribution.RoundID != round.ID {
		return ConfirmResult{}, fmt.Errorf("%w: contribution is not for the current round", model.ErrConflict)
	}

	now := s.now()
	if err := contribution.Confirm(now); err != nil {
		return ConfirmResult{}, err
	}

	result := repository.ConfirmationResult{
		Contribution: contribution,
		LedgerEntries: []model.LedgerEntry{{
			ID:        s.newID(),
			IqubID:    iqub.ID,
			RoundID:   round.ID,
			MemberID:  contribution.MemberID,
			Type:      model.LedgerContribution,
			Amount:    contribution.Amount,
			CreatedAt: now,
		}},
	}
	out := ConfirmResult{Contribution: contribution}

	all, err := s.repo.ListContributions(ctx, round.ID)
	if err != nil {
		return ConfirmResult{}, err
	}
	for k := range all {
		if all[k].ID == contribution.ID {
			all[k] = contribution // use the freshly confirmed version
		}
	}

	if model.AllConfirmed(iqub.MemberIDs, all) {
		if err := round.Complete(); err != nil {
			return ConfirmResult{}, err
		}
		payout := iqub.PayoutAmount()
		result.CompletedRound = &round
		result.LedgerEntries = append(result.LedgerEntries, model.LedgerEntry{
			ID:        s.newID(),
			IqubID:    iqub.ID,
			RoundID:   round.ID,
			MemberID:  round.RecipientID,
			Type:      model.LedgerPayout,
			Amount:    payout,
			CreatedAt: now,
		})
		out.RoundCompleted = true
		out.RecipientID = round.RecipientID
		out.PayoutAmount = payout

		if cur+1 < len(rounds) {
			next := rounds[cur+1]
			if err := next.Activate(); err != nil {
				return ConfirmResult{}, err
			}
			result.NextRound = &next
		} else {
			if err := iqub.Complete(); err != nil {
				return ConfirmResult{}, err
			}
			result.Iqub = &iqub
			out.GroupCompleted = true
		}
	}

	if err := s.repo.SaveConfirmation(ctx, result); err != nil {
		return ConfirmResult{}, err
	}
	return out, nil
}

// currentIndex returns the position of the CURRENT round, or -1.
func currentIndex(rounds []model.Round) int {
	for k, r := range rounds {
		if r.Status == model.RoundCurrent {
			return k
		}
	}
	return -1
}

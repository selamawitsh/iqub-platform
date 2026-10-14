package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/repository"
)

// fakeRepo is a hand-written stand-in for the repository contract.
// No database needed.
type fakeRepo struct {
	saved []model.Iqub
}

func (f *fakeRepo) Save(_ context.Context, i model.Iqub) error {
	f.saved = append(f.saved, i)
	return nil
}

func (f *fakeRepo) ListByMember(_ context.Context, memberID string) ([]model.Iqub, error) {
	var out []model.Iqub
	for _, i := range f.saved {
		for _, m := range i.MemberIDs {
			if m == memberID {
				out = append(out, i)
				break
			}
		}
	}
	return out, nil
}

func newTestService(repo repository.IqubRepository) *IqubService {
	svc := NewIqubService(repo)
	svc.now = func() time.Time { return time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC) }
	return svc
}

func validCmd(admin string) CreateIqubCommand {
	return CreateIqubCommand{
		AdminID: admin, Name: "Family Iqub", ContributionAmount: 50000,
		Frequency: model.Monthly, MaxMembers: 10,
	}
}

func TestCreateIqubSavesAndMakesCreatorAdminAndMember(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo)

	got, err := svc.CreateIqub(context.Background(), validCmd("m1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == "" {
		t.Error("expected a generated id")
	}
	if got.AdminID != "m1" || len(got.MemberIDs) != 1 || got.MemberIDs[0] != "m1" {
		t.Errorf("creator must be admin and first member, got %+v", got)
	}
	if len(repo.saved) != 1 {
		t.Errorf("expected 1 saved iqub, got %d", len(repo.saved))
	}
}

func TestCreateIqubRejectsInvalidInputAndSavesNothing(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo)

	cmd := validCmd("m1")
	cmd.ContributionAmount = 0

	_, err := svc.CreateIqub(context.Background(), cmd)
	if !errors.Is(err, model.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if len(repo.saved) != 0 {
		t.Errorf("nothing should be saved, got %d", len(repo.saved))
	}
}

func TestListMyIqubsReturnsOnlyOwnGroups(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo)
	ctx := context.Background()

	if _, err := svc.CreateIqub(ctx, validCmd("m1")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateIqub(ctx, validCmd("m2")); err != nil {
		t.Fatal(err)
	}

	got, err := svc.ListMyIqubs(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AdminID != "m1" {
		t.Errorf("expected only m1's group, got %+v", got)
	}
}

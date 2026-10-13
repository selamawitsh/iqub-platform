package memory

import (
	"context"
	"testing"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
)

func TestListByMemberFiltersAndNeverReturnsNil(t *testing.T) {
	repo := NewIqubRepository()
	ctx := context.Background()

	_ = repo.Save(ctx, model.Iqub{ID: "1", MemberIDs: []string{"m1"}})
	_ = repo.Save(ctx, model.Iqub{ID: "2", MemberIDs: []string{"m2"}})

	got, _ := repo.ListByMember(ctx, "m1")
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("expected only iqub 1, got %+v", got)
	}

	none, _ := repo.ListByMember(ctx, "nobody")
	if none == nil || len(none) != 0 {
		t.Fatalf("expected an empty non-nil slice, got %#v", none)
	}
}

func TestStoredDataCannotBeChangedFromOutside(t *testing.T) {
	repo := NewIqubRepository()
	ctx := context.Background()

	_ = repo.Save(ctx, model.Iqub{ID: "1", MemberIDs: []string{"m1"}})

	got, _ := repo.ListByMember(ctx, "m1")
	got[0].MemberIDs[0] = "hacked"

	again, _ := repo.ListByMember(ctx, "m1")
	if len(again) != 1 {
		t.Fatal("stored data was changed through a returned slice")
	}
}

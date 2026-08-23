package service

import (
	"context"
	"errors"
	"testing"

	"task165-collation/internal/model"
)

// seedAnchorProject builds a minimal project with one base passage and one
// witness passage so a candidate anchor can be proposed between them.
func seedAnchorProject(t *testing.T, svc *Service) (*model.Project, *model.Witness, *model.Passage, *model.Passage) {
	t.Helper()
	ctx := context.Background()
	p, err := svc.CreateProject(ctx, "锚点释放测试", "")
	if err != nil {
		t.Fatal(err)
	}
	base, err := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if err != nil {
		t.Fatal(err)
	}
	wit, err := svc.CreateWitness(ctx, p.ID, "W", "见证本", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, base.ID, "学而时习之，不亦说乎？"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit.ID, "学而时习之，不亦悦乎？"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)
	witPassages, _ := svc.ListPassages(ctx, wit.ID)
	return p, wit, basePassages[0], witPassages[0]
}

// TestDeprecateReleasesPassage is the regression test for the reported bug:
// after an editor withdraws a candidate anchor, the base passage must be free
// again so a replacement anchor can be proposed for the same position.
func TestDeprecateReleasesPassage(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	p, wit, basePassage, witPassage := seedAnchorProject(t, svc)

	// Propose a candidate anchor, which occupies the base passage.
	a1, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "候选锚点")
	if err != nil {
		t.Fatalf("propose first anchor: %v", err)
	}
	// A second candidate on the same passage must be refused while occupied.
	if _, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "重复候选"); !errors.Is(err, model.ErrAnchorOccupied) {
		t.Fatalf("expected ErrAnchorOccupied before deprecate, got %v", err)
	}
	// Withdraw the candidate — this must release the passage.
	if err := svc.DeprecateAnchor(ctx, p.ID, a1.ID); err != nil {
		t.Fatalf("deprecate anchor: %v", err)
	}
	// The deprecated anchor must no longer hold the passage: a fresh anchor is
	// allowed for the same position.
	a2, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "替代锚点")
	if err != nil {
		t.Fatalf("propose replacement after deprecate: %v", err)
	}
	// The released slot is now taken by the new candidate.
	if _, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "再次重复"); !errors.Is(err, model.ErrAnchorOccupied) {
		t.Fatalf("expected ErrAnchorOccupied after replacement, got %v", err)
	}
	// The deprecated anchor is visible with the deprecated status.
	got, err := svc.store.GetAnchor(p.ID, a1.ID)
	if err != nil {
		t.Fatalf("reload deprecated anchor: %v", err)
	}
	if got.Status != model.AnchorDeprecated {
		t.Fatalf("expected deprecated status, got %s", got.Status)
	}
	if a2.ID == a1.ID {
		t.Fatal("replacement anchor must be a new record")
	}
}

// TestDeprecateIdOnly resolves the anchor from its id alone, mirroring the
// /api/anchors/{id}/deprecate route which carries no project id.
func TestDeprecateIdOnly(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	p, wit, basePassage, witPassage := seedAnchorProject(t, svc)

	a1, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "候选锚点")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	// Empty project id — the service resolves the owning project itself.
	if err := svc.DeprecateAnchor(ctx, "", a1.ID); err != nil {
		t.Fatalf("deprecate by id only: %v", err)
	}
	if _, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "替代锚点"); err != nil {
		t.Fatalf("expected replacement allowed after id-only deprecate, got %v", err)
	}
}

// TestDeprecateIdempotentAndConfirmedRejected checks the edge cases: deprecating
// an already-deprecated anchor is a no-op, and a confirmed anchor cannot be
// deprecated (it is an alignment boundary).
func TestDeprecateIdempotentAndConfirmedRejected(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	p, wit, basePassage, witPassage := seedAnchorProject(t, svc)

	a1, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "候选锚点")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if err := svc.DeprecateAnchor(ctx, p.ID, a1.ID); err != nil {
		t.Fatalf("deprecate: %v", err)
	}
	// Deprecating twice is idempotent — the passage stays released.
	if err := svc.DeprecateAnchor(ctx, p.ID, a1.ID); err != nil {
		t.Fatalf("second deprecate should be idempotent, got %v", err)
	}

	// A fresh candidate that we confirm must not be deprecatable.
	a2, err := svc.ProposeAnchor(ctx, p.ID, basePassage.ID, wit.ID, witPassage.ID, "确认锚点")
	if err != nil {
		t.Fatalf("propose second: %v", err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, a2.ID, a2.Version); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := svc.DeprecateAnchor(ctx, p.ID, a2.ID); !errors.Is(err, model.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState deprecating confirmed anchor, got %v", err)
	}
}

// TestDeprecateUnknownAnchor ensures a missing anchor surfaces as not-found
// rather than a silent success.
func TestDeprecateUnknownAnchor(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	if err := svc.DeprecateAnchor(ctx, "", "no-such-anchor"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

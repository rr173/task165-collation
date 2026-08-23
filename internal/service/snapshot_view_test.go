package service

import (
	"context"
	"testing"

	"task165-collation/internal/collate"
	"task165-collation/internal/model"
)

// TestPublishedSnapshotViewRetainsEvidence guards the read-only snapshot
// projection: when a reader opens an archived/published snapshot, the view
// must carry the complete frozen evidence set — confirmed anchors, approved
// decisions and the per-passage integrity hashes — so any character can be
// re-checked even after the source witnesses change. Regression for the bug
// where the projection truncated links to empty and dropped passage hashes.
func TestPublishedSnapshotViewRetainsEvidence(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "论语·学而校勘", "")
	if err != nil {
		t.Fatal(err)
	}
	base, err := svc.CreateWitness(ctx, p.ID, "B", "底本", "宋刻本", true)
	if err != nil {
		t.Fatal(err)
	}
	wit, err := svc.CreateWitness(ctx, p.ID, "W", "见证本", "明刊本", false)
	if err != nil {
		t.Fatal(err)
	}
	baseText := "学而时习之，不亦说乎？\n\n有朋自远方来，不亦乐乎？"
	witText := "学而时习之，不亦悦乎？\n\n有朋自远方来，不亦乐乎？"
	if _, err := svc.ImportPassages(ctx, base.ID, baseText); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit.ID, witText); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)
	witPassages, _ := svc.ListPassages(ctx, wit.ID)

	// Two confirmed anchors so the freeze set is non-empty.
	a1, err := svc.ProposeAnchor(ctx, p.ID, basePassages[0].ID, wit.ID, witPassages[0].ID, "锚点1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, a1.ID, 1); err != nil {
		t.Fatal(err)
	}
	a2, err := svc.ProposeAnchor(ctx, p.ID, basePassages[1].ID, wit.ID, witPassages[1].ID, "锚点2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, a2.ID, 1); err != nil {
		t.Fatal(err)
	}

	// One approved decision on a synthetic variant so the decision chain is
	// frozen into the snapshot.
	v := &model.Variant{
		ID:            NewID(),
		ProjectID:     p.ID,
		BasePassageID: basePassages[0].ID,
		BaseStartChar: 6,
		BaseEndChar:   7,
		DiffType:      model.DiffWord,
		Status:        model.VariantUnhandled,
		Version:       1,
	}
	if err := svc.Store().CreateVariants([]*model.Variant{v}); err != nil {
		t.Fatal(err)
	}
	reading, err := svc.ProposeReading(ctx, v.ID, wit.ID, "悦", model.DiffWord)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := svc.ProposeDecision(ctx, collate.DecideRequest{
		VariantID:       v.ID,
		ReadingID:       reading.ID,
		Reason:          "通假",
		DecidedBy:       "editor",
		ExpectedVersion: v.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewDecision(ctx, collate.ReviewRequest{
		DecisionID:      decision.ID,
		Reviewer:        "reviewer",
		Approve:         true,
		ExpectedVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}

	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}

	view, err := svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot view: %v", err)
	}
	if view == nil {
		t.Fatal("nil view")
	}

	// The frozen anchor set (both confirmed anchors) must survive projection.
	if view.AnchorCount != 2 {
		t.Errorf("anchor evidence lost: want 2, got %d", view.AnchorCount)
	}
	// The approved decision must survive projection.
	if view.DecisionCount != 1 {
		t.Errorf("decision evidence lost: want 1, got %d", view.DecisionCount)
	}
	// Per-passage hashes (the integrity backbone) must be retained — one per
	// base passage. They are the evidence that lets a reader re-check any
	// character after source witnesses are supplemented.
	if len(view.PassageHashes) != len(basePassages) {
		t.Errorf("passage-hash evidence lost: want %d, got %d", len(basePassages), len(view.PassageHashes))
	}
	for _, bp := range basePassages {
		if view.PassageHashes[bp.ID] == "" {
			t.Errorf("missing passage hash for %s in projection", bp.ID)
		}
		if view.PassageHashes[bp.ID] != bp.TextHash {
			t.Errorf("passage hash mismatch for %s: frozen chain altered", bp.ID)
		}
	}
	// Integrity must verify against the recomputed frozen hash.
	if !view.IntegrityOK {
		t.Error("integrity_ok false: projection lost verifiable evidence")
	}
}

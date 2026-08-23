package service

import (
	"context"
	"testing"

	"task165-collation/internal/collate"
	"task165-collation/internal/model"
)

// TestSnapshotDetailProjectionKeepsFrozenEvidence is a regression test for the
// report: "after generating a snapshot, the first query looks fine but the
// frozen evidence proving the body's origin vanishes from the detail
// projection." The snapshot must carry its frozen anchor, decision and
// passage-hash evidence through the whole read path so a published snapshot
// stays verifiable even after the source witnesses are supplemented.
func TestSnapshotDetailProjectionKeepsFrozenEvidence(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "论语校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	wit, _ := svc.CreateWitness(ctx, p.ID, "W", "见证本", "", false)
	if _, err := svc.ImportPassages(ctx, base.ID, "学而时习之，不亦说乎。"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit.ID, "学而时习之，不亦悦乎。"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)
	witPassages, _ := svc.ListPassages(ctx, wit.ID)
	anchor, err := svc.ProposeAnchor(ctx, p.ID, basePassages[0].ID, wit.ID, witPassages[0].ID, "锚点")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, anchor.ID, 1); err != nil {
		t.Fatal(err)
	}
	v := newSyntheticVariant(t, svc, p.ID, basePassages[0].ID, 6, 7)
	reading, err := svc.ProposeReading(ctx, v.ID, wit.ID, "悦", model.DiffWord)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := svc.ProposeDecision(ctx, decideReq(v.ID, reading.ID, wit.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewDecision(ctx, reviewReq(decision.ID)); err != nil {
		t.Fatal(err)
	}
	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}

	// The first query after generation: the detail projection must still carry
	// the frozen evidence — anchor count, decision count, and every base
	// passage hash that backs the body.
	view, err := svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if view.AnchorCount == 0 {
		t.Fatalf("frozen anchor evidence vanished from projection: anchor_count=%d", view.AnchorCount)
	}
	if view.DecisionCount == 0 {
		t.Fatalf("frozen decision evidence vanished from projection: decision_count=%d", view.DecisionCount)
	}
	if len(view.PassageHashes) != len(basePassages) {
		t.Fatalf("frozen passage-hash evidence vanished from projection: want %d, got %d (map=%v)",
			len(basePassages), len(view.PassageHashes), view.PassageHashes)
	}
	for _, bp := range basePassages {
		if view.PassageHashes[bp.ID] != bp.TextHash {
			t.Fatalf("passage %s hash mismatch: want %s, got %s", bp.ID, bp.TextHash, view.PassageHashes[bp.ID])
		}
	}
}

func newSyntheticVariant(t *testing.T, svc *Service, projectID, basePassageID string, start, end int) *model.Variant {
	t.Helper()
	v := &model.Variant{
		ID:            NewID(),
		ProjectID:     projectID,
		BasePassageID: basePassageID,
		BaseStartChar: start,
		BaseEndChar:   end,
		DiffType:      model.DiffWord,
		Status:        model.VariantUnhandled,
		Version:       1,
	}
	if err := svc.Store().CreateVariants([]*model.Variant{v}); err != nil {
		t.Fatalf("create variant: %v", err)
	}
	return v
}

func decideReq(variantID, readingID, evidenceWitnessID string) collate.DecideRequest {
	return collate.DecideRequest{
		VariantID:         variantID,
		ReadingID:         readingID,
		Reason:            "通假",
		DecidedBy:         "editor",
		EvidenceWitnessID: evidenceWitnessID,
		ExpectedVersion:   1,
	}
}

func reviewReq(decisionID string) collate.ReviewRequest {
	return collate.ReviewRequest{
		DecisionID:      decisionID,
		Reviewer:        "reviewer",
		Approve:         true,
		ExpectedVersion: 1,
	}
}

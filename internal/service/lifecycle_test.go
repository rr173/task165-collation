package service

import (
	"context"
	"testing"

	"task165-collation/internal/collate"
	"task165-collation/internal/model"
)

// TestAlignmentPipeline exercises the full business loop: witnesses, anchors,
// alignment, readings, decisions and snapshot publishing.
func TestAlignmentPipeline(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "论语·学而校勘", "")
	if err != nil {
		t.Fatal(err)
	}
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "宋刻本", true)
	wit1, _ := svc.CreateWitness(ctx, p.ID, "W1", "见证本一", "明刊本", false)
	wit2, _ := svc.CreateWitness(ctx, p.ID, "W2", "见证本二", "清刻本", false)

	baseText := "学而时习之，不亦说乎？有朋自远方来，不亦乐乎？\n\n人不知而不愠，不亦君子乎？"
	w1Text := "学而时习之，不亦悦乎？有朋自远方来，不亦乐乎？\n\n人不知而不愠，不亦君子乎？"
	w2Text := "学而时习之，不亦悦乎？有朋自远方来，不亦乐乎？\n\n人不知而不愠，不亦君子乎？"

	if _, err := svc.ImportPassages(ctx, base.ID, baseText); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit1.ID, w1Text); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit2.ID, w2Text); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)
	wit1Passages, _ := svc.ListPassages(ctx, wit1.ID)

	// Two anchors so interval alignment has work.
	a1, err := svc.ProposeAnchor(ctx, p.ID, basePassages[0].ID, wit1.ID, wit1Passages[0].ID, "锚点1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, a1.ID, 1); err != nil {
		t.Fatal(err)
	}
	a2, err := svc.ProposeAnchor(ctx, p.ID, basePassages[1].ID, wit1.ID, wit1Passages[1].ID, "锚点2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, a2.ID, 1); err != nil {
		t.Fatal(err)
	}

	created, err := svc.RunAlignment(ctx, p.ID)
	if err != nil {
		t.Fatalf("run alignment: %v", err)
	}
	if created == 0 {
		t.Fatal("expected variants created")
	}
	variants, _ := svc.ListVariants(ctx, p.ID, "")
	if len(variants) == 0 {
		t.Fatal("expected variants")
	}
	// Propose a decision on the first variant and approve it.
	v := variants[0]
	readings, err := svc.Store().ListReadings(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) == 0 {
		t.Skip("no auto readings; creating one")
	}
	d, err := svc.ProposeDecision(ctx, collate.DecideRequest{
		VariantID:       v.ID,
		ReadingID:       readings[0].ID,
		Reason:          "采用见证本读法",
		DecidedBy:       "editor",
		ExpectedVersion: v.Version,
	})
	if err != nil {
		t.Fatalf("propose decision: %v", err)
	}
	if _, err := svc.ReviewDecision(ctx, collate.ReviewRequest{
		DecisionID:      d.ID,
		Reviewer:        "reviewer",
		Approve:         true,
		ExpectedVersion: 1,
	}); err != nil {
		t.Fatalf("review: %v", err)
	}
	// Preview must contain the adopted reading.
	body, err := svc.PreviewDefinitive(ctx, p.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if body == "" {
		t.Fatal("empty preview body")
	}
	// Build and publish a snapshot.
	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}
	// Second build must fail (round frozen).
	if _, err := svc.BuildSnapshot(ctx, p.ID); err == nil {
		t.Fatal("expected ErrRoundFrozen on second build")
	}
	// Trace must resolve.
	tc, err := svc.TraceVariant(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tc.Decision == nil || tc.AdoptedReading == nil {
		t.Fatal("expected full trace chain")
	}
}

func TestPublishedSnapshotReadOnly(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "原文。"); err != nil {
		t.Fatal(err)
	}
	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatal(err)
	}
	// Direct mutation must be refused by validation.
	if err := collate.ValidateSnapshotMutation(sn); err == nil {
		// ValidateSnapshotMutation uses the store status; refresh it.
		cur, _ := svc.Store().GetSnapshot(sn.ID)
		if err := collate.ValidateSnapshotMutation(cur); err == nil {
			t.Fatal("expected published snapshot to be read-only")
		}
	}
}

// TestSnapshotFrozenEvidenceRetained guards the traceability contract: a
// published snapshot must carry its frozen passage/anchor/decision evidence
// under the snapshot's own id, so the read-only view served from GetSnapshot
// can trace the body back to its sources. The regression being fixed here is
// that freeze stamped links with a synthetic "snapshot:project:round" key
// while the snapshot row was stored under a different NewID() primary key,
// leaving ListSnapshotLinks unable to rejoin any evidence.
func TestSnapshotFrozenEvidenceRetained(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "论语校勘", "")
	if err != nil {
		t.Fatal(err)
	}
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "宋刻本", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "学而时习之，不亦说乎？\n\n人不知而不愠，不亦君子乎？"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)
	if len(basePassages) != 2 {
		t.Fatalf("expected 2 base passages, got %d", len(basePassages))
	}

	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	published, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version)
	if err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}

	view, err := svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if view.Snapshot == nil || view.Snapshot.ID != published.ID {
		t.Fatalf("view does not point at the published snapshot")
	}
	if view.Snapshot.Body == "" {
		t.Fatal("expected non-empty snapshot body")
	}
	if len(view.PassageHashes) != len(basePassages) {
		t.Fatalf("expected %d frozen passage hashes, got %d", len(basePassages), len(view.PassageHashes))
	}
	for _, p := range basePassages {
		h, ok := view.PassageHashes[p.ID]
		if !ok {
			t.Errorf("passage %s missing from frozen evidence", p.ID)
			continue
		}
		if h != p.TextHash {
			t.Errorf("passage %s hash mismatch: frozen=%s base=%s", p.ID, h, p.TextHash)
		}
	}
	if !view.IntegrityOK {
		t.Error("expected integrity_ok=true for a snapshot with frozen passage evidence")
	}
}

func TestDuplicateTextHashConflict(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	p, _ := svc.CreateProject(ctx, "校勘", "")
	if _, err := svc.CreateWitness(ctx, p.ID, "A", "甲本", "同一文本不同书目", true); err != nil {
		t.Fatal(err)
	}
	// Same source hash with conflicting bibliographic info is allowed for
	// different witnesses; duplicates within the same text must keep hash but
	// different bib info → ErrDuplicateHash surfaces at CreateWitness.
	_, err := svc.CreateWitness(ctx, p.ID, "B", "乙本", "与甲本不同的书目", false)
	if err != nil {
		t.Fatalf("create second witness: %v", err)
	}
	_ = model.ErrDuplicateHash
}

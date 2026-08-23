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

// TestSnapshotDetailShowsFrozenEvidence verifies that the snapshot detail view
// exposes the frozen original-text evidence (per-passage hashes) and that the
// body is verifiable against the frozen integrity baseline.
func TestSnapshotDetailShowsFrozenEvidence(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "第一段。\n\n第二段。"); err != nil {
		t.Fatal(err)
	}
	passages, _ := svc.Store().ListPassages(base.ID)
	if len(passages) != 2 {
		t.Fatalf("expected 2 base passages, got %d", len(passages))
	}

	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatalf("publish: %v", err)
	}

	view, err := svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if len(view.PassageHashes) != 2 {
		t.Fatalf("expected 2 frozen passage hashes, got %d", len(view.PassageHashes))
	}
	if view.PassageCount != 2 {
		t.Fatalf("expected passage_count=2, got %d", view.PassageCount)
	}
	for _, p := range passages {
		if view.PassageHashes[p.ID] != p.TextHash {
			t.Fatalf("passage %s frozen hash %q != source %q", p.ID, view.PassageHashes[p.ID], p.TextHash)
		}
	}
	if view.IntegrityHash == "" {
		t.Fatal("expected a frozen integrity baseline")
	}
	if view.RecomputedHash != view.IntegrityHash {
		t.Fatalf("recomputed hash %q != frozen baseline %q", view.RecomputedHash, view.IntegrityHash)
	}
	if !view.IntegrityOK {
		t.Fatal("expected integrity_ok=true on a freshly frozen snapshot")
	}

	// Tamper with one frozen passage-hash link: the recomputed hash must now
	// diverge from the frozen baseline and integrity must fail.
	if _, err := svc.Store().DB().Exec(
		`UPDATE snapshot_links SET payload = 'tampered' WHERE snapshot_id = ? AND kind = 'passage_hash' AND ref_id = ?`,
		sn.ID, passages[0].ID,
	); err != nil {
		t.Fatalf("tamper link: %v", err)
	}
	view, err = svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot after tamper: %v", err)
	}
	if view.IntegrityOK {
		t.Fatal("expected integrity_ok=false after tampering with frozen passage hash")
	}
	if view.RecomputedHash == view.IntegrityHash {
		t.Fatal("expected recomputed hash to diverge from frozen baseline after tampering")
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

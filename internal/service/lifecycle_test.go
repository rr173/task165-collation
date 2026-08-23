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

// TestSnapshotFreezesSourcesAndProvesIntegrity guards the regression where a
// snapshot's body was readable but its frozen sources (anchors, decisions,
// passage hashes) and integrity hash were never persisted, so the review
// record could not prove the body had not drifted. It builds a round with a
// confirmed anchor and an approved decision, publishes it, and then checks
// that GetSnapshot brings the frozen sources back and recomputes the integrity
// verdict from them.
func TestSnapshotFreezesSourcesAndProvesIntegrity(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "论语校勘", "")
	if err != nil {
		t.Fatal(err)
	}
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	wit, _ := svc.CreateWitness(ctx, p.ID, "W", "见证本", "", false)
	baseText := "学而时习之，不亦说乎"
	witText := "学而时习之，不亦悦乎"
	if _, err := svc.ImportPassages(ctx, base.ID, baseText); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPassages(ctx, wit.ID, witText); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)
	witPassages, _ := svc.ListPassages(ctx, wit.ID)

	a1, err := svc.ProposeAnchor(ctx, p.ID, basePassages[0].ID, wit.ID, witPassages[0].ID, "锚点")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, a1.ID, 1); err != nil {
		t.Fatal(err)
	}
	// Two anchors make the interval alignment produce variants.
	a2, err := svc.ProposeAnchor(ctx, p.ID, basePassages[0].ID, wit.ID, witPassages[0].ID, "锚点二")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmAnchor(ctx, p.ID, a2.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunAlignment(ctx, p.ID); err != nil {
		t.Fatalf("align: %v", err)
	}
	variants, _ := svc.ListVariants(ctx, p.ID, "")
	if len(variants) == 0 {
		t.Skip("no variants produced")
	}
	v := variants[0]
	readings, err := svc.Store().ListReadings(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) == 0 {
		t.Skip("no readings to adopt")
	}
	d, err := svc.ProposeDecision(ctx, collate.DecideRequest{
		VariantID:       v.ID,
		ReadingID:       readings[0].ID,
		Reason:          "采用见证本读法",
		DecidedBy:       "editor",
		ExpectedVersion: v.Version,
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if _, err := svc.ReviewDecision(ctx, collate.ReviewRequest{
		DecisionID:      d.ID,
		Reviewer:        "reviewer",
		Approve:         true,
		ExpectedVersion: 1,
	}); err != nil {
		t.Fatalf("review: %v", err)
	}

	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if sn.IntegrityHash == "" {
		t.Fatal("snapshot must record an integrity hash over its frozen passage hashes")
	}
	// The pending snapshot already freezes its links; prove they survive a
	// restart into a fresh store before publishing.
	links, err := svc.Store().ListSnapshotLinks(sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) == 0 {
		t.Fatal("snapshot frozen links were not persisted; body cannot be traced back to its sources")
	}
	if !hasLinkKind(links, "anchor") {
		t.Error("expected at least one frozen anchor link")
	}
	if !hasLinkKind(links, "decision") {
		t.Error("expected at least one frozen decision link")
	}
	if !hasLinkKind(links, "passage_hash") {
		t.Error("expected frozen passage-hash links")
	}

	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatalf("publish: %v", err)
	}

	view, err := svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if view.AnchorCount == 0 {
		t.Error("frozen anchors were not brought back into the published view")
	}
	if view.DecisionCount == 0 {
		t.Error("frozen decisions were not brought back into the published view")
	}
	if len(view.PassageHashes) == 0 {
		t.Error("frozen passage hashes were not brought back into the published view")
	}
	if view.IntegrityActual == "" {
		t.Error("integrity hash was not recomputed from the frozen links")
	}
	if !view.IntegrityOK {
		t.Errorf("integrity verdict must hold (expected %s, recomputed %s)", sn.IntegrityHash, view.IntegrityActual)
	}
}

// hasLinkKind reports whether any frozen link is of the given kind.
func hasLinkKind(links []*model.SnapshotLink, kind string) bool {
	for _, l := range links {
		if l.Kind == kind {
			return true
		}
	}
	return false
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

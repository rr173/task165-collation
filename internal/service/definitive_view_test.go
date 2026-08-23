package service

import (
	"context"
	"path/filepath"
	"testing"

	"task165-collation/internal/store"
)

// TestHistoricalSnapshotReturnsPassageHashes guards the regression where
// opening a published (historical) snapshot dropped the per-passage summary
// hash: the read path (ListSnapshotLinks → SummarizeView) returned empty
// PassageHashes, so reviewers could not tell whether the content had been
// correctly frozen. The frozen text hash of each base passage must survive the
// round-trip and the database reopen.
func TestHistoricalSnapshotReturnsPassageHashes(t *testing.T) {
	svc, dbPath := newTestService(t)
	ctx := context.Background()

	p, err := svc.CreateProject(ctx, "校勘", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	base, err := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if err != nil {
		t.Fatalf("create base witness: %v", err)
	}
	if _, err := svc.ImportPassages(ctx, base.ID, "原文一段。\n\n原文二段。"); err != nil {
		t.Fatalf("import passages: %v", err)
	}
	// Capture the frozen passage hashes before publishing so the read path can
	// be compared against the source of truth.
	passages, err := svc.ListPassages(ctx, base.ID)
	if err != nil {
		t.Fatalf("list passages: %v", err)
	}
	if len(passages) != 2 {
		t.Fatalf("expected 2 passages, got %d", len(passages))
	}
	want := make(map[string]string, len(passages))
	for _, p := range passages {
		if p.TextHash == "" {
			t.Fatalf("passage %s has empty text hash", p.ID)
		}
		want[p.ID] = p.TextHash
	}

	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}

	// Reviewer opens the historical snapshot in the same session.
	view, err := svc.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	assertPassageHashes(t, view.PassageHashes, want)

	// Reopen the database (the real "reviewer opens it later" path) and read
	// the frozen snapshot from a fresh store + service: the summary values
	// must still be present.
	if err := svc.store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	s2, err := store.Open(filepath.Join(filepath.Dir(dbPath), filepath.Base(dbPath)))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer s2.Close()
	svc2 := New(s2)
	view2, err := svc2.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot after reopen: %v", err)
	}
	assertPassageHashes(t, view2.PassageHashes, want)
}

func assertPassageHashes(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d passage hashes, got %d (%v)", len(want), len(got), got)
	}
	for pid, h := range want {
		got, ok := got[pid]
		if !ok {
			t.Errorf("passage %s missing from historical snapshot passage_hashes", pid)
			continue
		}
		if got == "" {
			t.Errorf("passage %s summary hash lost (empty) in historical snapshot", pid)
			continue
		}
		if got != h {
			t.Errorf("passage %s hash mismatch: got %s want %s", pid, got, h)
		}
	}
}

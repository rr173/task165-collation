package service

import (
	"context"
	"path/filepath"
	"testing"

	"task165-collation/internal/store"
)

// TestSnapshotRetainsPassageDigests guards the regression where a snapshot's
// passage-hash links came back empty on read: reviewers could not tell which
// frozen version a published body belonged to. The digest must survive the
// populate → freeze → persist → read path, including across a restart.
func TestSnapshotRetainsPassageDigests(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	svc := New(s1)

	ctx := context.Background()
	p, err := svc.CreateProject(ctx, "校勘·学而", "")
	if err != nil {
		t.Fatal(err)
	}
	base, err := svc.CreateWitness(ctx, p.ID, "B", "底本", "宋刻本", true)
	if err != nil {
		t.Fatal(err)
	}
	// Two paragraphs → two passages, each with its own text hash.
	if _, err := svc.ImportPassages(ctx, base.ID, "学而时习之，不亦说乎？\n\n有朋自远方来，不亦乐乎？"); err != nil {
		t.Fatal(err)
	}
	passages, err := svc.ListPassages(ctx, base.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(passages) != 2 {
		t.Fatalf("expected 2 base passages, got %d", len(passages))
	}

	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}

	// Reviewer read path: every frozen passage must surface a usable digest.
	checkView := func(t *testing.T, svc *Service) {
		t.Helper()
		view, err := svc.GetSnapshot(ctx, sn.ID)
		if err != nil {
			t.Fatalf("get snapshot: %v", err)
		}
		if len(view.PassageHashes) != len(passages) {
			t.Fatalf("expected %d passage digests, got %d", len(passages), len(view.PassageHashes))
		}
		for _, p := range passages {
			got, ok := view.PassageHashes[p.ID]
			if !ok {
				t.Fatalf("snapshot view missing digest for passage %s", p.ID)
			}
			if got == "" {
				t.Fatalf("snapshot view has empty digest for passage %s", p.ID)
			}
			if got != p.TextHash {
				t.Fatalf("passage %s digest mismatch: snapshot froze %q, witness has %q", p.ID, got, p.TextHash)
			}
		}
	}

	checkView(t, svc)

	// The read path must hold across a restart: the bug was in the SELECT
	// that blanked passage_hash payloads, so a fresh connection is the real
	// test that the digest round-trips through persistence.
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer s2.Close()
	checkView(t, New(s2))
}

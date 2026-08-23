package service

import (
	"context"
	"testing"

	"task165-collation/internal/store"
)

// openServiceAt reopens a service on an existing database file (for the
// restart-recovery test).
func openServiceAt(dbPath string) (*Service, error) {
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return New(s), nil
}

// TestSnapshotPassageEvidenceMap guards the read path of a saved snapshot:
// after BuildSnapshot + PublishSnapshot, GetSnapshot must surface the
// passage→hash evidence map frozen at publish time, so reviewers can trace the
// body back to its source passages. The bug being fixed was that the read path
// dropped the passage_hash links before they ever reached the API.
func TestSnapshotPassageEvidenceMap(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "第一段。\n\n第二段。"); err != nil {
		t.Fatal(err)
	}
	// Capture the base passage hashes frozen into the snapshot before publish.
	basePassages, err := svc.ListPassages(ctx, base.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(basePassages) != 2 {
		t.Fatalf("expected 2 base passages, got %d", len(basePassages))
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
		t.Fatalf("get snapshot: %v", err)
	}
	// The frozen evidence map must be present and non-empty.
	if view.PassageHashes == nil {
		t.Fatal("PassageHashes is nil: the saved snapshot exposes no passage evidence map")
	}
	if len(view.PassageHashes) != len(basePassages) {
		t.Fatalf("expected %d passage hashes, got %d", len(basePassages), len(view.PassageHashes))
	}
	// Each base passage id must map to the exact hash frozen at publish time.
	for _, p := range basePassages {
		got, ok := view.PassageHashes[p.ID]
		if !ok {
			t.Errorf("passage %s missing from evidence map", p.ID)
			continue
		}
		if got != p.TextHash {
			t.Errorf("passage %s: evidence hash mismatch, got %s want %s", p.ID, got, p.TextHash)
		}
	}
}

// TestSnapshotPassageEvidencePersistsAcrossRestart ensures the evidence map is
// recovered from the stored snapshot_links after the database is closed and
// reopened, not just held in memory.
func TestSnapshotPassageEvidencePersistsAcrossRestart(t *testing.T) {
	svc, dbPath := newTestService(t)
	ctx := context.Background()

	p, _ := svc.CreateProject(ctx, "校勘", "")
	base, _ := svc.CreateWitness(ctx, p.ID, "B", "底本", "", true)
	if _, err := svc.ImportPassages(ctx, base.ID, "原文。"); err != nil {
		t.Fatal(err)
	}
	basePassages, _ := svc.ListPassages(ctx, base.ID)

	sn, err := svc.BuildSnapshot(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishSnapshot(ctx, sn.ID, sn.Version); err != nil {
		t.Fatal(err)
	}

	// Drop the in-memory service and reopen the on-disk database.
	if err := svc.Store().Close(); err != nil {
		t.Fatal(err)
	}
	svc2, err := openServiceAt(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer svc2.Store().Close()

	view, err := svc2.GetSnapshot(ctx, sn.ID)
	if err != nil {
		t.Fatalf("get snapshot after reopen: %v", err)
	}
	if view.PassageHashes == nil || len(view.PassageHashes) != len(basePassages) {
		t.Fatalf("evidence map not recovered from disk: %v", view.PassageHashes)
	}
	if got := view.PassageHashes[basePassages[0].ID]; got != basePassages[0].TextHash {
		t.Fatalf("recovered evidence hash mismatch: got %s want %s", got, basePassages[0].TextHash)
	}
}

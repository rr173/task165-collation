package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"task165-collation/internal/model"

	_ "modernc.org/sqlite"
)

// openRawDB opens a SQLite file through the driver directly, bypassing the
// store migration, so a test can lay down a legacy schema.
func openRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	return db
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// TestOpenMigratesLegacySnapshots verifies that a database created before the
// integrity_hash column existed still opens and that the column is added, so
// the frozen-evidence read path keeps working across an upgrade.
func TestOpenMigratesLegacySnapshots(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	// Build a legacy snapshots table by hand without integrity_hash.
	db := openRawDB(t, dbPath)
	exec(t, db, `CREATE TABLE snapshots (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		round_no INTEGER NOT NULL,
		status TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		body TEXT NOT NULL DEFAULT '',
		version INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		published_at TEXT
	)`)
	exec(t, db, `INSERT INTO snapshots (id, project_id, round_no, status, title, body, version, created_at)
		VALUES ('sn1', 'p1', 1, 'published', 't', 'b', 1, '2024-01-01T00:00:00Z')`)
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	// Open through the store: migration must add integrity_hash.
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer s.Close()

	sn, err := s.GetSnapshot("sn1")
	if err != nil {
		t.Fatalf("get legacy snapshot: %v", err)
	}
	// Legacy rows default to empty integrity hash; they must still load.
	if sn.ID != "sn1" {
		t.Fatalf("unexpected snapshot id: %s", sn.ID)
	}

	// A fresh write must round-trip the integrity hash through the new column.
	fresh := &model.Snapshot{
		ID:            "sn2",
		ProjectID:     "p1",
		RoundNo:       2,
		Status:        model.SnapshotPublished,
		Title:         "t2",
		Body:          "b2",
		IntegrityHash: "deadbeef",
		Version:       1,
	}
	if err := s.CreateSnapshot(fresh, nil); err != nil {
		t.Fatalf("create fresh snapshot: %v", err)
	}
	got, err := s.GetSnapshot("sn2")
	if err != nil {
		t.Fatalf("get fresh snapshot: %v", err)
	}
	if got.IntegrityHash != "deadbeef" {
		t.Fatalf("integrity hash not round-tripped: want deadbeef, got %q", got.IntegrityHash)
	}
}

// TestListSnapshotLinksKeepsPassageHashEvidence is a regression test: the read
// path must return every frozen evidence kind, including passage_hash links,
// so the detail projection can prove the body's origin.
func TestListSnapshotLinksKeepsPassageHashEvidence(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "links.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	sn := &model.Snapshot{ID: "sn1", ProjectID: "p1", RoundNo: 1, Status: model.SnapshotPendingP, Version: 1}
	links := []*model.SnapshotLink{
		{SnapshotID: "sn1", Kind: "anchor", RefID: "anc1"},
		{SnapshotID: "sn1", Kind: "decision", RefID: "dec1", Payload: "read1"},
		{SnapshotID: "sn1", Kind: "passage_hash", RefID: "p1", Payload: "hash-p1"},
	}
	if err := s.CreateSnapshot(sn, links); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	got, err := s.ListSnapshotLinks("sn1")
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	var kinds []string
	for _, l := range got {
		kinds = append(kinds, l.Kind)
	}
	if len(got) != 3 {
		t.Fatalf("frozen evidence lost: want 3 links, got %d (%v)", len(got), kinds)
	}
	var sawPassageHash bool
	for _, l := range got {
		if l.Kind == "passage_hash" && l.RefID == "p1" && l.Payload == "hash-p1" {
			sawPassageHash = true
		}
	}
	if !sawPassageHash {
		t.Fatalf("passage_hash evidence dropped from ListSnapshotLinks: got %v", kinds)
	}
}

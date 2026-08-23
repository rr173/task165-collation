// Package store provides SQLite-backed persistence for the collation
// workbench. All entities live in one database file so the service can be
// restarted and resume in-flight alignment and editorial work. Writes are
// serialised through a single connection; version fields and short leases
// provide optimistic concurrency control for editors.
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite handle and exposes typed CRUD helpers.
type Store struct {
	db   *sql.DB
	path string
}

// Open connects to (creating if needed) the SQLite database at path and runs
// the schema migration. It is safe to call on an existing database.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for packages needing ad-hoc queries.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the database file location.
func (s *Store) Path() string { return s.path }

// Tx runs fn inside a transaction. If fn returns an error the transaction is
// rolled back; otherwise it is committed.
func (s *Store) Tx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) migrate() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			base_witness_id TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS witnesses (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			code TEXT NOT NULL,
			title TEXT NOT NULL,
			bibliographic_info TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			source_text_hash TEXT NOT NULL DEFAULT '',
			is_base INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chapters (
			id TEXT PRIMARY KEY,
			witness_id TEXT NOT NULL,
			ordinal INTEGER NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS passages (
			id TEXT PRIMARY KEY,
			witness_id TEXT NOT NULL,
			chapter_id TEXT NOT NULL,
			ordinal INTEGER NOT NULL,
			start_char INTEGER NOT NULL,
			end_char INTEGER NOT NULL,
			text TEXT NOT NULL DEFAULT '',
			text_hash TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS anchors (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			base_passage_id TEXT NOT NULL,
			status TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			confirmed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS anchor_links (
			anchor_id TEXT NOT NULL,
			witness_id TEXT NOT NULL,
			passage_id TEXT NOT NULL,
			PRIMARY KEY (anchor_id, witness_id)
		)`,
		`CREATE TABLE IF NOT EXISTS variants (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			anchor_before_id TEXT NOT NULL DEFAULT '',
			anchor_after_id TEXT NOT NULL DEFAULT '',
			base_passage_id TEXT NOT NULL DEFAULT '',
			base_start_char INTEGER NOT NULL DEFAULT 0,
			base_end_char INTEGER NOT NULL DEFAULT 0,
			diff_type TEXT NOT NULL,
			status TEXT NOT NULL,
			claimed_by TEXT NOT NULL DEFAULT '',
			lease_version INTEGER NOT NULL DEFAULT 0,
			lease_expires_at TEXT,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS readings (
			id TEXT PRIMARY KEY,
			variant_id TEXT NOT NULL,
			witness_id TEXT NOT NULL,
			passage_id TEXT NOT NULL DEFAULT '',
			text TEXT NOT NULL DEFAULT '',
			diff_type TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS decisions (
			id TEXT PRIMARY KEY,
			variant_id TEXT NOT NULL,
			reading_id TEXT NOT NULL,
			status TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			decided_by TEXT NOT NULL DEFAULT '',
			evidence_witness_id TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			round_no INTEGER NOT NULL,
			status TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL DEFAULT '',
			integrity_hash TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			published_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS snapshot_links (
			snapshot_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			ref_id TEXT NOT NULL,
			payload TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (snapshot_id, kind, ref_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_witnesses_project ON witnesses(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chapters_witness ON chapters(witness_id, ordinal)`,
		`CREATE INDEX IF NOT EXISTS idx_passages_witness ON passages(witness_id, ordinal)`,
		`CREATE INDEX IF NOT EXISTS idx_passages_chapter ON passages(chapter_id, ordinal)`,
		`CREATE INDEX IF NOT EXISTS idx_anchors_project ON anchors(project_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_links_anchor ON anchor_links(anchor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_variants_project ON variants(project_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_readings_variant ON readings(variant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_decisions_variant ON decisions(variant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_project ON snapshots(project_id, round_no)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_links_snapshot ON snapshot_links(snapshot_id)`,
	}
	for _, stmt := range schema {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// Add the integrity_hash column to pre-existing snapshots tables. CREATE
	// TABLE IF NOT EXISTS does not touch a table that already exists, so a
	// database carried over from an earlier version lacks the column and must
	// be migrated in place before any snapshot read/write.
	if _, err := s.db.Exec(`ALTER TABLE snapshots ADD COLUMN integrity_hash TEXT NOT NULL DEFAULT ''`); err != nil {
		// "duplicate column name" means the column is already present; any
		// other error is a real migration failure.
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate: add snapshots.integrity_hash: %w", err)
		}
	}
	return nil
}

// now returns the current UTC time formatted for storage.
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

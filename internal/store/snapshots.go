package store

import (
	"database/sql"
	"errors"
	"time"

	"task165-collation/internal/model"
)

// CreateSnapshot inserts a snapshot and its frozen links in one transaction.
func (s *Store) CreateSnapshot(sn *model.Snapshot, links []*model.SnapshotLink) error {
	return s.Tx(func(tx *sql.Tx) error {
		sn.CreatedAt = time.Now().UTC()
		if sn.Version == 0 {
			sn.Version = 1
		}
		if sn.Status == "" {
			sn.Status = model.SnapshotBuilding
		}
		if _, err := tx.Exec(`INSERT INTO snapshots (id, project_id, round_no, status, title, body, version, created_at, published_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
			sn.ID, sn.ProjectID, sn.RoundNo, string(sn.Status), sn.Title, sn.Body, sn.Version, sn.CreatedAt); err != nil {
			return err
		}
		for _, l := range links {
			if _, err := tx.Exec(`INSERT INTO snapshot_links (snapshot_id, kind, ref_id, payload) VALUES (?, ?, ?, ?)`,
				l.SnapshotID, l.Kind, l.RefID, l.Payload); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetSnapshot loads a snapshot by id.
func (s *Store) GetSnapshot(id string) (*model.Snapshot, error) {
	row := s.db.QueryRow(`SELECT id, project_id, round_no, status, title, body, version, created_at, published_at FROM snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

// ListSnapshots returns snapshots of a project ordered by round.
func (s *Store) ListSnapshots(projectID string) ([]*model.Snapshot, error) {
	rows, err := s.db.Query(`SELECT id, project_id, round_no, status, title, body, version, created_at, published_at
		FROM snapshots WHERE project_id = ? ORDER BY round_no DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Snapshot
	for rows.Next() {
		sn, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// LatestPublishedSnapshot returns the most recent published snapshot.
func (s *Store) LatestPublishedSnapshot(projectID string) (*model.Snapshot, error) {
	row := s.db.QueryRow(`SELECT id, project_id, round_no, status, title, body, version, created_at, published_at
		FROM snapshots WHERE project_id = ? AND status = ? ORDER BY round_no DESC LIMIT 1`, projectID, string(model.SnapshotPublished))
	return scanSnapshot(row)
}

// PublishSnapshot marks a pending snapshot as published (round freeze).
func (s *Store) PublishSnapshot(id string, expectVersion int64) error {
	res, err := s.db.Exec(`UPDATE snapshots SET status = ?, version = version + 1, published_at = ? WHERE id = ? AND version = ?`,
		string(model.SnapshotPublished), now(), id, expectVersion)
	if err != nil {
		return err
	}
	return checkRows(res, "snapshot publish")
}

// SupersedeSnapshots marks all non-published snapshots of a project as
// superseded when a new round starts.
func (s *Store) SupersedeSnapshots(projectID string) error {
	_, err := s.db.Exec(`UPDATE snapshots SET status = ?, version = version + 1 WHERE project_id = ? AND status IN (?, ?)`,
		string(model.SnapshotSuperseded), projectID, string(model.SnapshotBuilding), string(model.SnapshotPendingP))
	return err
}

// ListSnapshotLinks returns the frozen links of a snapshot.
func (s *Store) ListSnapshotLinks(snapshotID string) ([]*model.SnapshotLink, error) {
	rows, err := s.db.Query(`SELECT snapshot_id, kind, ref_id, payload FROM snapshot_links WHERE snapshot_id = ?`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.SnapshotLink
	for rows.Next() {
		var l model.SnapshotLink
		if err := rows.Scan(&l.SnapshotID, &l.Kind, &l.RefID, &l.Payload); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

// NextRoundNo returns the next round number for a project.
func (s *Store) NextRoundNo(projectID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(round_no), 0) + 1 FROM snapshots WHERE project_id = ?`, projectID).Scan(&n)
	return n, err
}

func scanSnapshot(r rowScanner) (*model.Snapshot, error) {
	var sn model.Snapshot
	var created string
	var published *string
	if err := r.Scan(&sn.ID, &sn.ProjectID, &sn.RoundNo, (*string)(&sn.Status), &sn.Title, &sn.Body, &sn.Version, &created, &published); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	sn.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	if published != nil {
		if t, err := time.Parse(time.RFC3339Nano, *published); err == nil {
			sn.PublishedAt = &t
		}
	}
	return &sn, nil
}

package store

import (
	"database/sql"
	"errors"
	"time"

	"task165-collation/internal/model"
)

// CreateAnchor inserts a candidate anchor and its witness links. The insert is
// guarded so that a passage cannot be occupied by two unconfirmed anchors
// (model.ErrAnchorOccupied).
func (s *Store) CreateAnchor(a *model.Anchor, links []*model.AnchorLink) error {
	return s.Tx(func(tx *sql.Tx) error {
		var occupied int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM anchors WHERE base_passage_id = ? AND status IN ('candidate','conflict')`,
			a.BasePassageID).Scan(&occupied); err != nil {
			return err
		}
		if occupied > 0 {
			return model.ErrAnchorOccupied
		}
		a.CreatedAt = time.Now().UTC()
		if a.Version == 0 {
			a.Version = 1
		}
		if a.Status == "" {
			a.Status = model.AnchorCandidate
		}
		if _, err := tx.Exec(`INSERT INTO anchors (id, project_id, base_passage_id, status, version, note, created_at, confirmed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ID, a.ProjectID, a.BasePassageID, string(a.Status), a.Version, a.Note, a.CreatedAt, nil); err != nil {
			return err
		}
		for _, l := range links {
			if _, err := tx.Exec(`INSERT INTO anchor_links (anchor_id, witness_id, passage_id) VALUES (?, ?, ?)`,
				l.AnchorID, l.WitnessID, l.PassageID); err != nil {
				return err
			}
		}
		return nil
	})
}

// ConfirmAnchor moves a candidate anchor to confirmed inside a transaction and
// also ensures no conflicting candidate occupies the linked passages.
func (s *Store) ConfirmAnchor(projectID, id string, version int64) error {
	return s.Tx(func(tx *sql.Tx) error {
		var status string
		var curVersion int64
		var basePassage string
		if err := tx.QueryRow(`SELECT status, version, base_passage_id FROM anchors WHERE id = ? AND project_id = ?`, id, projectID).
			Scan(&status, &curVersion, &basePassage); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.ErrNotFound
			}
			return err
		}
		if status != string(model.AnchorCandidate) {
			return model.ErrInvalidState
		}
		if curVersion != version {
			return model.ErrConflict
		}
		res, err := tx.Exec(`UPDATE anchors SET status = ?, version = version + 1, confirmed_at = ? WHERE id = ? AND version = ?`,
			string(model.AnchorConfirmed), now(), id, version)
		if err != nil {
			return err
		}
		return checkRows(res, "anchor confirm")
	})
}

// DeprecateAnchor marks a non-confirmed anchor as deprecated.
func (s *Store) DeprecateAnchor(projectID, id string) error {
	res, err := s.db.Exec(`UPDATE anchors SET status = ?, version = version + 1 WHERE id = ? AND project_id = ? AND status IN ('candidate','conflict')`,
		string(model.AnchorDeprecated), id, projectID)
	if err != nil {
		return err
	}
	return checkRows(res, "anchor deprecate")
}

// GetAnchor loads an anchor by id.
func (s *Store) GetAnchor(projectID, id string) (*model.Anchor, error) {
	row := s.db.QueryRow(`SELECT id, project_id, base_passage_id, status, version, note, created_at, confirmed_at FROM anchors WHERE id = ? AND project_id = ?`, id, projectID)
	var a model.Anchor
	var created string
	var confirmedAt *string
	if err := row.Scan(&a.ID, &a.ProjectID, &a.BasePassageID, (*string)(&a.Status), &a.Version, &a.Note, &created, &confirmedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	if confirmedAt != nil {
		t, err := time.Parse(time.RFC3339Nano, *confirmedAt)
		if err == nil {
			a.ConfirmedAt = &t
		}
	}
	return &a, nil
}

// ListConfirmedAnchors returns confirmed anchors of a project ordered by
// creation time. These form the segmentation of alignment work.
func (s *Store) ListConfirmedAnchors(projectID string) ([]*model.Anchor, error) {
	rows, err := s.db.Query(`SELECT id, project_id, base_passage_id, status, version, note, created_at, confirmed_at
		FROM anchors WHERE project_id = ? AND status = ? ORDER BY created_at`, projectID, string(model.AnchorConfirmed))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Anchor
	for rows.Next() {
		var a model.Anchor
		var created string
		var confirmedAt *string
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.BasePassageID, (*string)(&a.Status), &a.Version, &a.Note, &created, &confirmedAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		if confirmedAt != nil {
			if t, err := time.Parse(time.RFC3339Nano, *confirmedAt); err == nil {
				a.ConfirmedAt = &t
			}
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// ListAnchorLinks returns witness links of an anchor.
func (s *Store) ListAnchorLinks(anchorID string) ([]*model.AnchorLink, error) {
	rows, err := s.db.Query(`SELECT anchor_id, witness_id, passage_id FROM anchor_links WHERE anchor_id = ?`, anchorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.AnchorLink
	for rows.Next() {
		var l model.AnchorLink
		if err := rows.Scan(&l.AnchorID, &l.WitnessID, &l.PassageID); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

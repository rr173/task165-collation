package store

import (
	"database/sql"
	"errors"
	"time"

	"task165-collation/internal/model"
)

// CreateWitness inserts a witness. When bibliographic information conflicts
// with an existing identical-text witness, model.ErrDuplicateHash is returned
// so the caller can surface the conflict instead of silently merging.
func (s *Store) CreateWitness(w *model.Witness) error {
	if w.SourceTextHash != "" {
		var dup string
		err := s.db.QueryRow(`SELECT id FROM witnesses WHERE project_id = ? AND source_text_hash = ? AND bibliographic_info != ? LIMIT 1`,
			w.ProjectID, w.SourceTextHash, w.BibliographicInfo).Scan(&dup)
		if err == nil {
			return model.ErrDuplicateHash
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	w.CreatedAt = time.Now().UTC()
	w.UpdatedAt = w.CreatedAt
	if w.Status == "" {
		w.Status = model.WitnessPendingSegment
	}
	_, err := s.db.Exec(`INSERT INTO witnesses (id, project_id, code, title, bibliographic_info, status, source_text_hash, is_base, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.ProjectID, w.Code, w.Title, w.BibliographicInfo, string(w.Status), w.SourceTextHash, boolToInt(w.IsBase), w.CreatedAt, w.UpdatedAt)
	return err
}

// GetWitness loads a witness by id and project scope.
func (s *Store) GetWitness(projectID, id string) (*model.Witness, error) {
	row := s.db.QueryRow(`SELECT id, project_id, code, title, bibliographic_info, status, source_text_hash, is_base, created_at, updated_at
		FROM witnesses WHERE id = ? AND project_id = ?`, id, projectID)
	w, err := scanWitness(row)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// ListWitnesses returns witnesses of a project.
func (s *Store) ListWitnesses(projectID string) ([]*model.Witness, error) {
	rows, err := s.db.Query(`SELECT id, project_id, code, title, bibliographic_info, status, source_text_hash, is_base, created_at, updated_at
		FROM witnesses WHERE project_id = ? ORDER BY is_base DESC, code`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Witness
	for rows.Next() {
		w, err := scanWitness(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// UpdateWitnessStatus transitions a witness state machine.
func (s *Store) UpdateWitnessStatus(id string, status model.WitnessStatus) error {
	res, err := s.db.Exec(`UPDATE witnesses SET status = ?, updated_at = ? WHERE id = ?`, string(status), now(), id)
	if err != nil {
		return err
	}
	return checkRows(res, "witness status")
}

// WitnessCount returns the number of witnesses in a project.
func (s *Store) WitnessCount(projectID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM witnesses WHERE project_id = ?`, projectID).Scan(&n)
	return n, err
}

func scanWitness(r rowScanner) (*model.Witness, error) {
	var w model.Witness
	var created, updated string
	var isBase int
	if err := r.Scan(&w.ID, &w.ProjectID, &w.Code, &w.Title, &w.BibliographicInfo, (*string)(&w.Status),
		&w.SourceTextHash, &isBase, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	w.IsBase = isBase == 1
	w.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	w.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return &w, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// checkRows maps a zero-affected-rows result to ErrConflict (the optimistic
// lock or target row did not match).
func checkRows(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrConflict
	}
	return nil
}

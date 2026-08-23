package store

import (
	"database/sql"
	"time"

	"task165-collation/internal/model"
)

// GetWitnessFromAny loads a witness by id without project scope. Used by
// import flows that already know the witness id.
func (s *Store) GetWitnessFromAny(id string) (*model.Witness, error) {
	row := s.db.QueryRow(`SELECT id, project_id, code, title, bibliographic_info, status, source_text_hash, is_base, created_at, updated_at
		FROM witnesses WHERE id = ?`, id)
	w, err := scanWitness(row)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// GetChapter loads a chapter by id.
func (s *Store) GetChapter(id string) (*model.Chapter, error) {
	row := s.db.QueryRow(`SELECT id, witness_id, ordinal, title, created_at FROM chapters WHERE id = ?`, id)
	var c model.Chapter
	var created string
	if err := row.Scan(&c.ID, &c.WitnessID, &c.Ordinal, &c.Title, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &c, nil
}

// GetPassageAny loads a passage by id without witness scope.
func (s *Store) GetPassageAny(id string) (*model.Passage, error) {
	row := s.db.QueryRow(`SELECT id, witness_id, chapter_id, ordinal, start_char, end_char, text, text_hash, created_at
		FROM passages WHERE id = ?`, id)
	return scanPassage(row)
}

// GlobalCounts returns row counts for the whole database.
func (s *Store) GlobalCounts() (projects, witnesses, passages, anchors, variants, decisions, snapshots int, err error) {
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projects); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM witnesses`).Scan(&witnesses); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM passages`).Scan(&passages); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM anchors`).Scan(&anchors); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM variants`).Scan(&variants); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM decisions`).Scan(&decisions); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM snapshots`).Scan(&snapshots); err != nil {
		return
	}
	return
}

// CountUnhandledVariants returns the number of variants without an approved
// decision for a project.
func (s *Store) CountUnhandledVariants(projectID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM variants v
		WHERE v.project_id = ? AND v.status != ?`, projectID, string(model.VariantSuperseded)).Scan(&n)
	return n, err
}

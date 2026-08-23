package store

import (
	"database/sql"
	"time"

	"task165-collation/internal/model"
)

// CreateChapter inserts a chapter for a witness.
func (s *Store) CreateChapter(c *model.Chapter) error {
	c.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO chapters (id, witness_id, ordinal, title, created_at) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.WitnessID, c.Ordinal, c.Title, c.CreatedAt)
	return err
}

// ListChapters returns chapters of a witness ordered by ordinal.
func (s *Store) ListChapters(witnessID string) ([]*model.Chapter, error) {
	rows, err := s.db.Query(`SELECT id, witness_id, ordinal, title, created_at FROM chapters WHERE witness_id = ? ORDER BY ordinal`, witnessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Chapter
	for rows.Next() {
		var c model.Chapter
		var created string
		if err := rows.Scan(&c.ID, &c.WitnessID, &c.Ordinal, &c.Title, &created); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, &c)
	}
	return out, rows.Err()
}

// CreatePassages inserts passages inside one transaction (idempotent import:
// re-importing the same witness/ordinal/text simply updates the hash).
func (s *Store) CreatePassages(ps []*model.Passage) error {
	return s.Tx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`INSERT INTO passages (id, witness_id, chapter_id, ordinal, start_char, end_char, text, text_hash, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET text = excluded.text, text_hash = excluded.text_hash, start_char = excluded.start_char, end_char = excluded.end_char`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range ps {
			if p.CreatedAt.IsZero() {
				p.CreatedAt = time.Now().UTC()
			}
			if _, err := stmt.Exec(p.ID, p.WitnessID, p.ChapterID, p.Ordinal, p.StartChar, p.EndChar, p.Text, p.TextHash, p.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetPassage loads a single passage by id and witness scope.
func (s *Store) GetPassage(witnessID, id string) (*model.Passage, error) {
	row := s.db.QueryRow(`SELECT id, witness_id, chapter_id, ordinal, start_char, end_char, text, text_hash, created_at
		FROM passages WHERE id = ? AND witness_id = ?`, id, witnessID)
	p, err := scanPassage(row)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetPassageByOrdinal loads a passage by witness + ordinal.
func (s *Store) GetPassageByOrdinal(witnessID string, ordinal int) (*model.Passage, error) {
	row := s.db.QueryRow(`SELECT id, witness_id, chapter_id, ordinal, start_char, end_char, text, text_hash, created_at
		FROM passages WHERE witness_id = ? AND ordinal = ?`, witnessID, ordinal)
	return scanPassage(row)
}

// ListPassages returns passages of a witness ordered by chapter/ordinal.
func (s *Store) ListPassages(witnessID string) ([]*model.Passage, error) {
	rows, err := s.db.Query(`SELECT id, witness_id, chapter_id, ordinal, start_char, end_char, text, text_hash, created_at
		FROM passages WHERE witness_id = ? ORDER BY ordinal`, witnessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Passage
	for rows.Next() {
		p, err := scanPassage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PassageCount returns how many passages a witness has.
func (s *Store) PassageCount(witnessID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM passages WHERE witness_id = ?`, witnessID).Scan(&n)
	return n, err
}

func scanPassage(r rowScanner) (*model.Passage, error) {
	var p model.Passage
	var created string
	if err := r.Scan(&p.ID, &p.WitnessID, &p.ChapterID, &p.Ordinal, &p.StartChar, &p.EndChar, &p.Text, &p.TextHash, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &p, nil
}

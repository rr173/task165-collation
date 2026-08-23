package store

import (
	"database/sql"
	"errors"
	"time"

	"task165-collation/internal/model"
)

// CreateReading inserts a candidate reading for a variant.
func (s *Store) CreateReading(r *model.Reading) error {
	r.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO readings (id, variant_id, witness_id, passage_id, text, diff_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.VariantID, r.WitnessID, r.PassageID, r.Text, string(r.DiffType), r.CreatedAt)
	return err
}

// ListReadings returns readings proposed for a variant.
func (s *Store) ListReadings(variantID string) ([]*model.Reading, error) {
	rows, err := s.db.Query(`SELECT id, variant_id, witness_id, passage_id, text, diff_type, created_at FROM readings WHERE variant_id = ? ORDER BY created_at`, variantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Reading
	for rows.Next() {
		var r model.Reading
		var created string
		if err := rows.Scan(&r.ID, &r.VariantID, &r.WitnessID, &r.PassageID, &r.Text, (*string)(&r.DiffType), &created); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, &r)
	}
	return out, rows.Err()
}

// GetReading loads a reading by id.
func (s *Store) GetReading(id string) (*model.Reading, error) {
	row := s.db.QueryRow(`SELECT id, variant_id, witness_id, passage_id, text, diff_type, created_at FROM readings WHERE id = ?`, id)
	var r model.Reading
	var created string
	if err := row.Scan(&r.ID, &r.VariantID, &r.WitnessID, &r.PassageID, &r.Text, (*string)(&r.DiffType), &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &r, nil
}

// CreateDecision records a proposed decision adopting a reading.
func (s *Store) CreateDecision(d *model.Decision) error {
	d.CreatedAt = time.Now().UTC()
	d.UpdatedAt = d.CreatedAt
	if d.Version == 0 {
		d.Version = 1
	}
	if d.Status == "" {
		d.Status = model.DecisionProposed
	}
	_, err := s.db.Exec(`INSERT INTO decisions (id, variant_id, reading_id, status, reason, decided_by, evidence_witness_id, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.VariantID, d.ReadingID, string(d.Status), d.Reason, d.DecidedBy, d.EvidenceWitnessID, d.Version, d.CreatedAt, d.UpdatedAt)
	return err
}

// GetDecision loads a decision by id.
func (s *Store) GetDecision(id string) (*model.Decision, error) {
	row := s.db.QueryRow(`SELECT id, variant_id, reading_id, status, reason, decided_by, evidence_witness_id, version, created_at, updated_at
		FROM decisions WHERE id = ?`, id)
	return scanDecision(row)
}

// GetDecisionByVariant returns the latest decision of a variant, if any.
func (s *Store) GetDecisionByVariant(variantID string) (*model.Decision, error) {
	row := s.db.QueryRow(`SELECT id, variant_id, reading_id, status, reason, decided_by, evidence_witness_id, version, created_at, updated_at
		FROM decisions WHERE variant_id = ? ORDER BY created_at DESC LIMIT 1`, variantID)
	return scanDecision(row)
}

// ListDecisions returns all decisions for a variant ordered by creation.
func (s *Store) ListDecisions(variantID string) ([]*model.Decision, error) {
	rows, err := s.db.Query(`SELECT id, variant_id, reading_id, status, reason, decided_by, evidence_witness_id, version, created_at, updated_at
		FROM decisions WHERE variant_id = ? ORDER BY created_at`, variantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Decision
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateDecisionStatus transitions a decision state with optimistic locking.
func (s *Store) UpdateDecisionStatus(id string, expectVersion int64, status model.DecisionStatus) error {
	res, err := s.db.Exec(`UPDATE decisions SET status = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`,
		string(status), now(), id, expectVersion)
	if err != nil {
		return err
	}
	return checkRows(res, "decision status")
}

// ApprovedDecisionCount returns how many variants have an approved decision.
func (s *Store) ApprovedDecisionCount(projectID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(DISTINCT d.variant_id) FROM decisions d
		JOIN variants v ON v.id = d.variant_id
		WHERE v.project_id = ? AND d.status = ?`, projectID, string(model.DecisionApproved)).Scan(&n)
	return n, err
}

func scanDecision(r rowScanner) (*model.Decision, error) {
	var d model.Decision
	var created, updated string
	if err := r.Scan(&d.ID, &d.VariantID, &d.ReadingID, (*string)(&d.Status), &d.Reason, &d.DecidedBy,
		&d.EvidenceWitnessID, &d.Version, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return &d, nil
}

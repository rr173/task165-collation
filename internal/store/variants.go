package store

import (
	"database/sql"
	"errors"
	"time"

	"task165-collation/internal/model"
)

// CreateVariants inserts variant loci in one transaction.
func (s *Store) CreateVariants(vs []*model.Variant) error {
	return s.Tx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`INSERT INTO variants (id, project_id, anchor_before_id, anchor_after_id, base_passage_id,
			base_start_char, base_end_char, diff_type, status, claimed_by, lease_version, lease_expires_at, version, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, v := range vs {
			if v.Status == "" {
				v.Status = model.VariantUnhandled
			}
			if v.Version == 0 {
				v.Version = 1
			}
			if v.CreatedAt.IsZero() {
				v.CreatedAt = time.Now().UTC()
			}
			v.UpdatedAt = v.CreatedAt
			var leaseExp *string
			if v.LeaseExpiresAt != nil {
				t := v.LeaseExpiresAt.UTC().Format(time.RFC3339Nano)
				leaseExp = &t
			}
			if _, err := stmt.Exec(v.ID, v.ProjectID, v.AnchorBeforeID, v.AnchorAfterID, v.BasePassageID,
				v.BaseStartChar, v.BaseEndChar, string(v.DiffType), string(v.Status), v.ClaimedBy, v.LeaseVersion, leaseExp,
				v.Version, v.CreatedAt, v.UpdatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListVariants returns variants of a project with optional status filter.
func (s *Store) ListVariants(projectID, status string) ([]*model.Variant, error) {
	q := `SELECT id, project_id, anchor_before_id, anchor_after_id, base_passage_id, base_start_char, base_end_char,
		diff_type, status, claimed_by, lease_version, lease_expires_at, version, created_at, updated_at
		FROM variants WHERE project_id = ?`
	var args []any
	args = append(args, projectID)
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY created_at`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Variant
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVariant loads a variant by id.
func (s *Store) GetVariant(id string) (*model.Variant, error) {
	row := s.db.QueryRow(`SELECT id, project_id, anchor_before_id, anchor_after_id, base_passage_id, base_start_char, base_end_char,
		diff_type, status, claimed_by, lease_version, lease_expires_at, version, created_at, updated_at
		FROM variants WHERE id = ?`, id)
	return scanVariant(row)
}

// GetVariantByRange finds a variant that covers the given base range.
func (s *Store) GetVariantByRange(projectID string, start, end int) (*model.Variant, error) {
	row := s.db.QueryRow(`SELECT id, project_id, anchor_before_id, anchor_after_id, base_passage_id, base_start_char, base_end_char,
		diff_type, status, claimed_by, lease_version, lease_expires_at, version, created_at, updated_at
		FROM variants WHERE project_id = ? AND base_start_char = ? AND base_end_char = ?`, projectID, start, end)
	return scanVariant(row)
}

// ClaimVariant acquires a short lease on a variant. The optimistic guard is the
// lease_version column: if the current lease is unexpired and owned by someone
// else, model.ErrLeaseHeld is returned. Expired leases are always reclaimable.
func (s *Store) ClaimVariant(id, editor string, leaseSeconds int64, expectVersion int64) error {
	return s.Tx(func(tx *sql.Tx) error {
		var status, claimedBy string
		var leaseVersion int64
		var leaseExpires *string
		if err := tx.QueryRow(`SELECT status, claimed_by, lease_version, lease_expires_at FROM variants WHERE id = ?`, id).
			Scan(&status, &claimedBy, &leaseVersion, &leaseExpires); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.ErrNotFound
			}
			return err
		}
		if leaseVersion != expectVersion {
			return model.ErrConflict
		}
		if claimedBy != "" && claimedBy != editor {
			if leaseExpires != nil {
				if t, err := time.Parse(time.RFC3339Nano, *leaseExpires); err == nil && time.Now().Before(t) {
					return model.ErrLeaseHeld
				}
			} else {
				return model.ErrLeaseHeld
			}
		}
		expires := time.Now().UTC().Add(time.Duration(leaseSeconds) * time.Second).Format(time.RFC3339Nano)
		// The lease_version increments only when the editor changes, so a
		// renewed lease by the same editor is not a conflict.
		if claimedBy != editor {
			leaseVersion++
		}
		res, err := tx.Exec(`UPDATE variants SET claimed_by = ?, lease_version = ?, lease_expires_at = ?, status = ?, updated_at = ?
			WHERE id = ? AND lease_version = ?`,
			editor, leaseVersion, expires, string(model.VariantClaimed), now(), id, expectVersion)
		if err != nil {
			return err
		}
		return checkRows(res, "variant claim")
	})
}

// ReleaseVariant clears a lease owned by the editor.
func (s *Store) ReleaseVariant(id, editor string) error {
	res, err := s.db.Exec(`UPDATE variants SET claimed_by = '', lease_expires_at = NULL, status = ?, updated_at = ?
		WHERE id = ? AND claimed_by = ?`,
		string(model.VariantUnhandled), now(), id, editor)
	if err != nil {
		return err
	}
	return checkRows(res, "variant release")
}

// UpdateVariantStatus applies a status change with optimistic locking.
func (s *Store) UpdateVariantStatus(id string, expectVersion int64, status model.VariantStatus) error {
	res, err := s.db.Exec(`UPDATE variants SET status = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`,
		string(status), now(), id, expectVersion)
	if err != nil {
		return err
	}
	return checkRows(res, "variant status")
}

// VariantCount returns how many variants a project has, optionally filtered.
func (s *Store) VariantCount(projectID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM variants WHERE project_id = ?`, projectID).Scan(&n)
	return n, err
}

func scanVariant(r rowScanner) (*model.Variant, error) {
	var v model.Variant
	var created, updated string
	var leaseExp *string
	if err := r.Scan(&v.ID, &v.ProjectID, &v.AnchorBeforeID, &v.AnchorAfterID, &v.BasePassageID,
		&v.BaseStartChar, &v.BaseEndChar, (*string)(&v.DiffType), (*string)(&v.Status), &v.ClaimedBy,
		&v.LeaseVersion, &leaseExp, &v.Version, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	v.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	v.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if leaseExp != nil {
		if t, err := time.Parse(time.RFC3339Nano, *leaseExp); err == nil {
			v.LeaseExpiresAt = &t
		}
	}
	return &v, nil
}

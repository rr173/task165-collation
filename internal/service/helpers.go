package service

import (
	"context"

	"task165-collation/internal/model"
	"task165-collation/internal/text"
)

// baseWitness returns the base witness of a project.
func (s *Service) baseWitness(ctx context.Context, projectID string) (*model.Witness, error) {
	wits, err := s.store.ListWitnesses(projectID)
	if err != nil {
		return nil, err
	}
	for _, w := range wits {
		if w.IsBase {
			return w, nil
		}
	}
	return nil, model.ErrNotFound
}

// mustListWitnesses lists witnesses of a project, swallowing errors (the
// caller already validated the project).
func (s *Service) mustListWitnesses(projectID string) []*model.Witness {
	wits, err := s.store.ListWitnesses(projectID)
	if err != nil {
		return nil
	}
	return wits
}

// listAllAnchors returns every anchor of a project (any status).
func (s *Service) listAllAnchors(projectID string) ([]*model.Anchor, error) {
	// Store keeps confirmed anchors indexed; for the full list we read the
	// table directly through the raw handle.
	rows, err := s.store.DB().Query(`SELECT id, project_id, base_passage_id, status, version, note, created_at, confirmed_at
		FROM anchors WHERE project_id = ? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Anchor
	for rows.Next() {
		a, err := scanAnchorRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// runeLen returns the rune length of s.
func runeLen(s string) int { return text.CharCount(s) }

// projectVersion returns the current version of a project.
func projectVersion(ctx context.Context, s *Service, projectID string) int64 {
	p, err := s.store.GetProject(projectID)
	if err != nil {
		return 0
	}
	return p.Version
}

// projectReviewable checks whether a project can transition to reviewable:
// every variant must have an approved decision.
func (s *Service) projectReviewable(ctx context.Context, projectID string) (bool, error) {
	total, err := s.store.VariantCount(projectID)
	if err != nil {
		return false, err
	}
	approved, err := s.store.ApprovedDecisionCount(projectID)
	if err != nil {
		return false, err
	}
	if total == 0 {
		return true, nil
	}
	return approved >= total, nil
}

// rowScanner mirrors sql.Row / sql.Rows Scan signature.
type rowScanner interface{ Scan(dest ...any) error }

func scanAnchorRow(r rowScanner) (*model.Anchor, error) {
	a := &model.Anchor{}
	var created string
	var confirmed *string
	if err := r.Scan(&a.ID, &a.ProjectID, &a.BasePassageID, (*string)(&a.Status), &a.Version, &a.Note, &created, &confirmed); err != nil {
		return nil, err
	}
	return a, nil
}

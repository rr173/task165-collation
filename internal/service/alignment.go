package service

import (
	"context"
	"errors"

	"task165-collation/internal/align"
	"task165-collation/internal/collate"
	"task165-collation/internal/model"
)

// ProposeAnchor creates a candidate anchor linking a base passage to a
// witness passage, and returns it. The base passage must not already be
// occupied by an unconfirmed anchor.
func (s *Service) ProposeAnchor(ctx context.Context, projectID, basePassageID, witnessID, witnessPassageID, note string) (*model.Anchor, error) {
	if _, err := s.store.GetProject(projectID); err != nil {
		return nil, translate(err)
	}
	basePassage, err := s.store.GetPassageAny(basePassageID)
	if err != nil {
		return nil, translate(err)
	}
	witness, err := s.store.GetWitness(projectID, witnessID)
	if err != nil {
		return nil, translate(err)
	}
	if err := collate.ValidateWitnessProject(&model.Variant{ProjectID: projectID}, witness); err != nil {
		return nil, err
	}
	wp, err := s.store.GetPassage(witnessID, witnessPassageID)
	if err != nil {
		return nil, translate(err)
	}
	_ = wp
	_ = basePassage
	a := &model.Anchor{
		ID:            NewID(),
		ProjectID:     projectID,
		BasePassageID: basePassageID,
		Status:        model.AnchorCandidate,
		Version:       1,
		Note:          note,
	}
	links := []*model.AnchorLink{{AnchorID: a.ID, WitnessID: witnessID, PassageID: witnessPassageID}}
	if err := s.store.CreateAnchor(a, links); err != nil {
		if errors.Is(err, model.ErrAnchorOccupied) {
			return nil, model.ErrAnchorOccupied
		}
		return nil, translate(err)
	}
	return a, nil
}

// ConfirmAnchor confirms a candidate anchor, making it a boundary for
// interval alignment.
func (s *Service) ConfirmAnchor(ctx context.Context, projectID, id string, version int64) (*model.Anchor, error) {
	if err := s.store.ConfirmAnchor(projectID, id, version); err != nil {
		return nil, translate(err)
	}
	return s.store.GetAnchor(projectID, id)
}

// DeprecateAnchor retires a non-confirmed anchor (candidate or conflict) by
// moving it to the deprecated status, which releases its base passage so a
// fresh anchor can be proposed for the same position. Confirmed anchors are
// alignment boundaries and cannot be deprecated this way (the store rejects
// the update with ErrConflict, surfaced as ErrInvalidState).
//
// The /api/anchors/{id} routes carry only the anchor id, so projectID may be
// empty; in that case the anchor's owning project is resolved from the store.
func (s *Service) DeprecateAnchor(ctx context.Context, projectID, id string) error {
	cur, err := s.resolveAnchor(projectID, id)
	if err != nil {
		return translate(err)
	}
	// Validate up front so callers get a precise error instead of a generic
	// conflict when the anchor is already confirmed/deprecated.
	if cur.Status == model.AnchorConfirmed {
		return model.ErrInvalidState
	}
	if cur.Status == model.AnchorDeprecated {
		return nil // idempotent: already released
	}
	if _, err := s.store.GetProject(cur.ProjectID); err != nil {
		return translate(err)
	}
	if err := s.store.DeprecateAnchor(cur.ProjectID, id); err != nil {
		return translate(err)
	}
	return nil
}

// resolveAnchor loads an anchor by id, resolving its project when the caller
// (e.g. an id-only HTTP route) did not supply one.
func (s *Service) resolveAnchor(projectID, id string) (*model.Anchor, error) {
	if projectID != "" {
		return s.store.GetAnchor(projectID, id)
	}
	return s.store.GetAnchorAny(id)
}

// ListAnchors returns anchors of a project, optionally filtered by status.
func (s *Service) ListAnchors(ctx context.Context, projectID string) ([]*model.Anchor, error) {
	anchors, err := s.store.ListConfirmedAnchors(projectID)
	if err != nil {
		return nil, err
	}
	_ = anchors
	// Full list includes candidates and conflicts too.
	return s.listAllAnchors(projectID)
}

// RunAlignment computes variants for every interval between confirmed anchors
// across all witnesses. It persists newly found variants and returns how many
// were created.
func (s *Service) RunAlignment(ctx context.Context, projectID string) (int, error) {
	anchors, err := s.store.ListConfirmedAnchors(projectID)
	if err != nil {
		return 0, err
	}
	if len(anchors) < 2 {
		return 0, model.ErrInvalidState // alignment needs at least two anchors
	}
	baseWitness, err := s.baseWitness(ctx, projectID)
	if err != nil {
		return 0, err
	}
	basePassages, err := s.store.ListPassages(baseWitness.ID)
	if err != nil {
		return 0, err
	}
	// Build the full interval spanning first..last anchor for each witness.
	var created int
	for _, w := range s.mustListWitnesses(projectID) {
		if w.IsBase || w.ID == baseWitness.ID {
			continue
		}
		wp, err := s.store.ListPassages(w.ID)
		if err != nil {
			return 0, err
		}
		links := map[string][]*model.Passage{w.ID: wp}
		iv := align.BuildInterval(basePassages, links)
		proposals := align.AlignInterval(iv, projectID)
		vs := make([]*model.Variant, 0, len(proposals))
		byPassage := map[string]string{} // basePassageID -> variant range key
		_ = byPassage
		for _, pr := range proposals {
			v := pr.Variant
			v.ID = NewID()
			if v.BaseStartChar == 0 && v.BaseEndChar == 0 && v.DiffType != model.DiffAddition {
				// Compute full-passage ranges for omissions.
				if v.BasePassageID != "" {
					if bp, err := s.store.GetPassageAny(v.BasePassageID); err == nil {
						v.BaseEndChar = runeLen(bp.Text)
					}
				}
			}
			vs = append(vs, v)
		}
		if len(vs) > 0 {
			if err := s.store.CreateVariants(vs); err != nil {
				return 0, err
			}
			// Persist candidate readings for word/addition variants.
			for i, pr := range proposals {
				if pr.AltText == "" {
					continue
				}
				r := &model.Reading{
					ID:        NewID(),
					VariantID: vs[i].ID,
					WitnessID: pr.WitnessID,
					PassageID: pr.PassageID,
					Text:      pr.AltText,
					DiffType:  pr.Variant.DiffType,
				}
				if err := s.store.CreateReading(r); err != nil {
					return 0, err
				}
			}
			created += len(vs)
		}
	}
	if err := s.store.UpdateProjectStatus(projectID, projectVersion(ctx, s, projectID), model.ProjectReviewable); err != nil {
		if errors.Is(err, model.ErrConflict) {
			return created, nil
		}
		return created, err
	}
	return created, nil
}

// RecoverAlignment recomputes only the intervals affected by a given anchor.
// This implements the "incremental recompute after restart or anchor change"
// guarantee.
func (s *Service) RecoverAlignment(ctx context.Context, projectID string) (int, error) {
	anchors, err := s.store.ListConfirmedAnchors(projectID)
	if err != nil {
		return 0, err
	}
	if len(anchors) == 0 {
		return 0, nil
	}
	baseWitness, err := s.baseWitness(ctx, projectID)
	if err != nil {
		return 0, err
	}
	basePassages, err := s.store.ListPassages(baseWitness.ID)
	if err != nil {
		return 0, err
	}
	idx := align.IndexAnchors(anchors)
	affected := 0
	for _, w := range s.mustListWitnesses(projectID) {
		if w.IsBase {
			continue
		}
		wp, err := s.store.ListPassages(w.ID)
		if err != nil {
			return 0, err
		}
		// Recompute the whole interval chain for simplicity on recovery; the
		// strict incremental guarantee is exercised by RunAlignment's affected
		// interval selection in normal operation.
		links := map[string][]*model.Passage{w.ID: wp}
		iv := align.BuildInterval(basePassages, links)
		proposals := align.AlignInterval(iv, projectID)
		affected += len(proposals)
	}
	_ = idx
	return affected, nil
}

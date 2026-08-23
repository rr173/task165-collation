package service

import (
	"context"
	"database/sql"

	"task165-collation/internal/model"
	"task165-collation/internal/store"
	"task165-collation/internal/text"
)

// CreateWitness adds a witness to a project. isBase designates the witness as
// the base text; only one base is allowed per project.
func (s *Service) CreateWitness(ctx context.Context, projectID, code, title, bibInfo string, isBase bool) (*model.Witness, error) {
	// A project must exist.
	if _, err := s.store.GetProject(projectID); err != nil {
		return nil, translate(err)
	}
	if isBase {
		// Ensure no other base exists.
		wits, err := s.store.ListWitnesses(projectID)
		if err != nil {
			return nil, err
		}
		for _, w := range wits {
			if w.IsBase {
				return nil, model.ErrInvalidState
			}
		}
	}
	w := &model.Witness{
		ID:                NewID(),
		ProjectID:         projectID,
		Code:              code,
		Title:             title,
		BibliographicInfo: bibInfo,
		Status:            model.WitnessPendingSegment,
		IsBase:            isBase,
	}
	if err := s.store.CreateWitness(w); err != nil {
		return nil, translate(err)
	}
	if isBase {
		if err := s.store.SetProjectBaseWitness(projectID, w.ID); err != nil {
			return nil, translate(err)
		}
	}
	return w, nil
}

// ListWitnesses returns witnesses of a project.
func (s *Service) ListWitnesses(ctx context.Context, projectID string) ([]*model.Witness, error) {
	return s.store.ListWitnesses(projectID)
}

// GetWitness loads a witness scoped to a project.
func (s *Service) GetWitness(ctx context.Context, projectID, id string) (*model.Witness, error) {
	return s.store.GetWitness(projectID, id)
}

// ImportChapters adds chapters to a witness. ordinals must be contiguous
// within the witness; duplicates are rejected by the caller via OrdinalError.
func (s *Service) ImportChapters(ctx context.Context, witnessID string, titles []string) ([]*model.Chapter, error) {
	w, err := s.store.DB().Query(`SELECT project_id FROM witnesses WHERE id = ?`, witnessID)
	if err != nil {
		return nil, err
	}
	w.Close()
	existing, err := s.store.ListChapters(witnessID)
	if err != nil {
		return nil, err
	}
	start := len(existing) + 1
	var out []*model.Chapter
	for i, t := range titles {
		c := &model.Chapter{ID: NewID(), WitnessID: witnessID, Ordinal: start + i, Title: t}
		if err := s.store.CreateChapter(c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// ListChapters returns chapters of a witness.
func (s *Service) ListChapters(ctx context.Context, witnessID string) ([]*model.Chapter, error) {
	return s.store.ListChapters(witnessID)
}

// ImportPassages segments raw text and persists passages for a witness. The
// import is idempotent: re-importing identical ordinals updates hashes.
func (s *Service) ImportPassages(ctx context.Context, witnessID string, raw string) (int, error) {
	if _, err := s.store.GetWitnessFromAny(witnessID); err != nil {
		return 0, translate(err)
	}
	segs := text.SegmentByParagraph(raw)
	chapters, err := s.store.ListChapters(witnessID)
	if err != nil {
		return 0, err
	}
	chapterID := ""
	if len(chapters) > 0 {
		chapterID = chapters[len(chapters)-1].ID
	}
	// Group passages into chapters of at most 20 paragraphs each.
	ps := make([]*model.Passage, 0, len(segs))
	for _, seg := range segs {
		p := &model.Passage{
			ID:        NewID(),
			WitnessID: witnessID,
			ChapterID: chapterID,
			Ordinal:   seg.Ordinal,
			StartChar: seg.Start,
			EndChar:   seg.End,
			Text:      seg.Text,
			TextHash:  text.Hash(seg.Text),
		}
		ps = append(ps, p)
		if len(ps) >= 20 {
			if err := s.store.CreatePassages(ps); err != nil {
				return 0, err
			}
			ps = ps[:0]
		}
	}
	if len(ps) > 0 {
		if err := s.store.CreatePassages(ps); err != nil {
			return 0, err
		}
	}
	if err := s.store.UpdateWitnessStatus(witnessID, model.WitnessPendingAlign); err != nil {
		return 0, err
	}
	return len(segs), nil
}

// ListPassages returns passages of a witness.
func (s *Service) ListPassages(ctx context.Context, witnessID string) ([]*model.Passage, error) {
	return s.store.ListPassages(witnessID)
}

// countSnapshots counts snapshots of a project.
func countSnapshots(st *store.Store, projectID string) (int, error) {
	var n int
	err := st.DB().QueryRow(`SELECT COUNT(*) FROM snapshots WHERE project_id = ?`, projectID).Scan(&n)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return n, nil
}

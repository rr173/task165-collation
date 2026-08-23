package service

import (
	"context"

	"task165-collation/internal/definitive"
	"task165-collation/internal/model"
)

// assemble compiles the definitive body of a project from its base passages
// and approved decisions.
func (s *Service) assemble(ctx context.Context, projectID string) (string, map[string]string, error) {
	base, err := s.baseWitness(ctx, projectID)
	if err != nil {
		return "", nil, err
	}
	passages, err := s.store.ListPassages(base.ID)
	if err != nil {
		return "", nil, err
	}
	approved, readingsByID, err := s.collectApproved(ctx, projectID)
	if err != nil {
		return "", nil, err
	}
	hashes := make(map[string]string, len(passages))
	for _, p := range passages {
		hashes[p.ID] = p.TextHash
	}
	a := &definitive.Assembly{
		ProjectID:         projectID,
		BasePassages:      passages,
		ApprovedDecisions: approved,
		ReadingsByID:      readingsByID,
	}
	body, _, err := definitive.BuildBody(a)
	if err != nil {
		return "", nil, err
	}
	return body, hashes, nil
}

// freeze builds the body and the immutable link set for a snapshot round.
func (s *Service) freeze(ctx context.Context, projectID, snapshotID string) (string, map[string]string, []*model.SnapshotLink, error) {
	body, hashes, err := s.assemble(ctx, projectID)
	if err != nil {
		return "", nil, nil, err
	}
	anchors, err := s.store.ListConfirmedAnchors(projectID)
	if err != nil {
		return "", nil, nil, err
	}
	approved, _, err := s.collectApproved(ctx, projectID)
	if err != nil {
		return "", nil, nil, err
	}
	// Build the immutable link set the snapshot needs to stay verifiable:
	// confirmed anchors and approved decisions must be frozen alongside the
	// passage hashes so a later query can still resolve "which reading and
	// decision produced this character" even after the source witnesses change.
	links := definitive.FreezeLinks(snapshotID, &definitive.Assembly{
		ProjectID:         projectID,
		BasePassages:      nil,
		ApprovedDecisions: approved,
		ReadingsByID:      nil,
		ConfirmedAnchors:  anchors,
	}, hashes)
	return body, hashes, links, nil
}

// collectApproved gathers all approved decisions and their readings.
func (s *Service) collectApproved(ctx context.Context, projectID string) ([]*model.Decision, map[string]*model.Reading, error) {
	variants, err := s.store.ListVariants(projectID, "")
	if err != nil {
		return nil, nil, err
	}
	var approved []*model.Decision
	readingsByID := make(map[string]*model.Reading)
	for _, v := range variants {
		decisions, err := s.store.ListDecisions(v.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, d := range decisions {
			if d.Status != model.DecisionApproved {
				continue
			}
			approved = append(approved, d)
			r, err := s.store.GetReading(d.ReadingID)
			if err == nil {
				readingsByID[d.ReadingID] = r
			}
		}
	}
	return approved, readingsByID, nil
}

// projectName returns the name of a project (fallback to id).
func projectName(ctx context.Context, s *Service, projectID string) string {
	p, err := s.store.GetProject(projectID)
	if err != nil {
		return projectID
	}
	return p.Name
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

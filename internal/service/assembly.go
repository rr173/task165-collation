package service

import (
	"context"

	"task165-collation/internal/definitive"
	"task165-collation/internal/model"
)

// assemble compiles the definitive body of a project from its base passages
// and approved decisions. It returns the body, the integrity hash computed over
// the ordered base-passage hashes, and the ordered passage hashes themselves.
func (s *Service) assemble(ctx context.Context, projectID string) (body string, integrityHash string, passageHashes []model.PassageHash, err error) {
	base, err := s.baseWitness(ctx, projectID)
	if err != nil {
		return "", "", nil, err
	}
	passages, err := s.store.ListPassages(base.ID)
	if err != nil {
		return "", "", nil, err
	}
	approved, readingsByID, err := s.collectApproved(ctx, projectID)
	if err != nil {
		return "", "", nil, err
	}
	passageHashes = make([]model.PassageHash, 0, len(passages))
	for _, p := range passages {
		passageHashes = append(passageHashes, model.PassageHash{PassageID: p.ID, Hash: p.TextHash})
	}
	a := &definitive.Assembly{
		ProjectID:         projectID,
		BasePassages:      passages,
		ApprovedDecisions: approved,
		ReadingsByID:      readingsByID,
	}
	body, integrityHash, err = definitive.BuildBody(a)
	if err != nil {
		return "", "", nil, err
	}
	return body, integrityHash, passageHashes, nil
}

// freeze builds the body, its integrity hash and the immutable link set for a
// snapshot round. The link set carries the confirmed anchors, the approved
// decisions and the ordered base-passage hashes, so the published snapshot can
// later recompute its integrity hash from the frozen links and answer which
// anchor/decision/reading produced each character even after the source
// witnesses are supplemented.
func (s *Service) freeze(ctx context.Context, projectID, snapshotID string) (body string, integrityHash string, links []*model.SnapshotLink, err error) {
	body, integrityHash, passageHashes, err := s.assemble(ctx, projectID)
	if err != nil {
		return "", "", nil, err
	}
	anchors, err := s.store.ListConfirmedAnchors(projectID)
	if err != nil {
		return "", "", nil, err
	}
	approved, _, err := s.collectApproved(ctx, projectID)
	if err != nil {
		return "", "", nil, err
	}
	links = definitive.FreezeLinks(snapshotID, &definitive.Assembly{
		ProjectID:         projectID,
		ApprovedDecisions: approved,
		ConfirmedAnchors:  anchors,
	}, passageHashes)
	return body, integrityHash, links, nil
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

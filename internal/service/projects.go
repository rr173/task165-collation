package service

import (
	"context"
	"time"

	"task165-collation/internal/model"
)

// CreateProject creates a new draft collation project.
func (s *Service) CreateProject(ctx context.Context, name, description string) (*model.Project, error) {
	p := &model.Project{ID: NewID(), Name: name, Description: description, Status: model.ProjectDraft, Version: 1}
	if err := s.store.CreateProject(p); err != nil {
		return nil, translate(err)
	}
	return p, nil
}

// GetProject loads a project by id.
func (s *Service) GetProject(ctx context.Context, id string) (*model.Project, error) {
	p, err := s.store.GetProject(id)
	if err != nil {
		return nil, translate(err)
	}
	return p, nil
}

// ListProjects returns all projects.
func (s *Service) ListProjects(ctx context.Context) ([]*model.Project, error) {
	return s.store.ListProjects()
}

// TransitionProject applies a state-machine transition with optimistic
// locking. Draft→Aligning is allowed once a base witness exists.
func (s *Service) TransitionProject(ctx context.Context, id string, expectVersion int64, status model.ProjectStatus) (*model.Project, error) {
	cur, err := s.store.GetProject(id)
	if err != nil {
		return nil, translate(err)
	}
	if cur.Version != expectVersion {
		return nil, model.ErrConflict
	}
	if !validProjectTransition(cur.Status, status) {
		return nil, model.ErrInvalidState
	}
	if status == model.ProjectAligning && cur.BaseWitnessID == "" {
		return nil, model.ErrInvalidState
	}
	if err := s.store.UpdateProjectStatus(id, expectVersion, status); err != nil {
		return nil, translate(err)
	}
	return s.store.GetProject(id)
}

func validProjectTransition(cur, next model.ProjectStatus) bool {
	switch cur {
	case model.ProjectDraft:
		return next == model.ProjectAligning
	case model.ProjectAligning:
		return next == model.ProjectReviewable || next == model.ProjectDraft || next == model.ProjectArchived
	case model.ProjectReviewable:
		return next == model.ProjectPublished || next == model.ProjectArchived
	case model.ProjectPublished:
		return next == model.ProjectArchived
	case model.ProjectArchived:
		return false
	}
	return false
}

// Stats aggregates counts across the whole store for the dashboard.
type Stats struct {
	Projects     int `json:"projects"`
	Witnesses    int `json:"witnesses"`
	Passages     int `json:"passages"`
	Anchors      int `json:"anchors"`
	Variants     int `json:"variants"`
	Decisions    int `json:"decisions"`
	Snapshots    int `json:"snapshots"`
	RoundNo      int `json:"round_no"`
	ReportedAt   time.Time `json:"reported_at"`
}

// ProjectStats returns aggregate counters for a project.
func (s *Service) ProjectStats(ctx context.Context, projectID string) (*Stats, error) {
	st := &Stats{ReportedAt: time.Now().UTC()}
	var err error
	if st.Variants, err = s.store.VariantCount(projectID); err != nil {
		return nil, err
	}
	if st.Snapshots, err = countSnapshots(s.store, projectID); err != nil {
		return nil, err
	}
	if st.RoundNo, err = s.store.NextRoundNo(projectID); err != nil {
		return nil, err
	}
	st.Witnesses, _ = s.store.WitnessCount(projectID)
	return st, nil
}

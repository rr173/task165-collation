package service

import (
	"context"
	"errors"

	"task165-collation/internal/definitive"
	"task165-collation/internal/model"
)

// PreviewDefinitive assembles the candidate definitive body from approved
// decisions without persisting a snapshot. It is the "candidate preview"
// entry point for the UI.
func (s *Service) PreviewDefinitive(ctx context.Context, projectID string) (string, error) {
	body, _, err := s.assemble(ctx, projectID)
	return body, err
}

// BuildSnapshot freezes the current approved decisions into a snapshot and
// persists it in pending state. Subsequent publishing freezes it read-only.
func (s *Service) BuildSnapshot(ctx context.Context, projectID string) (*model.Snapshot, error) {
	// A published snapshot already exists for this round → must start a new
	// round, not silently rewrite.
	pub, err := s.store.LatestPublishedSnapshot(projectID)
	if err == nil && pub != nil {
		return nil, model.ErrRoundFrozen
	}
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	round, err := s.store.NextRoundNo(projectID)
	if err != nil {
		return nil, err
	}
	// Mark older in-flight snapshots as superseded.
	if err := s.store.SupersedeSnapshots(projectID); err != nil {
		return nil, err
	}
	snapshotID := NewID()
	body, hashes, links, err := s.freeze(ctx, projectID, snapshotID)
	if err != nil {
		return nil, err
	}
	sn := &model.Snapshot{
		ID:        snapshotID,
		ProjectID: projectID,
		RoundNo:   round,
		Status:    model.SnapshotPendingP,
		Title:     definitive.BuildSnapshotTitle(projectName(ctx, s, projectID), round),
		Body:      body,
		Version:   1,
	}
	if err := s.store.CreateSnapshot(sn, links); err != nil {
		return nil, translate(err)
	}
	_ = hashes
	return sn, nil
}

// PublishSnapshot transitions a pending snapshot to published (frozen). The
// frozen body, anchors and decision chain become read-only.
func (s *Service) PublishSnapshot(ctx context.Context, snapshotID string, expectVersion int64) (*model.Snapshot, error) {
	sn, err := s.store.GetSnapshot(snapshotID)
	if err != nil {
		return nil, translate(err)
	}
	if sn.Status != model.SnapshotPendingP {
		return nil, model.ErrInvalidState
	}
	if sn.Version != expectVersion {
		return nil, model.ErrConflict
	}
	if err := s.store.PublishSnapshot(snapshotID, expectVersion); err != nil {
		return nil, translate(err)
	}
	// The project becomes published.
	p, err := s.store.GetProject(sn.ProjectID)
	if err == nil {
		_ = s.store.UpdateProjectStatus(sn.ProjectID, p.Version, model.ProjectPublished)
	}
	return s.store.GetSnapshot(snapshotID)
}

// ListSnapshots returns snapshots of a project.
func (s *Service) ListSnapshots(ctx context.Context, projectID string) ([]*model.Snapshot, error) {
	return s.store.ListSnapshots(projectID)
}

// GetSnapshot returns a snapshot with its frozen links (the read-only view).
func (s *Service) GetSnapshot(ctx context.Context, snapshotID string) (*definitive.PublishedView, error) {
	sn, err := s.store.GetSnapshot(snapshotID)
	if err != nil {
		return nil, translate(err)
	}
	links, err := s.store.ListSnapshotLinks(snapshotID)
	if err != nil {
		return nil, err
	}
	view := definitive.SummarizeView(sn, links, "")
	return view, nil
}

// RollbackToSnapshot serves the published body of the earliest published
// snapshot, emulating "rollback to a published version".
func (s *Service) RollbackToSnapshot(ctx context.Context, projectID string) (*model.Snapshot, error) {
	snaps, err := s.store.ListSnapshots(projectID)
	if err != nil {
		return nil, err
	}
	for _, sn := range snaps {
		if sn.Status == model.SnapshotPublished {
			return sn, nil
		}
	}
	return nil, model.ErrNotFound
}

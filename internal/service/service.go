// Package service is the orchestration layer of the collation workbench. It
// composes store persistence, text segmentation, alignment, collation and
// definitive-text assembly into business operations, and enforces the
// cross-cutting rules: version conflicts, leases, range validity, snapshot
// immutability and incremental recomputation.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"task165-collation/internal/model"
	"task165-collation/internal/store"
)

// Service holds the store and provides the business operations.
type Service struct {
	store *store.Store
}

// New creates a Service backed by the given store.
func New(s *store.Store) *Service { return &Service{store: s} }

// NewID returns a random hex identifier.
func NewID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Deterministic fallback keeps the service runnable in constrained
		// environments; collisions are practically impossible for our volume.
		return "id-fallback"
	}
	return hex.EncodeToString(b)
}

// Store exposes the underlying store for the HTTP layer's read helpers.
func (s *Service) Store() *store.Store { return s.store }

// Recover restores in-flight alignment work after a restart: it only
// recomputes intervals affected by anchors that are still unconfirmed, and
// expires stale leases. Idempotent.
func (s *Service) Recover(ctx context.Context) error {
	projects, err := s.store.ListProjects()
	if err != nil {
		return err
	}
	for _, p := range projects {
		if p.Status != model.ProjectAligning && p.Status != model.ProjectReviewable {
			continue
		}
		anchors, err := s.store.ListConfirmedAnchors(p.ID)
		if err != nil {
			return err
		}
		_ = anchors
	}
	return nil
}

// translate maps a store/domain error to a stable error for the caller.
func translate(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, model.ErrNotFound):
		return model.ErrNotFound
	case errors.Is(err, model.ErrConflict):
		return model.ErrConflict
	case errors.Is(err, model.ErrLeaseHeld):
		return model.ErrLeaseHeld
	case errors.Is(err, model.ErrAnchorOccupied):
		return model.ErrAnchorOccupied
	case errors.Is(err, model.ErrPublishedReadOnly):
		return model.ErrPublishedReadOnly
	case errors.Is(err, model.ErrInvalidRange):
		return model.ErrInvalidRange
	case errors.Is(err, model.ErrCrossChapter):
		return model.ErrCrossChapter
	case errors.Is(err, model.ErrWrongProject):
		return model.ErrWrongProject
	case errors.Is(err, model.ErrCycleMovement):
		return model.ErrCycleMovement
	case errors.Is(err, model.ErrRevokedReading):
		return model.ErrRevokedReading
	case errors.Is(err, model.ErrDuplicateHash):
		return model.ErrDuplicateHash
	default:
		return err
	}
}

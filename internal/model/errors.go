package model

import "errors"

// Domain errors shared across packages. The HTTP layer maps them to status
// codes; the store layer may wrap them with extra context.
var (
	ErrNotFound          = errors.New("record not found")
	ErrConflict          = errors.New("version conflict")
	ErrInvalidState      = errors.New("invalid state transition")
	ErrInvalidRange      = errors.New("invalid character range")
	ErrCrossChapter      = errors.New("anchor crosses chapters")
	ErrWrongProject      = errors.New("witness does not belong to project")
	ErrLeaseHeld         = errors.New("variant lease held by another editor")
	ErrLeaseExpired      = errors.New("variant lease expired")
	ErrPublishedReadOnly = errors.New("published snapshot is read-only")
	ErrDuplicateHash     = errors.New("text hash collision with different bibliographic info")
	ErrCycleMovement     = errors.New("cyclic movement relation detected")
	ErrRevokedReading    = errors.New("decision references a revoked reading")
	ErrRoundFrozen       = errors.New("a published snapshot already exists for this round")
	ErrAnchorOccupied    = errors.New("passage already occupied by an unconfirmed anchor")
)

// ConflictDetail carries enough context for the client to resolve a version
// conflict instead of silently losing work.
type ConflictDetail struct {
	Entity          string `json:"entity"`
	ID              string `json:"id"`
	ExpectedVersion int64  `json:"expected_version"`
	ActualVersion   int64  `json:"actual_version"`
	LatestDecision  string `json:"latest_decision,omitempty"`
	Message         string `json:"message"`
}

// Error implements the error interface so ConflictDetail can be returned from
// service methods and mapped to a 409 by the HTTP layer.
func (c *ConflictDetail) Error() string {
	if c.Message != "" {
		return c.Message
	}
	return "version conflict on " + c.Entity + " " + c.ID
}

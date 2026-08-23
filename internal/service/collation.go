package service

import (
	"context"
	"errors"
	"time"

	"task165-collation/internal/collate"
	"task165-collation/internal/model"
)

// ClaimVariant acquires a short lease on a variant for an editor. If the
// variant is claimed by another editor with an unexpired lease, model.
// ErrLeaseHeld is returned with the holder name.
func (s *Service) ClaimVariant(ctx context.Context, variantID, editor string, leaseSeconds int64, expectLease int64) (*collate.ClaimResult, error) {
	v, err := s.store.GetVariant(variantID)
	if err != nil {
		return nil, translate(err)
	}
	chk := collate.ValidClaimState(v, editor, expectLease)
	if !chk.OK {
		return chk, nil
	}
	if err := s.store.ClaimVariant(variantID, editor, leaseSeconds, expectLease); err != nil {
		if errors.Is(err, model.ErrLeaseHeld) {
			return &collate.ClaimResult{OK: false, HeldBy: v.ClaimedBy, Reason: "lease held"}, nil
		}
		return nil, translate(err)
	}
	now := time.Now().UTC()
	exp := now.Add(time.Duration(leaseSeconds) * time.Second)
	return &collate.ClaimResult{OK: true, Lease: expectLease + 1, ExpiresAt: &exp}, nil
}

// ReleaseVariant releases the lease owned by an editor.
func (s *Service) ReleaseVariant(ctx context.Context, variantID, editor string) error {
	if err := s.store.ReleaseVariant(variantID, editor); err != nil {
		return translate(err)
	}
	return nil
}

// ListVariants returns variants of a project, optionally filtered by status.
func (s *Service) ListVariants(ctx context.Context, projectID, status string) ([]*model.Variant, error) {
	return s.store.ListVariants(projectID, status)
}

// GetVariant loads a variant with its readings attached.
func (s *Service) GetVariant(ctx context.Context, variantID string) (*model.Variant, []*model.Reading, error) {
	v, err := s.store.GetVariant(variantID)
	if err != nil {
		return nil, nil, translate(err)
	}
	readings, err := s.store.ListReadings(variantID)
	if err != nil {
		return nil, nil, err
	}
	return v, readings, nil
}

// ProposeReading adds a candidate reading for a variant. The reading must
// come from a witness of the same project.
func (s *Service) ProposeReading(ctx context.Context, variantID, witnessID, text string, diffType model.DiffType) (*model.Reading, error) {
	v, err := s.store.GetVariant(variantID)
	if err != nil {
		return nil, translate(err)
	}
	w, err := s.store.GetWitnessFromAny(witnessID)
	if err != nil {
		return nil, translate(err)
	}
	if v.ProjectID != w.ProjectID {
		return nil, model.ErrWrongProject
	}
	// Deduplicate: an editor may not propose the same witness reading twice.
	existing, err := s.store.ListReadings(variantID)
	if err != nil {
		return nil, err
	}
	if collate.ReadingForWitness(existing, witnessID) != nil {
		return nil, model.ErrConflict
	}
	r := &model.Reading{
		ID:        NewID(),
		VariantID: variantID,
		WitnessID: witnessID,
		Text:      text,
		DiffType:  diffType,
	}
	if err := s.store.CreateReading(r); err != nil {
		return nil, translate(err)
	}
	return r, nil
}

// ProposeDecision records a proposed decision adopting a reading. The reading
// must belong to the variant, and the base version of the variant must match
// to avoid silently overwriting a colleague's decision.
func (s *Service) ProposeDecision(ctx context.Context, req collate.DecideRequest) (*model.Decision, error) {
	v, err := s.store.GetVariant(req.VariantID)
	if err != nil {
		return nil, translate(err)
	}
	if v.Version != req.ExpectedVersion {
		latest, _ := s.store.GetDecisionByVariant(req.VariantID)
		return nil, &model.ConflictDetail{
			Entity:         "variant",
			ID:             req.VariantID,
			ExpectedVersion: req.ExpectedVersion,
			ActualVersion:  v.Version,
			LatestDecision: decisionSummary(latest),
			Message:        "variant version changed since you read it",
		}
	}
	reading, err := s.store.GetReading(req.ReadingID)
	if err != nil {
		return nil, translate(err)
	}
	if reading.VariantID != req.VariantID {
		return nil, model.ErrRevokedReading
	}
	d := &model.Decision{
		ID:                NewID(),
		VariantID:         req.VariantID,
		ReadingID:         req.ReadingID,
		Status:            model.DecisionProposed,
		Reason:            req.Reason,
		DecidedBy:         req.DecidedBy,
		EvidenceWitnessID: req.EvidenceWitnessID,
		Version:           1,
	}
	if err := s.store.CreateDecision(d); err != nil {
		return nil, translate(err)
	}
	if err := s.store.UpdateVariantStatus(req.VariantID, v.Version, model.VariantPending); err != nil {
		return nil, translate(err)
	}
	return d, nil
}

// ReviewDecision approves or rejects a proposed decision. Only a decision in
// proposed state can be reviewed.
func (s *Service) ReviewDecision(ctx context.Context, req collate.ReviewRequest) (*model.Decision, error) {
	d, err := s.store.GetDecision(req.DecisionID)
	if err != nil {
		return nil, translate(err)
	}
	if d.Version != req.ExpectedVersion {
		return nil, model.ErrConflict
	}
	next := model.DecisionRejected
	if req.Approve {
		next = model.DecisionApproved
	}
	if !collate.ValidDecisionTransition(d.Status, next) {
		return nil, model.ErrInvalidState
	}
	// Traceability gate: an approved decision must reference a real reading.
	v, err := s.store.GetVariant(d.VariantID)
	if err != nil {
		return nil, translate(err)
	}
	readings, err := s.store.ListReadings(d.VariantID)
	if err != nil {
		return nil, err
	}
	if !collate.Traceable(v, d, readings) {
		return nil, model.ErrRevokedReading
	}
	if err := s.store.UpdateDecisionStatus(req.DecisionID, req.ExpectedVersion, next); err != nil {
		return nil, translate(err)
	}
	// Reflect approval in the variant status.
	if next == model.DecisionApproved {
		v2, _ := s.store.GetVariant(d.VariantID)
		if v2 != nil {
			_ = s.store.UpdateVariantStatus(d.VariantID, v2.Version, model.VariantDecided)
		}
	}
	return s.store.GetDecision(req.DecisionID)
}

// WithdrawDecision withdraws a decision (proposed or approved).
func (s *Service) WithdrawDecision(ctx context.Context, decisionID string, expectVersion int64) error {
	d, err := s.store.GetDecision(decisionID)
	if err != nil {
		return translate(err)
	}
	if d.Version != expectVersion {
		return model.ErrConflict
	}
	if !collate.ValidDecisionTransition(d.Status, model.DecisionWithdrawn) {
		return model.ErrInvalidState
	}
	if err := s.store.UpdateDecisionStatus(decisionID, expectVersion, model.DecisionWithdrawn); err != nil {
		return translate(err)
	}
	return nil
}

// TraceVariant builds the full evidence chain of a variant.
func (s *Service) TraceVariant(ctx context.Context, variantID string) (*collate.TraceChain, error) {
	v, err := s.store.GetVariant(variantID)
	if err != nil {
		return nil, translate(err)
	}
	readings, err := s.store.ListReadings(variantID)
	if err != nil {
		return nil, err
	}
	decisions, err := s.store.ListDecisions(variantID)
	if err != nil {
		return nil, err
	}
	tc := &collate.TraceChain{Variant: v, Readings: readings}
	if approved := collate.ApprovedDecision(decisions); approved != nil {
		tc.Decision = approved
		tc.Reason = approved.Reason
		tc.DecidedBy = approved.DecidedBy
		if approved.EvidenceWitnessID != "" {
			if w, err := s.store.GetWitnessFromAny(approved.EvidenceWitnessID); err == nil {
				tc.EvidenceWitness = w.Code
			}
		}
		for _, r := range readings {
			if r.ID == approved.ReadingID {
				tc.AdoptedReading = r
				break
			}
		}
	}
	return tc, nil
}

func decisionSummary(d *model.Decision) string {
	if d == nil {
		return ""
	}
	return string(d.Status) + ":" + d.Reason
}

// WaitForLeaseExpiry is a helper for tests that need to observe lease
// expiration deterministically; it blocks until the lease's expiry.
func WaitForLeaseExpiry(ctx context.Context, exp time.Time) {
	if time.Now().Before(exp) {
		time.Sleep(time.Until(exp))
	}
}

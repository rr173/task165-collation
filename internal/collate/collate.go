// Package collate implements the editorial layer: variant loci, candidate
// readings, decision state machine and traceability checks. It enforces the
// business rules about who may claim a variant, when a decision can be
// approved, and how the final text must be traceable to a concrete witness
// reading and decision.
package collate

import (
	"time"

	"task165-collation/internal/model"
)

// ClaimRequest describes an editor trying to claim a variant for editing.
type ClaimRequest struct {
	Editor        string
	LeaseSeconds  int64
	ExpectedLease int64 // optimistic version guard
}

// ClaimResult reports the outcome of a claim attempt.
type ClaimResult struct {
	OK        bool   `json:"ok"`
	Lease     int64  `json:"lease"`
	HeldBy    string `json:"held_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// DecideRequest carries everything needed to propose a decision adopting a
// reading.
type DecideRequest struct {
	VariantID         string
	ReadingID         string
	Reason            string
	DecidedBy         string
	EvidenceWitnessID string
	ExpectedVersion   int64
}

// ReviewRequest approves or rejects a proposed decision.
type ReviewRequest struct {
	DecisionID     string
	Reviewer       string
	Approve        bool
	Comment        string
	ExpectedVersion int64
}

// TraceChain is the full evidence chain of a variant: its readings, the
// adopted reading, the decision, and the evidence witness.
type TraceChain struct {
	Variant         *model.Variant   `json:"variant"`
	Readings        []*model.Reading `json:"readings"`
	AdoptedReading  *model.Reading   `json:"adopted_reading,omitempty"`
	Decision        *model.Decision  `json:"decision,omitempty"`
	EvidenceWitness string           `json:"evidence_witness,omitempty"`
	DecidedBy       string           `json:"decided_by,omitempty"`
	Reason          string           `json:"reason,omitempty"`
}

// ValidClaimState checks that a claim is only made on an unhandled or expired
// variant. It is a pure helper used by the service layer before touching the
// store.
func ValidClaimState(v *model.Variant, editor string, expectLease int64) *ClaimResult {
	if v.LeaseVersion != expectLease {
		return &ClaimResult{OK: false, Reason: "lease version changed"}
	}
	if v.ClaimedBy != "" && v.ClaimedBy != editor {
		if v.LeaseExpiresAt != nil && time.Now().Before(*v.LeaseExpiresAt) {
			return &ClaimResult{OK: false, HeldBy: v.ClaimedBy, Reason: "lease held"}
		}
	}
	return &ClaimResult{OK: true}
}

// ReadingForWitness returns the reading of a variant proposed by a witness, or
// nil. Used to deduplicate readings.
func ReadingForWitness(readings []*model.Reading, witnessID string) *model.Reading {
	for _, r := range readings {
		if r.WitnessID == witnessID {
			return r
		}
	}
	return nil
}

// ApprovedDecision returns the approved decision among a variant's decisions.
func ApprovedDecision(decisions []*model.Decision) *model.Decision {
	for _, d := range decisions {
		if d.Status == model.DecisionApproved {
			return d
		}
	}
	return nil
}

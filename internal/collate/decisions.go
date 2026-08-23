package collate

import (
	"task165-collation/internal/model"
	"task165-collation/internal/text"
)

// ApplyDecision determines the definitive text contribution of a variant given
// its adopted reading. For word/addition variants the reading text replaces
// the base range; for omissions the base range is deleted (independent of the
// reading, which may be a deletion marker); for movements the reading text is
// inserted at the base position.
func ApplyDecision(v *model.Variant, reading *model.Reading, baseText string) string {
	switch v.DiffType {
	case model.DiffOmission:
		return text.SliceByRunes(baseText, 0, v.BaseStartChar) +
			text.SliceByRunes(baseText, v.BaseEndChar, text.CharCount(baseText))
	case model.DiffWord, model.DiffAddition:
		if reading == nil {
			return baseText
		}
		return text.SliceByRunes(baseText, 0, v.BaseStartChar) +
			reading.Text +
			text.SliceByRunes(baseText, v.BaseEndChar, text.CharCount(baseText))
	case model.DiffMovement:
		if reading == nil {
			return baseText
		}
		return baseText + reading.Text
	default:
		return baseText
	}
}

// Traceable reports whether a decision is fully traceable: it must reference a
// reading of the same variant, and the reading must belong to a witness of the
// project. The reason field must be non-empty for approval.
func Traceable(v *model.Variant, d *model.Decision, readings []*model.Reading) bool {
	if v == nil || d == nil {
		return false
	}
	if d.ReadingID == "" {
		return false
	}
	r := findReading(readings, d.ReadingID)
	if r == nil {
		return false
	}
	if r.VariantID != v.ID {
		return false
	}
	return true
}

// ValidDecisionTransition validates a decision state transition.
func ValidDecisionTransition(current, next model.DecisionStatus) bool {
	switch current {
	case model.DecisionProposed:
		return next == model.DecisionApproved || next == model.DecisionRejected || next == model.DecisionWithdrawn
	case model.DecisionApproved:
		return next == model.DecisionWithdrawn
	case model.DecisionRejected, model.DecisionWithdrawn:
		return false
	}
	return false
}

// ValidVariantTransition validates a variant status transition.
func ValidVariantTransition(current, next model.VariantStatus) bool {
	switch current {
	case model.VariantUnhandled:
		return next == model.VariantClaimed || next == model.VariantSuperseded
	case model.VariantClaimed:
		return next == model.VariantPending || next == model.VariantUnhandled || next == model.VariantSuperseded
	case model.VariantPending:
		return next == model.VariantDecided || next == model.VariantSuperseded
	case model.VariantDecided:
		return next == model.VariantSuperseded
	case model.VariantSuperseded:
		return false
	}
	return false
}

// ProjectReviewable reports whether every variant of the project has an
// approved decision. Decided-but-not-approved variants block reviewability.
func ProjectReviewable(total, approved int) bool {
	return total > 0 && approved >= total
}

func findReading(readings []*model.Reading, id string) *model.Reading {
	for _, r := range readings {
		if r.ID == id {
			return r
		}
	}
	return nil
}

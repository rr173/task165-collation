package collate

import (
	"task165-collation/internal/model"
)

// ValidateWitnessProject rejects a witness that does not belong to the project
// referenced by a variant.
func ValidateWitnessProject(v *model.Variant, witness *model.Witness) error {
	if v == nil || witness == nil {
		return model.ErrWrongProject
	}
	if v.ProjectID != witness.ProjectID {
		return model.ErrWrongProject
	}
	return nil
}

// ValidateVariantRange checks that a variant's character range is valid and
// non-empty, and refuses zero-length ranges used by stray imports.
func ValidateVariantRange(start, end int) error {
	if start < 0 || end < start {
		return model.ErrInvalidRange
	}
	if end == start {
		return model.ErrInvalidRange
	}
	return nil
}

// ValidateAnchorScope rejects anchors that would cross chapters: the base
// passage and the witness passage must belong to chapters with matching
// ordinals.
func ValidateAnchorScope(baseChapter, witnessChapter *model.Chapter) error {
	if baseChapter == nil || witnessChapter == nil {
		return nil
	}
	if baseChapter.Ordinal != witnessChapter.Ordinal {
		return model.ErrCrossChapter
	}
	return nil
}

// ValidateMovement rejects cyclic movement relations before they are
// persisted. invertedEdges is the output of the align movement detector.
func ValidateMovement(invertedCount int, hasCycle bool) error {
	if hasCycle {
		return model.ErrCycleMovement
	}
	return nil
}

// ValidateSnapshotMutation rejects direct modification of a published
// snapshot: the business rule is that published bodies are frozen and any
// further work must happen in a new round.
func ValidateSnapshotMutation(sn *model.Snapshot) error {
	if sn == nil {
		return model.ErrNotFound
	}
	if sn.Status == model.SnapshotPublished {
		return model.ErrPublishedReadOnly
	}
	return nil
}

// ValidateAdoptedReading rejects a decision that adopts a reading which has
// been withdrawn (removed from the variant's active reading set).
func ValidateAdoptedReading(readings []*model.Reading, adoptedID string) error {
	for _, r := range readings {
		if r.ID == adoptedID {
			return nil
		}
	}
	return model.ErrRevokedReading
}

package collate

import (
	"testing"

	"task165-collation/internal/model"
)

func TestValidDecisionTransition(t *testing.T) {
	if !ValidDecisionTransition(model.DecisionProposed, model.DecisionApproved) {
		t.Fatal("proposed->approved must be valid")
	}
	if !ValidDecisionTransition(model.DecisionProposed, model.DecisionRejected) {
		t.Fatal("proposed->rejected must be valid")
	}
	if ValidDecisionTransition(model.DecisionRejected, model.DecisionApproved) {
		t.Fatal("rejected->approved must be invalid")
	}
	if !ValidDecisionTransition(model.DecisionApproved, model.DecisionWithdrawn) {
		t.Fatal("approved->withdrawn must be valid")
	}
}

func TestValidVariantTransition(t *testing.T) {
	if !ValidVariantTransition(model.VariantUnhandled, model.VariantClaimed) {
		t.Fatal("unhandled->claimed valid")
	}
	if !ValidVariantTransition(model.VariantClaimed, model.VariantPending) {
		t.Fatal("claimed->pending valid")
	}
	if ValidVariantTransition(model.VariantSuperseded, model.VariantClaimed) {
		t.Fatal("superseded is terminal")
	}
}

func TestTraceable(t *testing.T) {
	v := &model.Variant{ID: "v1"}
	r := &model.Reading{ID: "r1", VariantID: "v1"}
	d := &model.Decision{ID: "d1", VariantID: "v1", ReadingID: "r1"}
	if !Traceable(v, d, []*model.Reading{r}) {
		t.Fatal("expected traceable")
	}
	d2 := &model.Decision{ID: "d2", VariantID: "v1", ReadingID: "missing"}
	if Traceable(v, d2, []*model.Reading{r}) {
		t.Fatal("missing reading must not be traceable")
	}
}

func TestProjectReviewable(t *testing.T) {
	if !ProjectReviewable(3, 3) {
		t.Fatal("3/3 must be reviewable")
	}
	if ProjectReviewable(3, 2) {
		t.Fatal("2/3 must not be reviewable")
	}
	if ProjectReviewable(0, 0) {
		t.Fatal("empty project must not be reviewable")
	}
}

func TestValidateWitnessProject(t *testing.T) {
	v := &model.Variant{ProjectID: "p1"}
	w := &model.Witness{ProjectID: "p1"}
	if err := ValidateWitnessProject(v, w); err != nil {
		t.Fatalf("same project must pass: %v", err)
	}
	w2 := &model.Witness{ProjectID: "p2"}
	if err := ValidateWitnessProject(v, w2); err != model.ErrWrongProject {
		t.Fatalf("expected ErrWrongProject, got %v", err)
	}
}

func TestValidateVariantRange(t *testing.T) {
	if err := ValidateVariantRange(1, 3); err != nil {
		t.Fatalf("valid range rejected: %v", err)
	}
	if err := ValidateVariantRange(3, 1); err != model.ErrInvalidRange {
		t.Fatalf("expected invalid range")
	}
	if err := ValidateVariantRange(1, 1); err != model.ErrInvalidRange {
		t.Fatalf("expected invalid empty range")
	}
}

func TestValidateSnapshotMutation(t *testing.T) {
	if err := ValidateSnapshotMutation(&model.Snapshot{Status: model.SnapshotBuilding}); err != nil {
		t.Fatal("building snapshot is mutable")
	}
	if err := ValidateSnapshotMutation(&model.Snapshot{Status: model.SnapshotPublished}); err != model.ErrPublishedReadOnly {
		t.Fatalf("expected ErrPublishedReadOnly, got %v", err)
	}
}

func TestValidateAdoptedReading(t *testing.T) {
	rs := []*model.Reading{{ID: "r1"}, {ID: "r2"}}
	if err := ValidateAdoptedReading(rs, "r1"); err != nil {
		t.Fatal("existing reading must pass")
	}
	if err := ValidateAdoptedReading(rs, "r9"); err != model.ErrRevokedReading {
		t.Fatalf("expected revoked reading error")
	}
}

func TestApplyDecision(t *testing.T) {
	base := "学而时习之，不亦说乎"
	// 学(0)而(1)时(2)习(3)之(4)，(5)不(6)亦(7)说(8)乎(9)
	v := &model.Variant{BaseStartChar: 8, BaseEndChar: 9, DiffType: model.DiffWord}
	r := &model.Reading{Text: "悦"}
	out := ApplyDecision(v, r, base)
	if out != "学而时习之，不亦悦乎" {
		t.Fatalf("apply word: got %q", out)
	}
	// Omission deletes the range [0,7): 学(0)而(1)时(2)习(3)之(4)，(5)不(6)
	v2 := &model.Variant{BaseStartChar: 0, BaseEndChar: 7, DiffType: model.DiffOmission}
	out2 := ApplyDecision(v2, nil, base)
	if out2 != "亦说乎" {
		t.Fatalf("apply omission: got %q", out2)
	}
}

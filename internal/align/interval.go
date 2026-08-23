package align

import (
	"task165-collation/internal/model"
	"task165-collation/internal/text"
)

// VariantProposal is a concrete difference to be persisted as a variant locus.
type VariantProposal struct {
	Variant *model.Variant
	// AltText is the witness reading text, empty when the variant is an
	// omission (base text missing in the witness).
	AltText string
	// WitnessID is the witness that exhibits the difference.
	WitnessID string
	// PassageID is the witness passage carrying the reading (empty for pure
	// omissions at passage granularity).
	PassageID string
}

// AlignInterval compares every witness passage inside an interval with the
// corresponding base passage. When the number of passages matches, alignment
// is positional; otherwise extra/missing passages are classified as additions
// or omissions and the remaining positional pairs are diffed.
func AlignInterval(iv *Interval, projectID string) []VariantProposal {
	var out []VariantProposal
	base := iv.BasePassages
	for wid, ws := range iv.WitnessPassages {
		out = append(out, alignWitness(projectID, base, ws, wid)...)
	}
	return out
}

func alignWitness(projectID string, base, ws []*model.Passage, wid string) []VariantProposal {
	var out []VariantProposal
	// Movement check: if the base and witness have the same passages but in a
	// different order, emit a movement variant for the first discrepancy.
	if len(base) > 1 && len(ws) == len(base) && orderDiffers(base, ws) {
		out = append(out, VariantProposal{
			Variant: &model.Variant{
				ProjectID:     projectID,
				BasePassageID: base[0].ID,
				BaseStartChar: 0,
				BaseEndChar:   text.CharCount(base[0].Text),
				DiffType:      model.DiffMovement,
				Status:        model.VariantUnhandled,
			},
			WitnessID: wid,
		})
		return out
	}
	// Positional pairing on the shorter side; classify overflow as
	// addition/omission at passage granularity.
	n := len(base)
	if len(ws) < n {
		n = len(ws)
	}
	for i := 0; i < n; i++ {
		bp, wp := base[i], ws[i]
		if !text.HasDifference(bp.Text, wp.Text) {
			continue
		}
		diffs := text.DiffText(bp.Text, wp.Text)
		for _, d := range diffs {
			v := &model.Variant{
				ProjectID:     projectID,
				BasePassageID: bp.ID,
				BaseStartChar: d.Start,
				BaseEndChar:   d.End,
				DiffType:      classifyDiff(d),
				Status:        model.VariantUnhandled,
			}
			out = append(out, VariantProposal{
				Variant:   v,
				AltText:   d.Alt,
				WitnessID: wid,
				PassageID: wp.ID,
			})
		}
	}
	// Witness has extra passages → additions.
	for i := len(base); i < len(ws); i++ {
		wp := ws[i]
		out = append(out, VariantProposal{
			Variant: &model.Variant{
				ProjectID:     projectID,
				BasePassageID: "",
				BaseStartChar: 0,
				BaseEndChar:   0,
				DiffType:      model.DiffAddition,
				Status:        model.VariantUnhandled,
			},
			AltText:   wp.Text,
			WitnessID: wid,
			PassageID: wp.ID,
		})
	}
	// Base has extra passages → omissions.
	for i := len(ws); i < len(base); i++ {
		bp := base[i]
		out = append(out, VariantProposal{
			Variant: &model.Variant{
				ProjectID:     projectID,
				BasePassageID: bp.ID,
				BaseStartChar: 0,
				BaseEndChar:   text.CharCount(bp.Text),
				DiffType:      model.DiffOmission,
				Status:        model.VariantUnhandled,
			},
			WitnessID: wid,
		})
	}
	return out
}

// classifyDiff maps a character diff to a variant diff type: an empty base
// segment is an addition, an empty alt segment is an omission, otherwise a
// word difference.
func classifyDiff(d text.Diff) model.DiffType {
	if d.Base == "" && d.Alt != "" {
		return model.DiffAddition
	}
	if d.Alt == "" && d.Base != "" {
		return model.DiffOmission
	}
	return model.DiffWord
}

// orderDiffers reports whether the witness passage order differs from the base
// passage order when they have equal length (by ordinal).
func orderDiffers(base, ws []*model.Passage) bool {
	for i := range base {
		if base[i].Ordinal != ws[i].Ordinal {
			return true
		}
	}
	return false
}

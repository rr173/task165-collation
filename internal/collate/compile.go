package collate

import (
	"task165-collation/internal/model"
	"task165-collation/internal/text"
)

// CompilePassage builds the definitive text of one base passage by applying
// every variant decision that falls inside it, ordered by base range.
type passageEdit struct {
	start int
	end   int
	text  string
}

// CompileDefinitive assembles the definitive text for a set of base passages.
// Each passage starts from its base text; approved decisions inside the
// passage are applied in range order.
func CompileDefinitive(basePassages []*model.Passage, decisionsByPassage map[string][]editDecision) string {
	var out string
	for _, bp := range basePassages {
		edits := decisionsByPassage[bp.ID]
		out += applyEdits(bp.Text, edits)
	}
	return out
}

type editDecision struct {
	start   int
	end     int
	adopted string
}

// RegisterEdit converts an approved decision into an edit for the compiler.
func RegisterEdit(v *model.Variant, reading *model.Reading) editDecision {
	if v == nil {
		return editDecision{}
	}
	return editDecision{
		start:   v.BaseStartChar,
		end:     v.BaseEndChar,
		adopted: readingText(v, reading),
	}
}

// readingText returns the adopted text of a reading for a variant.
func readingText(v *model.Variant, reading *model.Reading) string {
	if reading == nil {
		return ""
	}
	switch v.DiffType {
	case model.DiffOmission:
		return ""
	default:
		return reading.Text
	}
}

// applyEdits applies non-overlapping edits sorted by start to the base text.
func applyEdits(base string, edits []editDecision) string {
	if len(edits) == 0 {
		return base
	}
	// Sort by start, stable for equal starts.
	for i := 1; i < len(edits); i++ {
		for j := i; j > 0 && edits[j].start < edits[j-1].start; j-- {
			edits[j], edits[j-1] = edits[j-1], edits[j]
		}
	}
	var out string
	cursor := 0
	for _, e := range edits {
		if e.start < cursor {
			continue // overlapping edit: keep the first
		}
		out += text.SliceByRunes(base, cursor, e.start)
		out += e.adopted
		cursor = e.end
	}
	out += text.SliceByRunes(base, cursor, text.CharCount(base))
	return out
}

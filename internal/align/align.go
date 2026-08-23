// Package align implements the alignment layer of the collation workbench:
// anchor candidates, interval alignment between consecutive confirmed anchors,
// movement detection and recomputation of only the intervals affected by
// added or revoked anchors.
package align

import (
	"sort"

	"task165-collation/internal/model"
	"task165-collation/internal/text"
)

// AnchorCandidate is a proposed pairing between a base passage and a witness
// passage, computed by head/tail word matching.
type AnchorCandidate struct {
	BasePassageID    string
	WitnessPassageID string
	WitnessID        string
	Score            int
}

// BuildCandidates matches base passages with witness passages whose first and
// last words agree. The returned candidates are sorted by score descending.
func BuildCandidates(basePassages, witnessPassages []*model.Passage) []AnchorCandidate {
	type key struct{ first, last string }
	index := make(map[key][]*model.Passage)
	for _, p := range witnessPassages {
		first, last := text.FirstLastWords(p.Text)
		index[key{first, last}] = append(index[key{first, last}], p)
	}
	var out []AnchorCandidate
	for _, bp := range basePassages {
		first, last := text.FirstLastWords(bp.Text)
		matches := index[key{first, last}]
		for _, wp := range matches {
			score := 2
			if text.Hash(bp.Text) == wp.TextHash {
				score = 5
			}
			out = append(out, AnchorCandidate{
				BasePassageID:    bp.ID,
				WitnessPassageID: wp.ID,
				WitnessID:        wp.WitnessID,
				Score:            score,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// Interval is a span of the base text between two anchors.
type Interval struct {
	BeforeAnchorID string
	AfterAnchorID  string
	BasePassages   []*model.Passage // base passages strictly between the anchors
	// WitnessPassages maps witnessID to the passages between the anchors for
	// that witness.
	WitnessPassages map[string][]*model.Passage
	// WitnessOrder records the ordinal order of each witness's passages so
	// movement can be detected.
	WitnessOrder map[string][]string // witnessID -> passage IDs in witness order
}

// BuildInterval assembles the passages of the base witness and every variant
// witness that lie strictly between two confirmed anchors (by ordinal).
func BuildInterval(base []*model.Passage, linksByWitness map[string][]*model.Passage) *Interval {
	iv := &Interval{WitnessPassages: make(map[string][]*model.Passage), WitnessOrder: make(map[string][]string)}
	for _, p := range base {
		iv.BasePassages = append(iv.BasePassages, p)
	}
	for wid, ps := range linksByWitness {
		sorted := append([]*model.Passage(nil), ps...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Ordinal < sorted[j].Ordinal })
		iv.WitnessPassages[wid] = sorted
		ids := make([]string, 0, len(sorted))
		for _, p := range sorted {
			ids = append(ids, p.ID)
		}
		iv.WitnessOrder[wid] = ids
	}
	return iv
}

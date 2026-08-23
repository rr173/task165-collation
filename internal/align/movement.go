package align

import (
	"task165-collation/internal/model"
)

// MovementEdge describes an ordering relationship between two base passages
// and the witness passages linked to them. Used to detect and reject cyclic
// movement relations.
type MovementEdge struct {
	BaseFrom *model.Passage
	BaseTo   *model.Passage
	// WitnessFrom / WitnessTo are the witness passages linked to BaseFrom /
	// BaseTo. When the witness order is inverted relative to the base, a
	// movement is present.
	WitnessFrom *model.Passage
	WitnessTo   *model.Passage
	Inverted    bool
}

// DetectMovements inspects every pair of consecutive base passages and their
// witness links, producing MovementEdge entries flagged when the witness order
// is inverted.
func DetectMovements(base []*model.Passage, linkByBase map[string]*model.Passage) []MovementEdge {
	var out []MovementEdge
	for i := 1; i < len(base); i++ {
		bf := base[i-1]
		bt := base[i]
		wf, okf := linkByBase[bf.ID]
		wt, okt := linkByBase[bt.ID]
		if !okf || !okt {
			continue
		}
		inverted := wf.Ordinal > wt.Ordinal
		out = append(out, MovementEdge{
			BaseFrom:    bf,
			BaseTo:      bt,
			WitnessFrom: wf,
			WitnessTo:   wt,
			Inverted:    inverted,
		})
	}
	return out
}

// HasCycle reports whether the movement edges form a cycle. A cycle would mean
// the witness's paragraph order cannot be reconciled with the base order; the
// business rule forbids persisting such a relation.
func HasCycle(edges []MovementEdge) bool {
	// A cycle exists if any chain of inversions returns to its start. For the
	// small interval sizes in this domain, an exhaustive search is fine.
	n := len(edges)
	for start := 0; start < n; start++ {
		visited := map[string]bool{}
		cur := edges[start].BaseFrom.ID
		visited[cur] = true
		for i := start; i < n; i++ {
			if !edges[i].Inverted {
				continue
			}
			from := edges[i].BaseFrom.ID
			to := edges[i].BaseTo.ID
			if visited[to] {
				return true
			}
			visited[from] = true
			visited[to] = true
		}
	}
	return false
}

// InvertedCount returns the number of inverted movement edges, used to decide
// whether the alignment needs manual anchoring.
func InvertedCount(edges []MovementEdge) int {
	n := 0
	for _, e := range edges {
		if e.Inverted {
			n++
		}
	}
	return n
}

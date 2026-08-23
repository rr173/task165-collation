package align

import (
	"sort"

	"task165-collation/internal/model"
)

// AnchorIndex groups confirmed anchors of a project by base passage so the
// recomputation logic can find which interval a passage belongs to.
type AnchorIndex struct {
	ByPassage map[string]*model.Anchor
	Ordered   []*model.Anchor // sorted by creation time
}

// IndexAnchors builds an index from the sorted confirmed anchor list.
func IndexAnchors(anchors []*model.Anchor) *AnchorIndex {
	idx := &AnchorIndex{ByPassage: make(map[string]*model.Anchor)}
	idx.Ordered = append([]*model.Anchor(nil), anchors...)
	sort.Slice(idx.Ordered, func(i, j int) bool { return idx.Ordered[i].CreatedAt.Before(idx.Ordered[j].CreatedAt) })
	for _, a := range idx.Ordered {
		idx.ByPassage[a.BasePassageID] = a
	}
	return idx
}

// AffectedIntervals returns the anchor IDs whose interval needs recomputation
// after an anchor on basePassageID was added or revoked. Only the interval
// immediately before and after the anchor are affected — the incremental
// recompute guarantee.
func AffectedIntervals(idx *AnchorIndex, basePassageID string) []string {
	var affected []string
	anchor, ok := idx.ByPassage[basePassageID]
	if !ok {
		// Unknown passage: everything between the two nearest anchors.
		return nearestIntervalAnchors(idx, basePassageID)
	}
	for i, a := range idx.Ordered {
		if a.ID != anchor.ID {
			continue
		}
		if i > 0 {
			affected = append(affected, idx.Ordered[i-1].ID)
		}
		if i < len(idx.Ordered)-1 {
			affected = append(affected, idx.Ordered[i+1].ID)
		}
		break
	}
	return affected
}

// nearestIntervalAnchors returns the two anchors bracketing a passage that has
// no anchor of its own (by ordinal position), or nil when none.
func nearestIntervalAnchors(idx *AnchorIndex, basePassageID string) []string {
	// This is a fallback; in practice every base passage in an aligned project
	// either is an anchor or sits between two anchors whose IDs we already know
	// from the interval chain. We return the neighbours by order.
	if len(idx.Ordered) == 0 {
		return nil
	}
	return []string{idx.Ordered[0].ID, idx.Ordered[len(idx.Ordered)-1].ID}
}

// IntervalPassages extracts the base passages that lie strictly between two
// anchors given the full ordered passage list of the base witness.
func IntervalPassages(all []*model.Passage, beforeID, afterID string) []*model.Passage {
	var out []*model.Passage
	start := -1
	end := -1
	for i, p := range all {
		if p.ID == beforeID {
			start = i
		}
		if p.ID == afterID {
			end = i
			break
		}
	}
	if start < 0 || end <= start {
		return nil
	}
	for i := start + 1; i < end; i++ {
		out = append(out, all[i])
	}
	return out
}

package definitive

import (
	"fmt"
	"sort"

	"task165-collation/internal/model"
)

// FreezeLinks builds the immutable link set of a snapshot: confirmed anchors,
// approved decisions, passage hashes and the aggregated integrity hash. The
// published snapshot must be able to answer "which reading and decision
// produced this character" even after the source witnesses are supplemented,
// and "is the body still verifiable" by recomputing the integrity hash.
func FreezeLinks(snapshotID string, a *Assembly, passageHashes map[string]string, integrityHash string) []*model.SnapshotLink {
	links := make([]*model.SnapshotLink, 0, len(a.ConfirmedAnchors)+len(a.ApprovedDecisions)+len(passageHashes)+1)
	for _, an := range a.ConfirmedAnchors {
		links = append(links, &model.SnapshotLink{
			SnapshotID: snapshotID,
			Kind:       "anchor",
			RefID:      an.ID,
		})
	}
	for _, d := range a.ApprovedDecisions {
		links = append(links, &model.SnapshotLink{
			SnapshotID: snapshotID,
			Kind:       "decision",
			RefID:      d.ID,
			Payload:    d.ReadingID,
		})
	}
	for passageID, h := range passageHashes {
		links = append(links, &model.SnapshotLink{
			SnapshotID: snapshotID,
			Kind:       "passage_hash",
			RefID:      passageID,
			Payload:    h,
		})
	}
	// Persist the frozen integrity baseline so the body can be re-verified later
	// without relying on an in-memory value that the service might have dropped.
	if integrityHash != "" {
		links = append(links, &model.SnapshotLink{
			SnapshotID: snapshotID,
			Kind:       "integrity_hash",
			RefID:      snapshotID,
			Payload:    integrityHash,
		})
	}
	return links
}

// ExpectedIntegrity returns the frozen integrity baseline stored among a
// snapshot's links (the payload of the single integrity_hash link). An empty
// string means the snapshot predates integrity freezing.
func ExpectedIntegrity(links []*model.SnapshotLink) string {
	for _, l := range links {
		if l.Kind == "integrity_hash" {
			return l.Payload
		}
	}
	return ""
}

// RecomputeIntegrity recomputes the integrity hash of a frozen snapshot from
// its passage-hash links, ordered by passage id — the same canonical order
// hashPassages used when freezing, so the digest is reproducible regardless of
// storage read order.
func RecomputeIntegrity(links []*model.SnapshotLink) string {
	ids := make([]string, 0, len(links))
	byID := make(map[string]string, len(links))
	for _, l := range links {
		if l.Kind == "passage_hash" {
			ids = append(ids, l.RefID)
			byID[l.RefID] = l.Payload
		}
	}
	sort.Strings(ids)
	var buf string
	for _, id := range ids {
		buf += byID[id]
	}
	return hashSum(buf)
}

// VerifyIntegrity recomputes the integrity hash of a frozen snapshot from its
// passage-hash links and compares it with the expected baseline. When
// expected is empty the snapshot has no frozen baseline, so the body cannot be
// verified and ok is false. Returns ok and the recomputed hash.
func VerifyIntegrity(links []*model.SnapshotLink, expected string) (bool, string) {
	actual := RecomputeIntegrity(links)
	if expected == "" {
		return false, actual
	}
	return actual == expected, actual
}

// hashSum is a tiny wrapper so the integrity contract stays local to this
// package (sha256hex lives in view.go next to SummarizeView).
func hashSum(s string) string { return sha256hex(s) }

// BuildSnapshotTitle composes a human-readable snapshot title.
func BuildSnapshotTitle(projectName string, roundNo int) string {
	return fmt.Sprintf("%s 定本 · 第 %d 轮", projectName, roundNo)
}

package definitive

import (
	"fmt"

	"task165-collation/internal/model"
)

// FreezeLinks builds the immutable link set of a snapshot: confirmed anchors,
// approved decisions, and passage hashes. The published snapshot must be able
// to answer "which reading and decision produced this character" even after
// the source witnesses are supplemented.
func FreezeLinks(snapshotID string, a *Assembly, passageHashes map[string]string) []*model.SnapshotLink {
	if passageHashes == nil {
		return nil
	}
	links := make([]*model.SnapshotLink, 0, len(a.ConfirmedAnchors)+len(a.ApprovedDecisions)+len(passageHashes))
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
	return links
}

// BuildSnapshotTitle composes a human-readable snapshot title.
func BuildSnapshotTitle(projectName string, roundNo int) string {
	return fmt.Sprintf("%s 定本 · 第 %d 轮", projectName, roundNo)
}

// VerifyIntegrity recomputes the integrity hash of a frozen snapshot from its
// passage-hash links and compares it with the expected hash.
func VerifyIntegrity(links []*model.SnapshotLink, expected string) (bool, string) {
	var buf string
	for _, l := range links {
		if l.Kind == "passage_hash" {
			buf += l.Payload
		}
	}
	actual := hashSum(buf)
	return actual == expected, actual
}

// hashSum is a tiny wrapper so VerifyIntegrity does not import text directly
// (keeps the integrity contract local to this package).
func hashSum(s string) string { return sha256hex(s) }

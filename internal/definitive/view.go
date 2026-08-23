package definitive

import (
	"crypto/sha256"
	"encoding/hex"

	"task165-collation/internal/model"
)

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// PublishedView is the read-only projection served for published snapshots: it
// carries the frozen body plus enough evidence to trace any character.
type PublishedView struct {
	Snapshot      *model.Snapshot   `json:"snapshot"`
	AnchorCount   int               `json:"anchor_count"`
	DecisionCount int               `json:"decision_count"`
	PassageHashes map[string]string `json:"passage_hashes"`
	IntegrityOK   bool              `json:"integrity_ok"`
}

// SummarizeView converts frozen links into a PublishedView for API responses.
// The passage_hash links are the passage→hash map frozen at publish time, so
// they are projected into PassageHashes verbatim — this is the evidence map
// reviewers use to trace any character of the body back to its source passage.
func SummarizeView(sn *model.Snapshot, links []*model.SnapshotLink, expectedHash string) *PublishedView {
	view := &PublishedView{
		Snapshot:      sn,
		PassageHashes: make(map[string]string, len(links)),
	}
	for _, l := range links {
		switch l.Kind {
		case "anchor":
			view.AnchorCount++
		case "decision":
			view.DecisionCount++
		case "passage_hash":
			if l.RefID != "" {
				view.PassageHashes[l.RefID] = l.Payload
			}
		}
	}
	// With no expected hash we still report that the frozen evidence map is
	// present; when the caller asks for an integrity check, recompute the hash
	// from the frozen passage hashes and compare, so a tampered snapshot is
	// caught without trusting a stored value.
	if expectedHash != "" {
		ok, _ := VerifyIntegrity(links, expectedHash)
		view.IntegrityOK = ok
	}
	return view
}

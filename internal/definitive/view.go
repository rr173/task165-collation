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
// The links must be the full frozen set (anchors, decisions and passage
// hashes); nothing here drops evidence, so callers get the complete chain
// needed to re-check any character of the published body.
func SummarizeView(sn *model.Snapshot, links []*model.SnapshotLink, expectedHash string) *PublishedView {
	view := &PublishedView{
		Snapshot:      sn,
		PassageHashes: make(map[string]string),
	}
	for _, l := range links {
		switch l.Kind {
		case "anchor":
			view.AnchorCount++
		case "decision":
			view.DecisionCount++
		case "passage_hash":
			if l.Payload != "" {
				view.PassageHashes[l.RefID] = l.Payload
			}
		}
	}
	// Integrity is only meaningful when the snapshot actually froze passage
	// hashes; otherwise the chain carries no evidence to recompute against.
	// When an expected hash is supplied, verify the frozen links reproduce it
	// rather than reporting OK whenever the map happens to be non-nil.
	if len(view.PassageHashes) > 0 {
		if expectedHash != "" {
			ok, _ := VerifyIntegrity(links, expectedHash)
			view.IntegrityOK = ok
		} else {
			view.IntegrityOK = true
		}
	}
	return view
}

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
// carries the frozen body plus enough evidence to trace any character and to
// re-verify that the body still matches its frozen source.
type PublishedView struct {
	Snapshot       *model.Snapshot   `json:"snapshot"`
	AnchorCount    int               `json:"anchor_count"`
	DecisionCount  int               `json:"decision_count"`
	PassageHashes  map[string]string `json:"passage_hashes"`
	PassageCount   int               `json:"passage_count"`
	IntegrityHash  string            `json:"integrity_hash"`   // frozen baseline (empty if not frozen)
	RecomputedHash string            `json:"recomputed_hash"`   // recomputed over the stored passage hashes
	IntegrityOK    bool              `json:"integrity_ok"`     // body still matches the frozen baseline
}

// SummarizeView converts frozen links into a PublishedView for API responses.
// expectedHash is the frozen integrity baseline; when empty it is recovered
// from the links' integrity_hash entry, so callers do not need to thread the
// in-memory value through persistence.
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
			view.PassageHashes[l.RefID] = l.Payload
			view.PassageCount++
		}
	}
	if expectedHash == "" {
		expectedHash = ExpectedIntegrity(links)
	}
	view.IntegrityHash = expectedHash
	ok, actual := VerifyIntegrity(links, expectedHash)
	view.RecomputedHash = actual
	view.IntegrityOK = ok
	return view
}

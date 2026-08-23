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
// carries the frozen body plus enough evidence to trace any character and a
// real integrity verdict proving the body has not drifted.
type PublishedView struct {
	Snapshot        *model.Snapshot    `json:"snapshot"`
	AnchorCount     int                `json:"anchor_count"`
	DecisionCount   int                `json:"decision_count"`
	PassageHashes   map[string]string  `json:"passage_hashes"`
	IntegrityOK     bool               `json:"integrity_ok"`
	IntegrityActual string             `json:"integrity_actual,omitempty"`
	IntegrityLinks  []*model.SnapshotLink `json:"links,omitempty"`
}

// SummarizeView converts frozen links into a PublishedView for API responses.
// When expectedHash is set, the integrity verdict is recomputed from the
// frozen passage-hash links and compared against it, so the view proves that
// the body's source passages have not drifted since the round was frozen.
func SummarizeView(sn *model.Snapshot, links []*model.SnapshotLink, expectedHash string) *PublishedView {
	view := &PublishedView{
		Snapshot:      sn,
		PassageHashes: make(map[string]string),
		IntegrityLinks: links,
	}
	for _, l := range links {
		switch l.Kind {
		case "anchor":
			view.AnchorCount++
		case "decision":
			view.DecisionCount++
		case "passage_hash":
			view.PassageHashes[l.RefID] = l.Payload
		}
	}
	if expectedHash != "" {
		ok, actual := VerifyIntegrity(links, expectedHash)
		view.IntegrityOK = ok
		view.IntegrityActual = actual
	}
	return view
}

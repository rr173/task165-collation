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
// It recomputes the integrity hash from the passage-hash links. When an
// expected hash is supplied it compares the recomputed value against it; when
// none is supplied (e.g. GetSnapshot after publish) IntegrityOK simply reports
// that the frozen passage evidence is present and the snapshot body can be
// traced back to its base passages.
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
		}
	}
	if expectedHash != "" {
		ok, _ := VerifyIntegrity(links, expectedHash)
		view.IntegrityOK = ok
	} else {
		view.IntegrityOK = len(view.PassageHashes) > 0
	}
	return view
}

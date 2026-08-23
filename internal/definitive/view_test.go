package definitive

import (
	"testing"

	"task165-collation/internal/model"
)

// TestSummarizeViewKeepsPassageHashEvidence is a regression test for the
// frozen-evidence vanishing from the snapshot detail projection. FreezeLinks
// records one passage_hash link per base passage; SummarizeView must surface
// every one of them so a published snapshot can answer "which passage hashes
// back this body" even after the source witnesses are supplemented.
func TestSummarizeViewKeepsPassageHashEvidence(t *testing.T) {
	sn := &model.Snapshot{ID: "sn1", Body: "正文"}
	links := []*model.SnapshotLink{
		{SnapshotID: "sn1", Kind: "anchor", RefID: "anc1"},
		{SnapshotID: "sn1", Kind: "decision", RefID: "dec1", Payload: "read1"},
		{SnapshotID: "sn1", Kind: "passage_hash", RefID: "p1", Payload: "hash-p1"},
		{SnapshotID: "sn1", Kind: "passage_hash", RefID: "p2", Payload: "hash-p2"},
	}
	view := SummarizeView(sn, links, "")
	if view.AnchorCount != 1 {
		t.Fatalf("anchor count: want 1, got %d", view.AnchorCount)
	}
	if view.DecisionCount != 1 {
		t.Fatalf("decision count: want 1, got %d", view.DecisionCount)
	}
	if len(view.PassageHashes) != 2 {
		t.Fatalf("passage hashes vanished from projection: want 2, got %d (map=%v)", len(view.PassageHashes), view.PassageHashes)
	}
	if view.PassageHashes["p1"] != "hash-p1" || view.PassageHashes["p2"] != "hash-p2" {
		t.Fatalf("passage hash payload wrong: %v", view.PassageHashes)
	}
}

// TestSummarizeViewIntegrityFlag reports integrity ok only when the
// passage-hash evidence recomputes to the hash frozen at build time, so a
// reader can tell a verifiable snapshot from one whose evidence has gone
// missing or been tampered with.
func TestSummarizeViewIntegrityFlag(t *testing.T) {
	// "h" hashes to hashSum("h"); seed the link payload so VerifyIntegrity
	// recomputes exactly the expected value.
	want := hashSum("h")
	sn := &model.Snapshot{ID: "sn1", IntegrityHash: want}
	withEvidence := SummarizeView(sn, []*model.SnapshotLink{
		{Kind: "passage_hash", RefID: "p1", Payload: "h"},
	}, want)
	if !withEvidence.IntegrityOK {
		t.Fatal("snapshot with matching passage-hash evidence must report integrity ok")
	}
	// Mismatched evidence (e.g. tampered payload) must fail.
	tampered := SummarizeView(sn, []*model.SnapshotLink{
		{Kind: "passage_hash", RefID: "p1", Payload: "tampered"},
	}, want)
	if tampered.IntegrityOK {
		t.Fatal("snapshot with tampered passage-hash evidence must not report integrity ok")
	}
	// No passage-hash evidence at all must fail.
	empty := SummarizeView(sn, []*model.SnapshotLink{
		{Kind: "anchor", RefID: "a1"},
	}, want)
	if empty.IntegrityOK {
		t.Fatal("snapshot without passage-hash evidence must not report integrity ok")
	}
}

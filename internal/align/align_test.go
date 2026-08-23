package align

import (
	"testing"
	"time"

	"task165-collation/internal/model"
	"task165-collation/internal/text"
)

func mkPassage(id string, ordinal int, txt string) *model.Passage {
	return &model.Passage{ID: id, Ordinal: ordinal, Text: txt, TextHash: text.Hash(txt)}
}

func TestBuildCandidates(t *testing.T) {
	// Same head/tail words — the anchor candidate matcher keys on first and
	// last words, so passages sharing them must pair.
	base := []*model.Passage{mkPassage("b1", 1, "学而时习之 不亦乐乎")}
	wit := []*model.Passage{mkPassage("w1", 1, "学而时习之 不亦乐乎")}
	cands := BuildCandidates(base, wit)
	if len(cands) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if cands[0].BasePassageID != "b1" || cands[0].WitnessPassageID != "w1" {
		t.Fatalf("wrong candidate: %+v", cands[0])
	}
}

func TestBuildCandidatesDifferentHeads(t *testing.T) {
	base := []*model.Passage{mkPassage("b1", 1, "学而时习之 不亦说乎")}
	wit := []*model.Passage{mkPassage("w1", 1, "人不知而不愠 不亦君子乎")}
	cands := BuildCandidates(base, wit)
	if len(cands) != 0 {
		t.Fatalf("expected no candidates for different head words, got %+v", cands)
	}
}

func TestAlignIntervalWordDiff(t *testing.T) {
	base := []*model.Passage{mkPassage("b1", 1, "学而时习之，不亦说乎？")}
	wit := []*model.Passage{mkPassage("w1", 1, "学而时习之，不亦悦乎？")}
	iv := BuildInterval(base, map[string][]*model.Passage{"w": wit})
	props := AlignInterval(iv, "p1")
	found := false
	for _, pr := range props {
		if pr.Variant.DiffType == model.DiffWord {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected word diff variant, got %+v", props)
	}
}

func TestAlignIntervalOmission(t *testing.T) {
	base := []*model.Passage{mkPassage("b1", 1, "第一段。"), mkPassage("b2", 2, "第二段。")}
	wit := []*model.Passage{mkPassage("w1", 1, "第一段。")}
	iv := BuildInterval(base, map[string][]*model.Passage{"w": wit})
	props := AlignInterval(iv, "p1")
	omission := false
	for _, pr := range props {
		if pr.Variant.DiffType == model.DiffOmission {
			omission = true
		}
	}
	if !omission {
		t.Fatalf("expected omission variant, got %+v", props)
	}
}

func TestDetectMovementAndCycle(t *testing.T) {
	base := []*model.Passage{mkPassage("b1", 1, "甲"), mkPassage("b2", 2, "乙")}
	links := map[string]*model.Passage{
		"b1": mkPassage("w2", 2, "乙"),
		"b2": mkPassage("w1", 1, "甲"),
	}
	edges := DetectMovements(base, links)
	inverted := false
	for _, e := range edges {
		if e.Inverted {
			inverted = true
		}
	}
	if !inverted {
		t.Fatal("expected inverted movement edge")
	}
	if HasCycle(edges) {
		t.Fatal("expected no cycle for a single inversion pair")
	}
}

func TestAffectedIntervals(t *testing.T) {
	t1 := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Second)
	t3 := t2.Add(time.Second)
	a1 := &model.Anchor{ID: "a1", BasePassageID: "b1", Status: model.AnchorConfirmed, CreatedAt: t1}
	a2 := &model.Anchor{ID: "a2", BasePassageID: "b2", Status: model.AnchorConfirmed, CreatedAt: t2}
	a3 := &model.Anchor{ID: "a3", BasePassageID: "b3", Status: model.AnchorConfirmed, CreatedAt: t3}
	idx := IndexAnchors([]*model.Anchor{a2, a1, a3})
	affected := AffectedIntervals(idx, "b2")
	// a2 sits between a1 and a3; both neighbours are affected.
	if len(affected) != 2 {
		t.Fatalf("expected 2 affected intervals, got %v", affected)
	}
}

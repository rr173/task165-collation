package text

import "testing"

func TestHashAndRange(t *testing.T) {
	if Hash("abc") == Hash("abd") {
		t.Fatal("hashes must differ")
	}
	if Hash("abc") != Hash("abc") {
		t.Fatal("hash must be deterministic")
	}
	if CharCount("学而时习之") != 5 {
		t.Fatalf("expected 5 runes, got %d", CharCount("学而时习之"))
	}
	if !IsValidRange(5, 1, 3) {
		t.Fatal("valid range rejected")
	}
	if IsValidRange(5, 3, 1) {
		t.Fatal("invalid range accepted")
	}
	if IsValidRange(5, 0, 0) {
		t.Fatal("empty range accepted")
	}
}

func TestDiffText(t *testing.T) {
	base := "学而时习之，不亦说乎"
	alt := "学而时习之，不亦悦乎"
	diffs := DiffText(base, alt)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Base != "说" || d.Alt != "悦" {
		t.Fatalf("unexpected diff: %+v", d)
	}
}

func TestDiffTextIdentical(t *testing.T) {
	if diffs := DiffText("相同文本", "相同文本"); diffs != nil {
		t.Fatalf("expected no diffs, got %+v", diffs)
	}
}

func TestSegmentByParagraph(t *testing.T) {
	raw := "第一段。\n\n第二段。\n\n第三段。"
	segs := SegmentByParagraph(raw)
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[0].Ordinal != 1 || segs[1].Ordinal != 2 {
		t.Fatalf("ordinals wrong")
	}
}

func TestFirstLastWords(t *testing.T) {
	first, last := FirstLastWords("学而时习之 不亦说乎")
	if first != "学而时习之" || last != "不亦说乎" {
		t.Fatalf("unexpected: %q %q", first, last)
	}
}

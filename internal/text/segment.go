package text

import (
	"strings"
	"unicode"
)

// Segment splits a source text into passages by paragraph boundaries (blank
// lines or single newlines within a normal flow). Each passage carries a
// rune offset so downstream character ranges are stable.
type Segment struct {
	Ordinal  int    // 1-based ordinal within the witness
	Text     string // passage text
	Start    int    // rune offset of the passage start in the source
	End      int    // rune offset of the passage end (exclusive)
	IsManual bool   // true when the boundary could not be auto-detected
}

// SegmentByParagraph splits raw text into passages separated by one or more
// blank lines. Newlines inside a passage are normalised to a single space so
// later diffs operate on coherent units.
func SegmentByParagraph(raw string) []Segment {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	blocks := strings.Split(raw, "\n\n")
	var out []Segment
	offset := 0
	ordinal := 1
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			offset += len(block) + 2 // account for the separator roughly
			continue
		}
		normalized := strings.Join(strings.Fields(block), " ")
		rStart := RuneOffset(raw, offset)
		rEnd := rStart + CharCount(normalized)
		out = append(out, Segment{Ordinal: ordinal, Text: normalized, Start: rStart, End: rEnd})
		ordinal++
		offset += len(block) + 2
	}
	return out
}

// RuneOffset converts a byte offset in raw into a rune offset. This keeps the
// passage ranges consistent with the rune-based range validation used by the
// rest of the system.
func RuneOffset(raw string, byteOffset int) int {
	if byteOffset >= len(raw) {
		return CharCount(raw)
	}
	return CharCount(raw[:byteOffset])
}

// IsPunct reports whether r is a CJK or ASCII punctuation character, used by
// boundary detection when deciding whether two passages belong together.
func IsPunct(r rune) bool {
	if unicode.IsPunct(r) {
		return true
	}
	return r == '，' || r == '。' || r == '；' || r == '：' || r == '！' || r == '？'
}

// LastRune returns the last rune of s, or 0 for empty strings.
func LastRune(s string) rune {
	if s == "" {
		return 0
	}
	rs := []rune(s)
	return rs[len(rs)-1]
}

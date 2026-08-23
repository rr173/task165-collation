// Package text provides the textual backbone of the collation workbench:
// SHA-256 hashing of passages, segmentation into characters and validation of
// character ranges. Everything downstream (alignment, variants, snapshots)
// anchors onto character positions defined here.
package text

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// Hash returns the hex SHA-256 digest of s. Used to detect identical passages
// across witnesses and to freeze snapshot integrity.
func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Normalize trims whitespace and folds full-width forms to half-width for
// comparison. It is intentionally conservative: no transliteration.
func Normalize(s string) string {
	s = strings.TrimSpace(s)
	return s
}

// CharCount returns the rune (character) count of s. All ranges in the system
// are expressed in runes, not bytes, so multi-byte CJK text is handled
// correctly.
func CharCount(s string) int { return utf8.RuneCountInString(s) }

// RuneAt returns the rune at 0-based index i, or 0 if out of range.
func RuneAt(s string, i int) rune {
	for idx, r := range s {
		if idx == i {
			return r
		}
	}
	return 0
}

// SliceByRunes returns the substring spanning rune indices [start, end).
func SliceByRunes(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	runes := []rune(s)
	if start >= len(runes) {
		return ""
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

// IsValidRange reports whether [start, end) is a valid rune range for text of
// rune length n.
func IsValidRange(n, start, end int) bool {
	return start >= 0 && end > start && end <= n
}

// CommonPrefixRunes returns the number of leading runes shared by a and b.
func CommonPrefixRunes(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n := len(ra)
	if len(rb) < n {
		n = len(rb)
	}
	i := 0
	for i < n && ra[i] == rb[i] {
		i++
	}
	return i
}

// CommonSuffixRunes returns the number of trailing runes shared by a and b.
func CommonSuffixRunes(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	i, j := len(ra)-1, len(rb)-1
	n := 0
	for i >= 0 && j >= 0 && ra[i] == rb[j] {
		i--
		j--
		n++
	}
	return n
}

// FirstLastWords returns the first and last words (space-separated tokens) of
// s. Used to build anchor candidates for automatic alignment.
func FirstLastWords(s string) (string, string) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ""
	}
	return fields[0], fields[len(fields)-1]
}

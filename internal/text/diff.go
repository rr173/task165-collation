package text

// Diff is a character-level difference between a base passage and a witness
// reading, expressed in rune offsets of the base text.
type Diff struct {
	Start int    // start rune offset in the base passage
	End   int    // end rune offset (exclusive) in the base passage
	Base  string // base text in the range
	Alt   string // witness reading in the range
}

// DiffText computes the minimal character-level differences between base and
// alt. It produces one Diff per differing contiguous region between the common
// prefix and suffix. This is deliberately simple but deterministic; it powers
// variant clustering in the align package.
func DiffText(base, alt string) []Diff {
	if base == alt {
		return nil
	}
	rb := []rune(base)
	ra := []rune(alt)
	prefix := CommonPrefixRunes(base, alt)
	suffix := CommonSuffixRunes(base, alt)
	// Guard against prefix/suffix overlap.
	if prefix+suffix > len(rb) {
		suffix = len(rb) - prefix
	}
	if prefix+suffix > len(ra) {
		suffix = len(ra) - prefix
	}
	if suffix < 0 {
		suffix = 0
	}
	baseMid := string(rb[prefix : len(rb)-suffix])
	altMid := string(ra[prefix : len(ra)-suffix])
	return []Diff{{
		Start: prefix,
		End:   len(rb) - suffix,
		Base:  baseMid,
		Alt:   altMid,
	}}
}

// HasDifference reports whether two texts differ at all.
func HasDifference(base, alt string) bool { return base != alt }

// MergeAdjacent merges diffs that touch or overlap each other. Currently the
// diff engine produces non-adjacent regions only; this guard keeps the shape
// stable if the engine is later upgraded.
func MergeAdjacent(diffs []Diff) []Diff {
	if len(diffs) < 2 {
		return diffs
	}
	out := make([]Diff, 0, len(diffs))
	for _, d := range diffs {
		if len(out) == 0 || d.Start > out[len(out)-1].End {
			out = append(out, d)
			continue
		}
		last := &out[len(out)-1]
		if d.End > last.End {
			last.End = d.End
			last.Base += d.Base
			last.Alt += d.Alt
		}
	}
	return out
}

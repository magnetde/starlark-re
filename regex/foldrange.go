package regex

import (
	"sort"
	"unicode"
)

// createFoldedRanges creates a slice of ranges, which contains all cases of the characters from `lo` to `hi`.
// If `ascii` is set to true, this function only determines different cases of ASCII characters.
// The resulting slice of ranges is sorted and any overlapping ranges are merged together.
// See also `appendFoldedRange` of package `regexp/syntax`.
func createFoldedRanges(lo, hi rune, ascii bool) []rune {
	var r []rune

	var maxFold rune
	if ascii {
		maxFold = maxFoldASCII
	} else {
		maxFold = maxFoldUnicode
	}

	// Optimizations.
	if lo <= minFold && hi >= maxFold {
		// Range is full: folding can't add more.
		return appendRange(r, lo, hi)
	}
	if hi < minFold || lo > maxFold {
		// Range is outside folding possibilities.
		return appendRange(r, lo, hi)
	}
	if lo < minFold {
		// [lo, minFold-1] needs no folding.
		r = appendRange(r, lo, minFold-1)
		lo = minFold
	}
	if hi > maxFold {
		// [maxFold+1, hi] needs no folding.
		r = appendRange(r, maxFold+1, hi)
		hi = maxFold
	}

	// Determine the folding function.
	var fold func(c rune) rune
	if ascii {
		fold = simpleFoldASCII
	} else {
		fold = simpleFold
	}

	// Brute force. Depend on appendRange to coalesce ranges on the fly.
	for c := lo; c <= hi; c++ {
		r = appendRange(r, c, c)
		for f := fold(c); f != c; f = fold(f) {
			r = appendRange(r, f, f)
		}
	}

	// Sort and simplify ranges.
	return cleanClass(&r)
}

// appendRange returns the result of appending the range lo-hi to the class r.
// Copied from module regexp/syntax.
func appendRange(r []rune, lo, hi rune) []rune {
	// Expand last range or next to last range if it overlaps or abuts.
	// Checking two ranges helps when appending case-folded
	// alphabets, so that one range can be expanding A-Z and the
	// other expanding a-z.
	n := len(r)
	for i := 2; i <= 4; i += 2 { // twice, using i=2, i=4
		if n >= i {
			rlo, rhi := r[n-i], r[n-i+1]
			if lo <= rhi+1 && rlo <= hi+1 {
				if lo < rlo {
					r[n-i] = lo
				}
				if hi > rhi {
					r[n-i+1] = hi
				}
				return r
			}
		}
	}

	return append(r, lo, hi)
}

// simpleFold is the equivalent function of `unicode.SimpleFold`
// with support for 'U+0130' and 'U+0131' for 'I' and 'i'
// and for 'U+FB05' and 'U+FB06'
func simpleFold(c rune) rune {
	switch c {
	case 'I':
		return 'i'
	case 'i':
		return '\u0130'
	case '\u0130':
		return '\u0131'
	case '\u0131':
		return 'I'
	case '\ufb05':
		return '\ufb06'
	case '\ufb06':
		return '\ufb05'
	default:
		return unicode.SimpleFold(c)
	}
}

// simpleFoldASCII is the equivalent function of `unicode.SimpleFold` limited to ASCII characters.
func simpleFoldASCII(c rune) rune {
	if inRange('A', 'Z', c) {
		return c - 'A' + 'a'
	} else if inRange('a', 'z', c) {
		return c - 'a' + 'A'
	} else {
		return c
	}
}

// ranges implements sort.Interface on a []rune.
// The choice of receiver type definition is strange
// but avoids an allocation since we already have
// a *[]rune.
// Copied from module regexp/syntax.
type ranges struct {
	p *[]rune
}

func (ra ranges) Less(i, j int) bool {
	p := *ra.p
	i *= 2
	j *= 2
	return p[i] < p[j] || p[i] == p[j] && p[i+1] > p[j+1]
}

func (ra ranges) Len() int {
	return len(*ra.p) / 2
}

func (ra ranges) Swap(i, j int) {
	p := *ra.p
	i *= 2
	j *= 2
	p[i], p[i+1], p[j], p[j+1] = p[j], p[j+1], p[i], p[i+1]
}

// cleanClass sorts the ranges (pairs of elements of r),
// merges them, and eliminates duplicates.
// Copied from module regexp/syntax.
func cleanClass(rp *[]rune) []rune {

	// Sort by lo increasing, hi decreasing to break ties.
	sort.Sort(ranges{rp})

	r := *rp
	if len(r) < 2 {
		return r
	}

	// Merge abutting, overlapping.
	w := 2 // write index
	for i := 2; i < len(r); i += 2 {
		lo, hi := r[i], r[i+1]
		if lo <= r[w-1]+1 {
			// merge with previous range
			if hi > r[w-1] {
				r[w-1] = hi
			}
			continue
		}
		// new disjoint range
		r[w] = lo
		r[w+1] = hi
		w += 2
	}

	return r[:w]
}

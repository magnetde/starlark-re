package re

import "unicode/utf8"

// Replacement for `FindStringSubmatchIndex`, to specify a starting position.
func findMatch(r regexEngine, s string, pos int, longest bool) ([]int, error) {
	in := r.BuildInput(s)
	return in.Find(pos, longest, nil)
}

// TODO
// Needs a special implementation, `FindAllStringSubmatchIndex` removes empty matches right after a prevous match.
// Is necessary because python includes empty matches right after a previous match.
// The deliver function must not modify the match.
func findMatches(r regexEngine, s string, pos int, n int, deliver func(a []int) error) error {
	if n <= 0 {
		n = len(s) + 1
	}

	in := r.BuildInput(s)

	end := len(s)
	lastMatch := [2]int{-1, 0}

	// The Go regex engine only finds one match at a given position, but there are rare cases,
	// where multiple matches exists at the same position.
	// To avoid this behavior, a position, where a empty match was found, is searched again in an second pass.
	// But at the second time, the longest match is searched.
	firstPass := true

	var dstCap [2]int
	for i := 0; i < n && pos <= end; {
		a, err := in.Find(pos, !firstPass, dstCap[:0])
		if err != nil {
			return err
		}

		if len(a) == 0 {
			break
		}

		// If the last match was different from the current:
		if a[0] != lastMatch[0] || a[1] != lastMatch[1] {
			err = deliver(a)
			if err != nil {
				return err
			}

			copy(lastMatch[:], a[:2])
			i++
		}

		if firstPass {
			// If an empty match was found, try to search this position again,
			// but now look for the longest match, but only if supported.
			if r.SupportsLongest() && a[0] == a[1] {
				firstPass = false
				continue
			}
		} else {
			firstPass = true
		}

		// Advance past this match; always advance at least one character.
		_, width := utf8.DecodeRuneInString(s[pos:])

		if pos+width > a[1] {
			pos += width
		} else if pos+1 > a[1] {
			// This clause is only needed at the end of the input
			// string. In that case, DecodeRuneInString returns width=0.
			pos++
		} else {
			pos = a[1]
		}
	}

	return nil
}

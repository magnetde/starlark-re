package re

import (
	"io"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode/utf8"
	"unsafe"

	// Necessary to link the function `doExecute`.
	_ "unsafe"
)

//go:linkname doExecute regexp.(*Regexp).doExecute
func doExecute(re *regexp.Regexp, r io.RuneReader, b []byte, s string, pos int, ncap int, dstCap []int) []int

// Replacement for `FindStringSubmatchIndex`, to specify a starting position.
func findMatch(r *regexp.Regexp, s string, pos int) []int {
	numcap := numCap(r)

	// Fix invalid codepoints
	s, offsets := replaceInvalidChars(s)

	res := pad(r, doExecute(r, nil, nil, s, pos, numcap, nil))

	if offsets != nil { // Apply the offsets
		applyOffsets(res, offsets)
	}

	return res
}

// TODO
// Needs a special implementation, `FindAllStringSubmatchIndex` removes empty matches right after a prevous match.
// Is necessary because python includes empty matches right after a previous match.
// The deliver function must not modify the match.
func findMatches(r *regexp.Regexp, s string, pos int, n int, deliver func(a []int) bool) {
	if n <= 0 {
		n = len(s) + 1
	}

	// Fix invalid codepoints
	s, offsets := replaceInvalidChars(s)

	numcap := numCap(r)
	end := len(s)
	lastMatch := [2]int{-1, 0}

	// The Go regex engine only finds one match at a given position, but there are rare cases,
	// where multiple matches exists at the same position.
	// To avoid this behavior, a position, where a empty match was found, is searched again in an second pass.
	// But at the second time, the longest match is searched.
	firstPass := true

	var dstCap [2]int
	for i := 0; i < n && pos <= end; {
		re := r
		if !firstPass {
			re = r.Copy()
			re.Longest()
		}

		a := doExecute(re, nil, nil, s, pos, numcap, dstCap[:0])
		if len(a) == 0 {
			break
		}

		// If the last match was different from the current:
		if a[0] != lastMatch[0] || a[1] != lastMatch[1] {
			a = pad(r, a)

			if offsets != nil {
				applyOffsets(a, offsets)
			}

			if !deliver(a) {
				return
			}

			copy(lastMatch[:], a[:2])
			i++
		}

		if firstPass {
			// If an empty match was found, try to search this position again,
			// but now look for the longest match.
			if a[0] == a[1] {
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
}

// numCap returns the unexported field `r.prog.NumCap`.
func numCap(r *regexp.Regexp) int {
	v := reflect.ValueOf(r).Elem()
	v = v.FieldByName("prog")
	p := unsafe.Pointer(v.Pointer())
	return (*syntax.Prog)(p).NumCap
}

// replaceInvalidChars replaces invalid UTF-8 codepoints with legal ones.
// This is necessary, because the Go regex engine can only match valid UTF-8 codepoints,
// instead of arbitrary bytes, even if a `[]byte` value instead of a string is passed to `doExecute`.
// If the string does not contain any invalid UTF-8 codepoints, the string is returned unchanged,
// and `nil` is returned for the offset slice.
func replaceInvalidChars(s string) (string, []int) {
	if utf8.ValidString(s) { // if no invalid utf8 values exists, wie can skip the everything else
		return s, nil
	}

	var b strings.Builder
	b.Grow(len(s) + 4) // reserve 4 extra bytes

	offsets := make([]int, 0, len(s)+4+1) // reserve 4 extra offsets (+1 for the last offset)
	offset := 0

	for len(s) > 0 {
		ch, size := utf8.DecodeRuneInString(s)

		if ch != utf8.RuneError {
			b.WriteRune(ch)

			for i := 0; i < size; i++ {
				offsets = append(offsets, offset)
			}
		} else {
			t := string(s[0])
			b.WriteString(t)

			// At this point, `s[0]` is in range 128-255, so
			// `t` is either of format '\xc2\x..' or '\xc3\x..',
			// so two offsets must be added to the offset slice.
			// Also because one more element is added to the string, that previously not existed,
			// The second offset must be increased by one.
			offsets = append(offsets, offset, offset+1)
			offset++
		}

		s = s[size:] // if the rune is not valid, the size returned is 1, so slicing with `size` is correct
	}

	// append a last offset value, that corresponds to `len(s)`
	offsets = append(offsets, offset)

	return b.String(), offsets
}

func applyOffsets(a []int, offsets []int) {
	if a == nil {
		return
	}
	for i, v := range a {
		a[i] = v - offsets[v]
	}
}

func pad(r *regexp.Regexp, a []int) []int {
	if a == nil {
		// No match.
		return nil
	}
	n := (1 + r.NumSubexp()) * 2
	for len(a) < n {
		a = append(a, -1)
	}
	return a
}

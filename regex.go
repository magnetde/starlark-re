package re

import (
	"errors"
	"io"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/dlclark/regexp2"

	sre "github.com/magnetde/starlark-re/syntax"
	"github.com/magnetde/starlark-re/util"
)

type regexEngine interface {
	SubexpNames() []string
	NumSubexp() int
	SubexpIndex(name string) int
	BuildInput(s string) regexInput // Fix invalid codepoints
}

type regexInput interface {
	Find(pos int, longest bool, dstCap []int) []int
}

// input must be preprocessed
func compileRegex(p *sre.Preprocessor, re2Fallback bool) (regexEngine, error) {
	var err error

	if !p.IsUnsupported() {
		var r *regexp.Regexp

		s := p.String()

		r, err = regexp.Compile(s)
		if err == nil {
			re := &stdRegex{
				re:     r,
				numCap: numCap(r),
			}

			return re, nil
		}
	} else if !re2Fallback {
		return nil, errors.New("regex has unsupported elements")
	}

	if re2Fallback {
		var r2 *regexp2.Regexp

		s := p.FallbackString()
		flags := p.Flags()
		options := regexp2.None | regexp2.RE2

		if flags&sre.FlagIgnoreCase != 0 {
			options |= regexp2.IgnoreCase
		}
		if flags&sre.FlagMultiline != 0 {
			options |= regexp2.Multiline
		}
		if flags&sre.FlagDotAll != 0 {
			options |= regexp2.Singleline
		}
		if flags&sre.FlagUnicode != 0 {
			options |= regexp2.Unicode
		}

		r2, err = regexp2.Compile(s, options)
		if err == nil {
			re := &advRegex{
				re:     r2,
				numCap: numCap2(r2),
			}

			return re, nil
		}
	}

	return nil, err // return the second error
}

// numCap returns the unexported field `r.prog.NumCap`.
func numCap(r *regexp.Regexp) int {
	v := reflect.ValueOf(r).Elem()
	v = v.FieldByName("prog")
	p := unsafe.Pointer(v.Pointer())
	return (*syntax.Prog)(p).NumCap
}

// numCap returns the unexported field `r.capsize`.
func numCap2(r *regexp2.Regexp) int {
	v := reflect.ValueOf(r).Elem()
	v = v.FieldByName("capsize")
	return int(v.Int())
}

type stdRegex struct {
	re     *regexp.Regexp
	numCap int
}

type stdInput struct {
	re      *stdRegex
	str     string
	offsets []int
}

type advRegex struct {
	re     *regexp2.Regexp
	numCap int
}

type advInput struct {
	re          *advRegex
	chars       []rune
	offsetsRune []int // offsets for converting byte indices to rune indices
	offsetsByte []int // offsets for converting rune indices to byte indices
}

var (
	_ regexEngine = (*stdRegex)(nil)
	_ regexInput  = (*stdInput)(nil)
	_ regexEngine = (*advRegex)(nil)
	_ regexInput  = (*advInput)(nil)
)

func (r *stdRegex) SubexpNames() []string {
	return r.re.SubexpNames()
}

func (r *stdRegex) NumSubexp() int {
	return r.re.NumSubexp()
}

func (r *stdRegex) SubexpIndex(name string) int {
	return r.re.SubexpIndex(name)
}

func (r *stdRegex) BuildInput(s string) regexInput {
	s, offsets := replaceInvalidChars(s)

	i := &stdInput{
		re:      r,
		str:     s,
		offsets: offsets,
	}

	return i
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
			offsets = append(offsets, offset, offset-1)
			offset--
		}

		s = s[size:] // if the rune is not valid, the size returned is 1, so slicing with `size` is correct
	}

	// append a last offset value, that corresponds to `len(s)`
	offsets = append(offsets, offset)

	return b.String(), offsets
}

//go:linkname doExecute regexp.(*Regexp).doExecute
func doExecute(re *regexp.Regexp, r io.RuneReader, b []byte, s string, pos int, ncap int, dstCap []int) []int

func (i *stdInput) Find(pos int, longest bool, dstCap []int) []int {
	re := i.re.re
	if longest {
		re = re.Copy()
		re.Longest()
	}

	a := i.pad(doExecute(re, nil, nil, i.str, pos, i.re.numCap, dstCap))

	applyOffsets(a, i.offsets)
	return a
}

func (i *stdInput) pad(a []int) []int {
	if a == nil {
		// No match.
		return nil
	}

	n := (1 + i.re.NumSubexp()) * 2
	for len(a) < n {
		a = append(a, -1)
	}

	return a
}

func applyOffsets(a []int, offsets []int) {
	if a == nil || offsets == nil {
		return
	}
	for i, v := range a {
		if v >= 0 {
			a[i] = v + offsets[v]
		}
	}
}

func (r *advRegex) SubexpNames() []string {
	names := r.re.GetGroupNames()

	// filter numerical group names
	for i, n := range names {
		if _, err := strconv.Atoi(n); err == nil {
			names[i] = ""
		}
	}

	return names
}

func (r *advRegex) NumSubexp() int {
	return r.numCap
}

func (r *advRegex) SubexpIndex(name string) int {
	return r.re.GroupNumberFromName(name)
}

func (r *advRegex) BuildInput(s string) regexInput {
	chars, offsetsRune, offsetsByte := getRuneOffsets(s)

	return &advInput{
		re:          r,
		chars:       chars,
		offsetsRune: offsetsRune,
		offsetsByte: offsetsByte,
	}
}

func getRuneOffsets(s string) ([]rune, []int, []int) {
	if util.IsASCIIString(s) { // if the string has only ASCII characters, offsets are not necessary
		return []rune(s), nil, nil
	}

	chars := make([]rune, 0, len(s))

	offsetsRune := make([]int, 0, len(s))
	offsetsByte := make([]int, 0, len(s)+4) // reserve 4 extra offsets
	offsetI := 0
	offsetO := 0

	for len(s) > 0 {
		ch, size := utf8.DecodeRuneInString(s)
		incr := size - 1

		if ch == utf8.RuneError {
			ch = rune(s[0])
			incr = utf8.RuneLen(ch) - 1
		}

		chars = append(chars, ch)

		for i := 0; i < size; i++ {
			offsetsRune = append(offsetsRune, offsetI)
		}
		offsetI++

		offsetsByte = append(offsetsByte, offsetO)
		offsetO += incr

		s = s[size:] // if the rune is not valid, the size returned is 1, so slicing with `size` is correct
	}

	offsetsByte = append(offsetsByte, offsetO)

	return chars, offsetsRune, offsetsByte
}

func (i *advInput) Find(pos int, longest bool, dstCap []int) []int {
	if i.offsetsRune != nil {
		pos = i.offsetsRune[pos]
	}

	m, err := i.re.re.FindRunesMatchStartingAt(i.chars, pos)
	if err != nil {
		return nil
	}

	if m == nil {
		return nil
	}

	groups := m.Groups()
	a := make([]int, 0, 2*len(groups))

	for _, g := range groups {
		if len(g.Captures) != 0 {
			a = append(a, g.Index, g.Index+g.Length)
		} else {
			a = append(a, -1, -1)
		}
	}

	applyOffsets(a, i.offsetsByte)
	return a
}

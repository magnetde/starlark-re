package regex

import (
	"io"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/dlclark/regexp2"
)

// Needs to have exported fields, because engine has to share common methods
// with interface `Subexp` in package `syntax`.
type Engine interface {
	Flags() uint32
	SubexpNames() []string
	SubexpCount() int
	SubexpIndex(name string) int
	SupportsLongest() bool     // support for the longest match function
	BuildInput(s string) Input // Fix invalid codepoints
}

type Input interface {
	Find(pos int, longest bool, dstCap []int) ([]int, error)
}

func Compile(pattern string, isStr bool, flags uint32, fallbackEnabled bool) (Engine, error) {
	// replace unicode patterns, that are supported by Python but not supported by Go
	p, err := newPreprocessor(pattern, isStr, flags)
	if err != nil {
		return nil, err
	}

	flags = p.flags()

	useFallback := false
	if fallbackEnabled {
		if flags&FlagFallback != 0 {
			useFallback = true
			flags &= ^FlagFallback
		} else if !p.isSupported() {
			useFallback = true
		}
	}

	if !useFallback {
		var r *regexp.Regexp

		s := p.stdPattern()

		r, err = regexp.Compile(s)
		if err != nil {
			return nil, err
		}

		re := &stdRegex{
			re:     r,
			flags:  flags,
			numCap: numCap(r),
		}

		return re, nil
	} else {
		var r2 *regexp2.Regexp

		s, remapping := p.fallbackPattern()

		options := regexp2.None | regexp2.RE2

		if flags&FlagIgnoreCase != 0 {
			options |= regexp2.IgnoreCase
		}
		if flags&FlagMultiline != 0 {
			options |= regexp2.Multiline
		}
		if flags&FlagDotAll != 0 {
			options |= regexp2.Singleline
		}
		if flags&FlagUnicode != 0 {
			options |= regexp2.Unicode
		}

		r2, err = regexp2.Compile(s, options)
		if err != nil {
			return nil, err
		}

		// Becase the regexp2 engine may reorder groups, so the order of groups for the same regex is not
		// the same at Python and .NET, we have to save a remapping.
		// This remapping is determined, by getting a mapping of all new group names and the ordered list
		// of the group names of the compiled regex and then all group names are compared.
		// If the position of a single group name does not match, the mapping is added to a map.

		var groupMapping map[int]int
		for newpos, group := range r2.GetGroupNames() {
			if oldpos, ok := remapping[group]; ok && oldpos != newpos { // only store the remapping of group positions, that have been changed
				if groupMapping == nil {
					groupMapping = make(map[int]int)
				}
				groupMapping[newpos] = oldpos
			} else {
				continue
			}
		}

		// TODO: I am not yet sure, how regexp2 reorders the groups.
		// Maybe groupMapping is always empty, but that whould be no issue and except at regexp compiling,
		// no overhead is produced.

		re := &fallbEngine{
			re:         r2,
			flags:      flags,
			numSubexp:  numCapFallb(r2) - 1,
			groupNames: p.groupNames(),
		}

		return re, nil
	}
}

// numCap returns the unexported field `r.prog.NumCap`.
func numCap(r *regexp.Regexp) int {
	v := reflect.ValueOf(r).Elem()
	v = v.FieldByName("prog")
	p := unsafe.Pointer(v.Pointer())
	return (*syntax.Prog)(p).NumCap
}

// numCapFallb returns the unexported field `r.capsize`.
func numCapFallb(r *regexp2.Regexp) int {
	v := reflect.ValueOf(r).Elem()
	v = v.FieldByName("capsize")
	return int(v.Int())
}

type stdRegex struct {
	re     *regexp.Regexp
	flags  uint32
	numCap int
}

type stdInput struct {
	re      *stdRegex
	str     string
	offsets []int
}

type fallbEngine struct {
	re        *regexp2.Regexp
	flags     uint32
	numSubexp int

	groupNames   map[string]int // fallback preprocessor renames the groups, so the original mapping must be saved
	groupMapping map[int]int    // regexp2 (and .NET) orders groups other than Python, so they must be reordered
}

type advInput struct {
	re          *fallbEngine
	chars       []rune
	offsetsRune []int // offsets for converting byte indices to rune indices
	offsetsByte []int // offsets for converting rune indices to byte indices
}

var (
	_ Engine = (*stdRegex)(nil)
	_ Input  = (*stdInput)(nil)
	_ Engine = (*fallbEngine)(nil)
	_ Input  = (*advInput)(nil)
)

func (r *stdRegex) Flags() uint32 {
	return r.flags
}

func (r *stdRegex) SubexpNames() []string {
	return r.re.SubexpNames()
}

func (r *stdRegex) SubexpCount() int {
	return r.re.NumSubexp()
}

func (r *stdRegex) SubexpIndex(name string) int {
	return r.re.SubexpIndex(name)
}

func (r *stdRegex) SupportsLongest() bool {
	return true
}

func (r *stdRegex) BuildInput(s string) Input {
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

func (i *stdInput) Find(pos int, longest bool, dstCap []int) ([]int, error) {
	re := i.re.re
	if longest {
		re = re.Copy()
		re.Longest()
	}

	a := i.pad(doExecute(re, nil, nil, i.str, pos, i.re.numCap, dstCap))

	applyOffsets(a, i.offsets)
	return a, nil
}

func (i *stdInput) pad(a []int) []int {
	if a == nil {
		// No match.
		return nil
	}

	n := (1 + i.re.SubexpCount()) * 2
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

func (r *fallbEngine) Flags() uint32 {
	return r.flags
}

func (r *fallbEngine) SubexpNames() []string {
	names := make([]string, 1+r.numSubexp)

	for group, i := range r.groupNames {
		names[i] = group
	}

	return names
}

func (r *fallbEngine) SubexpCount() int {
	return r.numSubexp
}

func (r *fallbEngine) SubexpIndex(name string) int {
	if i, ok := r.groupNames[name]; ok {
		return i
	}

	return -1
}

func (r *fallbEngine) SupportsLongest() bool {
	return false
}

func (r *fallbEngine) BuildInput(s string) Input {
	chars, offsetsRune, offsetsByte := getRuneOffsets(s)

	return &advInput{
		re:          r,
		chars:       chars,
		offsetsRune: offsetsRune,
		offsetsByte: offsetsByte,
	}
}

func getRuneOffsets(s string) ([]rune, []int, []int) {
	if isASCIIString(s) { // if the string has only ASCII characters, offsets are not necessary
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

func (i *advInput) Find(pos int, _ bool, dstCap []int) ([]int, error) {
	if i.offsetsRune != nil {
		pos = i.offsetsRune[pos]
	}

	m, err := i.re.re.FindRunesMatchStartingAt(i.chars, pos)
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, nil
	}

	groups := m.Groups()
	a := growSlice(dstCap, 2*len(groups))

	for index, g := range groups {
		if i.re.groupMapping != nil { // maybe the index needs a remap
			if indx, ok := i.re.groupMapping[index]; ok {
				index = indx
			}
		}

		start := -1
		end := -1

		if len(g.Captures) != 0 {
			start = g.Index
			end = g.Index + g.Length
		}

		a[2*index] = start
		a[2*index+1] = end
	}

	applyOffsets(a, i.offsetsByte)
	return a, nil
}

// growSlice increases the slice's size, if necessary, to guarantee a size
// if n. If the previous capacity was less than n, the slice is filled with
// elements with a value of zero. If n is negative or too large to allocate
// the memory, growSlice panics. For safety reasons, the resulting slice is
// filled with zero values.
// See also slices.Grow.
func growSlice[S ~[]E, E any](s S, n int) S {
	var zero E

	if n < 0 {
		panic("cannot be negative")
	}
	if cap(s) < n {
		s = append(s[:cap(s)], make([]E, n-cap(s))...)
	}

	s = s[:n]
	for i := range s {
		s[i] = zero
	}
	return s
}

package regex

import (
	"io"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/dlclark/regexp2"

	"github.com/magnetde/starlark-re/util"
)

// Engine is a wrapper type for the regex engine.
// Currently, the regex engines `regexp.Regexp` (see https://pkg.go.dev/regexp) and the fallback
// engine `regexp2.Regexp` (see https://pkg.go.dev/github.com/dlclark/regexp2) are supported.
// Since these engines work differently and do not share a common interface, wrapper functions are available.
type Engine interface {

	// Flags returns the regex flags. The flags are parsed from the regex pattern and
	// correspond to the regex flags of the Python re module.
	Flags() uint32

	// SubexpNames returns a slice of subexpression names. The first element represents
	// the entire regex pattern and always contains an empty string. The i-th element
	// represents the i-th group. If the i-th group is not named, the i-th element
	// in the slice is the empty string.
	// The resulting slice should not be modified.
	SubexpNames() []string

	// SubexpCount returns the number of parenthesized subexpressions in the regex pattern.
	SubexpCount() int

	// SubexpIndex returns the index of the subexpression with the given name, or -1 if
	// there is no subexpression with that name.
	SubexpIndex(name string) int

	// SupportsLongest returns, if the regex engine supports the longest match search.
	// Currently, the longest match search is only supported by `regexp.Regexp`.
	SupportsLongest() bool

	// BuildInput creates a input object that is used for searching the regex pattern.
	// The regex engines work differently in terms of match positions and how matches
	// are found in strings with illegal UTF-8 code points. So, before passing the
	// string to the regex engine, the search string has to be modified, and afterwards
	// the positions of matches have to be adjusted. To improve performance, the search
	// string is only be processed once and a new object is created for regex search
	// operations. A maximum of `endpos` bytes are extracted from `s`. `endpos` must be
	// within the range of `[0, len(s)]`.
	BuildInput(s string, endpos int) Input
}

// Input is the type to perform a regex search.
type Input interface {

	// Find searches the input for the next match starting at position `pos`.
	// If `longest` is set to true, the search prioritizes the longest match. In case
	// the longest match search is unsupported by the regex engine, a regular search is
	// performed instead. The match is returned as a slice with 2*(n+1) elements, where
	// n is the number of capture groups. The element at index 2*i represents the
	// starting position of capture group i and index 2*i+1 represents the ending position,
	// where i = 0 corresponds to the entire match. The positions are returned as byte
	// offsets in the string, So, if `s` is the search string and `start` and `end` are
	// the start and end positions of the match, then the matched portion is `s[start:end]`.
	// If group i wasn't matched, then both values are -1. It's recommended to use the
	// `dstCap` parameter as the output slice.
	Find(pos int, longest bool, dstCap []int) ([]int, error)
}

// Compile compiles the Python-compatible regex pattern and return a regex engine.
// If the fallback engine (`regexp2.Regexp`) is enabled and either unsupported subpatterns exist or
// the FALLBACK flag is enabled, then the fallback engine is used. Otherwise, the preprocessed regex
// pattern is compiled using the default regex engine (regexp.Regexp). If the DEBUG flag is enabled,
// the second return value is be a debug description of the parsed regex pattern.
func Compile(pattern string, isStr bool, flags uint32, fallbackEnabled bool) (Engine, string, error) {
	// Create a preprocessor of the regex string to replace unicode patterns,
	// that are supported by Python but not supported by Go.
	p, err := newPreprocessor(pattern, isStr, flags)
	if err != nil {
		return nil, "", err
	}

	flags = p.flags()
	useFallback := fallbackEnabled && (flags&FlagFallback != 0 || !p.isSupported())

	var e Engine
	if !useFallback {
		s := p.stdPattern()

		r, err := regexp.Compile(s)
		if err != nil {
			return nil, "", err
		}

		e = &stdRegex{
			re:     r,
			flags:  flags,
			isStr:  isStr,
			numCap: numCap(r),
		}
	} else {
		s := p.fallbackPattern()

		r2, err := regexp2.Compile(s, regexp2.RE2)
		if err != nil {
			return nil, "", err
		}

		e = &fallbEngine{
			re:         r2,
			flags:      flags,
			isStr:      isStr,
			numSubexp:  numCapFallb(r2) - 1,
			groupNames: p.groupNames(),
		}
	}

	// Create a debug information if needed.

	dump := ""
	if flags&FlagDebug != 0 {
		dump = p.p.dump()
	}

	return e, dump, nil
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

// stdRegex is the type, that represents the regex engine `regexp.Regexp`.
type stdRegex struct {
	re     *regexp.Regexp
	flags  uint32
	isStr  bool
	numCap int
}

// stdInput is the type, that represents the processed input of `stdRegex`.
type stdInput struct {
	re   *stdRegex
	str  string
	bits *util.BitArray
}

// fallbEngine is the type, that represents the regex engine `regexp.Regexp2`.
type fallbEngine struct {
	re         *regexp2.Regexp
	flags      uint32
	isStr      bool
	numSubexp  int
	groupNames map[string]int // fallback preprocessor removes group names, so the original mapping must be saved
}

// fallbInput is the type, that represents the processed input of `fallbEngine`.
type fallbInput struct {
	re    *fallbEngine
	chars []rune
	bits  *util.BitArray
}

// Check if the types satisfy the interfaces.
var (
	_ Engine = (*stdRegex)(nil)
	_ Input  = (*stdInput)(nil)
	_ Engine = (*fallbEngine)(nil)
	_ Input  = (*fallbInput)(nil)
)

// Flags is the implementation of the `Flags` function for the `Engine` interface.
func (r *stdRegex) Flags() uint32 {
	return r.flags
}

// SubexpNames is the implementation of the `SubexpNames` function for the `Engine` interface.
func (r *stdRegex) SubexpNames() []string {
	return r.re.SubexpNames()
}

// SubexpCount is the implementation of the `SubexpCount` function for the `Engine` interface.
func (r *stdRegex) SubexpCount() int {
	return r.re.NumSubexp()
}

// SubexpIndex is the implementation of the `SubexpIndex` function for the `Engine` interface.
func (r *stdRegex) SubexpIndex(name string) int {
	return r.re.SubexpIndex(name)
}

// SupportsLongest is the implementation of the `SupportsLongest` function for the `Engine` interface.
func (r *stdRegex) SupportsLongest() bool {
	return true
}

// BuildInput is the implementation of the `BuildInput` function for the `Engine` interface.
func (r *stdRegex) BuildInput(s string, endpos int) Input {
	s, bits := r.replaceInvalidChars(s, endpos)

	i := &stdInput{
		re:   r,
		str:  s,
		bits: bits,
	}

	return i
}

// replaceInvalidChars replaces invalid UTF-8 codepoints with legal ones.
// This is necessary because the Go regex engine requires valid UTF-8 codepoints instead of
// arbitrary bytes, even if a bytes value is passed to the `doExecute` function instead of a string.
// If the string does not contain any invalid illegal UTF-8 codepoints,
// it is returned without modification, and no bitarray is returned.
// A maximum of `endpos` bytes are extracted from `s`.
//
// If a bitarray is returned, it contains the original bytes of `s` in the returned string (`s'`).
// If the i-th bit is a 1-bit, then the i-th byte in the string `s'` is also present in the string
// `s` but may be at a different position since other bytes may have been added to the string.
// If the i-th bit is a 0-bit, then the i-th byte in the string `s'` is one of these additional
// bytes that where added to fix invalid UTF-8 codepoints.
//
// A byte position `i` in `s` is converted to a byte position `j` in `s'` using the bitarray `B` as follows:
//   - `j` is the position of the `i+1`-th 1-bit in `B`; determined with `select(B, i + 1)`
//   - `i` is the number of 1-bits from 0 to `j-1`; determined with `rank(B, j - 1)`
func (r *stdRegex) replaceInvalidChars(s string, endpos int) (string, *util.BitArray) {
	pre := s[:endpos]

	if r.isStr {
		if utf8.ValidString(pre) { // skip if no invalid utf8 values exist until endpos
			return pre, nil
		}
	} else {
		if isASCIIString(pre) { // skip if only ascii characters exist until endpos
			return pre, nil
		}
	}

	var b strings.Builder
	b.Grow(endpos + 4) // reserve 4 extra bytes

	var bits util.BitArray
	bits.Grow(endpos + 4 + 1) // reserve 4 extra bits (+1 for the last offset)

	if r.isStr {
		pos := 0

		for len(s) > 0 {
			// Get the next UTF-8 codepoint
			c, size := utf8.DecodeRuneInString(s)

			pos += size
			if pos > endpos {
				break
			}

			if c != utf8.RuneError {
				b.WriteRune(c)

				bits.AppendN(true, size)
			} else {
				b.WriteRune(rune(s[0]))

				// At this point, if `s[0]` is in range 128-255,
				// then `t` is in either of the format '\xc2\x..' or '\xc3\x..'.
				// So an additional element has been appended to the string,
				// which previously did not exist, and a 0-bit has to be added to the bitarray.
				bits.Append(true)
				bits.Append(false)
			}

			// If the character is not valid, the size returned is 1, so slicing with `size` is corrent.
			s = s[size:]
		}
	} else {
		// Iterate over the bytes in `s` instead of characters.
		for i := 0; i < endpos; i++ {
			c := s[i]
			if c <= unicode.MaxASCII {
				b.WriteByte(c)

				bits.Append(true)
			} else {
				b.WriteRune(rune(c))

				// See the comment above for invalid characters in strings.
				bits.Append(true)
				bits.Append(false)
			}
		}
	}

	// Append a last 1-bit, that corresponds to `endpos`.
	bits.Append(true)
	bits.Optimize()

	return b.String(), &bits
}

//go:linkname doExecute regexp.(*Regexp).doExecute
func doExecute(re *regexp.Regexp, r io.RuneReader, b []byte, s string, pos int, ncap int, dstCap []int) []int

// Find is the implementation of the `Find` function for the `Input` interface.
func (i *stdInput) Find(pos int, longest bool, dstCap []int) ([]int, error) {
	re := i.re.re
	if longest {
		re = re.Copy()
		re.Longest()
	}

	if i.bits != nil {
		pos = i.bits.Select(pos + 1)
		if pos < 0 {
			return nil, nil
		}
	}

	a := doExecute(re, nil, nil, i.str, pos, i.re.numCap, dstCap)

	applyBitsRank(a, i.bits)
	return a, nil
}

// applyBitsRank modifies the positions in `a` by applying `rank(a[i] - 1)` to each position.
// If `a[i]` is negative, it remains unchanged.
// If `a` or `bits` is `nil`, this function is a noop.
func applyBitsRank(a []int, bits *util.BitArray) {
	if a == nil || bits == nil {
		return
	}
	for i, v := range a {
		if v >= 0 {
			a[i] = bits.Rank(v - 1)
		}
	}
}

// Flags is the implementation of the `Flags` function for the `Engine` interface.
func (r *fallbEngine) Flags() uint32 {
	return r.flags
}

// SubexpNames is the implementation of the `SubexpNames` function for the `Engine` interface.
func (r *fallbEngine) SubexpNames() []string {
	names := make([]string, 1+r.numSubexp)

	for group, i := range r.groupNames {
		names[i] = group
	}

	return names
}

// SubexpCount is the implementation of the `SubexpCount` function for the `Engine` interface.
func (r *fallbEngine) SubexpCount() int {
	return r.numSubexp
}

// SubexpIndex is the implementation of the `SubexpIndex` function for the `Engine` interface.
func (r *fallbEngine) SubexpIndex(name string) int {
	if i, ok := r.groupNames[name]; ok {
		return i
	}

	return -1
}

// SupportsLongest is the implementation of the `SupportsLongest` function for the `Engine` interface.
func (r *fallbEngine) SupportsLongest() bool {
	return false
}

// BuildInput is the implementation of the `BuildInput` function for the `Engine` interface.
func (r *fallbEngine) BuildInput(s string, endpos int) Input {
	chars, bits := r.getRuneOffsets(s, endpos)

	return &fallbInput{
		re:    r,
		chars: chars,
		bits:  bits,
	}
}

// getRuneOffsets transforms the string into a slice of character to be compatible with the`regexp2.Regexp` regex engine.
// Additionally, any invalid UTF-8 codepoints are replaced with valid ones.
// In addition, a bitarray is needed to convert between byte positions in the input string `s` and corresponding indices
// of characters in the modified character slice (of type `[]rune`).
// If the string contains only ASCII characters, creating offset slices is not needed.
// A maximum of `endpos` bytes are extracted from `s`.
//
// If a bitarray is returned, it contains the starting positions of the characters in `s`.
// If the i-th bit is a 1-bit, then the i-th byte in the string `s` indicates the first byte of a character.
// The succeeding 0-bits are the following bytes of the same character.
//
// A byte position `i` of a character in `s` is converted to its index `j` in the slice of runes using the bitarray `B` as follows:
//   - `j` is the number if 1-bits from 0 to `i-1`; determined with `rank(B, i - 1)`
//   - `i` is the position of the `j+1`-th 1-bit in `B`; determined with `select(B, j + 1)`
func (r *fallbEngine) getRuneOffsets(s string, endpos int) ([]rune, *util.BitArray) {
	if !r.isStr || isASCIIString(s) {
		// For ascii strings and bytes, all bytes are converted to its rune value and positions will not change.

		chars := make([]rune, endpos)
		for i := 0; i < endpos; i++ {
			chars[i] = rune(s[i])
		}

		return chars, nil
	}

	guess := utf8.RuneCountInString(s[:endpos])
	chars := make([]rune, 0, guess)

	var bits util.BitArray
	bits.Grow(endpos + 1) // +1 for the last bit representing `endpos`

	pos := 0

	for len(s) > 0 {
		// Get the next UTF-8 codepoint
		c, size := utf8.DecodeRuneInString(s)

		pos += size
		if pos > endpos {
			break
		}

		if c == utf8.RuneError {
			c = rune(s[0])
		}

		chars = append(chars, c)

		bits.Append(true)
		bits.AppendN(false, size-1)

		s = s[size:] // if the rune is not valid, the size returned is 1, so slicing with `size` is correct
	}

	// Append a last element, corresponding to `endpos`.
	bits.Append(true)
	bits.Optimize()

	return chars, &bits
}

// Find is the implementation of the `Find` function for the `Input` interface.
func (i *fallbInput) Find(pos int, _ bool, dstCap []int) ([]int, error) {
	if i.bits != nil {
		pos = i.bits.Rank(pos - 1)
	}
	if pos > len(i.chars) {
		return nil, nil
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

	// Convert the slice of groups to a slice of start and end positions
	for index, g := range groups {
		start := -1
		end := -1

		if len(g.Captures) != 0 {
			start = g.Index
			end = g.Index + g.Length
		}

		a[2*index] = start
		a[2*index+1] = end
	}

	applyBitsSelect(a, i.bits)
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

// applyBitsRank modifies the positions in `a` by applying `select(a[i] + 1)` to each position.
// If `a[i]` is negative, it remains unchanged.
// If `a` or `bits` is `nil`, this function is a noop.
func applyBitsSelect(a []int, bits *util.BitArray) {
	if a == nil || bits == nil {
		return
	}
	for i, v := range a {
		if v >= 0 {
			a[i] = bits.Select(v + 1)
		}
	}
}

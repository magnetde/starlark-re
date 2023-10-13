package re

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const (
	// Maximum cache size; 32 should be more than enough, because Starlark scripts stay relatively small.
	maxRegexpCacheSize = 32

	// Maximum possible value of a position.
	// Should be used as the default value of `endpos`, because the position parameters always gets clamped.
	// See also `clamp()`.
	posMax = math.MaxInt
)

// Possible flags for the flag parameter.
const (
	_ = 1 << iota // `re.TEMPLATE`; unused
	reFlagIgnoreCase
	reFlagLocale
	reFlagMultiline
	reFlagDotAll
	reFlagUnicode
	reFlagVerbose
	reFlagDebug
	reFlagASCII
)

var zeroInt = starlark.MakeInt(0)

// Module is a module type used for the re module.
// A new type is implemented instead of using the previous `starlarkstruct.Module` type,
// since the module contains a LRU cache for compiled regexps.
// The cache is implemented with a map and a linked list.
// When the cache exceeds the maximum size, the oldest used element is purged.
// TODO: The regexp cache is not thread safe.
// TODO: when the modules are loaded with `load`, is should instaiate a new module.
type Module struct {
	members starlark.StringDict

	list        *list.List                 // Least recent used regexps
	cache       map[cacheKey]*list.Element // Mapping of patterns to list elements
	re2Fallback bool                       // create regexp2 fallbacks
}

// cacheKey is a type, that is used for cache key, containing the pattern and the flags.
type cacheKey struct {
	pattern string
	isStr   bool
	flags   int
}

// Is necessary, because each list element needs to store the key in the map.
type cacheValue struct {
	pattern *Pattern
	key     cacheKey
}

// NewModule creates a new re module with the given member dict.
func NewModule(re2Fallback bool) *Module {
	members := starlark.StringDict{
		"A":          starlark.MakeInt(reFlagASCII),
		"ASCII":      starlark.MakeInt(reFlagASCII),
		"DEBUG":      starlark.MakeInt(reFlagDebug),
		"I":          starlark.MakeInt(reFlagIgnoreCase),
		"IGNORECASE": starlark.MakeInt(reFlagIgnoreCase),
		"L":          starlark.MakeInt(reFlagLocale),
		"LOCALE":     starlark.MakeInt(reFlagLocale),
		"M":          starlark.MakeInt(reFlagMultiline),
		"MULTILINE":  starlark.MakeInt(reFlagMultiline),
		"NOFLAG":     zeroInt,
		"S":          starlark.MakeInt(reFlagDotAll),
		"DOTALL":     starlark.MakeInt(reFlagDotAll),
		"U":          starlark.MakeInt(reFlagUnicode),
		"UNICODE":    starlark.MakeInt(reFlagUnicode),
		"X":          starlark.MakeInt(reFlagVerbose),
		"VERBOSE":    starlark.MakeInt(reFlagVerbose),

		"compile": starlark.NewBuiltin("compile", reCompile),
		"purge":   starlark.NewBuiltin("purge", rePurge),

		"search":    starlark.NewBuiltin("search", reSearch),
		"match":     starlark.NewBuiltin("match", reMatch),
		"fullmatch": starlark.NewBuiltin("fullmatch", reFullmatch),
		"split":     starlark.NewBuiltin("split", reSplit),
		"findall":   starlark.NewBuiltin("findall", reFindall),
		"finditer":  starlark.NewBuiltin("finditer", reFinditer),
		"sub":       starlark.NewBuiltin("sub", reSub),
		"subn":      starlark.NewBuiltin("subn", reSub),
		"escape":    starlark.NewBuiltin("subn", reEscape),
	}

	r := Module{
		members:     members,
		list:        list.New(),
		cache:       make(map[cacheKey]*list.Element),
		re2Fallback: re2Fallback,
	}

	return &r
}

// Check, if the type satisfies the interfaces.
var (
	_ starlark.Value    = (*Module)(nil)
	_ starlark.HasAttrs = (*Module)(nil)
)

func (m *Module) Freeze()               { m.members.Freeze() }
func (m *Module) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: %s", m.Type()) }
func (m *Module) String() string        { return "<module re>" }
func (m *Module) Truth() starlark.Bool  { return true }
func (m *Module) Type() string          { return "module" }

func (m *Module) Attr(name string) (starlark.Value, error) {
	if v, ok := m.members[name]; ok {
		if b, ok := v.(*starlark.Builtin); ok {
			return b.BindReceiver(m), nil
		}

		return v, nil
	}

	return nil, nil
}
func (m *Module) AttrNames() []string { return m.members.Keys() }

// compile compiles a regex pattern. If the pattern is already in the cache,
// the compiled pattern is returned from the cache.
// Else, the pattern is compiled and then added to the cache.
// If the cache exceeds a certain size (`maxRegexpCacheSize`), the oldest element is purged from the cache.
func (m *Module) compile(pattern strOrBytes, flags int) (*Pattern, error) {
	key := cacheKey{
		pattern.value,
		pattern.isString,
		flags,
	}

	if e, ok := m.cache[key]; ok { // pattern found in the cache
		m.list.MoveToFront(e) // "refresh" the pattern in the linked list
		return e.Value.(*cacheValue).pattern, nil
	}

	// purge elements, if the size exceeds a certain threshold
	if m.list.Len() >= maxRegexpCacheSize {
		last := m.list.Back() // determine the oldest element
		lastValue := last.Value.(*cacheValue)
		lastKey := lastValue.key

		// Delete from map and list
		delete(m.cache, lastKey)
		m.list.Remove(last)
	}

	p, err := newPattern(pattern, flags, m.re2Fallback)
	if err != nil {
		return nil, err
	}

	// Add the compiled pattern to the cache.
	v := &cacheValue{
		pattern: p,
		key:     key,
	}

	m.cache[key] = m.list.PushFront(v)

	return p, nil
}

// purge clears the regex cache.
func (m *Module) purge() {
	m.list.Init()
	clear(m.cache)
}

// reCompile precompiles a regex string into a pattern object,
// which can be used for matching using its `search`, `match` and other methods.
// Because all member functions of the `re` module cache compiled patterns,
// this function is only necessary, if the number of regexes exceeds the maximum cache size (`maxRegexpCacheSize`).
func reCompile(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		flags   int
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "pattern", &pattern, "flags?", &flags); err != nil {
		return nil, err
	}

	return compilePattern(b, pattern, flags)
}

// patternParam is a Starlark type, representing the possible types of the pattern parameter.
type patternParam struct {
	compiled *Pattern
	raw      strOrBytes
}

type strOrBytes struct {
	value    string
	isString bool
}

var (
	_ starlark.Unpacker = (*strOrBytes)(nil)
	_ starlark.Unpacker = (*patternParam)(nil)
)

func (p *patternParam) Unpack(v starlark.Value) error {
	if c, ok := v.(*Pattern); ok {
		p.compiled = c
		return nil
	}

	err := p.raw.Unpack(v)
	if err != nil {
		return errors.New("first argument must be string or compiled pattern")
	}

	return nil
}

func (s *strOrBytes) Unpack(v starlark.Value) error {
	switch t := v.(type) {
	case starlark.String:
		s.value = string(t)
		s.isString = true
	case starlark.Bytes:
		s.value = string(t)
		s.isString = false
	default:
		return fmt.Errorf("got %s, want str or bytes", v.Type())
	}

	return nil
}

func (s *strOrBytes) sameType(v strOrBytes) error {
	if s.isString != v.isString {
		return fmt.Errorf("got %s, want %s", s.typeString(), v.typeString())
	}

	return nil
}

func (s *strOrBytes) typeString() string {
	if s.isString {
		return "str"
	}

	return "bytes"
}

func (s *strOrBytes) asType(v string) starlark.Value {
	if s.isString {
		return starlark.String(v)
	}

	return starlark.Bytes(v)
}

// reCompile clears the regular expression cache.
func rePurge(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Module)
	m.purge()

	return starlark.None, nil
}

// reSearch scans through the string looking for the first location where the regular expression pattern produces a match,
// and returns a corresponding `Match`. Returns `None` if no position in the string matches the pattern.
func reSearch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   int
	)
	if err := starlark.UnpackArgs("search", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := compilePattern(b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexpSearch(p, str, 0, posMax)
}

// regexpSearch - see `reSearch`.
func regexpSearch(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	str, pos, err := checkParams(p, str, pos, endpos)
	if err != nil {
		return nil, err
	}

	indx := findMatch(p.re, str.value, pos)
	if indx == nil {
		return starlark.None, nil
	}

	return newMatch(p, str, indx, 0, len(str.value)), nil
}

// checkParams, checks, if the parameter `str` matches the expected type.
// If not, an error is returned.
// Also, the parameters `pos` and `endpos` gets clamped in range [0, n], where n is the length of `str`.
// This function returns `s[:endpos]` and `pos`, where `pos` and `endpos` are clamped.
func checkParams(p *Pattern, str strOrBytes, pos, endpos int) (strOrBytes, int, error) {
	var zero strOrBytes

	err := p.pattern.sameType(str)
	if err != nil {
		return zero, 0, err
	}

	// Adjust boundaries
	n := len(str.value)
	pos = clamp(pos, n)
	endpos = clamp(endpos, n)

	str.value = str.value[:endpos]

	return str, pos, nil
}

// clamp clamps `pos` between 0 and `length`.
func clamp(pos, length int) int {
	return min(max(pos, 0), length)
}

// reMatch scan through string looking for the first location where the regular expression pattern produces a match,
// and return a corresponding `Match`. Return `None` if no position in the string matches the pattern
func reMatch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   int
	)
	if err := starlark.UnpackArgs("match", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := compilePattern(b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexpMatch(p, str, 0, posMax)
}

// regexpMatch - see `reMatch`.
func regexpMatch(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	str, pos, err := checkParams(p, str, pos, endpos)
	if err != nil {
		return nil, err
	}

	match := findMatch(p.re, str.value, pos)
	if match == nil || (len(match) > 0 && match[0] != pos) {
		return starlark.None, nil
	}

	return newMatch(p, str, match, 0, len(str.value)), nil
}

// reMatch scan through string looking for the first location where the regular expression pattern produces a match,
// and return a corresponding `Match`. Return `None` if no position in the string matches the pattern

// reFullMatch return a corresponding `Match`, if the whole string matches the regular expression pattern.
// This function returns `None` if the string does not match the pattern
func reFullmatch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   int
	)
	if err := starlark.UnpackArgs("match", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := compilePattern(b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexpFullmatch(p, str, 0, posMax)
}

// regexpFullmatch - see `reFullmatch`.
func regexpFullmatch(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	str, pos, err := checkParams(p, str, pos, endpos)
	if err != nil {
		return nil, err
	}

	match := findLongestMatch(p.re, str.value, pos)
	if match == nil || len(match) < 2 {
		return starlark.None, nil
	}

	if match[0] != pos || match[1] != len(str.value) {
		return starlark.None, nil
	}

	return newMatch(p, str, match, 0, len(str.value)), nil
}

// reSplit splits a string by the occurrences of a pattern.
// If maxsplit is nonzero, at most maxsplit splits occur, and the remainder of the string is returned as the final element of the list.
func reSplit(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern         patternParam
		str             strOrBytes
		maxSplit, flags int
	)
	if err := starlark.UnpackArgs("split", args, kwargs, "pattern", &pattern, "string", &str, "maxsplit?", &maxSplit, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := compilePattern(b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexpSplit(p, str, maxSplit)
}

// regexpSplit - see `reSplit`.
func regexpSplit(p *Pattern, str strOrBytes, maxSplit int) (starlark.Value, error) {
	err := p.pattern.sameType(str)
	if err != nil {
		return nil, err
	}

	return split(p, str, maxSplit), nil
}

// reFindAll returns all non-overlapping matches of pattern in string, as a list of strings or tuples.
// The string is scanned left-to-right, and matches are returned in the order found.
// If one or more groups are present in the pattern, return a list of groups;
// this will be a list of tuples if the pattern has more than one group.
// Empty matches are included in the result.
func reFindall(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   int
	)
	if err := starlark.UnpackArgs("findall", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := compilePattern(b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexpFindall(p, str, 0, posMax)
}

// regexpFindall - see `reFindAll`.
func regexpFindall(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	str, pos, err := checkParams(p, str, pos, endpos)
	if err != nil {
		return nil, err
	}

	s := str.value
	var l []starlark.Value

	findMatches(p.re, s, pos, 0, func(match []int) bool {
		n := len(match) / 2

		var v starlark.Value
		switch n {
		case 1:
			// Match contains no groups; element is the whole match.

			v = p.pattern.asType(s[match[0]:match[1]])
		case 2:
			// Match contains one group; element is this group.

			v = p.pattern.asType(s[match[2]:match[3]])
		default:
			// Match contains multiple groups; element is a tuple of groups.

			t := make(starlark.Tuple, 0, n-1)
			for j := 1; j < n; j++ {
				if match[2*j] >= 0 {
					t = append(t, p.pattern.asType(s[match[2*j]:match[2*j+1]]))
				} else {
					t = append(t, p.pattern.asType(""))
				}
			}

			v = t
		}

		l = append(l, v)
		return true
	})

	return starlark.NewList(l), nil
}

// reFindIter returns an list containing `Match` objects over all non-overlapping matches for the RE pattern in string.
// The string is scanned left-to-right, and matches are returned in the order found. Empty matches are included in the result.
func reFinditer(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   int
	)
	if err := starlark.UnpackArgs("finditer", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := compilePattern(b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexpFinditer(p, str, 0, posMax)
}

// regexpFinditer - see `reFinditer`.
func regexpFinditer(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	str, pos, err := checkParams(p, str, pos, endpos)
	if err != nil {
		return nil, err
	}

	var l []starlark.Value

	findMatches(p.re, str.value, pos, 0, func(match []int) bool {
		l = append(l, newMatch(p, str, match, 0, len(str.value)))
		return true
	})

	return starlark.NewList(l), nil
}

// reSub return the text obtained by replacing the leftmost non-overlapping occurrences of the pattern in the text by the replacement repl,
// replacing a maximum number of `count`.
// If the pattern is not found, the text is returned unchanged.
// If the name of the builtin in "subn", the return value is the tuple `(new_string, number_of_subs_made)` instead.
func reSub(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern      patternParam
		repl         starlark.Value
		str          strOrBytes
		count, flags int
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "pattern", &pattern, "repl", &repl, "string", &str, "count?", &count, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := compilePattern(b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexpSub(thread, b.Name(), p, repl, str, count)
}

// regexpSub - see `reSub`.
func regexpSub(thread *starlark.Thread, name string, p *Pattern, repl starlark.Value, str strOrBytes, count int) (starlark.Value, error) {
	err := p.pattern.sameType(str)
	if err != nil {
		return nil, err
	}

	r, err := getReplacer(thread, p, repl)
	if err != nil {
		return nil, err
	}

	return sub(p, r, str, count, name == "subn")
}

// reEscape escapes special characters in pattern.
// This is useful if you want to match an arbitrary literal string that may have regular expression metacharacters in it.
func reEscape(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern strOrBytes
	if err := starlark.UnpackArgs("escape", args, kwargs, "pattern", &pattern); err != nil {
		return nil, err
	}

	escaped := escapePattern(pattern.value)
	return pattern.asType(escaped), nil
}

// Additional functions

// compilePattern compiles a regex pattern by compiling the pattern using the regex cache.
// The builtin receiver of the first parameter must be of type `*reModule`.
// See also `reModule.compile`.
func compilePattern(b *starlark.Builtin, p patternParam, flags int) (*Pattern, error) {
	if p.compiled != nil {
		if flags != 0 {
			return nil, errors.New("cannot process flags argument with a compiled pattern")
		}

		return p.compiled, nil
	}

	return b.Receiver().(*Module).compile(p.raw, flags)
}

// Compiled regex

// Pattern is a starlark representation of a compiled regular expression.
type Pattern struct {
	re      regexEngine
	pattern strOrBytes
	flags   int

	groupDict map[string]int
}

// newPattern creates a new pattern object, which is also a Starlark value.
func newPattern(pattern strOrBytes, flags int, re2Fallback bool) (*Pattern, error) {
	p := pattern.value
	isStr := pattern.isString

	// replace unicode patterns, that are supported by Pathon but not supported by Go
	pp, err := newPreprocessor(p, isStr, flags)
	if err != nil {
		return nil, err
	}

	re, err := compileRegex(pp, re2Fallback)
	if err != nil {
		if e, ok := strings.CutPrefix(err.Error(), "error parsing regexp: "); ok {
			err = errors.New(e)
		}

		return nil, err
	}

	// Python does not allow multiple groups with the same name, so this must be checked after compilation.
	// Furthermore, the groupdict is created, since it may be needed for matches.
	groups := make(map[string]int)

	names := re.SubexpNames()
	for i, name := range names {
		if i != 0 && name != "" {
			groups[name] = i
		}
	}

	o := Pattern{
		re:        re,
		pattern:   pattern,
		flags:     flags,
		groupDict: groups,
	}

	return &o, nil
}

// Check, if the type satiesfies the interfaces.
var (
	_ starlark.Value      = (*Pattern)(nil)
	_ starlark.HasAttrs   = (*Pattern)(nil)
	_ starlark.Comparable = (*Pattern)(nil)
)

// pattern returns the original regex string.
func (p *Pattern) patternValue() starlark.String { return starlark.String(p.pattern.value) }

func (p *Pattern) String() string {
	s := p.pattern

	r := quoteString(s.value, s.isString, true)
	if len(r) > 200 {
		r = r[:200]
	}

	var b strings.Builder
	b.WriteString("re.compile(")
	b.WriteString(r)
	p.writeflags(&b)
	b.WriteByte(')')
	return b.String()
}

// Order must be in sync with the `ReFlag...` constants.
// The order must match.
var flagnames = []string{
	"TEMPLATE",
	"IGNORECASE",
	"LOCALE",
	"MULTILINE",
	"DOTALL",
	"UNICODE",
	"VERBOSE",
	"DEBUG",
	"ASCII",
}

func (p *Pattern) writeflags(b *strings.Builder) {
	flags := p.flags

	// Omit re.UNICODE for valid string patterns.
	if p.pattern.isString && flags&(reFlagLocale|reFlagUnicode|reFlagASCII) == reFlagUnicode {
		flags &= ^reFlagUnicode
	}

	if flags == 0 {
		return
	}

	first := true

	for i := 0; i < len(flagnames); i++ {
		f := (1 << i)
		if flags&f == 0 {
			continue
		}
		if first {
			b.WriteString(", ")
			first = false
		} else {
			b.WriteByte('|')
		}
		b.WriteString("re.")
		b.WriteString(flagnames[i])
		flags &= ^f
	}

	if flags != 0 {
		if first {
			b.WriteString(", ")
		} else {
			b.WriteByte('|')
		}
		b.WriteString("0x")
		b.WriteString(strconv.FormatUint(uint64(flags), 16))
	}
}

func (p *Pattern) Type() string          { return "pattern" }
func (p *Pattern) Freeze()               {}
func (p *Pattern) Truth() starlark.Bool  { return p.pattern.value != "" }
func (p *Pattern) Hash() (uint32, error) { return p.patternValue().Hash() }

// Methods of the pattern object.
var patternMethods = map[string]*starlark.Builtin{
	"search":    starlark.NewBuiltin("search", patternSearch),
	"match":     starlark.NewBuiltin("match", patternMatch),
	"fullmatch": starlark.NewBuiltin("fullmatch", patternFullmatch),
	"split":     starlark.NewBuiltin("split", patternSplit),
	"findall":   starlark.NewBuiltin("findall", patternFindall),
	"finditer":  starlark.NewBuiltin("finditer", patternFinditer),
	"sub":       starlark.NewBuiltin("sub", patternSub),
	"subn":      starlark.NewBuiltin("subn", patternSub),
}

// patternMembers contains members of the pattern object.
// TODO: move larger functions to the bottom.
var patternMembers = map[string]func(p *Pattern) starlark.Value{
	// Python also determines the flags from the regex string
	"flags":   func(p *Pattern) starlark.Value { return starlark.MakeInt(p.flags) },
	"pattern": func(p *Pattern) starlark.Value { return p.patternValue() },
	"groups":  func(p *Pattern) starlark.Value { return starlark.MakeInt(p.re.NumSubexp()) },
	"groupindex": func(p *Pattern) starlark.Value {
		names := p.re.SubexpNames()

		gi := starlark.NewDict(len(names))
		for i, name := range names {
			if len(name) > 0 {
				_ = gi.SetKey(starlark.String(name), starlark.MakeInt(i))
			}
		}

		// Freeze the result
		gi.Freeze()

		return gi
	},
}

// Attr gets a value for a string attribute.
func (p *Pattern) Attr(name string) (starlark.Value, error) {
	if o, ok := patternMethods[name]; ok {
		return o.BindReceiver(p), nil
	}

	if o, ok := patternMembers[name]; ok {
		return o(p), nil
	}

	return nil, nil
}

// AttrNames lists available dot expression strings.
func (p *Pattern) AttrNames() []string {
	names := make([]string, 0, len(patternMethods)+len(patternMembers))

	for name := range patternMethods {
		names = append(names, name)
	}
	for name := range patternMembers {
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func (p *Pattern) CompareSameType(op syntax.Token, y starlark.Value, _ int) (bool, error) {
	o := y.(*Pattern)

	switch op {
	case syntax.EQL:
		ok, err := patternEquals(p, o)
		return ok, err
	case syntax.NEQ:
		ok, err := patternEquals(p, o)
		return !ok, err
	default:
		return false, fmt.Errorf("%s %s %s not implemented", p.Type(), op, o.Type())
	}
}

func patternEquals(x, y *Pattern) (bool, error) {
	return x.pattern == y.pattern && x.flags == y.flags, nil
}

// patternSearch - see `reSearch`.
func patternSearch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		str    strOrBytes
		pos    = 0
		endpos = posMax
	)
	if err := starlark.UnpackArgs("search", args, kwargs, "string", &str, "pos?", &pos, "endpos?", &endpos); err != nil {
		return nil, err
	}

	p := b.Receiver().(*Pattern)
	return regexpSearch(p, str, pos, endpos)
}

// patternMatch - see `reMatch`.
func patternMatch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		str    strOrBytes
		pos    = 0
		endpos = posMax
	)
	if err := starlark.UnpackArgs("match", args, kwargs, "string", &str, "pos?", &pos, "endpos?", &endpos); err != nil {
		return nil, err
	}

	p := b.Receiver().(*Pattern)
	return regexpMatch(p, str, pos, endpos)
}

// patternFullmatch - see `reFullmatch`.
func patternFullmatch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		str    strOrBytes
		pos    = 0
		endpos = posMax
	)
	if err := starlark.UnpackArgs("fullmatch", args, kwargs, "string", &str, "pos?", &pos, "endpos?", &endpos); err != nil {
		return nil, err
	}

	p := b.Receiver().(*Pattern)
	return regexpFullmatch(p, str, pos, endpos)
}

// patternSplit - see `reSplit`.
func patternSplit(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		str      strOrBytes
		maxSplit int
	)
	if err := starlark.UnpackArgs("split", args, kwargs, "string", &str, "maxsplit?", &maxSplit); err != nil {
		return nil, err
	}

	p := b.Receiver().(*Pattern)
	return regexpSplit(p, str, maxSplit)
}

// patternFindall - see `reFindall`.
func patternFindall(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		str    strOrBytes
		pos    = 0
		endpos = posMax
	)
	if err := starlark.UnpackArgs("findall", args, kwargs, "string", &str, "pos?", &pos, "endpos?", &endpos); err != nil {
		return nil, err
	}

	p := b.Receiver().(*Pattern)
	return regexpFindall(p, str, pos, endpos)
}

// patternFinditer - see `reFinditer`.
func patternFinditer(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		str    strOrBytes
		pos    = 0
		endpos = posMax
	)
	if err := starlark.UnpackArgs("finditer", args, kwargs, "string", &str, "pos?", &pos, "endpos?", &endpos); err != nil {
		return nil, err
	}

	p := b.Receiver().(*Pattern)
	return regexpFinditer(p, str, pos, endpos)
}

// patternSub - see `reSub`.
func patternSub(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		repl  starlark.Value
		str   strOrBytes
		count int
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "repl", &repl, "string", &str, "count?", &count); err != nil {
		return nil, err
	}

	p := b.Receiver().(*Pattern)
	return regexpSub(thread, b.Name(), p, repl, str, count)
}

// Match object

// Match
type Match struct {
	pattern *Pattern
	str     strOrBytes

	groups []group

	pos       int
	endpos    int
	lastIndex int
}

// TODO: comment
type group struct {
	start int
	end   int
	str   string
}

func (g *group) empty() bool {
	return g.start < 0 && g.end < 0
}

// newMatch creates a new match object.
func newMatch(p *Pattern, str strOrBytes, a []int, pos, endpos int) *Match {
	n := 1 + p.re.NumSubexp()

	lastIndex := -1
	lastIndexEnd := -1

	groups := make([]group, 0, n)

	for i := 0; i < n; i++ {
		if 2*i < len(a) {
			s := a[2*i]
			e := a[2*i+1]

			g := group{
				start: s,
				end:   e,
			}

			if s >= 0 && e >= 0 {
				// determine the index of the last group
				if i > 0 && e > lastIndexEnd {
					lastIndex = i
					lastIndexEnd = e
				}

				g.str = str.value[s:e]
			}

			groups = append(groups, g)
		}
	}

	m := Match{
		pattern: p,
		str:     str,

		groups:    groups,
		pos:       pos,
		endpos:    endpos,
		lastIndex: lastIndex,
	}

	return &m
}

// Check, if the type satiesfies the interfaces.
var (
	_ starlark.Value      = (*Match)(nil)
	_ starlark.HasAttrs   = (*Match)(nil)
	_ starlark.Mapping    = (*Match)(nil)
	_ starlark.Comparable = (*Match)(nil)
)

func (m *Match) String() string { // TODO
	g := m.groups[0]
	return fmt.Sprintf("<re.match object; span=(%d, %d), match=%s>",
		g.start, g.end, quoteString(g.str, m.str.isString, true),
	)
}

func (m *Match) Type() string         { return "match" }
func (m *Match) Freeze()              {}
func (m *Match) Truth() starlark.Bool { return true }

func (m *Match) Hash() (uint32, error) {
	var tmp uint32

	h, _ := m.pattern.Hash() // string type; no error possible

	for _, g := range m.groups {
		if g.empty() {
			tmp = 0
		} else {
			tmp, _ = starlark.String(g.str).Hash() // string type; no error possible
			tmp ^= uint32(g.start) ^ uint32(g.end)
		}

		h ^= tmp
		h *= 16777619
	}

	return h, nil
}

// matchMethods contains methods of the match object.
var matchMethods = map[string]*starlark.Builtin{
	"expand":    starlark.NewBuiltin("expand", matchExpand),
	"group":     starlark.NewBuiltin("group", matchGroup),
	"groups":    starlark.NewBuiltin("groups", matchGroups),
	"groupdict": starlark.NewBuiltin("groupdict", matchGroupDict),
	"start":     starlark.NewBuiltin("start", matchStart),
	"end":       starlark.NewBuiltin("end", matchEnd),
	"span":      starlark.NewBuiltin("end", matchSpan),
}

// matchMethods contains members of the match object.
var matchMembers = map[string]func(m *Match) starlark.Value{
	"pos":    func(m *Match) starlark.Value { return starlark.MakeInt(m.pos) },
	"endpos": func(m *Match) starlark.Value { return starlark.MakeInt(m.endpos) },
	"lastindex": func(m *Match) starlark.Value {
		if m.lastIndex < 0 {
			return starlark.None
		}

		return starlark.MakeInt(m.lastIndex)
	},
	"lastgroup": func(m *Match) starlark.Value {
		if m.lastIndex < 0 {
			return starlark.None
		}

		names := m.pattern.re.SubexpNames()
		name := names[m.lastIndex]
		if name == "" {
			return starlark.None
		}

		return m.str.asType(name)
	},
	"re":     func(m *Match) starlark.Value { return m.pattern },
	"string": func(m *Match) starlark.Value { return m.str.asType(m.str.value) },
	"regs": func(m *Match) starlark.Value {
		r := make(starlark.Tuple, len(m.groups))
		for i, g := range m.groups {
			r[i] = starlark.Tuple{starlark.MakeInt(g.start), starlark.MakeInt(g.end)}
		}

		return r
	},
}

// Attr gets a value for a string attribute.
func (m *Match) Attr(name string) (starlark.Value, error) {
	if o, ok := matchMethods[name]; ok {
		return o.BindReceiver(m), nil
	}

	if o, ok := matchMembers[name]; ok {
		return o(m), nil
	}

	return nil, nil
}

// AttrNames lists available dot expression strings.
func (m *Match) AttrNames() []string {
	names := make([]string, 0, len(matchMethods)+len(matchMembers))

	for name := range matchMethods {
		names = append(names, name)
	}
	for name := range matchMembers {
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

// Get returns the value corresponding to the specified key.
// For the match object, this is equals with calling the `group` function.
func (m *Match) Get(v starlark.Value) (starlark.Value, bool, error) {
	g, err := m.group(v)
	if err != nil {
		return nil, false, err
	}

	return g, true, nil
}

func (m *Match) CompareSameType(op syntax.Token, y starlark.Value, _ int) (bool, error) {
	o := y.(*Match)

	switch op {
	case syntax.EQL:
		ok, err := matchEquals(m, o)
		return ok, err
	case syntax.NEQ:
		ok, err := matchEquals(m, o)
		return !ok, err
	default:
		return false, fmt.Errorf("%s %s %s not implemented", m.Type(), op, o.Type())
	}
}

func matchEquals(x, y *Match) (bool, error) {
	if eq, err := patternEquals(x.pattern, y.pattern); err != nil {
		return false, err
	} else if !eq {
		return false, nil
	}

	if x.str != y.str {
		return false, nil
	}
	if !slices.Equal(x.groups, y.groups) {
		return false, nil
	}

	return x.pos == y.pos && x.endpos == y.endpos && x.lastIndex == y.lastIndex, nil
}

func (m *Match) group(v starlark.Value) (starlark.Value, error) {
	if i, ok := m.getIndex(v); ok {
		g := &m.groups[i]
		if g.empty() {
			return starlark.None, nil
		}

		return m.str.asType(g.str), nil
	}

	return nil, errors.New("IndexError: no such group")
}

func (m *Match) getIndex(v starlark.Value) (int, bool) {
	switch t := v.(type) {
	case starlark.Int:
		i, ok := t.Int64()
		if ok && i >= 0 && i < int64(len(m.groups)) {
			return int(i), true
		}
	case starlark.String:
		if i, ok := m.pattern.groupDict[string(t)]; ok {
			return i, true
		}
	}

	return 0, false
}

func matchExpand(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var template strOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "template", &template); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	err := m.str.sameType(template)
	if err != nil {
		return nil, err
	}

	// Create a new replacer for the template
	r, err := newTemplateReplacer(m.pattern.re, template.value, template.isString)
	if err != nil {
		return nil, err
	}

	// Replace the template
	s, err := r.replace(m)
	if err != nil {
		return nil, err
	}

	return m.str.asType(s), nil
}

func matchGroup(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), nil, kwargs); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)
	size := len(args)

	switch size {
	case 0:
		return m.group(zeroInt)
	case 1:
		return m.group(args[0])
	default:
		result := make(starlark.Tuple, size)

		for i := range result {
			g, err := m.group(args[i])
			if err != nil {
				return nil, err
			}

			result[i] = g
		}

		return result, nil
	}
}

func matchGroups(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var defaultValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "default?", &defaultValue); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	result := make(starlark.Tuple, 0, len(m.groups)-1)

	for _, group := range m.groups[1:] {
		var g starlark.Value
		if group.empty() {
			g = defaultValue
		} else {
			g = m.str.asType(group.str)
		}

		result = append(result, g)
	}

	return result, nil
}

func matchGroupDict(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var defaultValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "default?", &defaultValue); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	names := m.pattern.re.SubexpNames() // do not use m.pattern.groupDict because the order should be retained
	result := starlark.NewDict(len(names))

	for i, name := range names {
		if i != 0 && name != "" {
			sname := starlark.String(name)

			v, err := m.group(sname)
			if err != nil {
				return nil, err
			}

			err = result.SetKey(sname, v)
			if err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

func matchStart(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = zeroInt
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "group?", &group); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	i, ok := m.getIndex(group)
	if !ok {
		return starlark.MakeInt(-1), nil
	}

	return starlark.MakeInt(m.groups[i].start), nil
}

func matchEnd(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = zeroInt
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "group?", &group); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	i, ok := m.getIndex(group)
	if !ok {
		return starlark.MakeInt(-1), nil
	}

	return starlark.MakeInt(m.groups[i].end), nil
}

func matchSpan(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = zeroInt
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "group?", &group); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	i, ok := m.getIndex(group)
	if !ok {
		v := starlark.MakeInt(-1)
		return starlark.Tuple{v, v}, nil
	}

	s := starlark.MakeInt(m.groups[i].start)
	e := starlark.MakeInt(m.groups[i].end)
	return starlark.Tuple{s, e}, nil
}

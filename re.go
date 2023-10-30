package re

import (
	"container/list"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/magnetde/starlark-re/regex"
	"github.com/magnetde/starlark-re/util"
)

const (
	// Default maximum cache size; 64 should be more than enough, because Starlark scripts stay relatively small.
	defaultMaxCacheSize = 64

	// Maximum possible value of a position.
	// Should be used as the default value of `endpos`, because the position parameters always gets clamped.
	// See also `clamp()`.
	posMax = math.MaxInt
)

var (
	// Starlark value representing the integer value 0.
	zeroInt = starlark.MakeInt(0)

	// All available members of the "re" module.
	members = starlark.StringDict{
		"A":          makeFlags(regex.FlagASCII),
		"ASCII":      makeFlags(regex.FlagASCII),
		"DEBUG":      makeFlags(regex.FlagDebug),
		"I":          makeFlags(regex.FlagIgnoreCase),
		"IGNORECASE": makeFlags(regex.FlagIgnoreCase),
		"L":          makeFlags(regex.FlagLocale),
		"LOCALE":     makeFlags(regex.FlagLocale),
		"M":          makeFlags(regex.FlagMultiline),
		"MULTILINE":  makeFlags(regex.FlagMultiline),
		"NOFLAG":     zeroInt,
		"S":          makeFlags(regex.FlagDotAll),
		"DOTALL":     makeFlags(regex.FlagDotAll),
		"U":          makeFlags(regex.FlagUnicode),
		"UNICODE":    makeFlags(regex.FlagUnicode),
		"X":          makeFlags(regex.FlagVerbose),
		"VERBOSE":    makeFlags(regex.FlagVerbose),
		"FALLBACK":   makeFlags(regex.FlagFallback),

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
		"escape":    starlark.NewBuiltin("escape", reEscape),
	}
)

// makeFlags converts the flags to a Starlark integer.
func makeFlags(flag uint32) starlark.Int {
	return starlark.MakeUint64(uint64(flag))
}

// ModuleOptions represents the available options when initializing the "re" module.
// There are three options:
//   - `DisableCache` disables to store compiled patterns in a pattern cache, resulting in higher runtimes.
//   - `MaxCacheSize` sets the maximum size of the cache.
//   - `DisableFallback` disables the fallback engine `regexp2.Regexp`.
//     Compiling patterns that are not supported by `regexp.Regexp' will then fail.
type ModuleOptions struct {
	DisableCache    bool
	MaxCacheSize    int
	DisableFallback bool
}

// Module is a module type used for the "re" module.
// the "re" module contains an LRU cache for compiled regex patterns.
// This cache is implemented using a map and a linked list.
// When the cache exceeds the maximum size, the least recently used element is removed.
// The module is designed to be thread-safe.
type Module struct {
	members starlark.StringDict

	enableCache    bool // cache for compiled patterns is enabled
	maxCacheSize   int  // maximum size of compiled patterns in the cache
	enableFallback bool // regexp2 fallback engine is enabled

	mu    sync.Mutex                 // mutex for the regex cache
	list  *list.List                 // least recent used regexes
	cache map[cacheKey]*list.Element // mapping of patterns to list elements

}

// cacheKey is the type, that is used for keys of the cache map, containing the pattern,
// its type and the flags.
type cacheKey struct {
	pattern string
	isStr   bool
	flags   uint32
}

// The cacheValue type represents elements in the linked list of the cache.
// This type is necessary because each list element must store the key within the linked list.
// When the last element is removed from the linked list, it is also be deleted from the map using the key.
type cacheValue struct {
	pattern *Pattern
	key     cacheKey
}

// NewModule creates the Starlark "re" module with the default options returned by `DefaultOptions`.
func NewModule() *Module {
	return NewModuleOptions(DefaultOptions())
}

// DefaultOptions returns the default options:
//   - pattern cache is enabled
//   - a maximum cache size of 64
//   - the fallback regex engine is enabled
func DefaultOptions() *ModuleOptions {
	options := ModuleOptions{
		DisableCache:    false,
		MaxCacheSize:    defaultMaxCacheSize,
		DisableFallback: false,
	}

	return &options
}

// NewModuleOptions creates the Starlark "re" module with custom options.
// The options may be nil. If this is the case then the default options are used.
// If the cache size is not a positive integer, the pattern cache is disabled.
func NewModuleOptions(opts *ModuleOptions) *Module {
	if opts == nil {
		opts = DefaultOptions()
	}

	enableCache := !opts.DisableCache
	maxCacheSize := opts.MaxCacheSize
	enableFallback := !opts.DisableFallback
	if maxCacheSize < 0 {
		enableCache = false // disable cache
	} else if maxCacheSize == 0 {
		maxCacheSize = defaultMaxCacheSize
	}

	// By default each "re" module instance shares the same map,
	// because `members` is never modified.
	modMembers := members

	if !enableFallback {
		// If the fallback engine is disabled, the map of members is cloned and the FALLBACK flag is removed.
		modMembers = maps.Clone(modMembers)
		delete(modMembers, "FALLBACK")
	}

	r := Module{
		members:        modMembers,
		enableCache:    enableCache,
		maxCacheSize:   maxCacheSize,
		enableFallback: enableFallback,
	}

	if enableCache {
		r.list = list.New()
		r.cache = make(map[cacheKey]*list.Element)
	}

	return &r
}

// Check if the type satisfies the interfaces.
var (
	_ starlark.Value    = (*Module)(nil)
	_ starlark.HasAttrs = (*Module)(nil)
)

// String returns the string representation of the value.
func (m *Module) String() string { return "<module re>" }

// Type returns a short string describing the value's type.
func (m *Module) Type() string { return "module" }

// Freeze marks the value and all members as frozen.
func (m *Module) Freeze() { m.members.Freeze() }

// Truth returns the truth value of the object.
func (m *Module) Truth() starlark.Bool { return true }

// Hash returns an error, because the re module is not hashable.
func (m *Module) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: %s", m.Type()) }

// Attr returns the member of the module with the given name.
// If the member exists and is of the type `*Builtin`, it becomes bound to this module.
// Otherwise, the member is returned as normal.
// If the member does not exist, `nil, nil` is returned.
func (m *Module) Attr(name string) (starlark.Value, error) {
	if v, ok := m.members[name]; ok {
		if b, ok := v.(*starlark.Builtin); ok {
			return b.BindReceiver(m), nil
		}

		return v, nil
	}

	return nil, nil
}

// AttrNames lists available dot expression members.
func (m *Module) AttrNames() []string { return m.members.Keys() }

// compile compiles a regex pattern.
// If the pattern cache is disabled, the regex pattern is compiled as normal.
// Otherwise, the pattern is compiled by using the cache (see `cachedCompile`).
func (m *Module) compile(thread *starlark.Thread, pattern strOrBytes, flags uint32) (*Pattern, error) {
	if !m.enableCache {
		p, _, err := newPattern(thread, pattern, flags, m.enableFallback)
		return p, err
	}

	return m.cachedCompile(thread, pattern, flags)
}

// cachedCompile compiles a regex pattern by using the cache.
// If the pattern already exists in the cache, the compiled pattern is returned.
// Otherwise, the compiled pattern is compiled an then added to the cache.
// When the cache exceeds `m.maxCacheSize`, the oldest entry is removed.
// Do not call this function directly. Use `regexCompile` or `Module.compile` instead.
func (m *Module) cachedCompile(thread *starlark.Thread, pattern strOrBytes, flags uint32) (*Pattern, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := cacheKey{
		pattern.value,
		pattern.isString,
		flags,
	}

	if e, ok := m.cache[key]; ok { // pattern found in the cache
		m.list.MoveToFront(e) // "refresh" the pattern in the linked list
		return e.Value.(*cacheValue).pattern, nil
	}

	p, add, err := newPattern(thread, pattern, flags, m.enableFallback)
	if err != nil {
		return nil, err
	}

	if add {
		// purge elements, if the size exceeds a certain threshold
		if m.list.Len() >= m.maxCacheSize {
			last := m.list.Back() // determine the oldest element
			lastValue := last.Value.(*cacheValue)
			lastKey := lastValue.key

			// Delete from map and list
			delete(m.cache, lastKey)
			m.list.Remove(last)
		}

		// Add the compiled pattern to the cache.
		v := &cacheValue{
			pattern: p,
			key:     key,
		}

		m.cache[key] = m.list.PushFront(v)
	}

	return p, nil
}

// purge clears the regex cache.
func (m *Module) purge() {
	if m.enableCache {
		m.mu.Lock()
		defer m.mu.Unlock()

		m.list.Init()
		clear(m.cache)
	}
}

// Function naming
// ===============
//
// For every member function of the "re" module, there is a corresponding Go function
// that is invoked when calling the member function.
// These functions follow the naming scheme `re*`, where `*` is the title case name of the member function.
// Most of these functions are also present in the pattern type, named `pattern*` with the same suffix `*`.
// Since it would be unnecessary to reimplement each of these functions for the type `Pattern`,
// each of the `re*` und `pattern*` functions call a subroutine called `regex*`, where `*` is the common suffix.
// The `regex*` functions then do all the logic.
//

// reCompile precompiles a regex string into a pattern object, allowing it to be used for matching,
// using its methods like `match` or `search`.
// Since all member functions of the `re` module cache compiled patterns,
// it is only necessary to use this function if the number of regexes exceeds the maximum cache size (`m.maxCacheSize`).
func reCompile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		flags   uint32
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "pattern", &pattern, "flags?", &flags); err != nil {
		return nil, err
	}

	return regexCompile(thread, b, pattern, flags)
}

// regexCompile returns a compiled regex pattern from the pattern parameter and the flags.
// If the parameter is already a compiled pattern, it is returned unchanged.
// If not, the pattern is compiled using the regex cache, if enabled.
// The builtin receiver must be of type `*Module`.
// See also `Module.compile`.
func regexCompile(thread *starlark.Thread, b *starlark.Builtin, pattern patternParam, flags uint32) (*Pattern, error) {
	if pattern.compiled != nil {
		if flags != 0 {
			return nil, errors.New("cannot process flags argument with a compiled pattern")
		}

		return pattern.compiled, nil
	}

	p, err := b.Receiver().(*Module).compile(thread, pattern.raw, flags)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// patternParam represents the unpacked pattern parameter value.
// It may be one of the Starlark types `str`, `bytes` or `Pattern`.
type patternParam struct {
	compiled *Pattern
	raw      strOrBytes
}

// strOrBytes represents a unpacked parameter value,
// that can be a Starlark type of either `str` or `bytes`.
type strOrBytes struct {
	value    string
	isString bool
}

// Check if the types satisfy the interface.
var (
	_ starlark.Unpacker = (*strOrBytes)(nil)
	_ starlark.Unpacker = (*patternParam)(nil)
)

// Unpack unpacks a Starlark value into a pattern parameter.
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

// Unpack unpacks a Starlark value, that must be either of type `str` or `bytes`.
func (s *strOrBytes) Unpack(v starlark.Value) error {
	switch v := v.(type) {
	case starlark.String:
		s.value = string(v)
		s.isString = true
	case starlark.Bytes:
		s.value = string(v)
		s.isString = false
	default:
		return fmt.Errorf("got %s, want str or bytes", v.Type())
	}

	return nil
}

// sameType tests, if `s` and `v` represent the same Starlark type.
// If not, an error is returned.
func (s *strOrBytes) sameType(v strOrBytes) error {
	if s.isString != v.isString {
		return fmt.Errorf("got %s, want %s", s.typeString(), v.typeString())
	}

	return nil
}

// typeString returns the name of the corresponding Starlark type.
func (s *strOrBytes) typeString() string {
	if s.isString {
		return "str"
	}

	return "bytes"
}

// asType returns a Starlark value of `v`, with the type of `s`.
func (s *strOrBytes) asType(v string) starlark.Value {
	if s.isString {
		return starlark.String(v)
	}

	return starlark.Bytes(v)
}

// reCompile clears the regex cache.
func rePurge(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Module)
	m.purge()

	return starlark.None, nil
}

// reSearch scans through the string looking for the first location where the regex pattern produces a match,
// and returns a corresponding `Match`. Returns `None` if no position in the string matches the pattern.
func reSearch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   uint32
	)
	if err := starlark.UnpackArgs("search", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := regexCompile(thread, b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexSearch(p, str, 0, posMax)
}

// regexSearch - see `reSearch`.
func regexSearch(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	err := checkParams(p, str, &pos, &endpos)
	if err != nil {
		return nil, err
	}

	s := str.value[:endpos]

	match, err := findMatch(p.re, s, pos, false)
	if err != nil {
		return nil, err
	}

	if match == nil {
		return starlark.None, nil
	}

	return newMatch(p, str, match, pos, endpos), nil
}

// checkParams checks, if the parameter `str` matches the expected type of the raw pattern of `p`.
// If it does not match, an error is returned.
// The parameters `pos` and `endpos` are limited to the range [0, n], where `n` is the length of `str`
// and writes the adjusted values back.
func checkParams(p *Pattern, str strOrBytes, pos, endpos *int) error {
	err := p.pattern.sameType(str)
	if err != nil {
		return err
	}

	// Adjust boundaries
	n := len(str.value)

	*pos = clamp(*pos, n)
	*endpos = clamp(*endpos, n)

	return nil
}

// clamp limits `pos` between 0 and `length`.
func clamp(pos, length int) int {
	return min(max(pos, 0), length)
}

// reMatch scan through string looking for the first location where the regex pattern produces a match,
// and return a corresponding `Match`. Returns `None` if no position in the string matches the pattern
func reMatch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   uint32
	)
	if err := starlark.UnpackArgs("match", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := regexCompile(thread, b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexMatch(p, str, 0, posMax)
}

// regexMatch - see `reMatch`.
func regexMatch(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	err := checkParams(p, str, &pos, &endpos)
	if err != nil {
		return nil, err
	}

	s := str.value[:endpos]

	match, err := findMatch(p.re, s, pos, false)
	if err != nil {
		return nil, err
	}

	if match == nil || match[0] != pos {
		return starlark.None, nil
	}

	return newMatch(p, str, match, pos, endpos), nil
}

// reFullMatch return a corresponding `Match`, if the whole string matches the regex pattern.
// This function returns `None` if the string does not match the pattern
func reFullmatch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   uint32
	)
	if err := starlark.UnpackArgs("match", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := regexCompile(thread, b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexFullmatch(p, str, 0, posMax)
}

// regexFullmatch - see `reFullmatch`.
func regexFullmatch(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	err := checkParams(p, str, &pos, &endpos)
	if err != nil {
		return nil, err
	}

	s := str.value[:endpos]

	match, err := findMatch(p.re, s, pos, true /* find longest */)
	if err != nil {
		return nil, err
	}

	if match == nil || match[0] != pos || match[1] != endpos {
		return starlark.None, nil
	}

	return newMatch(p, str, match, pos, endpos), nil
}

// reSplit splits a string by the occurrences of a pattern.
// If the pattern contains capturing parentheses, the text of all groups in the resulting list is also returned.
// If the maxsplit parameter is non-zero, it will split the string at most maxsplit times and
// return remaining string as the final element of the list.
func reSplit(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern  patternParam
		str      strOrBytes
		maxSplit int
		flags    uint32
	)
	if err := starlark.UnpackArgs("split", args, kwargs, "pattern", &pattern, "string", &str, "maxsplit?", &maxSplit, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := regexCompile(thread, b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexSplit(p, str, maxSplit)
}

// regexSplit - see `reSplit`.
func regexSplit(p *Pattern, str strOrBytes, maxSplit int) (starlark.Value, error) {
	err := p.pattern.sameType(str)
	if err != nil {
		return nil, err
	}

	return split(p, str, maxSplit)
}

// reFindAll returns all non-overlapping matches of pattern in string, as a list of strings or tuples.
// The string is scanned left-to-right, and matches are returned in the order found.
// If one or more groups are present in the pattern, return a list of groups;
// this will be a list of tuples if the pattern has more than one group.
// Empty matches are included in the result.
func reFindall(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   uint32
	)
	if err := starlark.UnpackArgs("findall", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := regexCompile(thread, b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexFindall(p, str, 0, posMax)
}

// regexFindall - see `reFindAll`.
func regexFindall(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	err := checkParams(p, str, &pos, &endpos)
	if err != nil {
		return nil, err
	}

	s := str.value[:endpos]
	var l []starlark.Value

	err = findMatches(p.re, s, pos, 0, func(match []int) error {
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
		return nil
	})
	if err != nil {
		return nil, err
	}

	return starlark.NewList(l), nil
}

// reFindIter returns an list containing `Match` objects over all non-overlapping matches for the RE pattern in string.
// The string is scanned left-to-right, and matches are returned in the order found. Empty matches are included in the result.
func reFinditer(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		str     strOrBytes
		flags   uint32
	)
	if err := starlark.UnpackArgs("finditer", args, kwargs, "pattern", &pattern, "string", &str, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := regexCompile(thread, b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexFinditer(p, str, 0, posMax)
}

// regexFinditer - see `reFinditer`.
func regexFinditer(p *Pattern, str strOrBytes, pos, endpos int) (starlark.Value, error) {
	err := checkParams(p, str, &pos, &endpos)
	if err != nil {
		return nil, err
	}

	s := str.value[:endpos]
	var v starlark.Tuple

	err = findMatches(p.re, s, pos, 0, func(match []int) error {
		v = append(v, newMatch(p, str, match, pos, endpos))
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &matchIter{v}, nil
}

// reSub return the text obtained by replacing the leftmost non-overlapping occurrences of the pattern in the text by the replacement repl,
// replacing a maximum number of `count`. If the pattern is not found, the text is returned unchanged.
// If the name of the builtin is "subn", the return value is the tuple `(new_string, number_of_subs_made)` instead.
func reSub(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		pattern patternParam
		repl    starlark.Value
		str     strOrBytes
		count   int
		flags   uint32
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "pattern", &pattern, "repl", &repl, "string", &str, "count?", &count, "flags?", &flags); err != nil {
		return nil, err
	}

	p, err := regexCompile(thread, b, pattern, flags)
	if err != nil {
		return nil, err
	}

	return regexSub(thread, b.Name(), p, repl, str, count)
}

// regexSub - see `reSub`.
func regexSub(thread *starlark.Thread, name string, p *Pattern, repl starlark.Value, str strOrBytes, count int) (starlark.Value, error) {
	err := p.pattern.sameType(str)
	if err != nil {
		return nil, err
	}

	r, err := buildReplacer(thread, p, repl)
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

// Compiled regex pattern

// Pattern is a starlark representation of a compiled regex.
type Pattern struct {
	re              regex.Engine
	pattern         strOrBytes
	flags           uint32
	fallbackEnabled bool // necessary to create a correct string representation
}

// newPattern creates a new pattern object, which is also a Starlark value.
// If the compiler returns a debug representation of the pattern,
// it will be printed to the print function of the current Starlark thread and the
// compiled pattern should not be cached, so the second return value is `false'.
// Do not call this function directly. Use `regexCompile` or `Module.compile` instead.
func newPattern(thread *starlark.Thread, pattern strOrBytes, flags uint32, fallbackEnabled bool) (*Pattern, bool, error) {
	re, debug, err := regex.Compile(pattern.value, pattern.isString, flags, fallbackEnabled)
	if err != nil {
		return nil, false, err
	}

	o := Pattern{
		re:              re,
		pattern:         pattern,
		flags:           re.Flags(),
		fallbackEnabled: fallbackEnabled,
	}

	// Dump the compiled regex if the DEBUG flag is passed.
	if debug != "" {
		if thread.Print != nil {
			thread.Print(thread, debug)
		} else {
			fmt.Fprintln(os.Stderr, debug)
		}
	}

	// The element should only be added to the cache, if the debug flag was not enabled
	return &o, debug == "", nil
}

// Check if the type satisfies the interfaces.
var (
	_ starlark.Value      = (*Pattern)(nil)
	_ starlark.HasAttrs   = (*Pattern)(nil)
	_ starlark.Comparable = (*Pattern)(nil)
)

// String returns the string representation of the value.
func (p *Pattern) String() string {
	s := p.pattern

	r := util.Repr(s.value, s.isString)
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

// Names of all available flags.
// The order has to match the order of the `Flag...` constants.
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
	"FALLBACK",
}

// writeflags writes a string representation of the regex flags to the string builder.
func (p *Pattern) writeflags(b *strings.Builder) {
	flags := p.flags

	// Omit re.UNICODE for valid string patterns.
	if p.pattern.isString && flags&(regex.FlagLocale|regex.FlagUnicode|regex.FlagASCII) == regex.FlagUnicode {
		flags &= ^regex.FlagUnicode
	}

	if flags == 0 {
		return
	}

	first := true

	for i := 0; i < len(flagnames); i++ {
		f := uint32(1) << i
		if flags&f == 0 {
			continue
		}

		// print the fallback flag only, if enabled
		if f == regex.FlagFallback && !p.fallbackEnabled {
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

// Type returns a short string describing the value's type.
func (p *Pattern) Type() string { return "Pattern" }

// Freeze marks the value and all members as frozen.
func (p *Pattern) Freeze() {}

// Truth returns the truth value of the object.
func (p *Pattern) Truth() starlark.Bool { return p.pattern.value != "" }

// Hash returns the hash value of this value.
func (p *Pattern) Hash() (uint32, error) { return starlark.String(p.pattern.value).Hash() }

// patternMethods contains methods of the pattern object.
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
var patternMembers = map[string]func(p *Pattern) starlark.Value{
	"flags":   func(p *Pattern) starlark.Value { return makeFlags(p.flags) },
	"pattern": func(p *Pattern) starlark.Value { return p.pattern.asType(p.pattern.value) },
	"groups":  func(p *Pattern) starlark.Value { return starlark.MakeInt(p.re.SubexpCount()) },
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

// Attr returns the member of the module with the given name.
// If the member exists in `patternMethods`, a bound method is returned.
// Alternatively, if the pattern member exists in `patternMembers`, the member value is returned instead.
// If the member does not exist, `nil, nil` is returned.
func (p *Pattern) Attr(name string) (starlark.Value, error) {
	if o, ok := patternMethods[name]; ok {
		return o.BindReceiver(p), nil
	}

	if o, ok := patternMembers[name]; ok {
		return o(p), nil
	}

	return nil, nil
}

// AttrNames lists available dot expression members.
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

// CompareSameType compares this pattern to another one.
// It is only possible to compare patterns for equality and inequality.
func (p *Pattern) CompareSameType(op syntax.Token, y starlark.Value, _ int) (bool, error) {
	o := y.(*Pattern)

	switch op {
	case syntax.EQL:
		ok := patternEquals(p, o)
		return ok, nil
	case syntax.NEQ:
		ok := patternEquals(p, o)
		return !ok, nil
	default:
		return false, fmt.Errorf("%s %s %s not implemented", p.Type(), op, o.Type())
	}
}

// patternEquals compares two patterns for equality.
func patternEquals(x, y *Pattern) bool {
	return x.pattern == y.pattern && x.flags == y.flags
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
	return regexSearch(p, str, pos, endpos)
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
	return regexMatch(p, str, pos, endpos)
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
	return regexFullmatch(p, str, pos, endpos)
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
	return regexSplit(p, str, maxSplit)
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
	return regexFindall(p, str, pos, endpos)
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
	return regexFinditer(p, str, pos, endpos)
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
	return regexSub(thread, b.Name(), p, repl, str, count)
}

// Match object

// Match represents a single regex match.
type Match struct {
	pattern *Pattern
	str     strOrBytes
	pos     int
	endpos  int

	groups    []group // first group represents the whole match
	lastIndex int
}

// group represents a matched group and has a start and end position.
type group struct {
	start int
	end   int
}

// empty checks, if the group is empty.
func (g *group) empty() bool {
	return g.start < 0 || g.end < 0
}

// newMatch creates a new match object.
func newMatch(p *Pattern, str strOrBytes, a []int, pos, endpos int) *Match {
	n := 1 + p.re.SubexpCount()

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

			groups = append(groups, g)

			if i > 0 && !g.empty() && e > lastIndexEnd {
				// determine the index of the last group
				lastIndex = i
				lastIndexEnd = e
			}
		}
	}

	m := Match{
		pattern: p,
		str:     str,
		pos:     pos,
		endpos:  endpos,

		groups:    groups,
		lastIndex: lastIndex,
	}

	return &m
}

// Check if the type satisfies the interfaces.
var (
	_ starlark.Value      = (*Match)(nil)
	_ starlark.HasAttrs   = (*Match)(nil)
	_ starlark.Mapping    = (*Match)(nil)
	_ starlark.Comparable = (*Match)(nil)
)

// String returns the string representation of the value.
func (m *Match) String() string {
	g := m.groups[0]
	return fmt.Sprintf("<re.Match object; span=(%d, %d), match=%s>",
		g.start, g.end, util.Repr(m.groupStr(&g), m.str.isString),
	)
}

// groupStr returns the matched string of the given group.
func (m *Match) groupStr(g *group) string {
	return m.str.value[g.start:g.end]
}

// Type returns a short string describing the value's type.
func (m *Match) Type() string { return "Match" }

// Freeze marks the value and all members as frozen.
func (m *Match) Freeze() {}

// Truth returns the truth value of the object.
func (m *Match) Truth() starlark.Bool { return true }

// Hash returns the hash value of this value.
func (m *Match) Hash() (uint32, error) {
	var tmp uint32

	h, _ := m.pattern.Hash() // string type; no error possible

	for _, g := range m.groups {
		if g.empty() {
			tmp = 0
		} else {
			tmp, _ = starlark.String(m.groupStr(&g)).Hash() // string type; no error possible
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

		return starlark.String(name)
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

// Attr returns the member of the module with the given name.
// If the member exists in `matchMethods`, a bound method is returned.
// Alternatively, if the pattern member exists in `matchMembers`, the member value is returned instead.
// If the member does not exist, `nil, nil` is returned.
func (m *Match) Attr(name string) (starlark.Value, error) {
	if o, ok := matchMethods[name]; ok {
		return o.BindReceiver(m), nil
	}

	if o, ok := matchMembers[name]; ok {
		return o(m), nil
	}

	return nil, nil
}

// AttrNames lists available dot expression members.
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
// For the match object, this is equal with calling the `group` function.
func (m *Match) Get(v starlark.Value) (starlark.Value, bool, error) {
	g, err := m.group(v)
	if err != nil {
		return nil, false, err
	}

	return g, true, nil
}

// group returns the group with the given key.
func (m *Match) group(v starlark.Value) (starlark.Value, error) {
	i, err := m.getIndex(v)
	if err != nil {
		return nil, err
	}

	g := &m.groups[i]
	if g.empty() {
		return starlark.None, nil
	}

	return m.str.asType(m.groupStr(g)), nil
}

// getIndex converts the group key into a valid integer index for one of the groups of this match.
// The key must be either of type `int` or `str`. Keys of type int are used as indices and keys
// of type str are used as group names. If the index key is out of range, does not exist or has
// an invalid type, then an index error is returned.
func (m *Match) getIndex(v starlark.Value) (int, error) {
	switch v := v.(type) {
	case starlark.Int:
		i, ok := v.Int64()
		if ok && i >= 0 && i < int64(len(m.groups)) {
			return int(i), nil
		}
	case starlark.String:
		if i := m.pattern.re.SubexpIndex(string(v)); i >= 0 {
			return i, nil
		}
	}

	return 0, errors.New("IndexError: no such group")
}

// CompareSameType compares this matches to another one.
// It is only supported, to compare matches for equality and inequality.
func (m *Match) CompareSameType(op syntax.Token, y starlark.Value, _ int) (bool, error) {
	o := y.(*Match)

	switch op {
	case syntax.EQL:
		ok := matchEquals(m, o)
		return ok, nil
	case syntax.NEQ:
		ok := matchEquals(m, o)
		return !ok, nil
	default:
		return false, fmt.Errorf("%s %s %s not implemented", m.Type(), op, o.Type())
	}
}

// matchEquals compares two matches for equality.
func matchEquals(x, y *Match) bool {
	if !patternEquals(x.pattern, y.pattern) {
		return false
	}
	if x.str != y.str {
		return false
	}
	if !slices.Equal(x.groups, y.groups) {
		return false
	}

	return x.pos == y.pos && x.endpos == y.endpos && x.lastIndex == y.lastIndex
}

// matchExpand returns the string obtained by doing backslash substitution on
// the template string template, as done by the sub() method.
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

	var w strings.Builder

	// Replace the template
	err = r.replace(&w, m)
	if err != nil {
		return nil, err
	}

	return m.str.asType(w.String()), nil
}

// matchGroup returns one or more subgroups of the match. If there is a single argument, the result is a
// single string; if there are multiple arguments, the result is a tuple with one item per argument.
// Without arguments, group1 defaults to zero (the whole match is returned).
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

// matchGroups returns a tuple containing all the subgroups of the match.
func matchGroups(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var defaultValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "default?", &defaultValue); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	result := make(starlark.Tuple, 0, len(m.groups)-1)

	for _, g := range m.groups[1:] {
		gv := defaultValue
		if !g.empty() {
			gv = m.str.asType(m.groupStr(&g))
		}

		result = append(result, gv)
	}

	return result, nil
}

// matchGroupDict returns a dictionary containing all the named subgroups of the match,
// keyed by the subgroup name. The default argument is used for groups that did not
// participate in the match; it defaults to None.
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

// matchStart returns the indices of the start of the substring matched by group.
// The group defaults to zero; -1 is returned, if the group does not exist.
func matchStart(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = zeroInt
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "group?", &group); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	i, err := m.getIndex(group)
	if err != nil {
		return nil, err
	}

	g := m.groups[i]
	if g.empty() {
		return starlark.MakeInt(-1), nil
	}

	return starlark.MakeInt(m.groups[i].start), nil
}

// matchEnd returns the indices of the end of the substring matched by group.
// The group defaults to zero; -1 is returned, if the group does not exist.
func matchEnd(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = zeroInt
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "group?", &group); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	i, err := m.getIndex(group)
	if err != nil {
		return nil, err
	}

	g := m.groups[i]
	if g.empty() {
		return starlark.MakeInt(-1), nil
	}

	return starlark.MakeInt(m.groups[i].end), nil
}

// matchSpan returns the 2-tuple `(m.start(group), m.end(group))` for a match `m`.
func matchSpan(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var group starlark.Value = zeroInt
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "group?", &group); err != nil {
		return nil, err
	}

	m := b.Receiver().(*Match)

	i, err := m.getIndex(group)
	if err != nil {
		return nil, err
	}

	g := m.groups[i]
	if g.empty() {
		v := starlark.MakeInt(-1)
		return starlark.Tuple{v, v}, nil
	}

	s := starlark.MakeInt(m.groups[i].start)
	e := starlark.MakeInt(m.groups[i].end)
	return starlark.Tuple{s, e}, nil
}

// matchIter is a type that allows the `finditer` functions to return an iterator instead of a list.
// It has no advantage other than of matching the Python standard.
type matchIter struct {
	values starlark.Tuple
}

// Check if the types satisfy the interface.
var (
	_ starlark.Value    = (*matchIter)(nil)
	_ starlark.Iterable = (*matchIter)(nil)
)

// String returns the string representation of the value.
func (it *matchIter) String() string {
	return fmt.Sprintf("<%s object at %p>", it.Type(), it)
}

// Type returns a short string describing the value's type.
func (it *matchIter) Type() string { return "match_iterator" }

// Freeze marks the value and all members as frozen.
func (it *matchIter) Freeze() {}

// Truth returns the truth value of the object.
func (it *matchIter) Truth() starlark.Bool { return true }

// Hash returns an error, because this value is not hashable.
func (it *matchIter) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: %s", it.Type()) }

// Iterate returns an iterator of matches.
func (it *matchIter) Iterate() starlark.Iterator {
	return it.values.Iterate()
}

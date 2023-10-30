package regex

import "math"

// Generate string representations of constants.
// To install stringer: go install golang.org/x/tools/cmd/stringer@latest
//go:generate stringer -type=opcode,atcode,catcode -linecomment -output=constants_string.go

// Possible flags for the flag parameter.
// See also https://docs.python.org/3/library/re.html#flags.
// Note, that the additional flag `FlagFallback` is specific to this Starlark implementation.
const (
	_              uint32 = 1 << iota // TEMPLATE; unused
	FlagIgnoreCase                    // i
	FlagLocale                        // L
	FlagMultiline                     // m
	FlagDotAll                        // s
	FlagUnicode                       // u
	FlagVerbose                       // x
	FlagDebug                         // -
	FlagASCII                         // a
	FlagFallback                      // -

	typeFlags      = FlagASCII | FlagLocale | FlagUnicode        // exclude flags in subpatterns
	globalFlags    = FlagDebug                                   // flags, that may only appear on global flags
	supportedFlags = FlagIgnoreCase | FlagMultiline | FlagDotAll // flags supported by the Go regex library
)

const (
	// maxRepeat is the maximum repeat count for the parser; the regex engines may have a lower maximum repeat value.
	maxRepeat = math.MaxInt32

	// maxRepeatEngine is the maximum repeat count for the default regex engine.
	maxRepeatEngine = 1000

	// assert, that the maximum repeat count for the parser is higher than the maximum repeat count of the engines
	_ uint = maxRepeat - maxRepeatEngine

	// maxGroups is used as the maximum number of groups for the parser; the regex engines may have a lower maximum number of groups.
	maxGroups = math.MaxInt32 / 2
)

// opcode is the type used for regex operators.
type opcode uint32

// Parsable regex operators.
// Note that not all of these operators are supported by both regex engines.
// The following operators exist (ordered by opcode value):
//
//   - FAILURE: empty negative lookups; `(?!)`
//   - ANY: matches any character; `.`
//   - ASSERT: positive lookahead or lookbehind; `(?=...)` or `(?<=...)`
//   - ASSERT_NOT: negative lookahead or lookbehind; `(?!...)` or `(?<!...)`
//   - AT: positional matches; `^`, `\A`, `\b`, `\B`, `$`, `\Z`
//   - BRANCH: list of subpatterns separated by `|`
//   - CATEGORY: character class;  `\d`, `\D`, `\s`, `\S`, `\w`, `\W`
//   - GROUPREF: backreference to another subgroup; `\1`
//   - GROUPREF_EXISTS: conditional expression; `(?(...)...|...)`
//   - IN: set of characters; `[...]` or an character class
//   - LITERAL: literal
//   - MIN_REPEAT: non-greedy match; `??`, `*?`, `+?`, `{...}?`
//   - MAX_REPEAT: greedy match; `?`, `*`, `+`, `{...}`
//   - NEGATE: negate character; `^`
//   - NOT_LITERAL: single negated literal; `[^...]`
//   - RANGE: character range; `...-...`
//   - SUBPATTERN:
//     subpattern, either as capture group or non-capturing with optional flags;
//     `(?P<...>...)`, `(?...:...)`
//   - ATOMIC_GROUP: possessive match; `(?>...)`
//   - POSSESSIVE_REPEAT: possessive repeat; `?+`, `*+`, `++`, `{...}+`
const (
	opFailure          opcode = iota // FAILURE
	opAny                            // ANY
	opAssert                         // ASSERT
	opAssertNot                      // ASSERT_NOT
	opAt                             // AT
	opBranch                         // BRANCH
	opCategory                       // CATEGORY
	opGroupref                       // GROUPREF
	opGrouprefExists                 // GROUPREF_EXISTS
	opIn                             // IN
	opLiteral                        // LITERAL
	opMinRepeat                      // MIN_REPEAT
	opMaxRepeat                      // MAX_REPEAT
	opNegate                         // NEGATE
	opNotLiteral                     // NOT_LITERAL
	opRange                          // RANGE
	opSubpattern                     // SUBPATTERN
	opAtomicGroup                    // ATOMIC_GROUP
	opPossessiveRepeat               // POSSESSIVE_REPEAT
)

// atcode is the type used to specify positions.
type atcode uint32

// Available regex position.
const (
	atBeginning       atcode = iota // AT_BEGINNING
	atBeginningString               // AT_BEGINNING_STRING
	atBoundary                      // AT_BOUNDARY
	atNonBoundary                   // AT_NON_BOUNDARY
	atEnd                           // AT_END
	atEndString                     // AT_END_STRING
)

// catcode is the type used to specify categories.
type catcode uint32

// Available regex category.
const (
	categoryDigit    catcode = iota // CATEGORY_DIGIT
	categoryNotDigit                // CATEGORY_NOT_DIGIT
	categorySpace                   // CATEGORY_SPACE
	categoryNotSpace                // CATEGORY_NOT_SPACE
	categoryWord                    // CATEGORY_WORD
	categoryNotWord                 // CATEGORY_NOT_WORD
)

package regex

import "math"

// Generate string representations of constants.
// To install stringer: go install golang.org/x/tools/cmd/stringer@latest
//go:generate stringer -type=opcode,atcode,catcode -linecomment -output=constants_string.go

// Possible flags for the flag parameter.
// See also https://docs.python.org/3/library/re.html#flags.
// Note, that the additional flag `FlagFallback` is specific for this Starlark implementation.
const (
	_ uint32 = 1 << iota // TEMPLATE; unused
	FlagIgnoreCase
	FlagLocale
	FlagMultiline
	FlagDotAll
	FlagUnicode
	FlagVerbose
	FlagDebug
	FlagASCII
	FlagFallback

	// Flags supported by the Go regex library.
	supportedFlags = FlagIgnoreCase | FlagMultiline | FlagDotAll
)

const (
	// maxRepeat is the maximum repeat count for the parser; the regex engines may have an lower maximum repeat value.
	maxRepeat = math.MaxInt

	// maxGroups is used as the maximum group count of the parser; the regex engines may have an lower maximum group count.
	maxGroups = math.MaxInt / 2
)

// opcode is the type used for regex operators.
type opcode uint32

// Parsable regex operator.
// Note, that not all of these operators are supported by both regex engines.
// The following operators exists (ordered by opcode value):
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
//   - POSSESSIVE_REPEAT: possesive repeat; `?+`, `*+`, `++`, `{...}+`
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

// atcode is the type to specify positions.
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

// catcode is the type to specify categories.
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

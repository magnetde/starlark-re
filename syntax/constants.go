package syntax

import "math"

// Possible flags for the flag parameter.
const (
	_ = 1 << iota // `re.TEMPLATE`; unused
	FlagIgnoreCase
	FlagLocale
	FlagMultiline
	FlagDotAll
	FlagUnicode
	FlagVerbose
	FlagDebug
	FlagASCII
)

const (
	MAXREPEAT = math.MaxInt
	MAXGROUPS = math.MaxInt / 2
)

// To install stringer: go install golang.org/x/tools/cmd/stringer@latest
//go:generate stringer -type=opcode,atCode,chCode -output=constants_string.go

type opcode uint32

const (
	// Skip zero opcode
	FAILURE opcode = iota

	ANY
	ASSERT
	ASSERT_NOT
	AT
	BRANCH
	CATEGORY
	GROUPREF
	GROUPREF_EXISTS
	IN
	LITERAL
	MIN_REPEAT
	MAX_REPEAT
	NOT_LITERAL
	NEGATE
	RANGE
	SUBPATTERN
	ATOMIC_GROUP
	POSSESSIVE_REPEAT
)

type atCode uint32

const (
	AT_BEGINNING atCode = iota
	AT_BEGINNING_STRING
	AT_BOUNDARY
	AT_NON_BOUNDARY
	AT_END
	AT_END_STRING
)

type chCode uint32

const (
	CATEGORY_DIGIT chCode = iota
	CATEGORY_NOT_DIGIT
	CATEGORY_SPACE
	CATEGORY_NOT_SPACE
	CATEGORY_WORD
	CATEGORY_NOT_WORD
)

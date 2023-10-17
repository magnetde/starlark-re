package syntax

import "math"

// To install stringer: go install golang.org/x/tools/cmd/stringer@latest
//
//go:generate stringer -type=opcode,atcode,catcode -linecomment -output=constants_string.go

// Possible flags for the flag parameter.
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
)

const (
	maxRepeat = math.MaxInt
	maxGroups = math.MaxInt / 2
)

type opcode uint32

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
	opNotLiteral                     // NOT_LITERAL
	opNegate                         // NEGATE
	opRange                          // RANGE
	opSubpattern                     // SUBPATTERN
	opAtomicGroup                    // ATOMIC_GROUP
	opPossessiveRepeat               // POSSESSIVE_REPEAT
)

type atcode uint32

const (
	atBeginning       atcode = iota // AT_BEGINNING
	atBeginningString               // AT_BEGINNING_STRING
	atBoundary                      // AT_BOUNDARY
	atNonBoundary                   // AT_NON_BOUNDARY
	atEnd                           // AT_END
	atEndString                     // AT_END_STRING
)

type catcode uint32

const (
	categoryDigit    catcode = iota // CATEGORY_DIGIT
	categoryNotDigit                // CATEGORY_NOT_DIGIT
	categorySpace                   // CATEGORY_SPACE
	categoryNotSpace                // CATEGORY_NOT_SPACE
	categoryWord                    // CATEGORY_WORD
	categoryNotWord                 // CATEGORY_NOT_WORD
)

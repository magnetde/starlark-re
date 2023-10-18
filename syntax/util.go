package syntax

import (
	"fmt"
	"slices"
	"strings"

	"github.com/magnetde/starlark-re/util"
)

// isASCIILetter checks if a given character is an ASCII letter.
func isASCIILetter(b rune) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

// isASCIILetterByte checks if a given byte is an ASCII letter.
func isASCIILetterByte(b byte) bool {
	return isASCIILetter(rune(b))
}

// isOctDigit checks if the given character is a decimal digit.
func isDigit(c rune) bool {
	return '0' <= c && c <= '9'
}

// isOctDigit checks if the given byte is a decimal digit.
func isDigitByte(c byte) bool {
	return isDigit(rune(c))
}

// isOctDigit checks if the given character is an octal digit.
func isOctDigit(b rune) bool {
	return '0' <= b && b <= '7'
}

// isOctDigit checks if the given byte is an octal digit.
func isOctDigitByte(b byte) bool {
	return isOctDigit(rune(b))
}

// toDigit returns the corresponding integer value of a character.
// The rune must be a digit character in the set "0123456789".
func toDigit(b rune) int {
	return int(b) - '0'
}

// digit returns the corresponding integer value of a byte representing a character.
// The byte must be a digit character in the set "0123456789".
func toDigitByte(b byte) int {
	return toDigit(rune(b))
}

// isWhitespace checks if a given rune is a whitespace character.
func isWhitespace(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

// lookupUnicodeName converts the unicode name to a corresponding character.
// If no character was found for the name, the second return value is false.
func lookupUnicodeName(name string) (rune, bool) {
	name = strings.ToUpper(name)

	i, ok := slices.BinarySearch(unicodeNames[:], name)
	if !ok {
		return 0, false
	}

	return unicodeCodepoints[i], true
}

// checkGroupName checks if a group name is valid.
// It ensures that group names in string patterns are valid unicode identifiers,
// and that group names in byte patterns are only made from ASCII characters.
func checkGroupName(name string, isStr bool) error {
	if !(isStr || util.IsASCIIString(name)) {
		return fmt.Errorf("bad character in group name %s", util.QuoteString(name, isStr, false))
	}
	if !isIdentifier(name) {
		return fmt.Errorf("bad character in group name %s", util.QuoteString(name, isStr, true))
	}
	return nil
}

// isIdentifier checks, whether name is a valid unicode identifier.
// See also: https://www.unicode.org/reports/tr31/
func isIdentifier(name string) bool {
	if name == "" {
		return false
	}

	for i, c := range name {
		if i == 0 {
			if !inTable(c, xidStartTable[:]) {
				return false
			}
		} else if !inTable(c, xidContinueTable[:]) {
			return false
		}
	}

	return true
}

// inTable checks whether the character c is in the table of ranges.
// The character c is considered to be in the table, if there exists a range in the table that includes c.
// The table must be sorted and should be a slice of either "xidStartTable" or "xidContinueTable".
func inTable(c rune, table []tableRange) bool {
	_, ok := slices.BinarySearchFunc(table, c, func(r tableRange, v rune) int {
		if r.lo > c {
			return +1
		} else if r.hi < c {
			return -1
		} else {
			return 0
		}
	})

	return ok
}

// isFlag checks if a given character is a valid regex flag.
// The characters 'a', 'i', 'L', 'm', 's', 'u' and 'x' represent valid regex flags.
func isFlag(c rune) bool {
	switch c {
	case 'i', 'L', 'm', 's', 'x', 'a', 'u':
		return true
	default:
		return false
	}
}

// getFlag converts a flag character to the corresponding integer flag value.
// If the character is not a valid flag character, it returns 0.
func getFlag(c rune) uint32 {
	switch c {
	// standard flags
	case 'i':
		return FlagIgnoreCase
	case 'L':
		return FlagLocale
	case 'm':
		return FlagMultiline
	case 's':
		return FlagDotAll
	case 'x':
		return FlagVerbose
	// extensions
	case 'a':
		return FlagASCII
	case 'u':
		return FlagUnicode
	default:
		return 0
	}
}

// isRepeatCode checks if the opcode represents a repeat operator.
// Valid repeat operators are "MIN_REPEAT", "MAX_REPEAT" or "POSSESSIVE_REPEAT".
func isRepeatCode(o opcode) bool {
	switch o {
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		return true
	default:
		return false
	}
}

package regex

import (
	"slices"
	"strings"
	"unicode"
)

// isASCIIString checks, if the string only contains ASCII characters.
func isASCIIString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

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

// toDigit returns the corresponding integer value of a character.
// The character must be a digit in the set "0123456789".
func toDigit(b rune) int {
	return int(b) - '0'
}

// digit returns the corresponding integer value of a byte representing a character.
// The byte must be a digit in the set "0123456789".
func toDigitByte(b byte) int {
	return toDigit(rune(b))
}

// isWhitespace checks if a given character is a whitespace character.
func isWhitespace(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

// lookupUnicodeName converts the unicode name into a corresponding character.
// If the name does not match any character, the second return value is false.
func lookupUnicodeName(name string) (rune, bool) {
	name = strings.ToUpper(name)

	i, ok := slices.BinarySearch(unicodeNames[:], name)
	if !ok {
		return 0, false
	}

	return unicodeCodepoints[i], true
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

// inTable verifies whether the character c is included in the table of ranges.
// If c is covered by any range in the table, it is considered to be in the table.
// The table needs to be sorted and should be a slice of either "xidStartTable" or "xidContinueTable".
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

// isFlag determines whether a character is a valid regex flag.
// The characters 'a', 'i', 'L', 'm', 's', 'u' and 'x' are considered as valid regex flags.
func isFlag(c rune) bool {
	switch c {
	case 'i', 'L', 'm', 's', 'x', 'a', 'u':
		return true
	default:
		return false
	}
}

// getFlag converts a flag character to its corresponding integer value.
// If the character is invalid for a flag, the function will return 0.
func getFlag(c rune) uint32 {
	switch c {
	// default flags
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
	default: // should never happen
		return 0
	}
}

// isRepeatCode checks if the opcode represents a repetition operator.
// Valid repeating operators are "MIN_REPEAT", "MAX_REPEAT" or "POSSESSIVE_REPEAT".
func isRepeatCode(o opcode) bool {
	switch o {
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		return true
	default:
		return false
	}
}

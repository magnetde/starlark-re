package syntax

import (
	"fmt"
	"slices"
	"strings"

	"github.com/magnetde/starlark-re/util"
)

func isASCIILetter(b rune) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isASCIILetterByte(b byte) bool {
	return isASCIILetter(rune(b))
}

func isOctDigit(b rune) bool {
	return '0' <= b && b <= '7'
}

func isOctDigitByte(b byte) bool {
	return isOctDigit(rune(b))
}

func isDigit(c rune) bool {
	return '0' <= c && c <= '9'
}

func isDigitByte(c byte) bool {
	return isDigit(rune(c))
}

// precondition: b must be in set "0123456789"
func digit(b rune) int {
	return int(b) - '0'
}

// precondition: b must be in set "0123456789"
func digitByte(b byte) int {
	return int(b) - '0'
}

func isWhitespace(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func lookupUnicodeName(name string) (rune, bool) {
	name = strings.ToUpper(name)

	i, ok := slices.BinarySearch(unicodeNames[:], name)
	if !ok {
		return 0, false
	}

	return unicodeCodepoints[i], true
}

func isFlag(c rune) bool {
	switch c {
	case 'i', 'L', 'm', 's', 'x', 'a', 'u':
		return true
	default:
		return false
	}
}

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

func isRepeatCode(o opcode) bool {
	switch o {
	case MIN_REPEAT, MAX_REPEAT, POSSESSIVE_REPEAT:
		return true
	default:
		return false
	}
}

func checkgroupname(name string, isStr bool) error {
	if !(isStr || util.IsASCIIString(name)) {
		return fmt.Errorf("bad character in group name %s", util.QuoteString(name, isStr, false))
	}
	if !isIdentifier(name) {
		return fmt.Errorf("bad character in group name %s", util.QuoteString(name, isStr, true))
	}
	return nil
}

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

func unescapeLetter(c byte) (string, bool) {
	var value string

	switch c {
	case 'a':
		value = "\a"
	case 'b':
		value = "\b"
	case 'f':
		value = "\f"
	case 'n':
		value = "\n"
	case 'r':
		value = "\r"
	case 't':
		value = "\t"
	case 'v':
		value = "\v"
	case '\\':
		value = "\\"
	}

	if value != "" {
		return value, true
	}

	return "", false
}

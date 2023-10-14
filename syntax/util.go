package syntax

import (
	"slices"
	"strings"

	"github.com/magnetde/starlark-re/util"
)

func isDigitString(s string) bool {
	for i := 0; i < len(s); i++ {
		if !util.IsDigit(s[i]) {
			return false
		}
	}

	return true
}

func isASCIILetter(b rune) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isOctDigit(b rune) bool {
	return '0' <= b && b <= '7'
}

func isDigit(c rune) bool {
	return '0' <= c && c <= '9'
}

// precondition: b must be in set "0123456789"
func digit(b rune) int {
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

func getFlag(c rune) int {
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

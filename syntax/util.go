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

// precondition: b must be in set "0123456789"
func digit(b rune) int {
	return int(b) - '0'
}

func lookupUnicodeName(name string) (rune, bool) {
	name = strings.ToUpper(name)

	i, ok := slices.BinarySearch(unicodeNames[:], name)
	if !ok {
		return 0, false
	}

	return unicodeCodepoints[i], true
}

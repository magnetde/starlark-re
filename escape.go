package re

import "unicode/utf8"

// specialBytes contains 16 * 8 = 128 bits, where each bit represents one byte value.
// If the i-th it is 1, the i-th byte character represents a special character, that
// needs to be escaped.
// This array represents the following bytes: "()[]{}?*+-|^$\\.&~# \t\n\r\v\f".
var specialBytes = [16]byte{
	0x04, 0x00, 0x00, 0x04, 0x04, 0x00, 0x04, 0x00,
	0x04, 0x05, 0x05, 0xa5, 0xa1, 0xa5, 0xa4, 0x08,
}

// special reports whether byte b needs to be escaped by escapePattern.
func special(b byte) bool {
	return b < utf8.RuneSelf && specialBytes[b%16]&(1<<(b/16)) != 0
}

// escapePattern returns a string that escapes all regular expression metacharacters
// inside the argument text; the returned string is a regular expression matching
// the literal text.
// This function works exactly like `regexp.QuoteMeta` but escapes the characters
// of string "()[]{}?*+-|^$\\.&~# \t\n\r\v\f".
func escapePattern(s string) string {
	// A byte loop is correct because all metacharacters are ASCII.
	var i int
	for i = 0; i < len(s); i++ {
		if special(s[i]) {
			break
		}
	}

	// No meta characters found, so return original string.
	if i >= len(s) {
		return s
	}

	b := make([]byte, 2*len(s)-i)
	copy(b, s[:i])
	j := i
	for ; i < len(s); i++ {
		if special(s[i]) {
			b[j] = '\\'
			j++
		}
		b[j] = s[i]
		j++
	}

	return string(b[:j])
}

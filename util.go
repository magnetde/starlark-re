package re

import "unicode/utf8"

func isOctDigit(b byte) bool {
	return '0' <= b && b <= '7'
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

var specialBytes = [16]byte{
	0x04, 0x00, 0x00, 0x04, 0x04, 0x00, 0x04, 0x00,
	0x04, 0x05, 0x05, 0xa5, 0xa1, 0xa5, 0xa4, 0x08,
}

// special reports whether byte b needs to be escaped by QuoteMeta.
func special(b byte) bool {
	return b < utf8.RuneSelf && specialBytes[b%16]&(1<<(b/16)) != 0
}

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

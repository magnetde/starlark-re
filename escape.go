package re

import "unicode/utf8"

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

package re

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	quoteReplacer = strings.NewReplacer(`'`, `\'`, `\"`, `"`)
	hexChars      = "0123456789abcdef"
)

func quoteString(s string, isString bool, bprefix bool) string {
	var b strings.Builder
	b.Grow(len(s) + 3)

	var quote byte
	if strings.IndexByte(s, '\'') < 0 || strings.IndexByte(s, '"') >= 0 {
		quote = '\''
	} else {
		quote = '"'
	}

	if !isString && bprefix {
		b.WriteByte('b')
	}

	if isString {
		s = strconv.Quote(s)

		if quote == '\'' {
			b.WriteByte('\'')
			quoteReplacer.WriteString(&b, s[1:len(s)-1])
			b.WriteByte('\'')
		} else {
			b.WriteString(s)
		}
	} else {
		b.WriteByte(quote)
		for i := 0; i < len(s); i++ {
			writeEscapedByte(s[i], quote, &b)
		}
		b.WriteByte(quote)
	}

	return b.String()
}

func writeEscapedByte(c byte, quote byte, b *strings.Builder) {
	if c == quote || c == '\\' { // always backslashed
		b.WriteByte('\\')
		b.WriteByte(c)
		return
	}

	switch c {
	case '\a':
		b.WriteString(`\a`)
	case '\b':
		b.WriteString(`\b`)
	case '\f':
		b.WriteString(`\f`)
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	case '\v':
		b.WriteString(`\v`)
	default:
		if c < utf8.RuneSelf && unicode.IsPrint(rune(c)) {
			b.WriteByte(c)
		} else {
			b.WriteString(`\x`)
			b.WriteByte(hexChars[c>>4])
			b.WriteByte(hexChars[c&0xF])
		}
	}
}

func isIdentifier(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		if c > unicode.MaxASCII {
			return false
		}

		if !isASCIILetter(c) && c != '_' && (i == 0 || !isDigit(c)) {
			return false
		}
	}

	return true
}

func isASCIILetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isASCIILetterC(b rune) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func isOctDigit(b byte) bool {
	return '0' <= b && b <= '7'
}

func isOctDigitC(b rune) bool {
	return '0' <= b && b <= '7'
}

// precondition: b must be in set "0123456789"
func digit(b byte) int {
	return int(b) - '0'
}

// precondition: b must be in set "0123456789"
func digitC(b rune) int {
	return int(b) - '0'
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

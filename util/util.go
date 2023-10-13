package util

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

func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func IsIdentifier(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		if c > unicode.MaxASCII {
			return false
		}

		if !IsASCIILetter(c) && c != '_' && (i == 0 || !IsDigit(c)) {
			return false
		}
	}

	return true
}

func IsDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

// precondition: b must be in set "0123456789"
func Digit(b byte) int {
	return int(b) - '0'
}

func IsASCIILetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func QuoteString(s string, isString bool, bprefix bool) string {
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
			_, _ = quoteReplacer.WriteString(&b, s[1:len(s)-1])
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

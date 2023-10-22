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

// Repr returns a representation of an string.
// The `isString` parameter determines whether the string should be treated as a string or a bytes object.
func Repr(s string, isString bool) string {
	return quoteString(s, isString, true, false)
}

// ASCII returns a string that escapes all non-printable or non-ASCII characters.
// The `isString` parameter determines whether the string should be treated as a string or a bytes object.
func ASCII(s string, isString bool) string {
	return quoteString(s, isString, false, true)
}

// quoteString quotes a string and escapes non-printable characters.
// The `isString` parameter determines whether the string should be interpreted as a string or a bytes object.
// If `bPrefix` is set to true, an "b" prefix will be prepended to the string.
// If `ascii` is true, all non-ASCII characters are escaped.
func quoteString(s string, isString bool, bPrefix bool, ascii bool) string {
	var b strings.Builder
	b.Grow(len(s) + 3)

	var quote byte
	if strings.IndexByte(s, '\'') < 0 || strings.IndexByte(s, '"') >= 0 {
		quote = '\''
	} else {
		quote = '"'
	}

	if !isString && bPrefix {
		b.WriteByte('b')
	}

	if isString {
		if !ascii {
			s = strconv.Quote(s)
		} else {
			s = strconv.QuoteToASCII(s)
		}

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
			c := s[i]
			WriteEscapedByte(&b, c, c == quote || c == '\\')
		}
		b.WriteByte(quote)
	}

	return b.String()
}

// WriteEscapedByte escapes a byte and writes it to the string builder,
// escaping all special and non-ASCII characters by default.
// Additionally, if `force` is true, the function will always escape the character 'c'.
func WriteEscapedByte(w *strings.Builder, c byte, force bool) {
	if force {
		w.WriteByte('\\')
		w.WriteByte(c)
		return
	}

	switch c {
	case '\a':
		w.WriteString(`\a`)
	case '\b':
		w.WriteString(`\b`)
	case '\f':
		w.WriteString(`\f`)
	case '\n':
		w.WriteString(`\n`)
	case '\r':
		w.WriteString(`\r`)
	case '\t':
		w.WriteString(`\t`)
	case '\v':
		w.WriteString(`\v`)
	default:
		if c < utf8.RuneSelf && unicode.IsPrint(rune(c)) {
			w.WriteByte(c)
		} else {
			w.WriteString(`\x`)
			w.WriteByte(hexChars[c>>4])
			w.WriteByte(hexChars[c&0xF])
		}
	}
}

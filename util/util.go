package util

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var hexDigits = "0123456789abcdef"

// Repr returns a representation of an string.
// The `isString` parameter determines whether the string should be treated as a string or a bytes object.
func Repr(s string, isString bool) string {
	return stringRepr(s, isString, true)
}

// ASCII returns a string that escapes all non-printable or non-ASCII characters.
// The `isString` parameter determines whether the string should be treated as a string or a bytes object.
func ASCII(s string, isString bool) string {
	return ASCIIReplace(stringRepr(s, isString, false))
}

// stringRepr returns a string representation of the string.
// The `isString` parameter determines whether the string should be interpreted as a string or a bytes object.
// If `bPrefix` is set to true, an "b" prefix will be prepended to the string.
func stringRepr(s string, isString bool, bPrefix bool) string {
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

	b.WriteByte(quote)

	var ch rune
	for size := 0; len(s) > 0; s = s[size:] {
		if isString {
			ch, size = utf8.DecodeRuneInString(s)
		} else {
			ch = rune(s[0])
			size = 1
		}

		// Handle utf8 errors
		if ch == utf8.RuneError {
			b.WriteString(`\x`)
			b.WriteByte(hexDigits[(s[0]>>4)&0x000F])
			b.WriteByte(hexDigits[s[0]&0x000F])

			size = 1
			continue
		}

		// Escape quotes and backslashes
		if ch == rune(quote) || ch == '\\' {
			b.WriteByte('\\')
			b.WriteByte(byte(ch))
			continue
		}

		// Map special whitespace to '\t', \n', '\r'
		if ch == '\t' {
			b.WriteByte('\\')
			b.WriteByte('t')
		} else if ch == '\n' {
			b.WriteByte('\\')
			b.WriteByte('n')
		} else if ch == '\r' {
			b.WriteByte('\\')
			b.WriteByte('r')
		} else if ch < ' ' || ch == unicode.MaxASCII { // Map non-printable US ASCII to '\xhh' */
			b.WriteString(`\x`)
			b.WriteByte(hexDigits[(ch>>4)&0x000F])
			b.WriteByte(hexDigits[ch&0x000F])
		} else if !unicode.IsPrint(ch) { // Escpae non-printable characters
			hexEscape(&b, ch)
		} else { // Copy characters as-is
			b.WriteRune(ch)
		}
	}

	b.WriteByte(quote)

	return b.String()
}

// hexEscape escapes the character to a hex sequence and writes it to the string builder.
func hexEscape(w *strings.Builder, ch rune) {
	w.WriteByte('\\')
	if ch <= 0xff { // Map 8-bit characters to '\xhh'
		w.WriteByte('x')
		w.WriteByte(hexDigits[(ch>>4)&0x000F])
		w.WriteByte(hexDigits[ch&0x000F])
	} else if ch <= 0xffff { // Map 16-bit characters to '\uxxxx'
		w.WriteByte('u')
		w.WriteByte(hexDigits[(ch>>12)&0xF])
		w.WriteByte(hexDigits[(ch>>8)&0xF])
		w.WriteByte(hexDigits[(ch>>4)&0xF])
		w.WriteByte(hexDigits[ch&0xF])
	} else { // Map 21-bit characters to '\U00xxxxxx'
		w.WriteByte('U')
		w.WriteByte(hexDigits[(ch>>28)&0xF])
		w.WriteByte(hexDigits[(ch>>24)&0xF])
		w.WriteByte(hexDigits[(ch>>20)&0xF])
		w.WriteByte(hexDigits[(ch>>16)&0xF])
		w.WriteByte(hexDigits[(ch>>12)&0xF])
		w.WriteByte(hexDigits[(ch>>8)&0xF])
		w.WriteByte(hexDigits[(ch>>4)&0xF])
		w.WriteByte(hexDigits[ch&0xF])
	}
}

// ASCIIReplace replaces all non-printable characters in a string with their respective
// escape sequence and replaces non-ascii bytes with an hexadecimal escape sequence.
func ASCIIReplace(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	var ch rune

	for size := 0; len(s) > 0; s = s[size:] {
		ch, size = utf8.DecodeRuneInString(s)
		if ch == utf8.RuneError {
			ch = rune(s[0])
			size = 1
		}

		if ch >= unicode.MaxASCII || !unicode.IsPrint(ch) {
			hexEscape(&b, ch)
		} else {
			b.WriteRune(ch)
		}
	}

	return b.String()
}

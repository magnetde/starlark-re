package re

import (
	"errors"
	"fmt"
	"regexp/syntax"
	"slices"
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
		if c < utf8.RuneSelf {
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

	for i, c := range name {
		if c > unicode.MaxASCII {
			return false
		}

		b := byte(c)
		if !isASCIILetter(b) && b != '_' && (i == 0 || !isDigit(b)) {
			return false
		}
	}

	return true
}

func isASCIILetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func isOctDigit(b byte) bool {
	return '0' <= b && b <= '7'
}

func unescape(c byte) (string, bool) {
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

// parses the regex flags (e.g. (?i))
// Very simple function, should only be called on valid regex.
// TODO: should be expanded, to support more complex flags, e.g. (?im).
func parseFlags(t string) int {
	flags := 0

	for len(t) >= 4 {
		if t[0] == '(' && t[1] == '?' && t[3] == ')' {
			switch t[2] {
			case 'i':
				flags |= reFlagIgnoreCase
			case 'm':
				flags |= reFlagMultiline
			case 's':
				flags |= reFlagDotAll
			}

			t = t[4:]
			continue
		}

		t = t[1:]
	}

	return flags
}

// unescapes the escapes \x, \u, \U, \N and octal escapes.
// \u, \U, \N are only allowed for strings.
// If not ASCII: replace e.g. \d with its unicode counterpart \p{Nd}.
// Also: {,n} is replaced with {0,n}
func replacePatterns(s string, isString, isASCII bool) (string, error) {
	var b strings.Builder

	isClass := false

	for len(s) > 0 {
		before, char, rest, ok := cutAny(s, `\[]{`) // search any of \, [ or ] d
		if !ok {
			break
		}

		b.WriteString(before)

		s = rest

		switch char {
		case '\\':
			parsed, rest, err := parseEscape(s, isString, isASCII, isClass)
			if err != nil {
				return "", err
			}

			b.WriteString(parsed)

			s = rest
		case '{':
			if isClass {
				// { is a literal
				b.WriteByte('{')
				break
			}

			repeat, rest, ok := strings.Cut(s, "}")
			if !ok {
				// { is a literal
				b.WriteByte('{')
				break
			}

			parsed, ok := parseRepeat(repeat)
			if !ok {
				// failed to parse the repeat content; let the regex parser do the rest
				b.WriteByte('{')
				break
			}

			b.WriteString(parsed)

			s = rest
		default: // '{' or '}'
			// Simple way of checking, if the current position is inside of an character set.
			// This does only work, because nested character sets are not allowed.
			isClass = char == '['
			b.WriteByte(char)
		}
	}

	b.WriteString(s)

	return b.String(), nil
}

func cutAny(s, chars string) (before string, char byte, after string, found bool) {
	if i := strings.IndexAny(s, chars); i >= 0 {
		return s[:i], s[i], s[i+1:], true
	}
	return s, 0, "", false
}

// parseEscape parses the part, that succeeds the backslash.
// Because the backslash char is removed, some cases needs to return it.
// The escape sequences are converted to a format, that is compatible to Go regex.
func parseEscape(s string, isStr, ascii, isCls bool) (string, string, error) {
	if s == "" {
		return `\`, s, nil
	}

	c := s[0]
	s = s[1:]

	switch c {
	case 'x':
		// x: hexadecimal escape

		e := nextHex(s, 2)
		if len(e) != 2 {
			return "", "", fmt.Errorf(`incomplete escape \%c%s`, c, e)
		}

		return `\x` + e, s[2:], nil
	case 'u', 'U':
		// u: unicode escape (exactly four digits)
		// U: unicode escape (exactly eight digits)

		if !isStr { // u and U escapes only allowed for strings
			return "", "", fmt.Errorf(`bad escape \%c`, c)
		}

		var size int
		if c == 'u' {
			size = 4
		} else {
			size = 8
		}

		e := nextHex(s, size)
		if len(e) != size {
			return "", "", fmt.Errorf(`incomplete escape \%c%s`, c, e)
		}

		r := parseIntRune(e, 16)
		if c == 'U' && utf8.RuneLen(r) < 0 {
			return "", "", fmt.Errorf(`bad escape \%c%s`, c, e)
		}

		return escapeRune(r, true), s[size:], nil
	case 'N':
		// named unicode escape e.g. \N{EM DASH}

		if !isStr {
			return "", "", errors.New(`bad escape \N`)
		}

		if s == "" || s[0] != '{' {
			return "", "", errors.New("missing {")
		}

		name, rest, ok := strings.Cut(s[1:], "}")
		if name == "" {
			return "", "", errors.New("missing character name")
		}
		if !ok {
			return "", "", errors.New("missing }, unterminated name")
		}

		r, ok := lookupUnicodeName(name)
		if !ok {
			return "", "", fmt.Errorf("undefined character name '%s'", name)
		}

		return escapeRune(r, true), rest, nil
	case '0':
		// octal escape

		e := nextOct(s, 2)
		r := parseIntRune(e, 8)

		return escapeRune(r, isStr), s[len(e):], nil
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// octal escape *or* decimal group reference (only if not in class)

		value := digit(c)

		if !isCls {
			if len(s) > 0 && isDigit(s[0]) {
				if isOctDigit(c) && isOctDigit(s[0]) &&
					len(s) > 1 && isOctDigit(s[1]) {

					value = 8*(8*value+digit(s[0])) + digit(s[1])
					if value > 0o377 {
						return "", "", fmt.Errorf(`octal escape value \%s outside of range 0-0o377`, string(c)+s[:2])
					}

					return escapeRune(rune(value), isStr), s[2:], nil
				}

				value = 10*value + digit(s[0])
				// s = s[1:]
			}

			// group references not supported
			return "", "", fmt.Errorf("invalid group reference %d", value)
		}

		if c >= '8' {
			return "", "", fmt.Errorf(`bad escape \%c`, c)
		}

		e := nextOct(s, 2)

		r := rune((1<<(3*len(e)))*value) + parseIntRune(e, 8) // 8 * value if len(e) == 1 else 64 * value
		if r > 0o377 {
			return "", "", fmt.Errorf(`octal escape value \%s outside of range 0-0o377`, string(c)+s[:2])
		}

		return escapeRune(r, isStr), s[len(e):], nil
	default:
		// All other cases

		if isCls && c == 'b' {
			// Go does not accept the '\b' escape in character sets, so a fix is needed.
			return `\x08`, s, nil
		}

		if isStr && !ascii {
			// If the current pattern is a string and the ASCII mode is not enabled,
			// some patterns had to be replaced with some equivalent unicode counterpart,
			// because by default, `regexp` only matches ASCII patterns.

			switch c {
			case 'd':
				return `\p{Nd}`, s, nil
			case 'D':
				return `\P{Nd}`, s, nil
			case 's':
				if isCls {
					return `\p{Z}\v`, s, nil
				}
				return `[\p{Z}\v]`, s, nil
			case 'S':
				if isCls {
					// While it is simple to include the negated character class of `\d` in a character set (by using \P{Nd}),
					// it is not trivial to include the negated character range of `\p{Z}\v`, since it is not possible to exclude
					// one of the sets `\p{Z}` AND `\v` at the same time. So, ranges must be included which contain ALL characters
					// which do not exist in `\p{Z}\v`.
					r, err := getRanges(`[^\p{Z}\v]`, isStr)
					if err != nil {
						return "", "", err
					}

					return r, s, nil
				}
				return `[^\p{Z}\v]`, s, nil
			case 'w':
				if isCls {
					return `\p{L}\p{N}_`, s, nil
				}
				return `[\p{L}\p{N}_]`, s, nil
			case 'W':
				if isCls {
					// See the comment at case 'S'.
					r, err := getRanges(`[^\p{L}\p{N}_]`, isStr)
					if err != nil {
						return "", "", err
					}

					return r, s, nil
				}
				return `[^\p{L}\p{N}_]`, s, nil
			}
		}

		// Return the escape sequence and let the regex parser do the rest.
		v := `\` + string(c)
		return v, s, nil
	}
}

// precondition: b must be in set "0123456789"
func digit(b byte) int {
	return int(b) - '0'
}

func nextHex(s string, n int) string {
	return nextFunc(s, n, func(r rune) bool {
		return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
	})
}

func nextOct(s string, n int) string {
	return nextFunc(s, n, func(r rune) bool {
		return '0' <= r && r <= '7'
	})
}

func nextFunc(s string, n int, fn func(r rune) bool) string {
	e := len(s)
	for i, c := range s {
		if i >= n || !fn(c) {
			e = i
			break
		}
	}

	return s[:e]
}

// TODO: is `isStr` necessary? "\xc2" is probably equivalent with "\x{00c2}".
func escapeRune(r rune, isStr bool) string {
	l := utf8.RuneLen(r)

	var b strings.Builder
	b.WriteString(`\x`)

	s := strconv.FormatInt(int64(r), 16)
	if l == 1 || (!isStr && r <= 0xff) {
		if r <= 0xf {
			b.WriteByte('0')
		}
		b.WriteString(s)
	} else {
		if l < 0 {
			l = 4
		}
		l *= 2 // 2 chars per byte

		b.WriteByte('{')
		if len(s) < l {
			b.WriteString(strings.Repeat("0", l-len(s)))
		}
		b.WriteString(s)
		b.WriteByte('}')
	}

	return b.String()
}

// assertion: hex string is valid and does not overflow int
func parseIntRune(s string, base int) rune {
	r, _ := strconv.ParseUint(s, base, 32)
	return rune(r)
}

func lookupUnicodeName(name string) (rune, bool) {
	name = strings.ToUpper(name)

	i, ok := slices.BinarySearch(unicodeNames[:], name)
	if !ok {
		return 0, false
	}

	return unicodeCodepoints[i], true
}

// if the repeat value had to be rewritten, the second value is true.
func parseRepeat(s string) (string, bool) {
	min, max, ok := strings.Cut(s, ",")

	if ok && min == "" && isDigitString(max) {
		var b strings.Builder
		b.Grow(len(max) + 4)
		b.WriteString("{0,")
		b.WriteString(max)
		b.WriteString("}")

		return b.String(), true
	}

	return s, false
}

func isDigitString(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}

	return true
}

func getRanges(s string, isStr bool) (string, error) {
	re, err := syntax.Parse(s, syntax.Perl)
	if err != nil {
		return "", err
	}

	if re.Op != syntax.OpCharClass {
		return "", fmt.Errorf("expected regex syntax type %s, got %s", syntax.OpCharClass, re.Op)
	}

	var b strings.Builder

	for i := 0; i < len(re.Rune); i += 2 {
		lo, hi := re.Rune[i], re.Rune[i+1]

		b.WriteString(escapeRune(lo, isStr))
		if lo != hi {
			b.WriteByte('-')
			b.WriteString(escapeRune(hi, isStr))
		}
	}

	return b.String(), nil
}

var specialBytes = [16]byte{
	0x04, 0x00, 0x00, 0x04, 0x04, 0x00, 0x04, 0x00,
	0x04, 0x05, 0x05, 0xa5, 0xa1, 0xa5, 0xa4, 0x08,
}

// special reports whether byte b needs to be escaped by QuoteMeta.
func special(b byte) bool {
	return b < utf8.RuneSelf && specialBytes[b%16]&(1<<(b/16)) != 0
}

func escape(s string) string {
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

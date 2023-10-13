package re

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type source struct {
	orig  string
	s     string
	isStr bool
}

func (s *source) init(src string, isStr bool) {
	s.orig = src
	s.s = src[:]
	s.isStr = isStr
}

func (s *source) until(src *source) string {
	before, found := strings.CutSuffix(s.s, src.s)
	if !found {
		return ""
	}

	return before
}

func (s *source) str() string {
	return s.s[:]
}

func (s *source) restore(src string) {
	s.s = src
}

func (s *source) tell() int {
	before, found := strings.CutSuffix(s.orig, s.s)
	if !found {
		return 0
	}

	return len(before)
}

func (s *source) read() (rune, bool) {
	if len(s.s) == 0 {
		return 0, false
	}

	c, size := utf8.DecodeRuneInString(s.s)
	if c == utf8.RuneError {
		c = rune(s.s[0])
		size = 1
	}

	s.s = s.s[size:]

	return c, true
}

func (s *source) peek() (rune, bool) {
	if len(s.s) == 0 {
		return 0, false
	}

	c, _ := utf8.DecodeRuneInString(s.s)
	if c == utf8.RuneError {
		c = rune(s.s[0])
	}

	return c, true
}

func (s *source) skipUntil(sep string) {
	_, s.s, _ = strings.Cut(s.s, "\n")
}

func (s *source) getUntil(c rune, name string) (string, error) {
	pre, rest, ok := strings.Cut(s.s, string(c))
	if !ok {
		return "", fmt.Errorf("missing %c, unterminated name", c)
	}
	if pre == "" {
		return "", fmt.Errorf("missing %s", name)
	}

	s.s = rest
	return pre, nil
}

func (s *source) match(c rune) bool {
	ch, width := utf8.DecodeRuneInString(s.s)
	if ch == c {
		s.s = s.s[width:]
		return true
	}

	return false
}

func (s *source) nextInt() int {
	i := 0
	for len(s.s) > 0 {
		if !isDigit(s.s[0]) {
			break
		}

		i = 10*i + digit(s.s[0])
		s.s = s.s[1:]
	}

	return i
}

func (s *source) nextHex(n int) string {
	return s.nextFunc(n, func(r rune) bool {
		return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
	})
}

func (s *source) nextOct(n int) string {
	return s.nextFunc(n, func(r rune) bool {
		return '0' <= r && r <= '7'
	})
}

func (s *source) nextFunc(n int, fn func(r rune) bool) string {
	e := len(s.s)
	for i, c := range s.s {
		if i >= n || !fn(c) {
			e = i
			break
		}
	}

	res := s.s[:e]
	s.s = s.s[e:]

	return res
}

func isWhitespace(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func isDigitC(c rune) bool {
	return '0' <= c && c <= '9'
}

func isSpecialChar(c rune) bool {
	switch c {
	case '.', '\\', '[', '{', '(', ')', '*', '+', '?', '^', '$', '|':
		return true
	default:
		return false
	}
}

func isRepeatChar(c rune) bool {
	switch c {
	case '*', '+', '?', '{':
		return true
	default:
		return false
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

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func checkgroupname(name string) error {
	if !isIdentifier(name) {
		return fmt.Errorf("bad character in group name '%s'", name)
	}
	return nil
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
		return reFlagIgnoreCase
	case 'L':
		return reFlagLocale
	case 'm':
		return reFlagMultiline
	case 's':
		return reFlagDotAll
	case 'x':
		return reFlagVerbose
	// extensions
	case 'a':
		return reFlagASCII
	case 'u':
		return reFlagUnicode
	default:
		return 0
	}
}

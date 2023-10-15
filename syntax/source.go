package syntax

import (
	"fmt"
	"strings"
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

func (s *source) str() string {
	return s.s[:]
}

func (s *source) restore(src string) {
	s.s = src
}

func (s *source) tell() int {
	return len(s.orig) - len(s.s)
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
	_, s.s, _ = strings.Cut(s.s, sep)
}

func (s *source) getUntil(c rune, name string) (string, error) {
	pre, rest, ok := strings.Cut(s.s, string(c))
	if pre == "" {
		return "", fmt.Errorf("missing %s", name)
	}
	if !ok {
		return "", fmt.Errorf("missing %c, unterminated name", c)
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

func (s *source) nextInt() (int, bool) {
	i := 0
	found := false

	for len(s.s) > 0 {
		if !isDigitByte(s.s[0]) {
			break
		}

		i = 10*i + digitByte(s.s[0])
		found = true

		s.s = s.s[1:]
	}

	return i, found
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

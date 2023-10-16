package syntax

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type source struct {
	orig  string // original source
	cur   string // current cursor
	isStr bool
}

func (s *source) init(src string, isStr bool) {
	s.orig = src
	s.cur = src[:]
	s.isStr = isStr
}

func (s *source) str() string {
	return s.cur[:]
}

func (s *source) restore(src string) {
	s.cur = src
}

func (s *source) tell() int {
	return len(s.orig) - len(s.cur)
}

func (s *source) read() (rune, bool) {
	if len(s.cur) == 0 {
		return 0, false
	}

	c, size := utf8.DecodeRuneInString(s.cur)
	if c == utf8.RuneError {
		c = rune(s.cur[0])
		size = 1
	}

	s.cur = s.cur[size:]

	return c, true
}

func (s *source) peek() (rune, bool) {
	if len(s.cur) == 0 {
		return 0, false
	}

	c, _ := utf8.DecodeRuneInString(s.cur)
	if c == utf8.RuneError {
		c = rune(s.cur[0])
	}

	return c, true
}

func (s *source) skipUntil(sep string) {
	_, s.cur, _ = strings.Cut(s.cur, sep)
}

func (s *source) getUntil(c rune, name string) (string, error) {
	pre, rest, ok := strings.Cut(s.cur, string(c))
	if pre == "" {
		return "", fmt.Errorf("missing %s", name)
	}
	if !ok {
		return "", fmt.Errorf("missing %c, unterminated name", c)
	}

	s.cur = rest
	return pre, nil
}

func (s *source) match(c rune) bool {
	ch, width := utf8.DecodeRuneInString(s.cur)
	if ch == c {
		s.cur = s.cur[width:]
		return true
	}

	return false
}

func (s *source) nextInt() (int, bool, error) {
	var i, prev int
	found := false

	for len(s.cur) > 0 {
		if !isDigitByte(s.cur[0]) {
			break
		}

		prev = i
		i = 10*i + digitByte(s.cur[0])
		if i < prev {
			return 0, false, errors.New("overflow error")
		}

		found = true

		s.cur = s.cur[1:]
	}

	return i, found, nil
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
	e := len(s.cur)
	for i, c := range s.cur {
		if i >= n || !fn(c) {
			e = i
			break
		}
	}

	res := s.cur[:e]
	s.cur = s.cur[e:]

	return res
}

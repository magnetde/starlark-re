package syntax

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/magnetde/starlark-re/util"
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

func (s *source) nextInt() int {
	i := 0
	for len(s.s) > 0 {
		if !util.IsDigit(s.s[0]) {
			break
		}

		i = 10*i + util.Digit(s.s[0])
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

func (s *source) checkgroupname(name string) error {
	if !(s.isStr || util.IsASCIIString(name)) {
		return fmt.Errorf("bad character in group name %s", util.QuoteString(name, s.isStr, false))
	}
	if !isIdentifier(name) {
		return fmt.Errorf("bad character in group name %s", util.QuoteString(name, s.isStr, true))
	}
	return nil
}

func isIdentifier(name string) bool {
	if name == "" {
		return false
	}

	for i, c := range name {
		if !unicode.IsLetter(c) && c != '_' && (i == 0 || !unicode.IsDigit(c)) {
			return false
		}
	}

	return true
}

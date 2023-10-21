package regex

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/magnetde/starlark-re/util"
)

// source represents a reader to read the regex string.
// The attributes may only be changed by using its functions.
type source struct {
	orig  string // original string
	cur   string // current cursor
	isStr bool
}

// init initializes the reader.
func (s *source) init(src string, isStr bool) {
	s.orig = src
	s.cur = src
	s.isStr = isStr
}

// tell returns the current read position.
func (s *source) tell() int {
	return len(s.orig) - len(s.cur)
}

// seek sets the current read position.
func (s *source) seek(pos int) {
	s.cur = s.orig[pos:]
}

// read reads the next UTF-8 character.
// If the current read position is at the end of the string, then the second return value is false.
// If the next character does not represent a valid UTF-8 character, then the next byte is returned.
// After reading, the current read position is increased.
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

// peek determines the next UTF-8 character.
// This function is equivalent with `read()`, except, that the current read position is not increased.
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

// skipUntil skips all characters, until the given character is found.
// The read position is then moved to the character that follows the specified character and the skipped characters are returned.
// If the rune is not found in the string, the read position is moved to the end of the string.
func (s *source) skipUntil(c rune) (string, bool) {
	pre, rest, ok := strings.Cut(s.cur, string(c))
	s.cur = rest
	return pre, ok
}

// getUntil returns all characters, until the given character is found.
// It is similar to `skipUntil`, except, it returns an error, if the string leading the given
// character is empty, or if the given character could not be found.
func (s *source) getUntil(c rune, name string) (string, error) {
	pre, rest, ok := strings.Cut(s.cur, string(c))
	if pre == "" {
		return "", s.errorh(fmt.Sprintf("missing %s", name))
	}
	if !ok {
		return "", s.errorh(fmt.Sprintf("missing %c, unterminated name", c))
	}

	s.cur = rest
	return pre, nil
}

// match returns, whether the next character matches the given character.
// If it does, the read position is then moved to the next character.
func (s *source) match(c rune) bool {
	ch, width := utf8.DecodeRuneInString(s.cur)
	if ch == c {
		s.cur = s.cur[width:]
		return true
	}

	return false
}

// nextInt returns the decimal integer at the current read position.
// If no integer exists, the second return value is false.
// If the integer overflows the type `int`, an error is returned.
// The read position is then moved to the position of the first character,
// that is no decimal digit.
func (s *source) nextInt() (int, bool, error) {
	var i, prev int
	found := false

	for len(s.cur) > 0 {
		if !isDigitByte(s.cur[0]) {
			break
		}

		prev = i
		i = 10*i + toDigitByte(s.cur[0])
		if i < prev {
			return 0, false, errors.New("overflow error")
		}

		found = true
		s.cur = s.cur[1:]
	}

	return i, found, nil
}

// nextHex returns the hexadecimal string at the current read position, with a maximum length of n.
// The read position is then moved to the position of the first character, that is no hexadecimal digit.
func (s *source) nextHex(n int) string {
	return s.nextFunc(n, func(r byte) bool {
		return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
	})
}

// nextOct returns the octal string at the current read position, with a maximum length of n.
// The read position is then moved to the position of the first character, that is no octal digit.
func (s *source) nextOct(n int) string {
	return s.nextFunc(n, func(r byte) bool {
		return '0' <= r && r <= '7'
	})
}

// nextFunc returns the string at the current read position, where each byte matches the function `fn`.
// The string has a maximum length of n bytes.
func (s *source) nextFunc(n int, fn func(r byte) bool) string {
	e := len(s.cur)
	for i := 0; i < len(s.cur); i++ {
		if i >= n || !fn(s.cur[i]) {
			e = i
			break
		}
	}

	res := s.cur[:e]
	s.cur = s.cur[e:]

	return res
}

// checkGroupName checks if a group name is valid.
// It ensures that group names in string patterns are valid unicode identifiers,
// and that group names in byte patterns are only made from ASCII characters.
func (s *source) checkGroupName(name string, offset int) error {
	if !(s.isStr || util.IsASCIIString(name)) {
		return s.erroro("bad character in group name "+util.ASCII(name, s.isStr), len(name)+offset)
	}
	if !isIdentifier(name) {
		return s.erroro("bad character in group name "+util.Repr(name, s.isStr), len(name)+offset)
	}
	return nil
}

// errorp returns a new error at the given position in the string of the source object.
// If the source represents a bytes object, all non-ascii characters in the message are escaped.
// If the string of the source object contains new line characters, the line and column number
// is also added to the error message.
func (s *source) errorp(msg string, pos int) error {
	if !s.isStr {
		msg = asciiEscape(msg)
	}

	msg = fmt.Sprintf("%s at position %d", msg, pos)

	if strings.Contains(s.orig, "\n") {
		lineno := strings.Count(s.orig[:pos], "\n") + 1
		colno := pos - strings.LastIndex(s.orig[:pos], "\n")

		msg = fmt.Sprintf("%s (line %d, column %d)", msg, lineno, colno)
	}

	return errors.New(msg)
}

// errorh is equivalent to errorp for the current position.
func (s *source) errorh(msg string) error {
	return s.errorp(msg, s.tell())
}

// erroro is equivalent to errorp for the current position minus the given offset.
func (s *source) erroro(msg string, offset int) error {
	return s.errorp(msg, s.tell()-offset)
}

// clen returns the number of bytes of the given character.
// This function can be used, to calculate the offset for an error.
func (s *source) clen(c rune) int {
	l := utf8.RuneLen(c)
	if l < 0 {
		l = 1
	}

	return l
}

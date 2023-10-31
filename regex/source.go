package regex

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/magnetde/starlark-re/util"
)

// Constants for detecting overflow in `nextInt()` function.
const (
	maxValueDiv10 = math.MaxInt / 10
	maxValueMod10 = math.MaxInt % 10
)

// source represents a reader to read the regex string.
// The attributes should only be changed using its functions.
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

// tell returns the current reading position.
func (s *source) tell() int {
	return len(s.orig) - len(s.cur)
}

// seek sets the current reading position.
func (s *source) seek(pos int) {
	s.cur = s.orig[pos:]
}

// read reads the next character.
// If the current reading position is at the end of the string, then the second return value is false.
// After reading, the current reading position is increased.
func (s *source) read() (rune, bool) {
	c, size, ok := s.next()
	if !ok {
		return 0, false
	}

	s.cur = s.cur[size:]
	return c, true
}

// next returns the next character.
// If the current reading position is at the end of the string, then the last return value is false.
// If the next character does not represent a valid UTF-8 character or if
// the string represents a bytes object, then the next byte is returned.
func (s *source) next() (rune, int, bool) {
	if len(s.cur) == 0 {
		return 0, 0, false
	}

	if s.isStr {
		c, size := utf8.DecodeRuneInString(s.cur)
		if c == utf8.RuneError {
			c = rune(s.cur[0])
			size = 1
		}

		return c, size, true
	} else {
		return rune(s.cur[0]), 1, true
	}
}

// peek determines the next UTF-8 character.
// This function is similar to `read()`, but the current reading position is not incremented.
func (s *source) peek() (rune, bool) {
	c, _, ok := s.next()
	if !ok {
		return 0, false
	}

	return c, true
}

// match returns, whether the next character matches the given character.
// If it does, the reading position is then moved to the next character.
func (s *source) match(c rune) bool {
	ch, size, ok := s.next()
	if !ok || ch != c {
		return false
	}

	s.cur = s.cur[size:]
	return true
}

// skipUntil skips all characters until the given character is found.
// The reading position is then moved to the character that follows the specified character and the skipped characters are returned.
// If the character is not found in the string, the reading position is moved to the end of the string.
func (s *source) skipUntil(c rune) (string, bool) {
	pre, rest, ok := strings.Cut(s.cur, string(c))
	s.cur = rest
	return pre, ok
}

// getUntil returns all characters until the given character is found.
// It is identical to `skipUntil` except that it returns an error if the string preceding the given
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

// nextInt returns the decimal integer at the current reading position.
// If there is no integer present, this function returns false as the second value.
// Afterwards, the cursor is moved to the first non-numeric character.
// If the integer exceeds the maximum value of type `int`, an error is returned.
func (s *source) nextInt() (int, bool, error) {
	i := 0
	found := false

	for len(s.cur) > 0 {
		if !isDigitByte(s.cur[0]) {
			break
		}

		d := toDigitByte(s.cur[0])

		if i > maxValueDiv10 || (i == maxValueDiv10 && d > maxValueMod10) {
			return 0, false, errors.New("overflow error")
		}

		i *= 10
		i += d

		found = true
		s.cur = s.cur[1:]
	}

	return i, found, nil
}

// nextHex returns a string of hexadecimal characters at the current reading position with a maximum length of n.
// The reading position is then moved to the position of the first non-hexadecimal character.
func (s *source) nextHex(n int) string {
	return s.nextFunc(n, func(r byte) bool {
		return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
	})
}

// nextOct returns a string of octal characters at the current reading position with a maximum length of n.
// The reading position is then moved to the position of the first non-octal character.
func (s *source) nextOct(n int) string {
	return s.nextFunc(n, func(r byte) bool {
		return '0' <= r && r <= '7'
	})
}

// nextFunc returns the string at the current reading position, where each byte matches the function `fn`.
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
// It ensures that names used in string patterns are valid unicode identifiers,
// and that group names used in byte patterns only consist of ASCII characters.
func (s *source) checkGroupName(name string, offset int) error {
	if !(s.isStr || isASCIIString(name)) {
		return s.erroro("bad character in group name "+util.ASCII(name, s.isStr), len(name)+offset)
	}
	if !isIdentifier(name) {
		return s.erroro("bad character in group name "+util.Repr(name, true), len(name)+offset)
	}
	return nil
}

// errorp returns a new error at the given position in the string of the source object.
// If the source represents a bytes object, any non-ascii characters in the message are escaped.
// If the string of the source object contains newline characters, the line and column number
// is also added to the error message.
func (s *source) errorp(msg string, pos int) error {
	if !s.isStr {
		msg = util.ASCIIReplace(msg)
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

// clen returns the byte count for a given UTF-8 character.
// This function can be used, to calculate the offset for an error.
func (s *source) clen(c rune) int {
	l := utf8.RuneLen(c)
	if l < 0 {
		l = 1
	}

	return l
}

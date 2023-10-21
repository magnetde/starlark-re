package regex

import (
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"
)

type preprocessor struct {
	isStr bool
	p     *subPattern
}

func newPreprocessor(s string, isStr bool, flags uint32) (*preprocessor, error) {
	sp, err := parse(s, isStr, flags)
	if err != nil {
		return nil, err
	}

	p := &preprocessor{
		isStr: isStr,
		p:     sp,
	}

	return p, nil
}

func (p *preprocessor) flags() uint32 {
	return p.p.state.flags
}

func (p *preprocessor) groupNames() map[string]int {
	return p.p.state.groupdict
}

// IsSupported checks, wether the current pattern is supported by the regexp engine of the Go stdlib.
func (p *preprocessor) isSupported() bool {
	if p.p.isUnsupported() {
		return false
	}

	for group := range p.p.state.groupdict {
		if !isGoIdentifer(group) {
			return false
		}
	}

	return true
}

func isGoIdentifer(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		if c > unicode.MaxASCII {
			return false
		}

		if !isASCIILetterByte(c) && c != '_' && (i == 0 || !isDigitByte(c)) {
			return false
		}
	}

	return true
}

func (p *preprocessor) stdPattern() string {
	var b strings.Builder

	flags := p.flags()
	if flags&FlagIgnoreCase != 0 && flags&FlagASCII != 0 {
		// Remove the ASCII flag if it is enabled together with the IGNORECASE flag,
		// because the ignore case handling is done by the preprocessor.
		flags &= ^FlagIgnoreCase
	}

	if flags&supportedFlags != 0 {
		b.WriteString("(?")

		if flags&FlagIgnoreCase != 0 {
			b.WriteByte('i')
		}
		if flags&FlagMultiline != 0 {
			b.WriteByte('m')
		}
		if flags&FlagDotAll != 0 {
			b.WriteByte('s')
		}

		b.WriteByte(')')
	}

	b.WriteString(p.p.string(p.isStr, func(w *subPatternWriter, t *regexNode, ctx *subPatternContext) bool {
		return p.defaultReplacer(w, t, ctx, true)
	}))

	return b.String()
}

var unicodeRanges = map[catcode]string{
	categoryDigit:    `[\p{Nd}]`,
	categoryNotDigit: `[^\p{Nd}]`,
	categorySpace:    `[\p{Z}\v]`,
	categoryNotSpace: `[^\p{Z}\v]`,
	categoryWord:     `[\p{L}\p{N}_]`,
	categoryNotWord:  `[^\p{L}\p{N}_]`,
}

// If UNICODE is enabled, the sets \d, \D, \s, ... are replaced with an unicode counterpart.
// If IGNORECASE and ASCII is enabled, every literal and range of ASCII characters is replaced with a character set,
// that contains all characters, that match all cases of this character.
func (p *preprocessor) defaultReplacer(w *subPatternWriter, t *regexNode, ctx *subPatternContext, std bool) bool {
	if !p.isStr {
		return false
	}

	flags := p.flags()
	if ctx.group != nil {
		addFlags := ctx.group.addFlags
		delFlags := ctx.group.delFlags

		if addFlags&FlagUnicode != 0 || delFlags&FlagASCII != 0 {
			flags |= FlagUnicode
			flags &= ^FlagASCII
		}
		if addFlags&FlagASCII != 0 || delFlags&FlagUnicode != 0 {
			flags |= FlagASCII
			flags &= ^FlagUnicode
		}
	}

	unicode := false
	asciiCase := false

	if flags&FlagUnicode != 0 {
		unicode = true
	} else if flags&FlagIgnoreCase != 0 && flags&FlagASCII != 0 {
		asciiCase = true
	}

	switch t.opcode {
	case opCategory:
		// If the current pattern is a string and the ASCII mode is not enabled,
		// some patterns had to be replaced with some equivalent unicode counterpart,
		// because by default, `regexp` only matches ASCII patterns.

		if !unicode {
			return false
		}

		// Always inside of character sets.
		category := t.params.(catcode)

		// Chrck, if the short unicode character sets can be added to the regex.
		// The shorter unicode classes are only supported by the standard regex engine,
		// and they can only be used, if the current category does not negate the set or if the current
		// category is the only element in the character set.

		if std {
			// Handle a complicated program flow with an switch statement with break and fallthrough.
			switch category {
			case categoryNotDigit, categoryNotSpace, categoryNotWord:
				if ctx.hasSiblings {
					break
				}

				fallthrough
			default:
				if r, ok := unicodeRanges[category]; ok {
					r = strings.TrimSuffix(strings.TrimPrefix(r, "["), "]") // remove the character set chars
					w.WriteString(r)                                        // write the character set
					return true
				}
			}
		}

		// While it is simple to include the negated character class of `\d` in a character set (by using \P{Nd}),
		// it is not trivial to include the negated character range of `\p{Z}\v`, since it is not possible to exclude
		// one of the sets `\p{Z}` AND `\v` at the same time. So, ranges must be included which contain ALL characters
		// which do not exist in `\p{Z}\v`.
		unirange, err := buildRange(category)
		if err != nil {
			return false
		}

		for i := 0; i < len(unirange); i += 2 {
			lo, hi := unirange[i], unirange[i+1]

			w.writeLiteral(lo)
			if lo != hi {
				w.WriteByte('-')
				w.writeLiteral(hi)
			}
		}
	case opLiteral:
		if !asciiCase {
			return false
		}

		o, ok := otherASCIICase(t.c)
		if !ok {
			return false
		}

		if !ctx.inSet {
			w.WriteByte('[')
			w.writeLiteral(t.c)
			w.writeLiteral(o)
			w.WriteByte(']')
		} else {
			w.writeLiteral(t.c)
			w.writeLiteral(o)
		}
	case opRange:
		if !asciiCase {
			return false
		}

		p := t.params.(rangeParams)

		lo, oklo := otherASCIICase(p.lo)
		if !oklo {
			return false
		}

		hi, okhi := otherASCIICase(p.hi)
		if !okhi {
			return false
		}

		w.writeLiteral(p.lo)
		w.WriteByte('-')
		w.writeLiteral(p.hi)
		w.writeLiteral(lo)
		w.WriteByte('-')
		w.writeLiteral(hi)
	}

	return false
}

func buildRange(c catcode) ([]rune, error) {
	r, ok := unicodeRanges[c]
	if ok {
		return nil, fmt.Errorf("unknown category %d", c)
	}

	re, err := syntax.Parse(r, syntax.Perl)
	if err != nil {
		return nil, err
	}

	if re.Op != syntax.OpCharClass {
		return nil, fmt.Errorf("expected regex syntax type %s, got %s", syntax.OpCharClass, re.Op)
	}

	return re.Rune, nil
}

// returns the character with the opposite case of `c`.
// Must only be called for ASCII chars.
func otherASCIICase(c rune) (rune, bool) {
	if c > unicode.MaxASCII {
		return 0, false
	}
	if 'a' <= c && c <= 'z' {
		return c - 'a' + 'A', true
	}
	if 'A' <= c && c <= 'Z' {
		return c - 'A' + 'a', true
	}
	return c, false
}

func (p *preprocessor) fallbackPattern() (string, map[string]int) {
	groupMapping := make(map[string]int)

	var b strings.Builder
	b.WriteString(p.p.string(p.isStr, func(w *subPatternWriter, t *regexNode, ctx *subPatternContext) bool {
		if p.defaultReplacer(w, t, ctx, false) {
			return true
		}

		if t.opcode == opSubpattern {
			// The preprocessor must only write subpatterns differently,
			// that have a group number.

			p := t.params.(subPatternParam)

			if p.group < 0 {
				return false
			}

			g := fmt.Sprintf("g%02d", p.group) // every group gets a unique group name to keep its order
			groupMapping[g] = p.group          // store the group position (determined from the parser) together with its new name

			w.WriteString("(?<") // No ? before P
			w.WriteString(g)
			w.WriteByte('>')
			if p.p.len() > 0 {
				w.writePattern(p.p, &p)
			}
			w.WriteByte(')')

			return true
		}

		return false
	}))

	return b.String(), groupMapping
}

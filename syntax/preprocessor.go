package syntax

import (
	"fmt"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Preprocessor struct {
	isStr bool
	p     *subPattern
}

func NewPreprocessor(s string, isStr bool, flags int) (*Preprocessor, error) {
	sp, err := parse(s, isStr, flags)
	if err != nil {
		return nil, err
	}

	sp.dump(nil) // TODO: remove

	p := &Preprocessor{
		isStr: isStr,
		p:     sp,
	}

	return p, nil
}

func (p *Preprocessor) HasBackrefs() bool {
	return p.p.hasBackrefs()
}

func (p *Preprocessor) String() string {
	flags := p.p.state.flags

	var b strings.Builder

	if flags&(FlagIgnoreCase|FlagMultiline|FlagDotAll) != 0 {
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

	b.WriteString(p.p.string(p.isStr, p.preReplacer))

	return b.String()
}

func (p *Preprocessor) preReplacer(t *token) (string, bool) {
	ascii := p.p.state.flags&FlagASCII != 0

	if p.isStr && !ascii {
		// If the current pattern is a string and the ASCII mode is not enabled,
		// some patterns had to be replaced with some equivalent unicode counterpart,
		// because by default, `regexp` only matches ASCII patterns.

		switch t.opcode {
		case CATEGORY:
			// not in class

			p := t.params.(paramCategory)

			switch chCode(p) {
			case CATEGORY_DIGIT:
				return `\p{Nd}`, true
			case CATEGORY_NOT_DIGIT:
				return `\P{Nd}`, true
			case CATEGORY_SPACE:
				return `[\p{Z}\v]`, true
			case CATEGORY_NOT_SPACE:
				return `[^\p{Z}\v]`, true
			case CATEGORY_WORD:
				return `[\p{L}\p{N}_]`, true
			case CATEGORY_NOT_WORD:
				return `[^\p{L}\p{N}_]`, true
			}
		case IN:
			// in class

			for _, item := range t.items {
				if item.opcode == CATEGORY {
					pm := item.params.(paramCategory)

					switch chCode(pm) {
					case CATEGORY_DIGIT:
						return `\p{Nd}`, true
					case CATEGORY_NOT_DIGIT:
						return `\P{Nd}`, true
					case CATEGORY_SPACE:
						return `\p{Z}\v`, true
					case CATEGORY_NOT_SPACE:
						// While it is simple to include the negated character class of `\d` in a character set (by using \P{Nd}),
						// it is not trivial to include the negated character range of `\p{Z}\v`, since it is not possible to exclude
						// one of the sets `\p{Z}` AND `\v` at the same time. So, ranges must be included which contain ALL characters
						// which do not exist in `\p{Z}\v`.
						r, err := getRanges(`[^\p{Z}\v]`, p.isStr)
						if err != nil {
							return "", false
						}

						return r, true
					case CATEGORY_WORD:
						return `\p{L}\p{N}_`, true
					case CATEGORY_NOT_WORD:
						// See the comment at case 'S'.
						r, err := getRanges(`[^\p{L}\p{N}_]`, p.isStr)
						if err != nil {
							return "", false
						}

						return r, true
					}
				}
			}
		}
	}

	return "", false
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

		b.WriteString(hexEscape(lo, isStr))
		if lo != hi {
			b.WriteByte('-')
			b.WriteString(hexEscape(hi, isStr))
		}
	}

	return b.String(), nil
}

func hexEscape(r rune, isStr bool) string {
	l := utf8.RuneLen(r)

	var w strings.Builder
	w.WriteString(`\x`)

	s := strconv.FormatInt(int64(r), 16)
	if l == 1 || (!isStr && r <= 0xff) {
		if r <= 0xf {
			w.WriteByte('0')
		}
		w.WriteString(s)
	} else {
		if l < 0 {
			l = 4
		}
		l *= 2 // 2 chars per byte

		w.WriteByte('{')
		if len(s) < l {
			w.WriteString(strings.Repeat("0", l-len(s)))
		}
		w.WriteString(s)
		w.WriteByte('}')
	}

	return w.String()
}

package syntax

import (
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"
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

func (p *Preprocessor) Flags() int {
	return p.p.state.flags
}

func (p *Preprocessor) GroupNames() map[string]int {
	return p.p.state.groupdict
}

// IsSupported checks, wether the current pattern is supported by the regexp engine of the Go stdlib.
func (p *Preprocessor) IsSupported() bool {
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

func (p *Preprocessor) String() string {
	flags := p.Flags()

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

	b.WriteString(p.p.string(p.isStr, p.replacer))

	return b.String()
}

func (p *Preprocessor) replacer(w *subPatternWriter, t *token) bool {
	ascii := p.Flags()&FlagASCII != 0

	if !p.isStr || ascii {
		return false
	}

	// If the current pattern is a string and the ASCII mode is not enabled,
	// some patterns had to be replaced with some equivalent unicode counterpart,
	// because by default, `regexp` only matches ASCII patterns.

	switch t.opcode {
	case CATEGORY:
		// Always inside of character sets.
		pm := t.params.(paramCategory)

		switch chCode(pm) {
		case CATEGORY_DIGIT:
			w.WriteString(`\p{Nd}`)
			return true
		case CATEGORY_NOT_DIGIT:
			w.WriteString(`\P{Nd}`)
			return true
		case CATEGORY_SPACE:
			w.WriteString(`\p{Z}\v`)
			return true
		case CATEGORY_NOT_SPACE:
			// While it is simple to include the negated character class of `\d` in a character set (by using \P{Nd}),
			// it is not trivial to include the negated character range of `\p{Z}\v`, since it is not possible to exclude
			// one of the sets `\p{Z}` AND `\v` at the same time. So, ranges must be included which contain ALL characters
			// which do not exist in `\p{Z}\v`.
			err := p.writeRange(w, `[^\p{Z}\v]`)
			return err == nil
		case CATEGORY_WORD:
			w.WriteString(`\p{L}\p{N}_`)
			return true
		case CATEGORY_NOT_WORD:
			// See the comment at case 'S'.
			err := p.writeRange(w, `[^\p{L}\p{N}_]`)
			return err == nil
		}
	}

	return false
}

func (p *Preprocessor) writeRange(w *subPatternWriter, s string) error {
	re, err := syntax.Parse(s, syntax.Perl)
	if err != nil {
		return err
	}

	if re.Op != syntax.OpCharClass {
		return fmt.Errorf("expected regex syntax type %s, got %s", syntax.OpCharClass, re.Op)
	}

	for i := 0; i < len(re.Rune); i += 2 {
		lo, hi := re.Rune[i], re.Rune[i+1]

		w.writeLiteral(lo)
		if lo != hi {
			w.WriteByte('-')
			w.writeLiteral(hi)
		}
	}

	return nil
}

func (p *Preprocessor) FallbackString() (string, map[string]int) {
	groupMapping := make(map[string]int)

	var b strings.Builder
	b.WriteString(p.p.string(p.isStr, func(w *subPatternWriter, t *token) bool {
		if t.opcode == SUBPATTERN {
			p := t.params.(*paramSubPattern)

			// The preprocessor must only write subpatterns differently,
			// that have a group number.

			if p.group < 0 {
				return false
			}

			g := fmt.Sprintf("g%02d", p.group) // every group gets a unique group name to keep its order
			groupMapping[g] = p.group          // store the group position (determined from the parser) together with its new name

			w.WriteString("(?<") // No ? before P
			w.WriteString(g)
			w.WriteByte('>')
			if p.p.len() > 0 {
				w.writePattern(p.p)
			}
			w.WriteByte(')')

			return true
		}

		return false
	}))

	return b.String(), groupMapping
}

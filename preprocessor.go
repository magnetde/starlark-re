package re

import (
	"fmt"
	"regexp/syntax"
	"strings"
)

type preprocessor struct {
	isStr bool
	p     *subPattern
}

func newPreprocessor(s string, isStr bool, flags int) (*preprocessor, error) {
	sp, err := parse(s, isStr, flags)
	if err != nil {
		return nil, err
	}

	fmt.Println(">>>", s)
	fmt.Println()
	sp.dump(nil) // TODO: remove

	p := &preprocessor{
		isStr: isStr,
		p:     sp,
	}

	return p, nil
}

func (p *preprocessor) flags() int {
	return p.p.state.flags
}

func (p *preprocessor) String() string {
	/**


	flags = pp.flags()

	if flags&(reFlagIgnoreCase|reFlagMultiline|reFlagDotAll) != 0 {
		var b strings.Builder
		b.Grow(len(p) + 3 + bits.OnesCount(uint(flags)))

		b.WriteString("(?")

		if flags&reFlagIgnoreCase != 0 {
			b.WriteByte('i')
		}
		if flags&reFlagMultiline != 0 {
			b.WriteByte('m')
		}
		if flags&reFlagDotAll != 0 {
			b.WriteByte('s')
		}

		b.WriteByte(')')
		b.WriteString(p)

		p = b.String()
	}
	*/

	return ""
}

func (p *preprocessor) String2() string {
	return ""
}

func (p *preprocessor) preReplacer(t *token) (string, bool) {
	ascii := p.p.state.flags&reFlagASCII != 0

	switch t.opcode {
	case IN:
		if p.isStr && !ascii {
			for _, item := range t.items {
				if item.opcode == CATEGORY {
					pm := item.params.(paramCategory)

					switch chCode(pm) {
					case CATEGORY_DIGIT:
						return `\p{Nd}`, true
					case CATEGORY_NOT_DIGIT:
						return `\P{Nd}`, true
					case CATEGORY_SPACE:
					case CATEGORY_NOT_SPACE:
					case CATEGORY_WORD:
					case CATEGORY_NOT_WORD:
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

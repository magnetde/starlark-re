package syntax

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type subPattern struct {
	state *state
	data  []*token
}

func newSubpattern(state *state) *subPattern {
	return &subPattern{
		state: state,
	}
}

func (p *subPattern) append(t *token) {
	p.data = append(p.data, t)
}
func (p *subPattern) len() int {
	return len(p.data)
}

func (p *subPattern) get(i int) *token {
	i = p.index(i)
	return p.data[i]
}
func (p *subPattern) index(i int) int {
	if i < 0 {
		i += len(p.data)
	}
	return i
}

func (p *subPattern) del(i int) {
	i = p.index(i)
	p.data = slices.Delete(p.data, i, i+1)
}

func (p *subPattern) set(i int, t *token) {
	i = p.index(i)
	p.data[i] = t
}

// isUnsupported returns, whether the subpattern is unsupported by the std regexp engine.
func (p *subPattern) isUnsupported() bool {
	for _, item := range p.data {
		if isUnsupported(item) {
			return true
		}
	}

	return false
}

func isUnsupported(t *token) bool {
	switch t.opcode {
	case ASSERT, ASSERT_NOT, GROUPREF, GROUPREF_EXISTS, ATOMIC_GROUP:
		return true
	case BRANCH:
		p := t.params.(*paramSubPatterns)

		for _, item := range p.items {
			if item.isUnsupported() {
				return true
			}
		}
	case MIN_REPEAT, MAX_REPEAT, POSSESSIVE_REPEAT:
		p := t.params.(*paramRepeat)

		return p.item.isUnsupported()
	case SUBPATTERN:
		p := t.params.(*paramSubPattern)

		return p.p.isUnsupported()
	}

	return false
}

func (p *subPattern) dump(print func(s string)) {
	var b strings.Builder
	p.dumpr(&b, 0)

	if print == nil {
		fmt.Print(b.String())
	} else {
		print(b.String())
	}
}

func (p *subPattern) dumpr(b *strings.Builder, level int) {
	for _, v := range p.data {
		op := v.opcode

		b.WriteString(strings.Repeat("  ", level))
		b.WriteString(op.String())
		dumpToken(v, b, level)
	}
}

func dumpToken(v *token, b *strings.Builder, level int) {
	printr := func(v ...any) {
		for i, a := range v {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(fmt.Sprint(a))
		}
	}

	print := func(v ...any) {
		printr(v...)
		b.WriteByte('\n')
	}

	printParams := func(av ...any) {
		nl := false
		for _, a := range av {
			if sp, ok := a.(*subPattern); ok {
				if !nl {
					print()
				}

				sp.dumpr(b, level+1)
				nl = true
			} else {
				if !nl {
					printr(" ")
				}
				printr(a)
				nl = false
			}
		}
		if !nl {
			print()
		}
	}

	switch v.opcode {
	case ASSERT, ASSERT_NOT:
		pv := v.params.(*paramAssert)
		printParams(pv.dir, pv.p)
	case AT:
		pv := v.params.(paramAt)
		printParams(atCode(pv).String())
	case BRANCH:
		print()
		pv := v.params.(*paramSubPatterns)
		for i, a := range pv.items {
			if i != 0 {
				print(strings.Repeat("  ", level) + "OR")
			}
			a.dumpr(b, level+1)
		}
	case CATEGORY:
		pv := v.params.(paramCategory)
		printParams(pv)
	case GROUPREF:
		pv := v.params.(paramInt)
		printParams(pv)
	case GROUPREF_EXISTS:
		pv := v.params.(*paramGrouprefEx)
		print("", pv.condgroup)

		pv.itemYes.dumpr(b, level+1)
		if pv.itemNo != nil {
			print(strings.Repeat("  ", level) + "ELSE")
			pv.itemNo.dumpr(b, level+1)
		}
	case IN:
		// member sublanguage
		print()

		// members in items are either of type LITERAL, RANGE or CATEGORY
		for _, v := range v.items {
			printr(strings.Repeat("  ", level+1) + v.opcode.String() + " ")
			switch v.opcode {
			case LITERAL:
				print(v.c)
			case RANGE:
				vp := v.params.(*paramRange)
				print(fmt.Sprintf("(%d, %d)", vp.lo, vp.hi))
			case CATEGORY:
				vp := v.params.(paramCategory)
				print(chCode(vp).String())
			}
		}
	case LITERAL, NOT_LITERAL:
		print("", v.c)
	case MIN_REPEAT, MAX_REPEAT, POSSESSIVE_REPEAT:
		pv := v.params.(*paramRepeat)
		var maxRepeat string
		if pv.max != MAXREPEAT {
			maxRepeat = strconv.Itoa(pv.max)
		} else {
			maxRepeat = "MAXREPEAT"
		}
		printParams(pv.min, maxRepeat, pv.item)
	case RANGE:
		pv := v.params.(*paramRange)
		printParams(pv.lo, pv.hi)
	case SUBPATTERN:
		pv := v.params.(*paramSubPattern)
		var group string
		if pv.group >= 0 {
			group = strconv.Itoa(pv.group)
		} else {
			group = "None"
		}
		printParams(group, pv.addFlags, pv.delFlags, pv.p)
	case ATOMIC_GROUP:
		pv := v.params.(*paramSubPatterns)
		print()
		pv.items[0].dumpr(b, level+1)
	}
}

// TODO: change the replacer parameters, so the *subPatternWriter can be used.
func (p *subPattern) string(isStr bool, replace func(w *subPatternWriter, t *token) bool) string {
	w := subPatternWriter{
		isStr:   isStr,
		replace: replace,
	}

	w.writePattern(p)
	return w.String()
}

type subPatternWriter struct {
	strings.Builder
	isStr   bool
	replace func(w *subPatternWriter, t *token) bool
}

func (w *subPatternWriter) writePattern(p *subPattern) {
	for _, item := range p.data {
		if !w.replace(w, item) {
			w.writeToken(item)
		}
	}
}

func (w *subPatternWriter) writeToken(t *token) {
	switch t.opcode {
	case ANY:
		w.WriteByte('.')
	case ASSERT, ASSERT_NOT:
		p := t.params.(*paramAssert)

		w.WriteString("(?")

		if p.dir < 0 {
			w.WriteByte('<')
			if t.opcode == ASSERT {
				w.WriteByte('=')
			} else {
				w.WriteByte('!')
			}
		} else if t.opcode == ASSERT {
			w.WriteByte('=')
		} else {
			w.WriteByte('!')
		}

		w.writePattern(p.p)
		w.WriteByte(')')
	case AT:
		p := t.params.(paramAt)

		switch atCode(p) {
		case AT_BEGINNING:
			w.WriteByte('^')
		case AT_BEGINNING_STRING:
			w.WriteString(`\A`)
		case AT_BOUNDARY:
			w.WriteString(`\b`)
		case AT_NON_BOUNDARY:
			w.WriteString(`\B`)
		case AT_END:
			w.WriteByte('$')
		case AT_END_STRING:
			w.WriteString(`\Z`)
		}
	case BRANCH:
		p := t.params.(*paramSubPatterns)

		for i, item := range p.items {
			if i > 0 {
				w.WriteByte('|')
			}
			w.writePattern(item)
		}
	case CATEGORY:
		w.writeCategory(t)
	case GROUPREF:
		p := t.params.(paramInt)

		w.WriteString(`\`)
		w.writeInt(int(p))
	case GROUPREF_EXISTS:
		p := t.params.(*paramGrouprefEx)

		w.WriteString("(?(")
		w.writeInt(p.condgroup)
		w.WriteByte(')')
		w.writePattern(p.itemYes)
		if p.itemNo != nil {
			w.WriteByte('|')
			w.writePattern(p.itemNo)
		}
		w.WriteByte(')')
	case IN:
		// members in items are either of type LITERAL, RANGE or CATEGORY
		w.WriteByte('[')
		for _, v := range t.items {
			switch v.opcode {
			case LITERAL:
				w.writeLiteral(v.c)
			case RANGE:
				p := v.params.(*paramRange)

				w.writeLiteral(p.lo)
				w.WriteByte('-')
				w.writeLiteral(p.hi)
			case CATEGORY:
				w.writeCategory(v)
			}
		}
		w.WriteByte(']')
	case LITERAL:
		w.writeLiteral(t.c)
	case MIN_REPEAT, MAX_REPEAT, POSSESSIVE_REPEAT:
		p := t.params.(*paramRepeat)

		needsGroup := p.item.len() > 1 || p.item.get(0).opcode == BRANCH

		if !needsGroup {
			w.writePattern(p.item)
		} else {
			w.WriteString("(?:")
			w.writePattern(p.item)
			w.WriteByte(')')
		}

		if p.min == 0 && p.max == 1 {
			w.WriteByte('?')
		} else if p.min == 0 && p.max == MAXREPEAT {
			w.WriteByte('*')
		} else if p.min == 1 && p.max == MAXREPEAT {
			w.WriteByte('+')
		} else {
			w.WriteByte('{')
			w.writeInt(p.min)
			w.WriteByte(',')
			if p.max != MAXREPEAT {
				w.writeInt(p.max)
			}
			w.WriteByte('}')
		}

	case NOT_LITERAL:
		w.WriteString("[^")
		w.writeLiteral(t.c)
		w.WriteByte(']')
	case NEGATE:
		w.WriteByte('^')
	case RANGE: // Should only occur in "IN" tokens
		p := t.params.(*paramRange)

		w.writeLiteral(p.lo)
		w.WriteByte('-')
		w.writeLiteral(p.hi)
	case SUBPATTERN:
		p := t.params.(*paramSubPattern)

		w.WriteByte('(')

		if p.group >= 0 {
			groupName := groupname(p.p, p.group)
			if groupName != "" {
				w.WriteString("?P<")
				w.WriteString(groupName)
				w.WriteByte('>')
			}
		} else if p.addFlags != 0 || p.delFlags != 0 {
			// Flags can only appear, when no group name exists

			w.WriteByte('?')
			if p.addFlags != 0 {
				w.writeFlags(p.addFlags)
			}
			if p.delFlags != 0 {
				w.WriteByte('-')
				w.writeFlags(p.addFlags)
			}

			if p.p.len() > 0 {
				w.WriteByte(':')
			}
		}

		if p.p.len() > 0 {
			w.writePattern(p.p)
		}

		w.WriteByte(')')
	case ATOMIC_GROUP:
		p := t.params.(*paramSubPatterns)

		w.WriteString("(?>")
		w.writePattern(p.items[0])
		w.WriteByte(')')
	}
}

func groupname(p *subPattern, gid int) string {
	for name, g := range p.state.groupdict {
		if gid == g {
			return name
		}
	}

	return ""
}

func (w *subPatternWriter) writeInt(i int) {
	w.WriteString(strconv.Itoa(i))
}

func (w *subPatternWriter) writeLiteral(r rune) {
	l := utf8.RuneLen(r)

	w.WriteString(`\x`)

	s := strconv.FormatInt(int64(r), 16)
	if l == 1 || (!w.isStr && r <= 0xff) {
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
}

// will panic, if token is not a category token
func (w *subPatternWriter) writeCategory(t *token) {
	p := t.params.(paramCategory)

	switch chCode(p) {
	case CATEGORY_DIGIT:
		w.WriteString(`\d`)
	case CATEGORY_NOT_DIGIT:
		w.WriteString(`\D`)
	case CATEGORY_SPACE:
		w.WriteString(`\s`)
	case CATEGORY_NOT_SPACE:
		w.WriteString(`\S`)
	case CATEGORY_WORD:
		w.WriteString(`\w`)
	case CATEGORY_NOT_WORD:
		w.WriteString(`\W`)
	}
}

func (w *subPatternWriter) writeFlags(flags int) {
	if flags&FlagASCII != 0 {
		w.WriteByte('a')
	}
	if flags&FlagIgnoreCase != 0 {
		w.WriteByte('i')
	}
	if flags&FlagLocale != 0 {
		w.WriteByte('L')
	}
	if flags&FlagMultiline != 0 {
		w.WriteByte('m')
	}
	if flags&FlagDotAll != 0 {
		w.WriteByte('s')
	}
	if flags&FlagUnicode != 0 {
		w.WriteByte('u')
	}
	if flags&FlagVerbose != 0 {
		w.WriteByte('x')
	}
}

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

func (p *subPattern) replace(i int, sp *subPattern) {
	i = p.index(i)
	p.data = slices.Delete(p.data, i, i+1)
	p.data = slices.Insert(p.data, i, sp.data...)
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
	case opAssert, opAssertNot,
		opGroupref, opGrouprefExists,
		opAtomicGroup,
		opPossessiveRepeat,
		opFailure:
		return true
	case opAt:
		p := t.params.(paramAt)

		if atcode(p) == atEndString {
			return true
		}
	case opBranch:
		p := t.params.(*paramSubPatterns)

		for _, item := range p.items {
			if item.isUnsupported() {
				return true
			}
		}
	case opMinRepeat, opMaxRepeat:
		p := t.params.(*paramRepeat)

		return p.item.isUnsupported()
	case opSubpattern:
		p := t.params.(*paramSubPattern)

		return p.p.isUnsupported()
	}

	return false
}

func (p *subPattern) dump(print func(s string)) {
	var b strings.Builder
	p.dumpPattern(&b, 0)

	if print == nil {
		fmt.Print(b.String())
	} else {
		print(b.String())
	}
}

func (p *subPattern) dumpPattern(b *strings.Builder, level int) {
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

				sp.dumpPattern(b, level+1)
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
	case opAssert, opAssertNot:
		pv := v.params.(*paramAssert)
		printParams(pv.dir, pv.p)
	case opAny:
		printParams("None")
	case opAt:
		pv := v.params.(paramAt)
		printParams(atcode(pv).String())
	case opBranch:
		print()
		pv := v.params.(*paramSubPatterns)
		for i, a := range pv.items {
			if i != 0 {
				print(strings.Repeat("  ", level) + "OR")
			}
			a.dumpPattern(b, level+1)
		}
	case opCategory: // tokens of type "CATEGORY" always appear in "IN" tokens
	case opGroupref:
		pv := v.params.(paramInt)
		printParams(pv)
	case opGrouprefExists:
		pv := v.params.(*paramGrouprefEx)
		print("", pv.condgroup)

		pv.itemYes.dumpPattern(b, level+1)
		if pv.itemNo != nil {
			print(strings.Repeat("  ", level) + "ELSE")
			pv.itemNo.dumpPattern(b, level+1)
		}
	case opIn:
		// member sublanguage
		print()

		// members in items are either of type LITERAL, RANGE or CATEGORY
		for _, v := range v.items {
			printr(strings.Repeat("  ", level+1) + v.opcode.String() + " ")
			switch v.opcode {
			case opLiteral:
				print(v.c)
			case opRange:
				vp := v.params.(*paramRange)
				print(fmt.Sprintf("(%d, %d)", vp.lo, vp.hi))
			case opCategory:
				vp := v.params.(paramCategory)
				print(catcode(vp).String())
			}
		}
	case opLiteral, opNotLiteral:
		print("", v.c)
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		pv := v.params.(*paramRepeat)
		var maxval string
		if pv.max != maxRepeat {
			maxval = strconv.Itoa(pv.max)
		} else {
			maxval = "MAXREPEAT"
		}
		printParams(pv.min, maxval, pv.item)
	case opRange:
		pv := v.params.(*paramRange)
		printParams(pv.lo, pv.hi)
	case opSubpattern:
		pv := v.params.(*paramSubPattern)
		var group string
		if pv.group >= 0 {
			group = strconv.Itoa(pv.group)
		} else {
			group = "None"
		}
		printParams(group, pv.addFlags, pv.delFlags, pv.p)
	case opAtomicGroup:
		pv := v.params.(*paramSubPatterns)
		print()
		pv.items[0].dumpPattern(b, level+1)
	case opFailure:
		print()
	}
}

func (p *subPattern) string(isStr bool, replace func(w *subPatternWriter, t *token, ctx *subPatternContext) bool) string {
	w := subPatternWriter{
		isStr:   isStr,
		replace: replace,
	}

	w.writePattern(p, nil)
	return w.String()
}

type subPatternWriter struct {
	strings.Builder
	isStr   bool
	replace func(w *subPatternWriter, t *token, ctx *subPatternContext) bool
}

type subPatternContext struct {
	hasSiblings bool
	inSet       bool
	group       *paramSubPattern
}

func (w *subPatternWriter) writePattern(p *subPattern, parent *paramSubPattern) {
	ctx := subPatternContext{
		hasSiblings: len(p.data) > 1,
		inSet:       false,
		group:       parent,
	}

	for _, item := range p.data {
		w.writeToken(item, &ctx)
	}
}

// Flags supported by the Go regex library.
const supportedFlags = FlagIgnoreCase | FlagMultiline | FlagDotAll

// `inSet` can only be true, of called from `IN` token.
func (w *subPatternWriter) writeToken(t *token, ctx *subPatternContext) {
	if w.replace(w, t, ctx) {
		return
	}

	switch t.opcode {
	case opAny:
		w.WriteByte('.')
	case opAssert, opAssertNot:
		p := t.params.(*paramAssert)

		w.WriteString("(?")

		if p.dir < 0 {
			w.WriteByte('<')
			if t.opcode == opAssert {
				w.WriteByte('=')
			} else {
				w.WriteByte('!')
			}
		} else if t.opcode == opAssert {
			w.WriteByte('=')
		} else {
			w.WriteByte('!')
		}

		w.writePattern(p.p, ctx.group)
		w.WriteByte(')')
	case opAt:
		p := t.params.(paramAt)

		switch atcode(p) {
		case atBeginning:
			w.WriteByte('^')
		case atBeginningString:
			w.WriteString(`\A`)
		case atBoundary:
			w.WriteString(`\b`)
		case atNonBoundary:
			w.WriteString(`\B`)
		case atEnd:
			w.WriteByte('$')
		case atEndString:
			w.WriteString(`\Z`)
		}
	case opBranch:
		p := t.params.(*paramSubPatterns)

		// Always wrap branches branches inside of an non-capture group, if the current
		// subpattern contains other token, than this branch token.
		if ctx.hasSiblings {
			w.WriteString("(?:")
		}
		for i, item := range p.items {
			if i > 0 {
				w.WriteByte('|')
			}
			w.writePattern(item, ctx.group)
		}
		if ctx.hasSiblings {
			w.WriteByte(')')
		}
	case opCategory:
		// Always inside of character sets.
		p := t.params.(paramCategory)

		switch catcode(p) {
		case categoryDigit:
			w.WriteString(`\d`)
		case categoryNotDigit:
			w.WriteString(`\D`)
		case categorySpace:
			w.WriteString(`\s`)
		case categoryNotSpace:
			w.WriteString(`\S`)
		case categoryWord:
			w.WriteString(`\w`)
		case categoryNotWord:
			w.WriteString(`\W`)
		}
	case opGroupref:
		p := t.params.(paramInt)

		w.WriteString(`\`)
		w.writeInt(int(p))
	case opGrouprefExists:
		p := t.params.(*paramGrouprefEx)

		w.WriteString("(?(")
		w.writeInt(p.condgroup)
		w.WriteByte(')')
		w.writePattern(p.itemYes, ctx.group)
		if p.itemNo != nil {
			w.WriteByte('|')
			w.writePattern(p.itemNo, ctx.group)
		}
		w.WriteByte(')')
	case opIn:
		// Members in items are either of type LITERAL, RANGE or CATEGORY.
		// IN tokens are always written as sets, because it is unknown, how the replacer function
		// rewrites elements inside of the set.

		newCtx := subPatternContext{
			hasSiblings: len(t.items) > 1,
			inSet:       true,
			group:       ctx.group,
		}

		w.WriteByte('[')
		for _, v := range t.items {
			w.writeToken(v, &newCtx)
		}
		w.WriteByte(']')
	case opLiteral:
		w.writeLiteral(t.c)
	case opNotLiteral:
		w.WriteString("[^")
		w.writeLiteral(t.c)
		w.WriteByte(']')
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		p := t.params.(*paramRepeat)

		needsGroup := false
		if p.item.len() > 1 {
			needsGroup = true
		} else {
			switch p.item.get(0).opcode {
			case opBranch, opMinRepeat, opMaxRepeat, opPossessiveRepeat:
				needsGroup = true
			}
		}

		if !needsGroup {
			w.writePattern(p.item, ctx.group)
		} else {
			w.WriteString("(?:")
			w.writePattern(p.item, ctx.group)
			w.WriteByte(')')
		}

		if p.min == 0 && p.max == 1 {
			w.WriteByte('?')
		} else if p.min == 0 && p.max == maxRepeat {
			w.WriteByte('*')
		} else if p.min == 1 && p.max == maxRepeat {
			w.WriteByte('+')
		} else {
			w.WriteByte('{')
			w.writeInt(p.min)
			w.WriteByte(',')
			if p.max != maxRepeat {
				w.writeInt(p.max)
			}
			w.WriteByte('}')
		}

		switch t.opcode {
		case opMinRepeat:
			w.WriteByte('?')
		case opPossessiveRepeat:
			w.WriteByte('+')
		}
	case opNegate:
		w.WriteByte('^')
	case opRange:
		p := t.params.(*paramRange)

		w.writeLiteral(p.lo)
		w.WriteByte('-')
		w.writeLiteral(p.hi)
	case opSubpattern:
		p := t.params.(*paramSubPattern)

		w.WriteByte('(')

		if p.group >= 0 {
			groupName := groupname(p.p, p.group)
			if groupName != "" {
				w.WriteString("?P<")
				w.WriteString(groupName)
				w.WriteByte('>')
			}
		} else {
			addFlags := p.addFlags & supportedFlags
			delFlags := p.delFlags & supportedFlags

			if addFlags != 0 || delFlags != 0 {
				// Flags can only appear, when no group name exists

				w.WriteByte('?')
				if addFlags != 0 {
					w.writeFlags(addFlags)
				}
				if delFlags != 0 {
					w.WriteByte('-')
					w.writeFlags(delFlags)
				}

				if p.p.len() > 0 {
					w.WriteByte(':')
				}
			}
		}

		if p.p.len() > 0 {
			w.writePattern(p.p, p)
		}

		w.WriteByte(')')
	case opAtomicGroup:
		p := t.params.(*paramSubPatterns)

		w.WriteString("(?>")
		w.writePattern(p.items[0], ctx.group)
		w.WriteByte(')')
	case opFailure:
		w.WriteString("(?!)")
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

func (w *subPatternWriter) writeFlags(flags uint32) {
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

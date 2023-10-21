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
	data  []*regexNode
}

func newSubpattern(state *state) *subPattern {
	return &subPattern{
		state: state,
	}
}

func (p *subPattern) append(t *regexNode) {
	p.data = append(p.data, t)
}

func (p *subPattern) len() int {
	return len(p.data)
}

func (p *subPattern) get(i int) *regexNode {
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

func (p *subPattern) set(i int, t *regexNode) {
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

func isUnsupported(t *regexNode) bool {
	switch t.opcode {
	case opAssert, opAssertNot,
		opGroupref, opGrouprefExists,
		opAtomicGroup,
		opPossessiveRepeat,
		opFailure:
		return true
	case opAt:
		c := t.params.(atcode)

		if c == atEndString {
			return true
		}
	case opBranch:
		items := t.params.([]*subPattern)

		for _, item := range items {
			if item.isUnsupported() {
				return true
			}
		}
	case opMinRepeat, opMaxRepeat:
		p := t.params.(repeatParams)

		return p.item.isUnsupported()
	case opSubpattern:
		p := t.params.(subPatternParam)

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
		dumpNode(v, b, level)
	}
}

func dumpNode(t *regexNode, b *strings.Builder, level int) {
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

	switch t.opcode {
	case opAssert, opAssertNot:
		p := t.params.(assertParams)
		printParams(p.dir, p.p)
	case opAny:
		printParams("None")
	case opAt:
		at := t.params.(atcode)
		printParams(at.String())
	case opBranch:
		items := t.params.([]*subPattern)

		print()
		for i, a := range items {
			if i != 0 {
				print(strings.Repeat("  ", level) + "OR")
			}
			a.dumpPattern(b, level+1)
		}
	case opCategory: // nodes of type "CATEGORY" always appear in "IN" nodes
	case opGroupref:
		group := t.params.(int)
		printParams(group)
	case opGrouprefExists:
		p := t.params.(grouprefExParam)
		print("", p.condgroup)

		p.itemYes.dumpPattern(b, level+1)
		if p.itemNo != nil {
			print(strings.Repeat("  ", level) + "ELSE")
			p.itemNo.dumpPattern(b, level+1)
		}
	case opIn:
		items := t.params.([]*regexNode)
		print()

		// members in items are either of type LITERAL, RANGE or CATEGORY
		for _, v := range items {
			printr(strings.Repeat("  ", level+1) + v.opcode.String() + " ")
			switch v.opcode {
			case opLiteral:
				print(v.c)
			case opRange:
				p := v.params.(rangeParams)
				print(fmt.Sprintf("(%d, %d)", p.lo, p.hi))
			case opCategory:
				p := v.params.(catcode)
				print(p.String())
			}
		}
	case opLiteral, opNotLiteral:
		print("", t.c)
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		p := t.params.(repeatParams)

		var maxval string
		if p.max != maxRepeat {
			maxval = strconv.Itoa(p.max)
		} else {
			maxval = "MAXREPEAT"
		}
		printParams(p.min, maxval, p.item)
	case opRange:
		p := t.params.(rangeParams)
		printParams(p.lo, p.hi)
	case opSubpattern:
		p := t.params.(subPatternParam)

		var group string
		if p.group >= 0 {
			group = strconv.Itoa(p.group)
		} else {
			group = "None"
		}
		printParams(group, p.addFlags, p.delFlags, p.p)
	case opAtomicGroup:
		p := t.params.(*subPattern)

		print()
		p.dumpPattern(b, level+1)
	case opFailure:
		print()
	}
}

func (p *subPattern) string(isStr bool, replace func(w *subPatternWriter, t *regexNode, ctx *subPatternContext) bool) string {
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
	replace func(w *subPatternWriter, t *regexNode, ctx *subPatternContext) bool
}

type subPatternContext struct {
	hasSiblings bool
	inSet       bool
	group       *subPatternParam
}

func (w *subPatternWriter) writePattern(p *subPattern, parent *subPatternParam) {
	ctx := subPatternContext{
		hasSiblings: len(p.data) > 1,
		inSet:       false,
		group:       parent,
	}

	for _, item := range p.data {
		w.writeNode(item, &ctx)
	}
}

// `inSet` can only be true, of called from `IN` node.
func (w *subPatternWriter) writeNode(t *regexNode, ctx *subPatternContext) {
	if w.replace(w, t, ctx) {
		return
	}

	switch t.opcode {
	case opAny:
		w.WriteByte('.')
	case opAssert, opAssertNot:
		p := t.params.(assertParams)

		w.WriteString("(?")

		if p.dir < 0 {
			w.WriteByte('<')
		}
		if t.opcode == opAssert {
			w.WriteByte('=')
		} else {
			w.WriteByte('!')
		}

		w.writePattern(p.p, ctx.group)
		w.WriteByte(')')
	case opAt:
		at := t.params.(atcode)

		switch at {
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
		items := t.params.([]*subPattern)

		// Always wrap branches branches inside of an non-capture group, if the current
		// subpattern contains other node, than this branch node.
		if ctx.hasSiblings {
			w.WriteString("(?:")
		}
		for i, item := range items {
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
		category := t.params.(catcode)

		switch category {
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
		group := t.params.(int)

		w.WriteString(`\`)
		w.writeInt(group)
	case opGrouprefExists:
		p := t.params.(grouprefExParam)

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
		// IN nodes are always written as sets, because it is unknown, how the replacer function
		// rewrites elements inside of the set.

		items := t.params.([]*regexNode)

		newCtx := subPatternContext{
			hasSiblings: len(items) > 1,
			inSet:       true,
			group:       ctx.group,
		}

		w.WriteByte('[')
		for _, v := range items {
			w.writeNode(v, &newCtx)
		}
		w.WriteByte(']')
	case opLiteral:
		w.writeLiteral(t.c)
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		p := t.params.(repeatParams)

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
	case opNotLiteral:
		w.WriteString("[^")
		w.writeLiteral(t.c)
		w.WriteByte(']')
	case opRange:
		p := t.params.(rangeParams)

		w.writeLiteral(p.lo)
		w.WriteByte('-')
		w.writeLiteral(p.hi)
	case opSubpattern:
		p := t.params.(subPatternParam)

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
			w.writePattern(p.p, &p)
		}

		w.WriteByte(')')
	case opAtomicGroup:
		p := t.params.(*subPattern)

		w.WriteString("(?>")
		w.writePattern(p, ctx.group)
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

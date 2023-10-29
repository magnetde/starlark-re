package regex

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// subPattern is a type, that represents a regex subpattern.
// It contains a list of regex nodes that represents a sequence of subpatterns.
type subPattern struct {
	state *state
	data  []*regexNode
}

// newSubpattern creates a new empty subpattern.
func newSubpattern(state *state) *subPattern {
	return &subPattern{
		state: state,
	}
}

// append adds a new regex node to the sequence of regex nodes of this subpattern.
func (p *subPattern) append(n *regexNode) {
	p.data = append(p.data, n)
}

// len returns the number of regex nodes of this subpattern.
func (p *subPattern) len() int {
	return len(p.data)
}

// get returns the i-th regex node of this subpattern.
func (p *subPattern) get(i int) *regexNode {
	i = p.index(i)
	return p.data[i]
}

// index returns `i` if it is a non-negative index, otherwise it will return `p.len() + i`.
// This allows, to pass negative indices.
func (p *subPattern) index(i int) int {
	if i < 0 {
		i += len(p.data)
	}
	return i
}

// del deletes the i-th item of this subpattern.
func (p *subPattern) del(i int) {
	i = p.index(i)
	p.data = slices.Delete(p.data, i, i+1)
}

// set sets the i-th item of this subpattern to `t`.
func (p *subPattern) set(i int, n *regexNode) {
	i = p.index(i)
	p.data[i] = n
}

// replace replaces the i-th item of this subpattern with all nodes of subpattern `sp`.
func (p *subPattern) replace(i int, sp *subPattern) {
	i = p.index(i)
	p.data = slices.Replace(p.data, i, i+1, sp.data...)
}

// isUnsupported returns, whether the subpattern is not supported by the regexp engine `regex.Regexp`.
func (p *subPattern) isUnsupported() bool {
	for _, item := range p.data {
		if isUnsupported(item) {
			return true
		}
	}

	return false
}

// isUnsupported returns, whether the regex node is not supported by the regexp engine `regex.Regexp`.
// Currently, the following regex node types are not supported:
// ASSERT, ASSERT_NOT, GROUPREF, GROUPREF_EXISTS, ATOMIC_GROUP, POSSESSIVE_REPEAT, FAILURE and AT with the AT_END_STRING position.
// If the regex node is a repetion with the format `{m,n}` and the minimum and maximum repetion counts exceed the value `maxRepeatEngine`,
// then the regex node is also not supported.
func isUnsupported(n *regexNode) bool {
	switch n.opcode {
	case opAssert, opAssertNot,
		opGroupref, opGrouprefExists,
		opAtomicGroup,
		opPossessiveRepeat,
		opFailure:
		return true
	case opAt:
		c := n.params.(atcode)

		if c == atEndString {
			return true
		}
	case opBranch:
		items := n.params.([]*subPattern)

		for _, item := range items {
			if item.isUnsupported() {
				return true
			}
		}
	case opMinRepeat, opMaxRepeat:
		p := n.params.(repeatParams)

		if p.min > 1 || p.max < maxRepeat {
			// the repetition has the format `{m,n}`
			if p.min > maxRepeatEngine || p.max > maxRepeatEngine {
				return true
			}
		}

		return p.item.isUnsupported()
	case opSubpattern:
		p := n.params.(subPatternParam)

		return p.p.isUnsupported()
	}

	return false
}

// dump returns debug information about the compiled expression.
func (p *subPattern) dump() string {
	var b strings.Builder
	p.dumpPattern(&b, 0)
	return strings.TrimRight(b.String(), "\n ") // trim newlines and spaces at the end
}

// dumpPattern writes the debug information of this subpattern to the string builder.
// The `level` parameter is used to indent the debug information.
func (p *subPattern) dumpPattern(b *strings.Builder, level int) {
	for _, v := range p.data {
		op := v.opcode

		b.WriteString(strings.Repeat("  ", level))
		b.WriteString(op.String())
		dumpNode(b, v, level)
	}
}

// dumpNode writes the debug information of the parameters, excluding the opcode for node 't', to the string builder.
func dumpNode(b *strings.Builder, n *regexNode, level int) {
	write := func(v ...any) {
		for i, a := range v {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(fmt.Sprint(a))
		}
	}

	writeln := func(v ...any) {
		write(v...)
		b.WriteByte('\n')
	}

	writeParams := func(av ...any) {
		nl := false
		for _, a := range av {
			if sp, ok := a.(*subPattern); ok {
				if !nl {
					writeln()
				}

				sp.dumpPattern(b, level+1)
				nl = true
			} else {
				if !nl {
					write(" ")
				}
				write(a)
				nl = false
			}
		}
		if !nl {
			writeln()
		}
	}

	switch n.opcode {
	case opAssert, opAssertNot:
		p := n.params.(assertParams)
		writeParams(p.dir, p.p)
	case opAny:
		writeParams("None")
	case opAt:
		at := n.params.(atcode)
		writeParams(at.String())
	case opBranch:
		items := n.params.([]*subPattern)

		writeln()
		for i, a := range items {
			if i != 0 {
				writeln(strings.Repeat("  ", level) + "OR")
			}
			a.dumpPattern(b, level+1)
		}
	case opCategory: // nodes of type "CATEGORY" always appear in "IN" nodes
	case opGroupref:
		group := n.params.(int)
		writeParams(group)
	case opGrouprefExists:
		p := n.params.(grouprefExParam)
		writeln("", p.condgroup)

		p.itemYes.dumpPattern(b, level+1)
		if p.itemNo != nil {
			writeln(strings.Repeat("  ", level) + "ELSE")
			p.itemNo.dumpPattern(b, level+1)
		}
	case opIn:
		items := n.params.([]*regexNode)
		writeln()

		// members in items are either of type LITERAL, RANGE or CATEGORY
		for _, v := range items {
			write(strings.Repeat("  ", level+1) + v.opcode.String() + " ")
			switch v.opcode {
			case opLiteral:
				writeln(v.c)
			case opRange:
				p := v.params.(rangeParams)
				writeln(fmt.Sprintf("(%d, %d)", p.lo, p.hi))
			case opCategory:
				p := v.params.(catcode)
				writeln(p.String())
			}
		}
	case opLiteral, opNotLiteral:
		writeln("", n.c)
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		p := n.params.(repeatParams)

		var maxval string
		if p.max != maxRepeat {
			maxval = strconv.Itoa(p.max)
		} else {
			maxval = "MAXREPEAT"
		}
		writeParams(p.min, maxval, p.item)
	case opRange:
		p := n.params.(rangeParams)
		writeParams(p.lo, p.hi)
	case opSubpattern:
		p := n.params.(subPatternParam)

		var group string
		if p.group >= 0 {
			group = strconv.Itoa(p.group)
		} else {
			group = "None"
		}
		writeParams(group, p.addFlags, p.delFlags, p.p)
	case opAtomicGroup:
		p := n.params.(*subPattern)

		writeln()
		p.dumpPattern(b, level+1)
	case opFailure:
		writeln()
	}
}

// toString converts a subpattern into a regex pattern string that is compatible with the Python regex engine.
// The replace function is used to change subelements of the pattern and cannot be null.
// If the replace function rewrites a subelement, it must return true and the subelement is then ignored by this function.
// The parameter `w` of the replace function is used to build the pattern string.
// The current regex node is represented by the parameter 't'. 'ctx' is the context of the node 't'.
func (p *subPattern) toString(isStr bool, replace func(w *subPatternWriter, n *regexNode, ctx *subPatternContext) bool) string {
	w := subPatternWriter{
		isStr:   isStr,
		replace: replace,
	}

	w.writePattern(p, nil)
	return w.String()
}

// subPatternWriter is a type to write the regex pattern.
// It uses a `strings.Builder` and provides functions to write subpatterns, regex nodes, integers, and literals.
type subPatternWriter struct {
	strings.Builder
	isStr   bool
	replace func(w *subPatternWriter, n *regexNode, ctx *subPatternContext) bool
}

// subPatternContext describes the content of the current regex node.
// The field `hasSiblings` indicates whether the current regex node has other siblings.
// This may be necessary to rewrite the negated character class \D for unicode digits differently
// (see `defaultReplacer()` in preprocessor.go).
// The field `inSet` describes whether the current regex node is in a character set (a node of type IN).
// This may be necessary to handle literals inside and outside of sets differently.
// The last field `group` is the group of the current regex node, making it possible to handle regex nodes
// differently based on the flags of the current group. The group may be nil.
type subPatternContext struct {
	hasSiblings bool
	inSet       bool
	group       *subPatternParam
}

// writePattern writes the subpattern to the subpattern writer.
// The `parent` parameter specifies the current group of the subpattern.
// The parent changes with each regex node of type `SUBPATTERN`.
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

// writeNodes writes the regex node to the subpattern writer.
// Initially, the regex node is passed to the replacer function.
// If the replacer function did not rewrote the regular expression node,
// this function transforms the regex node into a regular expression string.
func (w *subPatternWriter) writeNode(n *regexNode, ctx *subPatternContext) {
	if w.replace(w, n, ctx) {
		return
	}

	switch n.opcode {
	case opAny:
		w.WriteByte('.')
	case opAssert, opAssertNot:
		p := n.params.(assertParams)

		w.WriteString("(?")

		if p.dir < 0 {
			w.WriteByte('<')
		}
		if n.opcode == opAssert {
			w.WriteByte('=')
		} else {
			w.WriteByte('!')
		}

		w.writePattern(p.p, ctx.group)
		w.WriteByte(')')
	case opAt:
		at := n.params.(atcode)

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
		items := n.params.([]*subPattern)

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
		category := n.params.(catcode)

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
		group := n.params.(int)

		w.WriteString(`\`)
		w.writeInt(group)
	case opGrouprefExists:
		p := n.params.(grouprefExParam)

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

		items := n.params.([]*regexNode)

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
		w.writeLiteral(n.c)
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		p := n.params.(repeatParams)

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

		switch n.opcode {
		case opMinRepeat:
			w.WriteByte('?')
		case opPossessiveRepeat:
			w.WriteByte('+')
		}
	case opNegate:
		w.WriteByte('^')
	case opNotLiteral:
		w.WriteString("[^")
		w.writeLiteral(n.c)
		w.WriteByte(']')
	case opRange:
		p := n.params.(rangeParams)

		w.writeLiteral(p.lo)
		w.WriteByte('-')
		w.writeLiteral(p.hi)
	case opSubpattern:
		p := n.params.(subPatternParam)

		w.WriteByte('(')

		if p.group >= 0 {
			groupName := groupName(p.p, p.group)
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
		p := n.params.(*subPattern)

		w.WriteString("(?>")
		w.writePattern(p, ctx.group)
		w.WriteByte(')')
	case opFailure:
		w.WriteString("(?!)")
	}
}

// groupName returns the name of the group at the given index or an empty string if the group is unnamed.
func groupName(p *subPattern, gid int) string {
	for name, g := range p.state.groupdict {
		if gid == g {
			return name
		}
	}

	return ""
}

// writeInt writes the integer to the subpattern writer.
func (w *subPatternWriter) writeInt(i int) {
	w.WriteString(strconv.Itoa(i))
}

// writeLiteral writes the literal to the subpattern writer.
// The value is always appended in hexadecimal format, as it is clear and unambiguous
// without introducing any scoping errors in the parser of the regex engine.
// It will have either the format "\x.." or "\x{...}".
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
		l = 2 * min(l, 2) // 2 chars per byte

		w.WriteByte('{')
		if len(s) < l {
			w.WriteString(strings.Repeat("0", l-len(s)))
		}
		w.WriteString(s)
		w.WriteByte('}')
	}
}

// writeFlags writes the regex flags to the subpattern writer.
// They are ordered as follows: "aiLmsux".
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

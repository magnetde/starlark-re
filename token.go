package re

import (
	"fmt"
	"slices"
	"strings"
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
		printParams(pv)
	case BRANCH:
		print()
		pv := v.params.(*paramSubPatterns)
		for i, a := range pv.p {
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
		for _, v := range v.items {
			op := v.opcode
			printr(strings.Repeat("  ", level+1) + op.String() + " (")
			dumpToken(v, b, level)
			print(")")
		}
	case LITERAL, NOT_LITERAL:
		print("", "'"+string(v.c)+"'")
	case MIN_REPEAT, MAX_REPEAT, POSSESSIVE_REPEAT:
		pv := v.params.(*paramRepeat)
		printParams(pv.min, pv.max, pv.item)
	case RANGE:
		pv := v.params.(*paramRange)
		printParams(pv.min, pv.max)
	case SUBPATTERN:
		pv := v.params.(*paramSubPattern)
		printParams(pv.group, pv.addFlags, pv.delFlags, pv.p)
	case ATOMIC_GROUP:
		pv := v.params.(*paramSubPatterns)
		print()
		pv.p[0].dumpr(b, level+1)
	}
}

func (p *subPattern) string(replacer func(t *token) (string, bool)) string {
	return ""
}

// use interfaces because unions do not exists
type token struct {
	opcode opcode

	// common used params
	c     rune
	items []*token

	// more specific parameter
	params params
}

type params interface {
	equals(o params) bool
}

func (t *token) equals(o *token) bool {
	if t.opcode != o.opcode {
		return false
	}
	if t.c != o.c {
		return false
	}

	eq := slices.EqualFunc(t.items, o.items, func(i1, i2 *token) bool {
		return i1.equals(i2)
	})
	if !eq {
		return false
	}

	if t.params == nil {
		return o.params == nil
	}

	return t.params.equals(o.params)
}

func (t *token) string() string {
	return ""
}

type (
	paramInt      int
	paramAt       atCode
	paramCategory chCode

	paramAssert struct {
		dir int
		p   *subPattern
	}

	paramSubPatterns struct {
		p []*subPattern
	}

	paramGrouprefEx struct {
		condgroup int
		itemYes   *subPattern
		itemNo    *subPattern
	}

	paramRange struct {
		min rune
		max rune
	}

	paramRepeat struct {
		min  int
		max  int
		item *subPattern
	}

	paramSubPattern struct {
		group    int
		addFlags int
		delFlags int
		p        *subPattern
	}
)

var (
	_ params = (*paramInt)(nil)
	_ params = (*paramAt)(nil)
	_ params = (*paramCategory)(nil)
	_ params = (*paramAssert)(nil)
	_ params = (*paramSubPatterns)(nil)
	_ params = (*paramGrouprefEx)(nil)
	_ params = (*paramRange)(nil)
	_ params = (*paramRepeat)(nil)
	_ params = (*paramSubPattern)(nil)
)

func (p paramInt) equals(o params) bool {
	if pp, ok := o.(paramInt); ok {
		return p == pp
	} else {
		return false
	}
}

func (p paramAt) equals(o params) bool {
	if pp, ok := o.(paramAt); ok {
		return p == pp
	} else {
		return false
	}
}

func (p paramCategory) equals(o params) bool {
	if pp, ok := o.(paramCategory); ok {
		return p == pp
	} else {
		return false
	}
}

func (p *paramAssert) equals(o params) bool {
	if pp, ok := o.(*paramAssert); ok {
		return p.dir == pp.dir && p.p == pp.p // only compare the pointers of subpatterns
	} else {
		return false
	}
}

func (p *paramSubPatterns) equals(o params) bool {
	if pp, ok := o.(*paramSubPatterns); ok {
		return slices.Equal(p.p, pp.p)
	} else {
		return false
	}
}

func (p *paramGrouprefEx) equals(o params) bool {
	if pp, ok := o.(*paramGrouprefEx); ok {
		return p.condgroup == pp.condgroup && p.itemYes == pp.itemYes && p.itemNo == pp.itemNo
	} else {
		return false
	}
}

func (p *paramRange) equals(o params) bool {
	if pp, ok := o.(*paramRange); ok {
		return p.min == pp.min && p.max == pp.max
	} else {
		return false
	}
}

func (p *paramRepeat) equals(o params) bool {
	if pp, ok := o.(*paramRepeat); ok {
		return p.min == pp.min && p.max == pp.max && p.item == pp.item
	} else {
		return false
	}
}

func (p *paramSubPattern) equals(o params) bool {
	if pp, ok := o.(*paramSubPattern); ok {
		return p.group == pp.group && p.addFlags == pp.addFlags && p.delFlags == pp.delFlags && p.p == pp.p
	} else {
		return false
	}
}

func newEmptyToken(opcode opcode) *token {
	return &token{
		opcode: opcode,
		params: nil,
	}
}

// additional function, because it is called very often
func newLiteral(c rune) *token {
	return newCharToken(LITERAL, c)
}

func newCharToken(op opcode, c rune) *token {
	return &token{
		opcode: op,
		c:      c,
	}
}

func newAssertToken(op opcode, dir int, p *subPattern) *token {
	return &token{
		opcode: op,
		params: &paramAssert{
			dir: dir,
			p:   p,
		},
	}
}

func newAtToken(op opcode, at atCode) *token {
	return &token{
		opcode: op,
		params: paramAt(at),
	}
}

func newSubPatternsToken(op opcode, items []*subPattern) *token {
	return &token{
		opcode: op,
		params: &paramSubPatterns{
			p: items,
		},
	}
}

func newCategoryToken(op opcode, code chCode) *token {
	return &token{
		opcode: op,
		params: paramCategory(code),
	}
}

func newGrouprefToken(op opcode, ref int) *token {
	return &token{
		opcode: op,
		params: paramInt(ref),
	}
}

func newGrouprefExistsToken(op opcode, condgroup int, itemYes, itemNo *subPattern) *token {
	return &token{
		opcode: op,
		params: &paramGrouprefEx{
			condgroup: condgroup,
			itemYes:   itemYes,
			itemNo:    itemNo,
		},
	}
}

func newItemsToken(op opcode, items []*token) *token {
	return &token{
		opcode: op,
		items:  items,
	}
}

func newRange(op opcode, min, max rune) *token {
	return &token{
		opcode: op,
		params: &paramRange{
			min: min,
			max: max,
		},
	}
}

func newRepeat(op opcode, min, max int, item *subPattern) *token {
	return &token{
		opcode: op,
		params: &paramRepeat{
			min:  min,
			max:  max,
			item: item,
		},
	}
}

func newSubPattern(op opcode, group, addFlags int, delFlags int, p *subPattern) *token {
	return &token{
		opcode: op,
		params: &paramSubPattern{
			group:    group,
			addFlags: addFlags,
			delFlags: delFlags,
			p:        p,
		},
	}
}

func newWithSubPattern(op opcode, p *subPattern) *token {
	return &token{
		opcode: op,
		params: &paramSubPatterns{
			p: []*subPattern{p},
		},
	}
}

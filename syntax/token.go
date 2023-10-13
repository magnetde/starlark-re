package syntax

import "slices"

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

type (
	paramInt      int
	paramAt       atCode
	paramCategory chCode

	paramAssert struct {
		dir int
		p   *subPattern
	}

	paramSubPatterns struct {
		items []*subPattern
	}

	paramGrouprefEx struct {
		condgroup int
		itemYes   *subPattern
		itemNo    *subPattern
	}

	paramRange struct {
		lo rune
		hi rune
	}

	paramRepeat struct {
		min  int
		max  int
		item *subPattern
	}

	paramSubPattern struct {
		group    int // -1: no group
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
		return slices.Equal(p.items, pp.items)
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
		return p.lo == pp.lo && p.hi == pp.hi
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
			items: items,
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

func newRange(op opcode, lo, hi rune) *token {
	return &token{
		opcode: op,
		params: &paramRange{
			lo: lo,
			hi: hi,
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

func newSubPattern(op opcode, group, addFlags, delFlags int, p *subPattern) *token {
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
			items: []*subPattern{p},
		},
	}
}

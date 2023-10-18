package syntax

import "slices"

type token struct {
	opcode opcode
	c      rune // literals are the most used token so
	params any
}

type assertParams struct {
	dir int
	p   *subPattern
}

type grouprefExParam struct {
	condgroup int
	itemYes   *subPattern
	itemNo    *subPattern
}

type repeatParams struct {
	min  int
	max  int
	item *subPattern
}

type rangeParams struct {
	lo rune
	hi rune
}

type subPatternParam struct {
	group    int
	addFlags uint32
	delFlags uint32
	p        *subPattern
}

func (t *token) equals(o *token) bool {
	if t.opcode != o.opcode {
		return false
	}

	switch t.opcode {
	case opFailure:
		return true
	case opAny:
		return true
	case opAssert, opAssertNot:
		return t.params.(assertParams) == o.params.(assertParams)
	case opAt:
		return t.params.(atcode) == o.params.(atcode)
	case opBranch:
		p1 := t.params.([]*subPattern)
		p2 := o.params.([]*subPattern)
		return slices.Equal(p1, p2)
	case opCategory:
		return t.params.(catcode) == o.params.(catcode)
	case opGroupref:
		return t.params.(int) == o.params.(int)
	case opGrouprefExists:
		return t.params.(grouprefExParam) == o.params.(grouprefExParam)
	case opIn:
		p1 := t.params.([]*token)
		p2 := o.params.([]*token)
		return slices.EqualFunc(p1, p2, func(t1, t2 *token) bool {
			return t1.equals(t2)
		})
	case opLiteral, opNotLiteral:
		return t.c == o.c
	case opMinRepeat, opMaxRepeat, opPossessiveRepeat:
		return t.params.(repeatParams) == o.params.(repeatParams)
	case opNegate:
		return true
	case opRange:
		p1 := t.params.(rangeParams)
		p2 := o.params.(rangeParams)
		return p1.lo == p2.lo && p1.hi == p2.hi
	case opSubpattern:
		return t.params.(subPatternParam) == o.params.(subPatternParam)
	case opAtomicGroup:
		return t.params.(*subPattern) == o.params.(*subPattern)
	}

	return false
}

func newEmptyToken(op opcode) *token {
	return &token{
		opcode: op,
	}
}

// additional function, because it is called very often
func newLiteral(c rune) *token {
	return newCharToken(opLiteral, c)
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
		params: assertParams{
			dir: dir,
			p:   p,
		},
	}
}

func newAtToken(op opcode, at atcode) *token {
	return &token{
		opcode: op,
		params: at,
	}
}

func newSubPatternsToken(op opcode, items []*subPattern) *token {
	return &token{
		opcode: op,
		params: items,
	}
}

func newCategoryToken(op opcode, code catcode) *token {
	return &token{
		opcode: op,
		params: code,
	}
}

func newGrouprefToken(op opcode, ref int) *token {
	return &token{
		opcode: op,
		params: ref,
	}
}

func newGrouprefExistsToken(op opcode, condgroup int, itemYes, itemNo *subPattern) *token {
	return &token{
		opcode: op,
		params: grouprefExParam{
			condgroup: condgroup,
			itemYes:   itemYes,
			itemNo:    itemNo,
		},
	}
}

func newItemsToken(op opcode, items []*token) *token {
	return &token{
		opcode: op,
		params: items,
	}
}

func newRepeat(op opcode, min, max int, item *subPattern) *token {
	return &token{
		opcode: op,
		params: repeatParams{
			min:  min,
			max:  max,
			item: item,
		},
	}
}

func newRange(op opcode, lo, hi rune) *token {
	return &token{
		opcode: op,
		params: rangeParams{
			lo: lo,
			hi: hi,
		},
	}
}

func newSubPattern(op opcode, group int, addFlags, delFlags uint32, p *subPattern) *token {
	return &token{
		opcode: op,
		params: subPatternParam{
			group:    group,
			addFlags: addFlags,
			delFlags: delFlags,
			p:        p,
		},
	}
}

func newSubPatternAtomic(op opcode, p *subPattern) *token {
	return &token{
		opcode: op,
		params: p,
	}
}

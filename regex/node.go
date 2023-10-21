package regex

import "slices"

// regexNode represents a node in the parsed regex tree.
type regexNode struct {
	opcode opcode // regex operator
	c      rune   // literals are the most common node, so add an extra field for them
	params any    // additional parameters; may be nil
}

// Extra types, when more then one field exists in the extra parameters:

// assertParams represents the parameters for the "ASSERT" and "ASSERT_NOT" operators.
type assertParams struct {
	dir int
	p   *subPattern
}

// grouprefExParam represents the parameters for the "GROUPREF_EXISTS" operator.
type grouprefExParam struct {
	condgroup int
	itemYes   *subPattern
	itemNo    *subPattern
}

// repeatParams represents the parameters for the "REPEAT" operator.
type repeatParams struct {
	min  int
	max  int
	item *subPattern
}

// rangeParams represents the parameters for the "RANGE" operator.
type rangeParams struct {
	lo rune
	hi rune
}

// rangeParams represents the parameters for the "SUBPATTERN" operator.
type subPatternParam struct {
	group    int
	addFlags uint32
	delFlags uint32
	p        *subPattern
}

// equals checks, if two nodes are equal by checking its opcodes and its parameters.
func (t *regexNode) equals(o *regexNode) bool {
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
		p1 := t.params.([]*regexNode)
		p2 := o.params.([]*regexNode)
		return slices.EqualFunc(p1, p2, func(t1, t2 *regexNode) bool {
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

// newEmptyNode creates a new node with the specified opcode and no additional parameters.
// Valid operators are FAILURE, ANY and NEGATE.
func newEmptyNode(op opcode) *regexNode {
	return &regexNode{
		opcode: op,
	}
}

// newCharNode creates a new node, that holds an extra character.
// Valid operators are LITERAL and NOT_LITERAL.
func newCharNode(op opcode, c rune) *regexNode {
	return &regexNode{
		opcode: op,
		c:      c,
	}
}

// newLiteral creates a new node with operator "LITERAL".
// This function is additional, because LITERAL nodes are very often created.
func newLiteral(c rune) *regexNode {
	return newCharNode(opLiteral, c)
}

// newAssertNode creates a new node, that holds assert parameters.
// Valid operators are ASSERT and ASSERT_NOT.
func newAssertNode(op opcode, dir int, p *subPattern) *regexNode {
	return &regexNode{
		opcode: op,
		params: assertParams{
			dir: dir,
			p:   p,
		},
	}
}

// newAssertNode creates a new node, that holds an AT code.
// The only valid operator is AT.
func newAtNode(op opcode, at atcode) *regexNode {
	return &regexNode{
		opcode: op,
		params: at,
	}
}

// newSubPatternsNode creates a new node, that holds an slice of subpatterns.
// The only valid operator is BRANCH.
func newSubPatternsNode(op opcode, items []*subPattern) *regexNode {
	return &regexNode{
		opcode: op,
		params: items,
	}
}

// newCategoryNode creates a new node, that holds an CATEGORY code.
// The only valid operator is CATEGORY.
func newCategoryNode(op opcode, code catcode) *regexNode {
	return &regexNode{
		opcode: op,
		params: code,
	}
}

// newGrouprefNode creates a new node, that holds an group reference as an int value.
// The only valid operator is GROUPREF.
func newGrouprefNode(op opcode, ref int) *regexNode {
	return &regexNode{
		opcode: op,
		params: ref,
	}
}

// newGrouprefExistsNode creates a new node, that holds parameters of a conditional regex expression.
// The only valid operator is GROUPREF_EXISTS.
func newGrouprefExistsNode(op opcode, condgroup int, itemYes, itemNo *subPattern) *regexNode {
	return &regexNode{
		opcode: op,
		params: grouprefExParam{
			condgroup: condgroup,
			itemYes:   itemYes,
			itemNo:    itemNo,
		},
	}
}

// newGrouprefExistsNode creates a new node, that holds a slice of regex nodes.
// The only valid operator is IN.
func newItemsNode(op opcode, items []*regexNode) *regexNode {
	return &regexNode{
		opcode: op,
		params: items,
	}
}

// newRepeatNode creates a new node, that holds the parameters of a repetition.
// The only valid operator is REPEAT.
func newRepeatNode(op opcode, min, max int, item *subPattern) *regexNode {
	return &regexNode{
		opcode: op,
		params: repeatParams{
			min:  min,
			max:  max,
			item: item,
		},
	}
}

// newRangeNode creates a new node, that holds a character range.
// The only valid operator is RANGE.
func newRangeNode(op opcode, lo, hi rune) *regexNode {
	return &regexNode{
		opcode: op,
		params: rangeParams{
			lo: lo,
			hi: hi,
		},
	}
}

// newSubPatternNode creates a new node, that holds a subpattern.
// A subpattern node may have a group id, added and deleted flags an a subpattern.
// The only valid operator is SUBPATTERN.
func newSubPatternNode(op opcode, group int, addFlags, delFlags uint32, p *subPattern) *regexNode {
	return &regexNode{
		opcode: op,
		params: subPatternParam{
			group:    group,
			addFlags: addFlags,
			delFlags: delFlags,
			p:        p,
		},
	}
}

// newAtomicGroupNode creates a new node, that holds a atomic group.
// The only valid operator is ATOMIC_GROUP.
func newAtomicGroupNode(op opcode, p *subPattern) *regexNode {
	return &regexNode{
		opcode: op,
		params: p,
	}
}

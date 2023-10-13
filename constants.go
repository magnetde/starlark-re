package re

const (
	MAXREPEAT = 1000 // maximum repeat value of the regexp package
	MAXGROUPS = 128
)

type opcode uint32

const (
	// Skip zero opcode
	FAILURE opcode = iota

	ANY
	ASSERT
	ASSERT_NOT
	AT
	BRANCH
	CATEGORY
	GROUPREF
	GROUPREF_EXISTS
	IN
	LITERAL
	MIN_REPEAT
	MAX_REPEAT
	NOT_LITERAL
	NEGATE
	RANGE
	SUBPATTERN
	ATOMIC_GROUP
	POSSESSIVE_REPEAT
)

func (c opcode) String() string {
	switch c {
	case FAILURE:
		return "FAILURE"
	case ANY:
		return "ANY"
	case ASSERT:
		return "ASSERT"
	case ASSERT_NOT:
		return "ASSERT_NOT"
	case AT:
		return "AT"
	case BRANCH:
		return "BRANCH"
	case CATEGORY:
		return "CATEGORY"
	case GROUPREF:
		return "GROUPREF"
	case GROUPREF_EXISTS:
		return "GROUPREF_EXISTS"
	case IN:
		return "IN"
	case LITERAL:
		return "LITERAL"
	case MIN_REPEAT:
		return "MIN_REPEAT"
	case MAX_REPEAT:
		return "MAX_REPEAT"
	case NOT_LITERAL:
		return "NOT_LITERAL"
	case NEGATE:
		return "NEGATE"
	case RANGE:
		return "RANGE"
	case SUBPATTERN:
		return "SUBPATTERN"
	case ATOMIC_GROUP:
		return "ATOMIC_GROUP"
	case POSSESSIVE_REPEAT:
		return "POSSESSIVE_REPEAT"
	default:
		return ""
	}
}

type atCode uint32

const (
	AT_BEGINNING atCode = iota
	AT_BEGINNING_STRING
	AT_BOUNDARY
	AT_NON_BOUNDARY
	AT_END
	AT_END_STRING
)

type chCode uint32

const (
	CATEGORY_DIGIT chCode = iota
	CATEGORY_NOT_DIGIT
	CATEGORY_SPACE
	CATEGORY_NOT_SPACE
	CATEGORY_WORD
	CATEGORY_NOT_WORD
)

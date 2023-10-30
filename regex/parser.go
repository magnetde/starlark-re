package regex

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/magnetde/starlark-re/util"
)

const (
	typeFlags   = FlagASCII | FlagLocale | FlagUnicode // exclude flags in subpatterns
	globalFlags = FlagDebug                            // flags, that may only appear on global flags
)

// state represents the current parser state.
// It contains global flags, a mapping of group names to group indices, a list of open / closed groups,
// a number of valid look-behind groups, and a mapping of groups to their positions in the pattern.
type state struct {
	flags            uint32
	groupdict        map[string]int
	groupsclosed     []bool
	lookbehindgroups int
	grouprefpos      map[int]int
}

// init initializes the parser state.
func (s *state) init(flags uint32) {
	s.flags = flags
	s.groupdict = make(map[string]int)
	s.groupsclosed = []bool{false}
	s.lookbehindgroups = -1
	s.grouprefpos = make(map[int]int)
}

// group returns the current number of groups.
func (s *state) groups() int {
	return len(s.groupsclosed)
}

// openGroup opens a new group. If the group has no name, the name value may be empty.
// An error is returned if the group name already exists or if the number of groups exceeds the limit.
func (s *state) openGroup(name string) (int, error) {
	gid := s.groups()
	s.groupsclosed = append(s.groupsclosed, false)
	if s.groups() > maxGroups {
		return 0, errors.New("too many groups")
	}
	if name != "" {
		ogid, ok := s.groupdict[name]
		if ok {
			return 0, fmt.Errorf("redefinition of group name %s as group %d; was group %d", util.Repr(name, true), gid, ogid)
		}

		s.groupdict[name] = gid
	}

	return gid, nil
}

// closeGroup marks the specific group as closed.
func (s *state) closeGroup(gid int) {
	s.groupsclosed[gid] = true
}

// checkGroup returns true, if the specific group is both existing and closed.
func (s *state) checkGroup(gid int) bool {
	return gid < s.groups() && s.groupsclosed[gid]
}

// checkLookbehindGroup checks, if the specific group is valid for a lookbehind.
// If not, an error is returned.
func (s *state) checkLookbehindGroup(gid int, src *source) error {
	if s.lookbehindgroups != -1 {
		if !s.checkGroup(gid) {
			return src.errorh("cannot refer to an open group")
		}
		if gid >= s.lookbehindgroups {
			return src.errorh("cannot refer to group defined in the same lookbehind subpattern")
		}
	}

	return nil
}

// parse parses a regex pattern into a subpattern object.
// The parser is based on the parser used in the Python "re" module,
// with all errors corresponding to those of the Python parser.
func parse(str string, isStr bool, flags uint32) (*subPattern, error) {
	var s source
	s.init(str, isStr)

	var state state
	state.init(flags)

	p, err := parseSub(&s, &state, flags&FlagVerbose != 0, 0)
	if err != nil {
		return nil, err
	}

	f, err := checkFlags(p.state.flags, isStr)
	if err != nil {
		return nil, err
	}

	p.state.flags = f

	if _, ok := s.peek(); ok {
		return nil, s.errorh("unbalanced parenthesis")
	}

	for g := range p.state.grouprefpos {
		if g >= p.state.groups() {
			return nil, s.errorp(fmt.Sprintf("invalid group reference %d", g), p.state.grouprefpos[g])
		}
	}

	return p, nil
}

// checkFlags checks, if the global flags are valid.
func checkFlags(flags uint32, isStr bool) (uint32, error) {
	// check for incompatible flags
	if isStr {
		if flags&FlagLocale != 0 {
			return 0, errors.New("cannot use LOCALE flag with a str pattern")
		}
		if flags&FlagASCII == 0 {
			flags |= FlagUnicode
		} else if flags&FlagUnicode != 0 {
			return 0, errors.New("ASCII and UNICODE flags are incompatible")
		}
	} else {
		if flags&FlagUnicode != 0 {
			return 0, errors.New("cannot use UNICODE flag with a bytes pattern")
		}
		if flags&FlagLocale != 0 && flags&FlagASCII != 0 {
			return 0, errors.New("ASCII and LOCALE flags are incompatible")
		}
	}

	return flags, nil
}

// parseSub parses a regex alternation and is the primary parsing subroutine of "parse" to parse a regex string.
// If the alternation only contains one element, it is returned instead of a subpattern containing the alternation.
// Additionally, the alternation is simplified by extracting a common prefix and replacing subpatterns with character
// sets when possible.
// If the alternation contains multiple subpatterns, which are single literals or character classes, the alternation
// is converted to a character set.
func parseSub(s *source, state *state, verbose bool, nested int) (*subPattern, error) {
	// parse an alternation: a|b|c

	var items []*subPattern

	for {
		t, err := parseInternal(s, state, verbose, nested+1, nested == 0 && len(items) == 0)
		if err != nil {
			return nil, err
		}

		items = append(items, t)

		if !s.match('|') {
			break
		}

		if nested == 0 {
			verbose = state.flags&FlagVerbose != 0
		}
	}

	if len(items) == 1 {
		return items[0], nil
	}

	sp := newSubpattern(state)

	// check if all items share a common prefix
	for {
		var prefix *regexNode
		hasPrefix := true

		for _, item := range items {
			if item.len() == 0 {
				hasPrefix = false
				break
			}

			if prefix == nil {
				prefix = item.get(0)
			} else if !item.get(0).equals(prefix) {
				hasPrefix = false
				break
			}
		}

		if hasPrefix {
			// all subitems start with a common "prefix".
			// move it out of the branch
			for _, item := range items {
				item.del(0)
			}
			sp.append(prefix)
			continue // check next one
		}

		break
	}

	appendSet := true

	// check if the branch can be replaced by a character set
	var set []*regexNode
	for _, item := range items {
		if item.len() != 1 {
			appendSet = false
			break
		}
		t := item.get(0)
		op := t.opcode
		if op == opLiteral {
			set = append(set, t)
		} else if op == opIn && t.params.([]*regexNode)[0].opcode != opNegate {
			set = append(set, t.params.([]*regexNode)...)
		} else {
			appendSet = false
			break
		}
	}

	if appendSet {
		// we can store this as a character set instead of a
		// branch (the compiler may optimize this even more)
		sp.append(newItemsNode(opIn, unique(set)))
	} else {
		sp.append(newSubPatternsNode(opBranch, items))
	}

	return sp, nil
}

// parseInternal parses a subpattern.
// See the comment at the enum of opcodes for a list of possible subpatterns.
func parseInternal(s *source, state *state, verbose bool, nested int, first bool) (*subPattern, error) {
	// parse a simple pattern

	sp := newSubpattern(state)

	var err error
	for {
		c, ok := s.peek()
		if !ok {
			break // end of pattern
		}

		if c == '|' || c == ')' {
			break // end of subpattern
		}

		s.read()

		if verbose {
			// skip whitespace and comments
			if isWhitespace(c) {
				continue
			}
			if c == '#' {
				s.skipUntil('\n')
				continue
			}
		}

		switch c {
		default:
			sp.append(newLiteral(c))
		// ')', '|' already handled
		case '\\':
			code, err := parseEscape(s, state, false /* not class */)
			if err != nil {
				return nil, err
			}

			sp.append(code)

		case '[':
			here := s.tell() - 1

			// character set
			var set []*regexNode
			negate := s.match('^')

			// check remaining characters
			for {
				start := s.tell() // determine the current position; necessary for the err message

				c, ok = s.read()
				if !ok {
					return nil, s.errorp("unterminated character set", here)
				}

				var code1, code2 *regexNode

				if c == ']' && len(set) > 0 {
					break
				} else if c == '\\' {
					code1, err = parseEscape(s, state, true /* is class */)
					if err != nil {
						return nil, err
					}
				} else {
					code1 = newLiteral(c)
				}

				if s.match('-') {
					// potential range
					ch, ok := s.read()
					if !ok {
						return nil, s.errorp("unterminated character set", here)
					}

					if ch == ']' {
						if code1.opcode == opIn {
							items := code1.params.([]*regexNode)
							code1 = items[0]
						}

						set = append(set, code1, newLiteral('-'))
						break
					}

					if ch == '\\' {
						code2, err = parseEscape(s, state, true /* is class */)
						if err != nil {
							return nil, err
						}
					} else {
						code2 = newLiteral(ch)
					}

					if code1.opcode != opLiteral || code2.opcode != opLiteral {
						return nil, s.errorp(fmt.Sprintf("bad character range %s", s.orig[start:s.tell()]), start)
					}

					lo := code1.c
					hi := code2.c

					if hi < lo {
						return nil, s.errorp(fmt.Sprintf("bad character range %s", s.orig[start:s.tell()]), start)
					}

					set = append(set, newRangeNode(opRange, lo, hi))
				} else {
					if code1.opcode == opIn {
						items := code1.params.([]*regexNode)
						code1 = items[0]
					}

					set = append(set, code1)
				}
			}

			set = unique(set)

			if len(set) == 1 && set[0].opcode == opLiteral {
				// optimization
				if negate {
					sp.append(newCharNode(opNotLiteral, set[0].c))
				} else {
					sp.append(set[0])
				}
			} else {
				if negate {
					set = slices.Insert(set, 0, newEmptyNode(opNegate))
				}

				// charmap optimization can't be added here because
				// global flags still are not known
				sp.append(newItemsNode(opIn, set))
			}

		case '?', '*', '+', '{':
			// repeat previous item
			here := s.tell()

			var min, max int
			switch c {
			case '?':
				min, max = 0, 1
			case '*':
				min, max = 0, maxRepeat
			case '+':
				min, max = 1, maxRepeat
			case '{':
				if next, ok := s.peek(); ok && next == '}' {
					sp.append(newLiteral(c))
					continue
				}

				var lo, hi int // temporary values
				var hasLo, hasHi bool

				lo, hasLo, err = s.nextInt()
				if err != nil {
					return nil, err
				}

				if s.match(',') {
					hi, hasHi, err = s.nextInt()
					if err != nil {
						return nil, err
					}
				} else {
					hi = lo
					hasHi = hasLo
				}

				if !s.match('}') {
					sp.append(newLiteral(c))
					s.seek(here)
					continue
				}

				if hasLo {
					min = lo

					if min >= maxRepeat {
						return nil, errors.New("the repetition number is too large")
					}
				} else {
					min = 0
				}

				if hasHi {
					max = hi

					if max >= maxRepeat {
						return nil, errors.New("the repetition number is too large")
					}
					if max < min {
						return nil, s.errorp("min repeat greater than max repeat", here)
					}
				} else {
					max = maxRepeat
				}
			default: // cannot happen
			}

			// figure out which item to repeat
			var item *regexNode
			if sp.len() > 0 {
				item = sp.get(-1)
			}
			if item == nil || item.opcode == opAt {
				return nil, s.errorp("nothing to repeat", here-1)
			}
			if isRepeatCode(item.opcode) {
				return nil, s.errorp("multiple repeat", here-1)
			}

			var subitem *subPattern
			if item.opcode == opSubpattern {
				p := item.params.(subPatternParam)
				if p.group == -1 && p.addFlags == 0 && p.delFlags == 0 {
					subitem = p.p
				}
			}
			if subitem == nil {
				subitem = newSubpattern(state)
				subitem.append(item)
			}

			if s.match('?') {
				// Non-Greedy Match
				sp.set(-1, newRepeatNode(opMinRepeat, min, max, subitem))
			} else if s.match('+') {
				// Possessive Match (Always Greedy)
				sp.set(-1, newRepeatNode(opPossessiveRepeat, min, max, subitem))
			} else {
				// Greedy Match
				sp.set(-1, newRepeatNode(opMaxRepeat, min, max, subitem))
			}

		case '.':
			sp.append(newEmptyNode(opAny))

		case '(':
			start := s.tell() - 1

			capture := true
			atomic := false
			name := ""
			addFlags := uint32(0)
			delFlags := uint32(0)

			if s.match('?') {
				// options
				char, ok := s.read()
				if !ok {
					return nil, s.errorh("unexpected end of pattern")
				}

				switch char {
				case 'P':
					// python extensions
					if s.match('<') {
						// named group: skip forward to end of name
						name, err = s.getUntil('>', "group name")
						if err != nil {
							return nil, err
						}

						err = s.checkGroupName(name, 1)
						if err != nil {
							return nil, err
						}
					} else if s.match('=') {
						// named backreference
						name, err = s.getUntil(')', "group name")
						if err != nil {
							return nil, err
						}

						err = s.checkGroupName(name, 1)
						if err != nil {
							return nil, err
						}

						gid, ok := state.groupdict[name]
						if !ok {
							return nil, s.erroro(fmt.Sprintf("unknown group name %s", util.Repr(name, true)), len(name)+1)
						}
						if !state.checkGroup(gid) {
							return nil, s.erroro("cannot refer to an open group", len(name)+1)
						}

						err = state.checkLookbehindGroup(gid, s)
						if err != nil {
							return nil, err
						}

						sp.append(newGrouprefNode(opGroupref, gid))
						continue

					} else {
						char, ok = s.read()
						if !ok {
							return nil, s.errorh("unexpected end of pattern")
						}

						return nil, s.erroro(fmt.Sprintf("unknown extension ?P%c", char), s.clen(char)+2)
					}
				case ':':
					// non-capturing group
					capture = false
				case '#':
					// comment
					for {
						if _, ok = s.peek(); !ok {
							return nil, s.errorp("missing ), unterminated comment", start)
						}

						if ch, ok := s.read(); ok && ch == ')' {
							break
						}
					}

					continue

				case '=', '!', '<':
					// lookahead assertions
					dir := 1
					lookbehindgroups := -1

					if char == '<' {
						char, ok = s.read()
						if !ok {
							return nil, s.errorh("unexpected end of pattern")
						}
						if char != '=' && char != '!' {
							return nil, s.erroro(fmt.Sprintf("unknown extension ?<%c", char), s.clen(char)+2)
						}

						dir = -1 // lookbehind
						lookbehindgroups = state.lookbehindgroups
						if lookbehindgroups == -1 {
							state.lookbehindgroups = state.groups()
						}
					}

					p, err := parseSub(s, state, verbose, nested+1)
					if err != nil {
						return nil, err
					}

					if dir < 0 {
						if lookbehindgroups == -1 {
							state.lookbehindgroups = -1
						}
					}

					if !s.match(')') {
						return nil, s.errorp("missing ), unterminated subpattern", start)
					}

					if char == '=' {
						sp.append(newAssertNode(opAssert, dir, p))
					} else if p.len() > 0 {
						sp.append(newAssertNode(opAssertNot, dir, p))
					} else {
						sp.append(newEmptyNode(opFailure))
					}

					continue

				case '(':
					// conditional backreference group
					condname, err := s.getUntil(')', "group name")
					if err != nil {
						return nil, err
					}

					var condgroup int
					if ugroup, e := strconv.ParseUint(condname, 10, 32); e != nil {
						err = s.checkGroupName(condname, 1)
						if err != nil {
							return nil, err
						}

						condgroup, ok = state.groupdict[condname]
						if !ok {
							return nil, s.erroro(fmt.Sprintf("unknown group name %s", util.Repr(condname, true)), len(condname)+1)
						}
					} else {
						if ugroup == 0 {
							return nil, s.erroro("bad group number", len(condname)+1)
						}
						if ugroup >= maxGroups {
							return nil, s.erroro(fmt.Sprintf("invalid group reference %d", condgroup), len(condname)+1)
						}

						condgroup = int(ugroup)

						if _, ok = state.grouprefpos[condgroup]; !ok {
							state.grouprefpos[condgroup] = s.tell() - len(condname) - 1
						}
					}

					err = state.checkLookbehindGroup(condgroup, s)
					if err != nil {
						return nil, err
					}

					var itemYes, itemNo *subPattern

					itemYes, err = parseInternal(s, state, verbose, nested+1, false)
					if err != nil {
						return nil, err
					}

					if s.match('|') {
						itemNo, err = parseInternal(s, state, verbose, nested+1, false)
						if err != nil {
							return nil, err
						}

						if next, ok := s.peek(); ok && next == '|' {
							return nil, s.errorh("conditional backref with more than two branches")
						}
					}

					if !s.match(')') {
						return nil, s.errorp("missing ), unterminated subpattern", start)
					}

					sp.append(newGrouprefExistsNode(opGrouprefExists, condgroup, itemYes, itemNo))
					continue

				case '>':
					// non-capturing, atomic group
					capture = false
					atomic = true
				default:
					if isFlag(char) || char == '-' {
						// flags
						addFlags, delFlags, ok, err = parseFlags(s, state, char)
						if err != nil {
							return nil, err
						}

						if !ok { // global flags
							if !first || sp.len() > 0 {
								return nil, s.errorp("global flags not at the start of the expression", start)
							}

							verbose = state.flags&FlagVerbose != 0
							continue
						}

						capture = false
					} else {
						return nil, s.erroro(fmt.Sprintf("unknown extension ?%c", char), s.clen(char)+1)
					}
				}
			}

			// parse group contents

			group := -1
			if capture {
				group, err = state.openGroup(name)
				if err != nil {
					return nil, s.erroro(err.Error(), len(name)+1)
				}
			}

			subVerbose := (verbose || (addFlags&FlagVerbose != 0)) && !(delFlags&FlagVerbose != 0)

			p, err := parseSub(s, state, subVerbose, nested+1)
			if err != nil {
				return nil, err
			}

			if !s.match(')') {
				return nil, s.errorp("missing ), unterminated subpattern", start)
			}

			if group != -1 {
				state.closeGroup(group)
			}

			if atomic {
				sp.append(newAtomicGroupNode(opAtomicGroup, p))
			} else {
				sp.append(newSubPatternNode(opSubpattern, group, addFlags, delFlags, p))
			}

		case '^':
			sp.append(newAtNode(opAt, atBeginning))
		case '$':
			sp.append(newAtNode(opAt, atEnd))
		}
	}

	// unpack non-capturing groups
	for i := sp.len() - 1; i >= 0; i-- {
		t := sp.get(i)
		if t.opcode == opSubpattern {
			p := t.params.(subPatternParam)
			if p.group == -1 && p.addFlags == 0 && p.delFlags == 0 {
				sp.replace(i, p.p)
			}
		}
	}

	return sp, nil
}

// parseEscape parses an escape sequence.
// This function is only called if the last character was a backslash.
// The result regex nodes are of type LITERAL, GROUPREF, AT or IN.
func parseEscape(s *source, state *state, inCls bool) (*regexNode, error) {
	// handle escape code in expression

	c, ok := s.read()
	if !ok {
		return nil, s.erroro("bad escape (end of pattern)", 1)
	}

	switch c {
	case 'x':
		// hexadecimal escape

		e := s.nextHex(2)
		if len(e) != 2 {
			return nil, s.erroro(fmt.Sprintf(`incomplete escape \%c%s`, c, e), len(e)+2)
		}

		return newLiteral(parseIntRune(e, 16)), nil
	case 'u', 'U':
		// u: unicode escape (exactly four digits)
		// U: unicode escape (exactly eight digits)

		if !s.isStr { // u and U escapes only allowed for strings
			return nil, s.erroro(fmt.Sprintf(`bad escape \%c`, c), 2)
		}

		var size int
		if c == 'u' {
			size = 4
		} else {
			size = 8
		}

		e := s.nextHex(size)
		if len(e) != size {
			return nil, s.erroro(fmt.Sprintf(`incomplete escape \%c%s`, c, e), len(e)+2)
		}

		r := parseIntRune(e, 16)
		if c == 'U' && utf8.RuneLen(r) < 0 {
			return nil, s.erroro(fmt.Sprintf(`bad escape \%c%s`, c, e), len(e)+2)
		}

		return newLiteral(parseIntRune(e, 16)), nil
	case 'N':
		// named unicode escape e.g. \N{EM DASH}

		if !s.isStr {
			return nil, s.erroro(`bad escape \N`, 2)
		}

		if !s.match('{') {
			return nil, s.errorh("missing {")
		}

		name, err := s.getUntil('}', "character name")
		if err != nil {
			return nil, err
		}

		r, ok := lookupUnicodeName(name)
		if !ok {
			return nil, s.erroro(fmt.Sprintf("undefined character name %s", util.Repr(name, true)), len(name)+len(`\N{}`))
		}

		return newLiteral(r), nil
	case '0':
		// octal escape

		e := s.nextOct(2)
		r := parseIntRune(e, 8)

		return newLiteral(r), nil
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// octal escape *or* decimal group reference (only if not in class)

		start := s.tell() - 2 // save the start of the escape

		value := toDigit(c)

		if !inCls {
			if c1, ok := s.peek(); ok && isDigit(c1) {
				s.read()

				if isOctDigit(c) && isOctDigit(c1) {
					if c2, ok := s.peek(); ok && isOctDigit(c2) {
						s.read()

						value = 8*(8*value+toDigit(c1)) + toDigit(c2)
						if value > 0o377 {
							return nil, s.errorp(fmt.Sprintf(`octal escape value \%c%c%c outside of range 0-0o377`, c, c1, c2), start)
						}

						return newLiteral(rune(value)), nil
					}
				}

				value = 10*value + toDigit(c1)
			}

			// not an octal escape, so this is a group reference
			group := value
			if group < state.groups() {
				if !state.checkGroup(group) {
					return nil, s.errorp("cannot refer to an open group", start)
				}

				err := state.checkLookbehindGroup(group, s)
				if err != nil {
					return nil, err
				}

				return newGrouprefNode(opGroupref, group), nil
			}

			return nil, s.errorp(fmt.Sprintf("invalid group reference %d", value), start+1)
		}

		if c >= '8' {
			return nil, s.errorp(fmt.Sprintf(`bad escape \%c`, c), start)
		}

		e := s.nextOct(2)

		r := rune((1<<(3*len(e)))*value) + parseIntRune(e, 8) // 8 * value if len(e) == 1 else 64 * value
		if r > 0o377 {
			return nil, s.errorp(fmt.Sprintf(`octal escape value \%c%s outside of range 0-0o377`, c, e), start)
		}

		return newLiteral(r), nil

	// escapes
	case 'a':
		return newLiteral('\a'), nil
	case 'b':
		if !inCls {
			return newAtNode(opAt, atBoundary), nil
		}

		return newLiteral('\b'), nil
	case 'f':
		return newLiteral('\f'), nil
	case 'n':
		return newLiteral('\n'), nil
	case 'r':
		return newLiteral('\r'), nil
	case 't':
		return newLiteral('\t'), nil
	case 'v':
		return newLiteral('\v'), nil
	case '\\':
		return newLiteral('\\'), nil

		// categories
	case 'A':
		// start of string
		if !inCls {
			return newAtNode(opAt, atBeginningString), nil
		}
	case 'B':
		if !inCls {
			return newAtNode(opAt, atNonBoundary), nil
		}
	case 'd':
		return newItemsNode(opIn, []*regexNode{newCategoryNode(opCategory, categoryDigit)}), nil
	case 'D':
		return newItemsNode(opIn, []*regexNode{newCategoryNode(opCategory, categoryNotDigit)}), nil
	case 's':
		return newItemsNode(opIn, []*regexNode{newCategoryNode(opCategory, categorySpace)}), nil
	case 'S':
		return newItemsNode(opIn, []*regexNode{newCategoryNode(opCategory, categoryNotSpace)}), nil
	case 'w':
		return newItemsNode(opIn, []*regexNode{newCategoryNode(opCategory, categoryWord)}), nil
	case 'W':
		return newItemsNode(opIn, []*regexNode{newCategoryNode(opCategory, categoryNotWord)}), nil
	case 'Z':
		// end of string
		if !inCls {
			return newAtNode(opAt, atEndString), nil
		}
	default:
		if !isASCIILetter(c) {
			return newLiteral(c), nil
		}
	}

	return nil, s.erroro(fmt.Sprintf(`bad escape \%c`, c), 2)
}

// parseIntRune parses a string representation of a number in the given base and returns the corresponding rune value.
// The input string is expected to be valid for the given base and should not overflow the int32 type.
func parseIntRune(s string, base int) rune {
	r, _ := strconv.ParseInt(s, base, 32)
	return rune(r)
}

// parseFlags parses the regex flags in an group.
// If no flags where found, the return value of "result" is false.
// An error is returned, if incompatible or unknown flags where found.
func parseFlags(s *source, state *state, char rune) (addFlags, delFlags uint32, result bool, err error) {
	var ok bool

	if char != '-' {
		for {
			if s.isStr {
				if char == 'L' {
					err = s.errorh("bad inline flags: cannot use 'L' flag with a str pattern")
					return
				}
			} else {
				if char == 'u' {
					err = s.errorh("bad inline flags: cannot use 'u' flag with a bytes pattern")
					return
				}
			}

			flag := getFlag(char)

			addFlags |= flag
			if (flag&typeFlags != 0) && (addFlags&typeFlags) != flag {
				err = s.errorh("bad inline flags: flags 'a', 'u' and 'L' are incompatible")
				return
			}

			char, ok = s.read()
			if !ok {
				err = s.errorh("missing -, : or )")
				return
			}

			if char == ')' || char == '-' || char == ':' {
				break
			}

			if !isFlag(char) {
				if isASCIILetter(char) {
					err = s.erroro("unknown flag", s.clen(char))
					return
				}

				err = s.erroro("missing -, : or )", s.clen(char))
				return
			}
		}
	}

	if char == ')' {
		state.flags |= addFlags
		return
	}

	if addFlags&globalFlags != 0 {
		err = s.erroro("bad inline flags: cannot turn on global flag", 1)
		return
	}

	if char == '-' {
		char, ok = s.read()
		if !ok {
			err = s.errorh("missing flag")
			return
		}

		if !isFlag(char) {
			if isASCIILetter(char) {
				err = s.erroro("unknown flag", s.clen(char))
				return
			}

			err = s.erroro("missing flag", s.clen(char))
			return
		}

		for {
			flag := getFlag(char)
			if flag&typeFlags != 0 {
				err = s.errorh("bad inline flags: cannot turn off flags 'a', 'u' and 'L'")
				return
			}

			delFlags |= flag

			char, ok = s.read()
			if !ok {
				err = s.errorh("missing :")
				return
			}

			if char == ':' {
				break
			}

			if !isFlag(char) {
				if isASCIILetter(char) {
					err = s.erroro("unknown flag", s.clen(char))
					return
				}

				err = s.erroro("missing :", s.clen(char))
				return
			}
		}
	}

	if delFlags&globalFlags != 0 {
		err = s.erroro("bad inline flags: cannot turn off global flag", 1)
		return
	}
	if addFlags&delFlags != 0 {
		err = s.erroro("bad inline flags: flag turned on and off", 1)
		return
	}

	result = true
	return
}

// unique removes duplicate regex nodes from a slice inplace.
// This functions is a modified version of `slices.DeleteFunc`.
// If the length of the slice exceeds a certain size, it may be better to use a hashset
// as the worst case runtime is O(n^2).
// TODO: validate the performance bottleneck with benchmarks.
func unique(s []*regexNode) []*regexNode {
	// Don't start copying elements until we find one to delete.
	for i, v := range s {
		if contains(s, i, v) {
			j := i
			for i++; i < len(s); i++ {
				v := s[i]
				if !contains(s, j, v) {
					s[j] = v
					j++
				} else {
					// Zero element, so it can garbage collected;
					// see comment at `slices.DeleteFunc`.
					s[i] = nil
				}
			}
			return s[:j]
		}
	}

	return s
}

// contains checks, if the regex node `e` exists in the slice `items[:end]`.
func contains(items []*regexNode, end int, e *regexNode) bool {
	for i := 0; i < end; i++ {
		if items[i].equals(e) {
			return true
		}
	}
	return false
}

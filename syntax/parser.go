package syntax

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/magnetde/starlark-re/util"
)

const (
	typeFlags   = FlagASCII | FlagLocale | FlagUnicode
	globalFlags = FlagDebug
)

type state struct {
	flags            int
	groupdict        map[string]int
	groupwidths      []bool
	lookbehindgroups int
	grouprefpos      map[int]int
}

func (s *state) init() {
	s.flags = 0
	s.groupdict = make(map[string]int)
	s.groupwidths = []bool{false}
	s.lookbehindgroups = -1
	s.grouprefpos = make(map[int]int)
}

func (s *state) groups() int {
	return len(s.groupwidths)
}

func (s *state) opengroup(name string) (int, error) {
	gid := s.groups()
	s.groupwidths = append(s.groupwidths, false)
	if s.groups() > MAXGROUPS {
		return 0, errors.New("too many groups")
	}
	if name != "" {
		ogid, ok := s.groupdict[name]
		if ok {
			return 0, fmt.Errorf("redefinition of group name '%s' as group %d; was group %d", name, gid, ogid)
		}

		s.groupdict[name] = gid
	}

	return gid, nil
}

func (s *state) closegroup(gid int) {
	s.groupwidths[gid] = true
}

func (s *state) checkgroup(gid int) bool {
	return gid < s.groups() && s.groupwidths[gid]
}

func (s *state) checklookbehindgroup(gid int) error {
	if s.lookbehindgroups != -1 {
		if !s.checkgroup(gid) {
			return errors.New("cannot refer to an open group")
		}
		if gid >= s.lookbehindgroups {
			return errors.New("cannot refer to group defined in the same lookbehind subpattern")
		}
	}

	return nil
}

// preprocess may not be efficient but it is necessary, if ALL syntax error messages should be
// equal to the Python re module.
func parse(str string, isStr bool, flags int) (*subPattern, error) {
	var state state
	state.init()

	var s source
	s.init(str, isStr)

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
		return nil, errors.New("unbalanced parenthesis")
	}

	for g := range p.state.grouprefpos {
		if g >= p.state.groups() {
			return nil, fmt.Errorf("invalid group reference %d", g)
		}
	}

	if flags&FlagDebug != 0 {
		p.dump(nil) // TODO: call later
	}

	return p, nil
}

func checkFlags(flags int, isStr bool) (int, error) {
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
		var prefix *token
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

	appendSub := true

	// check if the branch can be replaced by a character set
	var set []*token
	for _, item := range items {
		if item.len() != 1 {
			appendSub = false
			break
		}
		t := item.get(0)
		op := t.opcode
		if op == LITERAL {
			set = append(set, t)
		} else if op == IN && t.items[0].opcode != NEGATE {
			set = append(set, t.items...)
		} else {
			appendSub = false
			break
		}
	}

	if appendSub {
		// we can store this as a character set instead of a
		// branch (the compiler may optimize this even more)
		sp.append(newItemsToken(IN, unique(set)))
	} else {
		sp.append(newSubPatternsToken(BRANCH, items))
	}

	return sp, nil
}

func unique(items []*token) []*token {
	m := make(map[opcode][]*token)

	for _, t := range items {
		if l, ok := m[t.opcode]; ok {
			add := true
			for _, i := range l {
				if i.equals(t) {
					add = false
					break
				}
			}

			if add {
				m[t.opcode] = append(l, t)
			}
		} else {
			m[t.opcode] = []*token{t}
		}
	}

	var new []*token
	for _, l := range m {
		new = append(new, l...)
	}

	return new
}

func parseInternal(s *source, state *state, verbose bool, nested int, first bool) (*subPattern, error) {
	// parse a simple pattern

	sp := newSubpattern(state)

	var err error
	for {
		c, ok := s.peek()
		if !ok {
			break // end of pattern
		}

		if c == '|' || c == ')' { // end of subpattern
			break
		}

		s.read()

		if verbose {
			// skip whitespace and comments
			if isWhitespace(c) {
				continue
			}
			if c == '#' {
				s.skipUntil("\n")
				continue
			}
		}

		switch c {
		default:
			sp.append(newLiteral(c))
		// ')', '|' already handled
		case '\\':
			token, err := parseEscape(s, state, false /* not class */)
			if err != nil {
				return nil, err
			}

			sp.append(token)
		case '[':
			negate := s.match('^')

			// character set
			var set []*token

			// check remaining characters
			for {
				c, ok = s.read()
				if !ok {
					return nil, errors.New("unterminated character set")
				}

				var code1, code2 *token

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
						return nil, errors.New("unterminated character set")
					}

					if ch == ']' {
						if code1.opcode == IN {
							code1 = code1.items[0]
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

					if code1.opcode != LITERAL || code2.opcode != LITERAL {
						return nil, fmt.Errorf("bad character range %c-%c", c, ch)
					}

					lo := code1.c
					hi := code2.c
					if hi < lo {
						return nil, fmt.Errorf("bad character range %c-%c", c, ch)
					}

					set = append(set, newRange(RANGE, lo, hi))
				} else {
					if code1.opcode == IN {
						code1 = code1.items[0]
					}
					set = append(set, code1)
				}
			}

			set = unique(set)

			if len(set) == 1 && set[0].opcode == LITERAL {
				if negate {
					sp.append(newCharToken(NOT_LITERAL, set[0].c))
				} else {
					sp.append(set[0])
				}
			} else {
				if negate {
					set = slices.Insert(set, 0, newEmptyToken(NEGATE))
				}

				// charmap optimization can't be added here because
				// global flags still are not known
				sp.append(newItemsToken(IN, set))
			}

		case '*', '+', '?', '{':
			// repeat previous item
			here := s.str()

			var min, max int
			switch c {
			case '?':
				min, max = 0, 1
			case '*':
				min, max = 0, MAXREPEAT
			case '+':
				min, max = 1, MAXREPEAT
			case '{':
				if next, ok := s.peek(); ok && next == '}' {
					sp.append(newLiteral(c))
					continue
				}

				min = s.nextInt()

				if s.match(',') {
					max = s.nextInt()
				} else {
					max = min
				}

				if !s.match('}') {
					sp.append(newLiteral(c))
					s.restore(here)
					continue
				}

				if min > MAXREPEAT {
					return nil, errors.New("the repetition number is too large")
				}
				if max > MAXREPEAT {
					return nil, errors.New("the repetition number is too large")
				}
				if max < min {
					return nil, errors.New("min repeat greater than max repeat")
				}
			default:
				return nil, fmt.Errorf("unsupported quantifier '%c'", c)
			}

			// figure out which item to repeat
			var item *token
			if sp.len() > 0 {
				item = sp.get(-1)
			}
			if item == nil || item.opcode == AT {
				return nil, errors.New("nothing to repeat")
			}
			if isRepeatCode(item.opcode) {
				return nil, errors.New("multiple repeat")
			}

			var subitem *subPattern
			if item.opcode == SUBPATTERN {
				p := item.params.(*paramSubPattern)
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
				sp.set(-1, newRepeat(MIN_REPEAT, min, max, subitem))
			} else if s.match('+') {
				// Possessive Match (Always Greedy)
				sp.set(-1, newRepeat(POSSESSIVE_REPEAT, min, max, subitem))
			} else {
				// Greedy Match
				sp.set(-1, newRepeat(MAX_REPEAT, min, max, subitem))
			}

		case '.':
			sp.append(newEmptyToken(ANY))

		case '(':
			capture := true
			atomic := false
			name := ""
			addFlags := 0
			delFlags := 0

			if s.match('?') {
				// options
				char, ok := s.read()
				if !ok {
					return nil, errors.New("unexpected end of pattern")
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

						err = checkgroupname(name, s.isStr)
						if err != nil {
							return nil, err
						}
					} else if s.match('=') {
						// named backreference
						name, err = s.getUntil(')', "group name")
						if err != nil {
							return nil, err
						}

						err = checkgroupname(name, s.isStr)
						if err != nil {
							return nil, err
						}

						gid, ok := state.groupdict[name]
						if !ok {
							return nil, fmt.Errorf("unknown group name '%s'", name)
						}
						if !state.checkgroup(gid) {
							return nil, errors.New("cannot refer to an open group")
						}

						err = state.checklookbehindgroup(gid)
						if err != nil {
							return nil, err
						}

						sp.append(newGrouprefToken(GROUPREF, gid))
						continue

					} else {
						char, ok = s.read()
						if !ok {
							return nil, errors.New("unexpected end of pattern")
						}

						return nil, fmt.Errorf("unknown extension ?P%c", char)
					}
				case ':':
					// non-capturing group
					capture = false
				case '#':
					// comment
					for {
						if _, ok = s.peek(); !ok {
							return nil, errors.New("missing ), unterminated comment")
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
							return nil, errors.New("unexpected end of pattern")
						}
						if !strings.ContainsRune("=!", char) {
							return nil, fmt.Errorf("unknown extension ?<%c", char)
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
						return nil, errors.New("missing ), unterminated subpattern")
					}

					if char == '=' {
						sp.append(newAssertToken(ASSERT, dir, p))
					} else if p.len() > 0 {
						sp.append(newAssertToken(ASSERT_NOT, dir, p))
					} else {
						sp.append(newEmptyToken(FAILURE))
					}

					continue
				case '(':
					// conditional backreference group
					var condgroup int
					condname, err := s.getUntil(')', "group name")
					if err != nil {
						return nil, err
					}

					if !(isDigitString(condname) && util.IsASCIIString(condname)) {
						err = checkgroupname(condname, s.isStr)
						if err != nil {
							return nil, err
						}

						condgroup, ok = state.groupdict[condname]
						if !ok {
							return nil, fmt.Errorf("unknown group name '%s'", condname)
						}
					} else {
						condgroup, err = strconv.Atoi(condname)
						if err != nil || condgroup == 0 {
							return nil, errors.New("bad group number")
						}

						if condgroup >= MAXGROUPS {
							return nil, fmt.Errorf("invalid group reference %d", condgroup)
						}

						if _, ok = state.grouprefpos[condgroup]; !ok {
							state.grouprefpos[condgroup] = s.tell() - len(condname) - 1
						}
						if !(isDigitString(condname) && util.IsASCIIString(condname)) {
							return nil, fmt.Errorf("bad character in group name %s", util.QuoteString(condname, s.isStr, false))
						}
					}

					err = state.checklookbehindgroup(condgroup)
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
							return nil, errors.New("conditional backref with more than two branches")
						}
					}

					if !s.match(')') {
						return nil, errors.New("missing ), unterminated subpattern")
					}

					sp.append(newGrouprefExistsToken(GROUPREF_EXISTS, condgroup, itemYes, itemNo))
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
								return nil, errors.New("global flags not at the start of the expression")
							}

							verbose = state.flags&FlagVerbose != 0
							continue
						}

						capture = false
					} else {
						return nil, fmt.Errorf("unknown extension ?%c", char)
					}
				}
			}

			// parse group contents

			group := -1
			if capture {
				group, err = state.opengroup(name)
				if err != nil {
					return nil, err
				}
			}

			subVerbose := ((verbose || (addFlags&FlagVerbose != 0)) && !(delFlags&FlagVerbose != 0))

			p, err := parseSub(s, state, subVerbose, nested+1)
			if err != nil {
				return nil, err
			}

			if !s.match(')') {
				return nil, errors.New("missing ), unterminated subpattern")
			}

			if group != -1 {
				state.closegroup(group)
			}

			if atomic {
				sp.append(newWithSubPattern(ATOMIC_GROUP, p))
			} else {
				sp.append(newSubPattern(SUBPATTERN, group, addFlags, delFlags, p))
			}

		case '^':
			sp.append(newAtToken(AT, AT_BEGINNING))
		case '$':
			sp.append(newAtToken(AT, AT_END))
		}
	}

	// unpack non-capturing groups
	for i := sp.len() - 1; i >= 0; i-- {
		t := sp.get(i)
		if t.opcode == SUBPATTERN {
			p := t.params.(*paramSubPattern)
			if p.group == -1 && p.addFlags == 0 && p.delFlags == 0 {
				sp.replace(i, p.p)
			}
		}
	}

	return sp, nil
}

func parseEscape(s *source, state *state, inCls bool) (*token, error) {
	// handle escape code in expression

	c, ok := s.read()
	if !ok {
		return nil, errors.New("bad escape")
	}

	switch c {
	case 'x':
		// hexadecimal escape

		e := s.nextHex(2)
		if len(e) != 2 {
			return nil, fmt.Errorf(`incomplete escape \%c%s`, c, e)
		}

		return newLiteral(parseIntRune(e, 16)), nil
	case 'u', 'U':
		// u: unicode escape (exactly four digits)
		// U: unicode escape (exactly eight digits)

		if !s.isStr { // u and U escapes only allowed for strings
			return nil, fmt.Errorf(`bad escape \%c`, c)
		}

		var size int
		if c == 'u' {
			size = 4
		} else {
			size = 8
		}

		e := s.nextHex(size)
		if len(e) != size {
			return nil, fmt.Errorf(`incomplete escape \%c%s`, c, e)
		}

		r := parseIntRune(e, 16)
		if c == 'U' && utf8.RuneLen(r) < 0 {
			return nil, fmt.Errorf(`bad escape \%c%s`, c, e)
		}

		return newLiteral(parseIntRune(e, 16)), nil
	case 'N':
		// named unicode escape e.g. \N{EM DASH}

		if !s.isStr {
			return nil, errors.New(`bad escape \N`)
		}

		if !s.match('{') {
			return nil, errors.New("missing {")
		}

		name, err := s.getUntil('}', "character name")
		if err != nil {
			return nil, err
		}

		r, ok := lookupUnicodeName(name)
		if !ok {
			return nil, fmt.Errorf("undefined character name '%s'", name)
		}

		return newLiteral(r), nil
	case '0':
		// octal escape

		e := s.nextOct(2)
		r := parseIntRune(e, 8)

		return newLiteral(r), nil
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// octal escape *or* decimal group reference (only if not in class)

		value := digit(c)

		if !inCls {
			if c1, ok := s.peek(); ok && isDigit(c1) {
				s.read()

				if isOctDigit(c) && isOctDigit(c1) {
					if c2, ok := s.peek(); ok && isOctDigit(c2) {
						s.read()

						value = 8*(8*value+digit(c1)) + digit(c2)
						if value > 0o377 {
							return nil, fmt.Errorf(`octal escape value \%c%c%c outside of range 0-0o377`, c, c1, c2)
						}

						return newLiteral(rune(value)), nil
					}
				}

				value = 10*value + digit(c1)
			}

			// not an octal escape, so this is a group reference
			group := value
			if group < state.groups() {
				if !state.checkgroup(group) {
					return nil, errors.New("cannot refer to an open group")
				}

				err := state.checklookbehindgroup(group)
				if err != nil {
					return nil, err
				}

				return newGrouprefToken(GROUPREF, group), nil
			}

			return nil, fmt.Errorf("invalid group reference %d", value)
		}

		if c >= '8' {
			return nil, fmt.Errorf(`bad escape \%c`, c)
		}

		e := s.nextOct(2)

		r := rune((1<<(3*len(e)))*value) + parseIntRune(e, 8) // 8 * value if len(e) == 1 else 64 * value
		if r > 0o377 {
			return nil, fmt.Errorf(`octal escape value \%c%s outside of range 0-0o377`, c, e)
		}

		return newLiteral(r), nil

	// escapes
	case 'a':
		return newLiteral('\a'), nil
	case 'b':
		if inCls {
			return newLiteral('\b'), nil
		} else {
			return newAtToken(AT, AT_BOUNDARY), nil
		}
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
			return newAtToken(AT, AT_BEGINNING_STRING), nil
		}
	case 'B':
		if !inCls {
			return newAtToken(AT, AT_NON_BOUNDARY), nil
		}
	case 'd':
		return newItemsToken(IN, []*token{newCategoryToken(CATEGORY, CATEGORY_DIGIT)}), nil
	case 'D':
		return newItemsToken(IN, []*token{newCategoryToken(CATEGORY, CATEGORY_NOT_DIGIT)}), nil
	case 's':
		return newItemsToken(IN, []*token{newCategoryToken(CATEGORY, CATEGORY_SPACE)}), nil
	case 'S':
		return newItemsToken(IN, []*token{newCategoryToken(CATEGORY, CATEGORY_NOT_SPACE)}), nil
	case 'w':
		return newItemsToken(IN, []*token{newCategoryToken(CATEGORY, CATEGORY_WORD)}), nil
	case 'W':
		return newItemsToken(IN, []*token{newCategoryToken(CATEGORY, CATEGORY_NOT_WORD)}), nil
	case 'Z':
		// end of string
		if !inCls {
			return newAtToken(AT, AT_END_STRING), nil
		}
	default:
		if !isASCIILetter(c) {
			return newLiteral(c), nil
		}
	}

	return nil, fmt.Errorf("bad escape %c", c)
}

// assertion: string is valid and does not overflow int
func parseIntRune(s string, base int) rune {
	r, _ := strconv.ParseUint(s, base, 32)
	return rune(r)
}

func parseFlags(s *source, state *state, char rune) (addFlags int, delFlags int, result bool, err error) {
	var ok bool

	if char != '-' {
		for {
			if s.isStr {
				if char == 'L' {
					err = errors.New("bad inline flags: cannot use 'L' flag with a str pattern")
					return
				}
			} else {
				if char == 'u' {
					err = errors.New("bad inline flags: cannot use 'u' flag with a bytes pattern")
					return
				}
			}

			flag := getFlag(char)

			addFlags |= flag
			if (flag&typeFlags != 0) && (addFlags&typeFlags) != flag {
				err = errors.New("bad inline flags: flags 'a', 'u' and 'L' are incompatible")
				return
			}

			char, ok = s.read()
			if !ok {
				err = errors.New("missing -, : or )")
				return
			}

			if strings.ContainsRune(")-:", char) {
				break
			}

			if !isFlag(char) {
				if isASCIILetter(char) {
					err = errors.New("unknown flag")
					return
				}

				err = errors.New("missing -, : or )")
				return
			}
		}
	}

	if char == ')' {
		state.flags |= addFlags
		return
	}

	if addFlags&globalFlags != 0 {
		err = errors.New("bad inline flags: cannot turn on global flag")
		return
	}

	if char == '-' {
		char, ok = s.read()
		if !ok {
			err = errors.New("missing flag")
			return
		}

		if !isFlag(char) {
			if isASCIILetter(char) {
				err = errors.New("unknown flag")
				return
			}

			err = errors.New("missing flag")
			return
		}

		for {
			flag := getFlag(char)
			if flag&typeFlags != 0 {
				err = errors.New("bad inline flags: cannot turn off flags 'a', 'u' and 'L'")
				return
			}

			delFlags |= flag

			char, ok = s.read()
			if !ok {
				err = errors.New("missing :")
				return
			}

			if char == ':' {
				break
			}

			if !isFlag(char) {
				if isASCIILetter(char) {
					err = errors.New("unknown flag")
					return
				}

				err = errors.New("missing :")
				return
			}
		}
	}

	if delFlags&globalFlags != 0 {
		err = errors.New("bad inline flags: cannot turn off global flag")
		return
	}
	if addFlags&delFlags != 0 {
		err = errors.New("bad inline flags: flag turned on and off")
		return
	}

	result = true
	return
}

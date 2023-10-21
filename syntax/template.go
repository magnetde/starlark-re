package syntax

import (
	"fmt"
	"strconv"

	"github.com/magnetde/starlark-re/util"
)

// TemplateRule is a rule of the template. A single template rule can either represent a literal or a group index.
// If the group index is -1, this rule is interpreted as a literal.
type TemplateRule struct {
	Literal string
	Group   int
}

// IsLiteral returns, if the rule represents a literal or a group index.
func (t *TemplateRule) IsLiteral() bool {
	return t.Group < 0
}

// ParseTemplate parses a template string into a list of rules.
// The list of rules is an alternating sequence of literal rules and group rules.
// For example, the template "x\1yz" results in the following list:
//   - 0: literal "x"
//   - 1: group 1
//   - 2: literal "yz"
func ParseTemplate(r Engine, template string, isString bool) ([]TemplateRule, error) {
	var s source
	s.init(template, isString)

	var rules []TemplateRule

	addLiteral := func(s string) {
		if s != "" {
			if len(rules) > 0 {
				lastRule := &rules[len(rules)-1]

				if lastRule.Group < 0 { // if last rule is also a literal, then concat the strings
					lastRule.Literal += s
					return
				}
			}

			rules = append(rules, TemplateRule{Literal: s, Group: -1})
		}
	}

	addIndex := func(i int, offset int) error {
		if i > r.SubexpCount() {
			return s.erroro(fmt.Sprintf("invalid group reference %d", i), offset)
		}

		rules = append(rules, TemplateRule{Group: i})
		return nil
	}

	for {
		// skip to the next '\'
		before, ok := s.skipUntil('\\')
		addLiteral(before)

		if !ok {
			break
		}

		c, ok := s.read()
		if !ok {
			return nil, s.erroro("bad escape (end of pattern)", 1)
		}

		switch c {
		case 'g': // group found
			if !s.match('<') {
				return nil, s.erroro("missing <", 0)
			}

			name, err := s.getUntil('>', "group name")
			if err != nil {
				return nil, err
			}

			var index int

			if uindex, e := strconv.ParseUint(name, 10, 32); e != nil {
				err = s.checkGroupName(name, 1)
				if err != nil {
					return nil, err
				}

				index = r.SubexpIndex(name)
				if index < 0 {
					return nil, s.erroro(fmt.Sprintf("unknown group name %s", util.Repr(name, true)), len(name)+1)
				}
			} else {
				if uindex >= maxGroups {
					return nil, s.erroro(fmt.Sprintf("invalid group reference %d", index), len(name)+1)
				}

				index = int(uindex)
			}

			err = addIndex(index, len(name)+1)
			if err != nil {
				return nil, err
			}
		case '0': // octal string
			// octal escape

			e := s.nextOct(2)
			r := parseIntRune(e, 8)

			addLiteral(string(r))
		case '1', '2', '3', '4', '5', '6', '7', '8', '9': // index or octal string
			start := s.tell() - 2 // save the start of the escape

			index := toDigit(c)

			if c1, ok := s.peek(); ok && isDigit(c1) {
				s.read()

				if isOctDigit(c) && isOctDigit(c1) {
					if c2, ok := s.peek(); ok && isOctDigit(c2) {
						s.read()

						index = 8*(8*index+toDigit(c1)) + toDigit(c2)
						if index > 0o377 {
							return nil, s.errorp(fmt.Sprintf(`octal escape value \%c%c%c outside of range 0-0o377`, c, c1, c2), start)
						}

						addLiteral(string(rune(index)))
						break // break out of case
					}
				}

				index = 10*index + toDigit(c1)
			}

			err := addIndex(index, s.tell()-start-1)
			if err != nil {
				return nil, err
			}
		case 'a':
			addLiteral("\a")
		case 'b':
			addLiteral("\b")
		case 'f':
			addLiteral("\f")
		case 'n':
			addLiteral("\n")
		case 'r':
			addLiteral("\r")
		case 't':
			addLiteral("\t")
		case 'v':
			addLiteral("\v")
		case '\\':
			addLiteral("\\")
		default:
			if isASCIILetter(c) {
				return nil, s.erroro(fmt.Sprintf(`bad escape \%c`, c), 2)
			}

			addLiteral(`\`)
			addLiteral(string(c))
		}
	}

	return rules, nil
}

package syntax

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/magnetde/starlark-re/util"
)

type TemplateRule struct {
	Literal string
	Index   int // is -1 if template rule is literal
}

func (t *TemplateRule) IsLiteral() bool {
	return t.Index < 0
}

type Indexer interface {
	SubexpIndex(name string) int
	NumSubexp() int
}

func ParseTemplate(s Indexer, template string, isString bool) ([]TemplateRule, error) {
	var rules []TemplateRule

	addLiteral := func(s string) {
		if s != "" {
			if len(rules) > 0 {
				lastRule := &rules[len(rules)-1]

				if lastRule.Index < 0 { // if last rule is also a literal, then concat the strings
					lastRule.Literal += s
					return
				}
			}

			rules = append(rules, TemplateRule{Literal: s, Index: -1})
		}
	}

	addIndex := func(i int) error {
		if i > s.NumSubexp() {
			return fmt.Errorf("invalid group reference %d", i)
		}

		rules = append(rules, TemplateRule{Index: i})
		return nil
	}

	for len(template) > 0 {
		before, rest, ok := strings.Cut(template, `\`)
		if !ok {
			break
		}

		addLiteral(before)

		template = rest

		if template == "" {
			return nil, errors.New("bad escape (end of pattern)")
		}

		c := template[0]

		template = template[1:]

		switch c {
		case 'g': // group found
			index, rest, err := extractGroup(s, template, isString)
			if err != nil {
				return nil, err
			}

			template = rest

			err = addIndex(index)
			if err != nil {
				return nil, err
			}
		case '0': // octal string
			chr := 0

			if len(template) > 0 && isOctDigitByte(template[0]) {
				chr = digitByte(template[0])

				if len(template) > 1 && isOctDigitByte(template[1]) {
					chr = 8*chr + digitByte(template[1])
					template = template[2:]
				} else {
					template = template[1:]
				}
			}

			addLiteral(string(rune(chr)))
		case '1', '2', '3', '4', '5', '6', '7', '8', '9': // index or octal string
			index := digitByte(c)

			if len(template) > 0 && isDigitByte(template[0]) {
				if isOctDigitByte(c) && isOctDigitByte(template[0]) &&
					len(template) > 1 && isOctDigitByte(template[1]) {

					index = 8*(8*index+digitByte(template[0])) + digitByte(template[1])
					if index > 0o377 {
						return nil, fmt.Errorf(`octal escape value \%s outside of range 0-0o377`, string(c)+template[:2])
					}

					template = template[2:]

					addLiteral(string(rune(index)))
					break // break out of case
				}

				index = 10*index + digitByte(template[0])
				template = template[1:]
			}

			err := addIndex(index)
			if err != nil {
				return nil, err
			}
		default:
			if escape, ok := unescapeLetter(c); ok {
				addLiteral(escape)
			} else {
				if isASCIILetterByte(c) {
					return nil, fmt.Errorf("bad escape \\%c", c)
				}

				addLiteral(`\`)
				addLiteral(string(c))
			}
		}
	}

	addLiteral(template)

	return rules, nil
}

func extractGroup(s Indexer, template string, isString bool) (index int, rest string, err error) {
	if template == "" || template[0] != '<' {
		err = errors.New("missing <")
		return
	}

	name, rest, ok := strings.Cut(template[1:], ">")

	if name == "" { // check first, if the name is empty to match Python errors
		err = errors.New("missing group name")
		return
	}

	if !ok {
		err = errors.New("missing >, unterminated name")
		return
	}

	if uindex, e := strconv.ParseUint(name, 10, 32); e != nil {
		err = checkgroupname(name, isString)
		if err != nil {
			return
		}

		index = s.SubexpIndex(name)
		if index < 0 {
			err = fmt.Errorf("unknown group name %s", util.QuoteString(name, isString, true))
			return
		}
	} else {
		if uindex >= MAXGROUPS {
			err = fmt.Errorf("invalid group reference %d", index)
			return
		}

		index = int(uindex)
	}

	return
}

package re

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.starlark.net/starlark"
)

type replacer interface {
	withMatch() bool
	replace(m *Match) (string, error)
}

// Replacer implementations

// Replacer for templates.
// If the replace string does not contain any references, the rule slice contains a single item,
// representing a literal.
type templateReplacer struct {
	rules []templateRule
	match bool
}

type templateRule struct {
	literal string
	index   int // is -1 if template rule is literal
}

// Replacer for functions
type functionReplacer func(m *Match) (string, error)

// Check if the types satiesfy the replacer interface.
var (
	_ replacer = (*templateReplacer)(nil)
	_ replacer = (*functionReplacer)(nil)
)

func (r *templateReplacer) withMatch() bool {
	return r.match
}

func (r *templateReplacer) replace(m *Match) (string, error) {
	var b strings.Builder

	for _, t := range r.rules {
		if t.index < 0 {
			b.WriteString(t.literal)
		} else {
			g := m.groups[t.index]
			if !g.empty() {
				b.WriteString(g.str)
			}
		}
	}

	return b.String(), nil
}

func (r functionReplacer) withMatch() bool {
	return true
}

func (r functionReplacer) replace(m *Match) (string, error) {
	return r(m)
}

func getReplacer(thread *starlark.Thread, p *Pattern, r starlark.Value) (replacer, error) {
	switch t := r.(type) {
	case starlark.String:
		if !p.pattern.isString {
			return nil, errors.New("got str, want bytes")
		}

		return newTemplateReplacer(p.re, string(t), true /* is string */)
	case starlark.Bytes:
		if p.pattern.isString {
			return nil, errors.New("got bytes, want str")
		}

		return newTemplateReplacer(p.re, string(t), false /* is not string */)
	case starlark.Callable:
		fn := func(m *Match) (string, error) {
			raw, err := t.CallInternal(thread, starlark.Tuple{m}, nil)
			if err != nil {
				return "", err
			}

			// check if result is str or bytes
			var res strOrBytes
			err = res.Unpack(raw)
			if err != nil {
				return "", err
			}

			err = p.pattern.sameType(res)
			if err != nil {
				return "", err
			}

			return res.value, nil
		}

		return functionReplacer(fn), nil
	default:
		return nil, fmt.Errorf("got %s, want str, bytes or function", r.Type())
	}
}

// if returned slice empty: contains no backreferences and is just a literal
func newTemplateReplacer(r regexEngine, repl string, isString bool) (replacer, error) {
	var rules []templateRule
	withMatch := false

	if !strings.ContainsRune(repl, '\\') { // check, if the template should be parsed
		rules = []templateRule{{
			literal: repl,
			index:   -1,
		}}
	} else {
		var err error

		rules, err = parseTemplate(r, repl, isString)
		if err != nil {
			return nil, err
		}

		withMatch = true
	}

	tr := &templateReplacer{
		rules: rules,
		match: withMatch,
	}

	return tr, nil
}

func parseTemplate(r regexEngine, template string, isString bool) ([]templateRule, error) {
	var rules []templateRule

	addLiteral := func(s string) {
		if s != "" {
			if len(rules) > 0 {
				lastRule := &rules[len(rules)-1]

				if lastRule.index < 0 { // if last rule is also a literal, then concat the strings
					lastRule.literal += s
					return
				}
			}

			rules = append(rules, templateRule{literal: s, index: -1})
		}
	}

	addIndex := func(i int) {
		rules = append(rules, templateRule{index: i})
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
			index, rest, err := extractGroup(r, template, isString)
			if err != nil {
				return nil, err
			}

			template = rest

			addIndex(index)
		case '0': // octal string
			chr := 0

			if len(template) > 0 && isOctDigit(template[0]) {
				chr = digit(template[0])

				if len(template) > 1 && isOctDigit(template[1]) {
					chr = 8*chr + digit(template[1])
					template = template[2:]
				} else {
					template = template[1:]
				}
			}

			addLiteral(string(rune(chr)))
		case '1', '2', '3', '4', '5', '6', '7', '8', '9': // index or octal string
			index := digit(c)

			if len(template) > 0 && isDigit(template[0]) {
				if isOctDigit(c) && isOctDigit(template[0]) &&
					len(template) > 1 && isOctDigit(template[1]) {

					index = 8*(8*index+digit(template[0])) + digit(template[1])
					if index > 0o377 {
						return nil, fmt.Errorf(`octal escape value \%s outside of range 0-0o377`, string(c)+template[:2])
					}

					template = template[2:]

					addLiteral(string(rune(index)))
					break // break out of case
				}

				index = 10*index + digit(template[0])
				template = template[1:]
			}

			// not octal
			if index >= r.NumSubexp() {
				return nil, fmt.Errorf("invalid group reference %d", index)
			}

			addIndex(index)
		default:
			if escape, ok := unescapeLetter(c); ok {
				addLiteral(escape)
			} else {
				if isASCIILetter(c) {
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

func extractGroup(r regexEngine, template string, isString bool) (index int, rest string, err error) {
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

	uindex, intErr := strconv.ParseUint(name, 10, 0)
	if intErr != nil {
		if !isIdentifier(name) {
			err = fmt.Errorf("bad character in group name %s", quoteString(name, isString, false))
			return
		}

		index = r.SubexpIndex(name)
		if index < 0 {
			err = fmt.Errorf("unknown group name '%s'", name)
			return
		}
	} else {
		index = int(uindex)
		if index > r.NumSubexp() {
			err = fmt.Errorf("invalid group reference %d", index)
			return
		}
	}

	return
}

func sub(p *Pattern, r replacer, str strOrBytes, count int, subn bool) (starlark.Value, error) {
	s := str.value

	var replaced strings.Builder
	var err error

	matches := 0
	beg := 0
	end := 0

	findMatches(p.re, s, 0, count, func(match []int) bool {
		end = match[0]

		replaced.WriteString(s[beg:end])

		var m *Match
		if r.withMatch() {
			m = newMatch(p, str, match, 0, len(str.value))
		}

		r, er := r.replace(m) // assign the outer error
		if er != nil {
			err = er
			return false
		}

		replaced.WriteString(r)
		matches++

		beg = match[1]
		return true
	})
	if err != nil {
		return nil, err
	}

	if end != len(s) {
		replaced.WriteString(s[beg:])
	}

	res := p.pattern.asType(replaced.String())

	if subn {
		subs := starlark.MakeInt(matches)
		return starlark.Tuple{res, subs}, nil
	}

	return res, nil
}

package re

import (
	"errors"
	"fmt"
	"strings"

	"go.starlark.net/starlark"

	sre "github.com/magnetde/starlark-re/syntax"
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
	rules []sre.TemplateRule
	match bool
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
		if t.IsLiteral() {
			b.WriteString(t.Literal)
		} else {
			g := m.groups[t.Index]
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
	var rules []sre.TemplateRule
	withMatch := false

	if !strings.ContainsRune(repl, '\\') { // check, if the template should be parsed
		rules = []sre.TemplateRule{{
			Literal: repl,
			Index:   -1,
		}}
	} else {
		var err error

		rules, err = sre.ParseTemplate(r, repl, isString)
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

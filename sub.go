package re

import (
	"errors"
	"fmt"
	"strings"

	"go.starlark.net/starlark"

	"github.com/magnetde/starlark-re/regex"
)

// matchReplacer is the common interface for match replacers,
// whether they are a template of `str` / `bytes` or a replace function.
type matchReplacer interface {

	// withMatch should return true if the replace function requires a non-nil match parameter.
	// It would lead to unnecessary overhead if the match was created but not used.
	// At the moment the match parameter can only be nil if the template does not contain any references.
	withMatch() bool

	// replace replaces the match `m` by writing its substitution to `w`.
	replace(w *strings.Builder, m *Match) error
}

// Replacer implementations

// templateReplacer is the replacer for templates.
// If the replace string did not contain any references, the slice of rules contains a single item,
// representing a literal and the `match` member is set to false.
type templateReplacer struct {
	rules []regex.TemplateRule
	match bool
}

// callableReplacer is the replacer for replace functions.
type callableReplacer struct {
	c      starlark.Callable
	thread *starlark.Thread
	p      *Pattern
}

// Check if the types satisfy the replacer interface.
var (
	_ matchReplacer = (*templateReplacer)(nil)
	_ matchReplacer = (*callableReplacer)(nil)
)

// withMatch returns true, if the template does not contain any references.
func (r *templateReplacer) withMatch() bool {
	return r.match
}

// replace replaces the current match.
func (r *templateReplacer) replace(w *strings.Builder, m *Match) error {
	for _, t := range r.rules {
		if t.IsLiteral() {
			w.WriteString(t.Literal)
		} else {
			g := &m.groups[t.Group]
			if !g.empty() {
				w.WriteString(m.groupStr(g))
			}
		}
	}

	return nil
}

// withMatch always returns true.
func (r *callableReplacer) withMatch() bool {
	return true
}

// replace replaces the current match by calling the replacer function.
func (r *callableReplacer) replace(w *strings.Builder, m *Match) error {
	raw, err := r.c.CallInternal(r.thread, starlark.Tuple{m}, nil)
	if err != nil {
		return err
	}

	// check if the result is str or bytes
	var res strOrBytes
	err = res.Unpack(raw)
	if err != nil {
		return err
	}

	// check if the result matches the expected type
	err = r.p.pattern.sameType(res)
	if err != nil {
		return err
	}

	w.WriteString(res.value)

	return nil
}

// buildReplacer creates a new match replacer based on the input parameter type.
// If the parameter is of type `str` or `bytes`, a template replacer is created.
// If the parameter is callable, a function replacer is returned instead.
func buildReplacer(thread *starlark.Thread, p *Pattern, r starlark.Value) (matchReplacer, error) {
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
		c := callableReplacer{
			c:      t,
			thread: thread,
			p:      p,
		}

		return &c, nil
	default:
		return nil, fmt.Errorf("got %s, want %s or function", r.Type(), p.pattern.typeString())
	}
}

// newTemplateReplacer creates a new template replacer for a template.
// If the template does not contain any backslashes, it does need to be parsed.
// Otherwise, the template must be parsed and transformed into a slice of template rules.
func newTemplateReplacer(r regex.Engine, repl string, isString bool) (matchReplacer, error) {
	var rules []regex.TemplateRule
	withMatch := false

	if !strings.ContainsRune(repl, '\\') { // check, if the template needs to be parsed
		rules = []regex.TemplateRule{{
			Literal: repl,
			Group:   -1,
		}}
	} else {
		var err error

		rules, err = regex.ParseTemplate(r, repl, isString)
		if err != nil {
			return nil, err
		}

		// `rules` is an slice with literals and group references, having at least one element.
		// If there are at least two elements or the only element is a group reference,
		// the slice contains group references because no two literals appear next to each other,.
		withMatch = len(rules) >= 2 || !rules[0].IsLiteral()
	}

	tr := &templateReplacer{
		rules: rules,
		match: withMatch,
	}

	return tr, nil
}

// sub replaces all matches of the pattern `p` in `str` with the replacement `r`.
// At most `count` matches will be replaced. If `subn` is true, then the number of replacements is also returned.
func sub(p *Pattern, r matchReplacer, str strOrBytes, count int, subn bool) (starlark.Value, error) {
	s := str.value

	var b strings.Builder

	matches := 0
	beg := 0
	end := 0

	err := findMatches(p.re, s, 0, count, func(match []int) error {
		end = match[0]

		b.WriteString(s[beg:end])

		var m *Match
		if r.withMatch() {
			m = newMatch(p, str, match, 0, len(str.value))
		}

		err := r.replace(&b, m) // assign the outer error
		if err != nil {
			return err
		}

		matches++

		beg = match[1]
		return nil
	})
	if err != nil {
		return nil, err
	}

	if end != len(s) {
		b.WriteString(s[beg:])
	}

	res := p.pattern.asType(b.String())

	if subn {
		subs := starlark.MakeInt(matches)
		return starlark.Tuple{res, subs}, nil
	}

	return res, nil
}

package re

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

//go:embed re_test.py
var reScript string

func TestRe(t *testing.T) {
	asserts := map[string]func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error){
		"assertEqual":       assertEqual,
		"assertNotEqual":    assertNotEqual,
		"assertIs":          assertIs,
		"assertIsNot":       assertIsNot,
		"assertIsNone":      assertIsNone,
		"assertIsNotNone":   assertIsNotNone,
		"assertFalse":       assertFalse,
		"assertTrue":        assertTrue,
		"assertIn":          assertIn,
		"assertNotIn":       assertNotIn,
		"assertLess":        assertLess,
		"assertIsInstance":  assertIsInstance,
		"assertRegex":       assertRegex,
		"assertRaises":      assertRaises,
		"assertRaisesRegex": assertRaisesRegex,
		"measure":           measureTime,
		"eval":              evalFunc,
		"trycatch":          tryCatchFunc,
	}

	predeclared := starlark.StringDict{
		"re": NewModule(true),
	}

	for name, fn := range asserts {
		predeclared[name] = starlark.NewBuiltin(name, fn)
	}

	opts := syntax.FileOptions{
		Set:             true,
		While:           true,
		TopLevelControl: true,
		GlobalReassign:  true,
		Recursion:       true,
	}

	_, prog, err := starlark.SourceProgramOptions(&opts, "re_test.py", reScript, predeclared.Has)
	if err != nil {
		t.Fatal(err)
	}

	thread := &starlark.Thread{
		Name: "test re",
		Print: func(thread *starlark.Thread, msg string) {
			fmt.Println(msg)
		},
	}

	_, err = prog.Init(thread, predeclared)
	if err != nil {
		e := err.(*starlark.EvalError)

		t.Fatal(e.Backtrace())
	}
}

func assertionOk() (starlark.Value, error) {
	return starlark.None, nil
}

type msg string

var _ starlark.Unpacker = (*msg)(nil)

func (m *msg) Unpack(v starlark.Value) error {
	if isNone(v) {
		*m = ""
	} else {
		switch t := v.(type) {
		case starlark.String:
			*m = msg(t)
		case starlark.Bytes:
			*m = msg(t)
		default:
			return fmt.Errorf("got %s, want string or None", v.Type())
		}
	}

	return nil
}

func isNone(v starlark.Value) bool {
	if v == starlark.None {
		return true
	}
	if _, ok := v.(starlark.NoneType); ok {
		return true
	}

	return false
}

func assertionFail(msg msg, standardMsg string) (starlark.Value, error) {
	if msg == "" {
		return nil, errors.New(standardMsg)
	}
	return nil, fmt.Errorf("%s : %s", standardMsg, msg)
}

func assertEqual(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return assertEq(b, args, kwargs, true)
}

func assertNotEqual(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return assertEq(b, args, kwargs, false)
}

func assertEq(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, equal bool) (starlark.Value, error) {
	var (
		x, y starlark.Value
		msg  msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &x, "y", &y, "msg?", &msg); err != nil {
		return nil, err
	}

	eq, err := starlark.Equal(x, y)
	if err != nil {
		return nil, err
	}

	if eq == equal {
		return assertionOk()
	}

	op := "=="
	if !equal {
		op = "!="
	}

	return assertionFail(msg, fmt.Sprintf("%v %s %v", x, op, y))
}

func assertIs(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		x, y starlark.Value
		msg  msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &x, "y", &y, "msg?", &msg); err != nil {
		return nil, err
	}

	if x == y {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("%v is not %v", x, y))
}

func assertIsNot(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		x, y starlark.Value
		msg  msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &x, "y", &y, "msg?", &msg); err != nil {
		return nil, err
	}

	if x != y {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("unexpectedly identical: %s", x))
}

func assertIsNone(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		v   starlark.Value
		msg msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "v", &v, "msg?", &msg); err != nil {
		return nil, err
	}

	if isNone(v) {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("%s is not None", v))
}

func assertIsNotNone(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		v   starlark.Value
		msg msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "v", &v, "msg?", &msg); err != nil {
		return nil, err
	}

	if !isNone(v) {
		return assertionOk()
	}

	return assertionFail(msg, "unexpectedly None")
}

func assertFalse(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		v   starlark.Value
		msg msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "v", &v, "msg?", &msg); err != nil {
		return nil, err
	}

	if !v.Truth() {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("%s is not false", v))
}

func assertTrue(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		v   starlark.Value
		msg msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "v", &v, "msg?", &msg); err != nil {
		return nil, err
	}

	if v.Truth() {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("%s is not true", v))
}

func assertIn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return assertBinary(b, args, kwargs, syntax.IN, "%s not found in %s")
}

func assertBinary(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, token syntax.Token, format string) (starlark.Value, error) {
	var (
		x, y starlark.Value
		msg  msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &x, "y", &y, "msg?", &msg); err != nil {
		return nil, err
	}

	res, err := starlark.Binary(token, x, y)
	if err != nil {
		return nil, err
	}

	if res.Truth() {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf(format, x, y))
}

func assertNotIn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return assertBinary(b, args, kwargs, syntax.NOT_IN, "%s unexpectedly found in %s")
}

func assertLess(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		x, y starlark.Value
		msg  msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &x, "y", &y, "msg?", &msg); err != nil {
		return nil, err
	}

	ok, err := starlark.Compare(syntax.LT, x, y)
	if err != nil {
		return nil, err
	}

	if ok {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("%s not less than %s", x, y))
}

func assertIsInstance(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		v            starlark.Value
		expectedType string
		msg          msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "v", &v, "expected", &expectedType, "msg?", &msg); err != nil {
		return nil, err
	}

	if v.Type() == expectedType {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("%s is not an instance of %s", v, expectedType))
}

func assertRegex(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		text, expectedRegex string
		msg                 msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text, "expected", &expectedRegex, "msg?", &msg); err != nil {
		return nil, err
	}

	ok, err := regexSearch(text, expectedRegex)
	if err != nil {
		return nil, err
	}

	if ok {
		return assertionOk()
	}

	return assertionFail(msg, fmt.Sprintf("Regex didn't match: %s not found in %s", expectedRegex, text))
}

func regexSearch(text, expectedRegex string) (bool, error) {
	r, err := regexp.Compile(expectedRegex)
	if err != nil {
		return false, err
	}

	return r.FindStringIndex(text) != nil, nil
}

func assertRaises(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		fn     starlark.Callable
		errmsg string
		msg    msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn, "expected?", &errmsg, "msg?", &msg); err != nil {
		return nil, err
	}

	_, err := starlark.Call(thread, fn, nil, nil)
	if err != nil {
		if errmsg == "" || err.Error() == errmsg {
			return assertionOk()
		}
	}

	return assertionFail(msg, fmt.Sprintf("%s not raised", errmsg))
}

func assertRaisesRegex(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		fn            starlark.Callable
		expectedRegex string
		msg           msg
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn, "expected", &expectedRegex, "msg?", &msg); err != nil {
		return nil, err
	}

	_, err := starlark.Call(thread, fn, nil, nil)
	if err != nil {
		ok, err := regexSearch(err.Error(), expectedRegex)
		if err != nil {
			return nil, err
		}

		if ok {
			return assertionOk()
		}
	}

	return assertionFail(msg, fmt.Sprintf("%s not raised", expectedRegex))
}

func measureTime(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn); err != nil {
		return nil, err
	}

	start := time.Now()
	_, err := starlark.Call(thread, fn, nil, nil)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)
	return starlark.Float(elapsed.Seconds()), nil
}

func evalFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		code string
		vars *starlark.Dict
	)
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &code, &vars); err != nil {
		return nil, err
	}

	env := starlark.StringDict{}

	if vars != nil {
		iter := vars.Iterate()
		defer iter.Done()

		var k starlark.Value
		for iter.Next(&k) {
			v, ok, err := vars.Get(k)
			if err != nil {
				return nil, err
			}
			if ok {
				if s, ok := k.(starlark.String); ok {
					env[string(s)] = v
				} else {
					env[fmt.Sprint(k)] = v
				}
			}
		}
	}

	opts := syntax.FileOptions{
		Set:             true,
		While:           true,
		TopLevelControl: true,
		GlobalReassign:  true,
		Recursion:       true,
	}

	return starlark.EvalOptions(&opts, thread, "eval", code, env)
}

func tryCatchFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("%s: got %d arguments, want at least 1", b.Name(), len(args))
	}

	fn, ok := args[0].(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("got %s, want callable", args[0].Type())
	}

	res, err := fn.CallInternal(thread, args[1:], kwargs)
	if err != nil {
		return starlark.Tuple{starlark.None, starlark.String(err.Error())}, nil
	}

	return starlark.Tuple{res, starlark.None}, nil
}

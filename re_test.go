package re

import (
	_ "embed"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

//go:embed re_test.py
var reScript string

func TestRe(t *testing.T) {
	asserts := map[string]func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error){
		"same":           sameFunc,
		"measure":        measureTime,
		"eval":           evalFunc,
		"trycatch":       tryCatchFunc,
		"capture_output": captureOutput,
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

func sameFunc(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var x, y starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &x, &y); err != nil {
		return nil, err
	}

	return starlark.Bool(x == y), nil
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

func captureOutput(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fn starlark.Callable
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fn", &fn); err != nil {
		return nil, err
	}

	oldPrint := thread.Print

	var output strings.Builder
	thread.Print = func(thread *starlark.Thread, msg string) {
		output.WriteString(msg)
		output.WriteByte('\n')
	}

	_, err := fn.CallInternal(thread, nil, nil)
	thread.Print = oldPrint

	if err != nil {
		return nil, err
	}

	return starlark.String(output.String()), nil
}

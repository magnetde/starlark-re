package re

import (
	_ "embed"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	re "github.com/magnetde/starlark-re"
)

//go:embed re_test.py
var reScript string

// TestRe is the main function for regex tests.
// Tests must be defined within the `re_test.py` file and are interpreted here.
func TestRe(t *testing.T) {
	predeclared := starlark.StringDict{
		"re": re.NewModule(),
	}

	helpers := map[string]func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error){
		"same":           sameHelper,
		"measure":        measureHelper,
		"eval":           evalHelper,
		"trycatch":       tryCatchHelper,
		"capture_output": captureOutput,
	}

	for name, fn := range helpers {
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

// sameHelper tests, whether two Starlark values are identical.
func sameHelper(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var x, y starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &x, &y); err != nil {
		return nil, err
	}

	return starlark.Bool(x == y), nil
}

// measureHelper measures the duration of a call to a Starlark function in seconds.
func measureHelper(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

// evalHelper implements the `eval` builtin function from Python in Starlark.
func evalHelper(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
			if v, ok, err := vars.Get(k); err != nil {
				return nil, err
			} else if ok {
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

// tryCatchHelper is used to call a Starlark function without raising any errors that result in
// terminating the Starlark script. The function returns a tuple, `(v, e)`, where `v` is the
// returned value of the function and `e` is the error raised during execution. Exactly one of
// these two values is `None`.
func tryCatchHelper(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

// captureOutput calls a Starlark function and captures its output.
// The output is then returned as a string.
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

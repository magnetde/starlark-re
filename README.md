# starlark-re

Starlark-re is an implementation of Python's [re](https://docs.python.org/3/library/re.html)
module for [Starlark](https://github.com/google/starlark-go).
Its interface is almost entirely compatible with the Python module,
so please refer to the Python documentation to learn how to use Starlark-re.

## Getting started

The `re.NewModule()` function returns a new Starlark value, that represents the re module:

```go
import (
    "go.starlark.net/starlark"
    re "github.com/magnetde/starlark-re"
)

// Add the re module to the globals dict.
globals := starlark.StringDict{
    "re": re.NewModule(),
}

// Execute a Starlark program using the re module.
opts := &syntax.FileOptions{GlobalReassign:  true}
thread := &starlark.Thread{Name: "re thread"}
globals, err := starlark.ExecFileOptions(opts, thread, "example.star", nil, globals)
if err != nil { ... }
```

**example.star:**

```python
p = re.compile('(a(b)c)d')
m = p.match('abcd')
print(m.group(2, 1, 0))  # prints: ("b", "abc", "abcd")

m = re.match(r'(?P<first>\w+) (?P<last>\w+)', 'Jane Doe')
print(m.groupdict())  # prints: {"first": "Jane", "last": "Doe"}

s = re.split(r'[\W]+', 'Words, words, words.', 1)
print(s)  # prints: ["Words", "words, words."]

p = re.compile('(blue|white|red)')
s = p.subn('colour', 'blue socks and red shoes')
print(s)  # prints: ("colour socks and colour shoes", 2)

p = re.compile('section{ ( [^}]* ) }', re.VERBOSE)
s = p.sub(r'subsection{\1}','section{First} section{second}')
print(s)  # prints: subsection{First} subsection{second}

p = re.compile(r'\d+')
s = p.findall('12 drummers drumming, 11 pipers piping, 10 lords a-leaping')
print(s)  # prints: ["12", "11", "10"]

s = [m.span() for m in p.finditer('12 drummers drumming, 11 ... 10 ...')]
print(s)  # prints: [(0, 2), (22, 24), (29, 31)]

plusone = lambda m: str(int(m.group(0)) + 1)
s = p.sub(plusone, '4 + 7 = 13', 2)
print(s)  # prints: 5 + 8 = 13

re.purge()
```

Alternatively, the module can be initialized with other parameters:

```go
options := &ModuleOptions{
    DisableCache:    false,
    MaxCacheSize:    128,
    DisableFallback: true,
}

m := re.NewModuleOptions(options)
```

## How it works

When compiling a regular expression pattern, it is first parsed using a Go implementation of the Python regex parser.
This allows to raise the same error messages as the Python module does.
The parser yields a tree representation of the pattern, which is then checked for any elements
that are currently not supported by the default regex engine
([regexp.Regexp](https://pkg.go.dev/regexp)).
These unsupported elements include:
- lookahead and lookbehind: `(?=...)`, `(?<=...)`, `(?!...)` or `(?<!...)`
- backreferences: e.g, `\1` or `(?P=name)`
- conditional expression: `(?(id/name)yes-pattern|no-pattern)`
- repetition of type `{m,n}` where `m` or `n` exceeds 1000
- possessive repetition: `?+`, `*+`, `++`, `{...}+`

If the regular expression pattern does not include any unsupported elements, it is preprocessed and
then compiled with the default regex engine.
The preprocessor will make necessary modifications to literals, ranges and character classes in the
pattern so flags such as `re.UNICODE`, `re.IGNORECASE` or `re.ASCII` work exactly like expected.

In case that the regex pattern includes unsupported elements, the regex engine [regexp2.Regexp](https://pkg.go.dev/github.com/dlclark/regexp2),
that supports all of these elements except for possessive repeat, is used instead.
However, it should be noted that the using ´regexp2´ may result in higher runtimes,
so this engine is only used as a fallback when dealing with regex patterns that contain unsupported elements.
Compiled patterns are stored in an LRU cache.

The module was tested against all supported Python tests for the re module
(see [test_re.py](https://github.com/python/cpython/blob/main/Lib/test/test_re.py)).

## Limitations

Currently, there are some differences to the Python re module:

- The `re.LOCALE` flag has no effect.
- Positions are given as byte offsets instead of character offsets (which is the default for Go and Starlark).
- The fallback engine does not support the longest match search, so some matches starting at the same position may be not found.
  This may result in differing outcomes compared to Python, especially at the `fullmatch` function.
- The default regex engine does not match `\b` at unicode word boundaries, while the fallback engine does.
- There is no support for possessive repetion operators and `Pattern.scanner`.

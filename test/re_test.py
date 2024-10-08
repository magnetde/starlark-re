# This file contains tests for the Starlark re module.
# All tests where taken from CPython.
# See also https://github.com/python/cpython/blob/main/Lib/test/test_re.py
# The tests where modified to work with Starlark.



# Add dummy assignments to fix Python warnings.

re = re # type: ignore
MAXREPEAT = MAXREPEAT # type: ignore
WITH_CACHE = WITH_CACHE # type: ignore
WITH_FALLBACK = WITH_FALLBACK # type: ignore
same = same # type: ignore
measure = measure # type: ignore
trycatch = trycatch # type: ignore
fail = fail # type: ignore
capture_output = capture_output # type: ignore

# All assertion functions

def assertionFail(msg, standardMsg):
    if msg == None:
        fail(standardMsg)
    else:
        fail("%s : %s" % (standardMsg, msg))

def assertEqual(x, y, msg=None):
    if x != y: assertionFail(msg, "%r != %r" % (x, y))

def assertNotEqual(x, y, msg=None):
    if x == y: assertionFail(msg, "%r == %r" % (x, y))

def assertIs(x, y, msg=None):
    if not same(x, y): assertionFail(msg, "%r is not %r" % (x, y))

def assertIsNot(x, y, msg=None):
    if same(x, y): assertionFail(msg, "unexpectedly identical: %r" % x)

def assertIsNone(v, msg=None):
    if v != None: assertionFail(msg, "%r is not None" % v)

def assertIsNotNone(v, msg=None):
    if v == None: assertionFail(msg, "unexpectedly None" % v)

def assertFalse(v, msg=None):
    if v: assertionFail(msg, "%r is not false" % v)

def assertTrue(v, msg=None):
    if not v: assertionFail(msg, "%r is not true" % v)

def assertIn(x, y, msg=None):
    if x not in y: assertionFail(msg, "%r not found in %r" % (x, y))

def assertLess(x, y, msg=None):
    if not x < y: assertionFail(msg, "%r not less than %r" % (x, y))

def assertGreater(x, y, msg=None):
    if not x > y: assertionFail(msg, "%r not greater than %r" % (x, y))

def assertIsInstance(v, t, msg=None):
    if type(v) != t: assertionFail(msg, "%r is not an instance of %r" % (v, t))

def assertRegex(v, r, msg=None):
    if not re.search(r, v): assertionFail(msg, "Regex didn't match: %r not found in %r" % (v, r))

def assertRaises(v, e=None, msg=None):
    err = trycatch(v)[1]
    if err == None or (e != None and err != e):
        if e == None: e = "error"
        assertionFail(msg, "%r not raised" % e)

def assertRaisesRegex(v, e=None, msg=None):
    err = trycatch(v)[1]
    if err == None or not re.search(e, err):
        if e == None: e = "error"
        assertionFail(msg, "%r not raised" % e)

# replacement for str % (...)
def format(s, *args):
    r = re.compile(r'%(?P<flags>[-+#0])?(?P<width>\d+|\*)?(?:\.(?P<precision>\d+|\*))?(?P<length>[hljztL]|hh|ll)?(?P<specifier>[diuoxXfFeEgGaAcspn])')

    def to_base(n, b):
        res = ""
        while n:
            res += "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"[n % b]
            n //= b
        return res[::-1] or "0"

    index = [0] # workaround for missing nonlocal

    def subfmt(m):
        i = index[0]
        arg = args[i]
        if i > len(args): return repr(arg)

        flags, width, precision, length, specifier = m.groups()

        if specifier == "o": v = to_base(int(arg), 8)
        elif specifier == "x": v = to_base(int(arg), 16)
        else: fail("format:", flags, width, precision, length, specifier)

        if width != None:
            w = int(width)
            if w > 0:
                v = max(w - len(v), 0) * (" " if flags == None else flags) + v

        index[0] += 1
        return v

    return r.sub(subfmt, s)

# fix for starlark; bcat = bytes concat
def bcat(*args):
    l = []
    for arg in args:
        l += list(arg.elems())
    return bytes(l)


# Re test suite and benchmark suite v1.5

# The 3 possible outcomes for each pattern
[SUCCEED, FAIL, SYNTAX_ERROR] = range(3)

# Benchmark suite (needs expansion)
#
# The benchmark suite does not test correctness, just speed.  The
# first element of each tuple is the regex pattern; the second is a
# string to match it against.  The benchmarking code will embed the
# second string inside several sizes of padding, to test how regex
# matching performs on large strings.

benchmarks = [

    # test common prefix
    ('Python|Perl', 'Perl'),    # Alternation
    ('(Python|Perl)', 'Perl'),  # Grouped alternation

    ('Python|Perl|Tcl', 'Perl'),        # Alternation
    ('(Python|Perl|Tcl)', 'Perl'),      # Grouped alternation

    ('(Python)\\1', 'PythonPython'),    # Backreference
    ('([0a-z][a-z0-9]*,)+', 'a5,b7,c9,'), # Disable the fastmap optimization
    ('([a-z][a-z0-9]*,)+', 'a5,b7,c9,'), # A few sets

    ('Python', 'Python'),               # Simple text literal
    ('.*Python', 'Python'),             # Bad text literal
    ('.*Python.*', 'Python'),           # Worse text literal
    ('.*(Python)', 'Python'),           # Bad text literal with grouping

]

# Test suite (for verifying correctness)
#
# The test suite is a list of 5- or 3-tuples.  The 5 parts of a
# complete tuple are:
# element 0: a string containing the pattern
#         1: the string to match against the pattern
#         2: the expected result (SUCCEED, FAIL, SYNTAX_ERROR)
#         3: a string that will be eval()'ed to produce a test string.
#            This is an arbitrary Python expression; the available
#            variables are "found" (the whole match), and "g1", "g2", ...
#            up to "g99" contain the contents of each group, or the
#            string 'None' if the group wasn't given a value, or the
#            string 'Error' if the group index was out of range;
#            also "groups", the return value of m.group() (a tuple).
#         4: The expected result of evaluating the expression.
#            If the two don't match, an error is reported.
#
# If the regex isn't expected to work, the latter two elements can be omitted.

tests = [
    # Test ?P< and ?P= extensions
    ('(?P<foo_123', '', SYNTAX_ERROR),      # Unterminated group identifier
    ('(?P<1>a)', '', SYNTAX_ERROR),         # Begins with a digit
    ('(?P<!>a)', '', SYNTAX_ERROR),         # Begins with an illegal char
    ('(?P<foo!>a)', '', SYNTAX_ERROR),      # Begins with an illegal char

    # Same tests, for the ?P= form
    ('(?P<foo_123>a)(?P=foo_123', 'aa', SYNTAX_ERROR),
    ('(?P<foo_123>a)(?P=1)', 'aa', SYNTAX_ERROR),
    ('(?P<foo_123>a)(?P=!)', 'aa', SYNTAX_ERROR),
    ('(?P<foo_123>a)(?P=foo_124', 'aa', SYNTAX_ERROR),  # Backref to undefined group

    ('(?P<foo_123>a)', 'a', SUCCEED, 'g1', 'a'),
    ('(?P<foo_123>a)(?P=foo_123)', 'aa', SUCCEED, 'g1', 'a'),

    # Test octal escapes
    ('\\1', 'a', SYNTAX_ERROR),    # Backreference
    ('[\\1]', '\1', SUCCEED, 'found', '\1'),  # Character
    ('\\09', chr(0) + '9', SUCCEED, 'found', chr(0) + '9'),
    ('\\141', 'a', SUCCEED, 'found', 'a'),
    ('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)(k)(l)\\119', 'abcdefghijklk9', SUCCEED, 'found+"-"+g11', 'abcdefghijklk9-k'),

    # Test \0 is handled everywhere
    (r'\0', '\0', SUCCEED, 'found', '\0'),
    (r'[\0a]', '\0', SUCCEED, 'found', '\0'),
    (r'[a\0]', '\0', SUCCEED, 'found', '\0'),
    (r'[^a\0]', '\0', FAIL),

    # Test various letter escapes
    (r'\a[\b]\f\n\r\t\v', '\a\b\f\n\r\t\v', SUCCEED, 'found', '\a\b\f\n\r\t\v'),
    (r'[\a][\b][\f][\n][\r][\t][\v]', '\a\b\f\n\r\t\v', SUCCEED, 'found', '\a\b\f\n\r\t\v'),
    # NOTE: not an error under PCRE/PRE:
    (r'\u', '', SYNTAX_ERROR),    # A Perl escape
    # (r'\c\e\g\h\i\j\k\m\o\p\q\y\z', 'ceghijkmopqyz', SUCCEED, 'found', 'ceghijkmopqyz'),
    # new \x semantics
    (r'\x00ffffffffffffff', '\u00FF', FAIL, 'found', chr(255)),
    (r'\x00f', '\017', FAIL, 'found', chr(15)),
    (r'\x00fe', '\u00FE', FAIL, 'found', chr(254)),
    # (r'\x00ffffffffffffff', '\377', SUCCEED, 'found', chr(255)),
    # (r'\x00f', '\017', SUCCEED, 'found', chr(15)),
    # (r'\x00fe', '\376', SUCCEED, 'found', chr(254)),

    (r"^\w+=(\\[\000-\277]|[^\n\\])*", "SRC=eval.c g.c blah blah blah \\\\\n\tapes.c",
     SUCCEED, 'found', "SRC=eval.c g.c blah blah blah \\\\"),

    # Test that . only matches \n in DOTALL mode
    ('a.b', 'acb', SUCCEED, 'found', 'acb'),
    ('a.b', 'a\nb', FAIL),
    ('a.*b', 'acc\nccb', FAIL),
    ('a.{4,5}b', 'acc\nccb', FAIL),
    ('a.b', 'a\rb', SUCCEED, 'found', 'a\rb'),
    ('(?s)a.b', 'a\nb', SUCCEED, 'found', 'a\nb'),
    ('(?s)a.*b', 'acc\nccb', SUCCEED, 'found', 'acc\nccb'),
    ('(?s)a.{4,5}b', 'acc\nccb', SUCCEED, 'found', 'acc\nccb'),
    ('(?s)a.b', 'a\rb', SUCCEED, 'found', 'a\rb'),

    (')', '', SYNTAX_ERROR),           # Unmatched right bracket
    ('', '', SUCCEED, 'found', ''),    # Empty pattern
    ('abc', 'abc', SUCCEED, 'found', 'abc'),
    ('abc', 'xbc', FAIL),
    ('abc', 'axc', FAIL),
    ('abc', 'abx', FAIL),
    ('abc', 'xabcy', SUCCEED, 'found', 'abc'),
    ('abc', 'ababc', SUCCEED, 'found', 'abc'),
    ('ab*c', 'abc', SUCCEED, 'found', 'abc'),
    ('ab*bc', 'abc', SUCCEED, 'found', 'abc'),
    ('ab*bc', 'abbc', SUCCEED, 'found', 'abbc'),
    ('ab*bc', 'abbbbc', SUCCEED, 'found', 'abbbbc'),
    ('ab+bc', 'abbc', SUCCEED, 'found', 'abbc'),
    ('ab+bc', 'abc', FAIL),
    ('ab+bc', 'abq', FAIL),
    ('ab+bc', 'abbbbc', SUCCEED, 'found', 'abbbbc'),
    ('ab?bc', 'abbc', SUCCEED, 'found', 'abbc'),
    ('ab?bc', 'abc', SUCCEED, 'found', 'abc'),
    ('ab?bc', 'abbbbc', FAIL),
    ('ab?c', 'abc', SUCCEED, 'found', 'abc'),
    ('^abc$', 'abc', SUCCEED, 'found', 'abc'),
    ('^abc$', 'abcc', FAIL),
    ('^abc', 'abcc', SUCCEED, 'found', 'abc'),
    ('^abc$', 'aabc', FAIL),
    ('abc$', 'aabc', SUCCEED, 'found', 'abc'),
    ('^', 'abc', SUCCEED, 'found+"-"', '-'),
    ('$', 'abc', SUCCEED, 'found+"-"', '-'),
    ('a.c', 'abc', SUCCEED, 'found', 'abc'),
    ('a.c', 'axc', SUCCEED, 'found', 'axc'),
    ('a.*c', 'axyzc', SUCCEED, 'found', 'axyzc'),
    ('a.*c', 'axyzd', FAIL),
    ('a[bc]d', 'abc', FAIL),
    ('a[bc]d', 'abd', SUCCEED, 'found', 'abd'),
    ('a[b-d]e', 'abd', FAIL),
    ('a[b-d]e', 'ace', SUCCEED, 'found', 'ace'),
    ('a[b-d]', 'aac', SUCCEED, 'found', 'ac'),
    ('a[-b]', 'a-', SUCCEED, 'found', 'a-'),
    ('a[\\-b]', 'a-', SUCCEED, 'found', 'a-'),
    # NOTE: not an error under PCRE/PRE:
    # ('a[b-]', 'a-', SYNTAX_ERROR),
    ('a[]b', '-', SYNTAX_ERROR),
    ('a[', '-', SYNTAX_ERROR),
    ('a\\', '-', SYNTAX_ERROR),
    ('abc)', '-', SYNTAX_ERROR),
    ('(abc', '-', SYNTAX_ERROR),
    ('a]', 'a]', SUCCEED, 'found', 'a]'),
    ('a[]]b', 'a]b', SUCCEED, 'found', 'a]b'),
    ('a[\\]]b', 'a]b', SUCCEED, 'found', 'a]b'),
    ('a[^bc]d', 'aed', SUCCEED, 'found', 'aed'),
    ('a[^bc]d', 'abd', FAIL),
    ('a[^-b]c', 'adc', SUCCEED, 'found', 'adc'),
    ('a[^-b]c', 'a-c', FAIL),
    ('a[^]b]c', 'a]c', FAIL),
    ('a[^]b]c', 'adc', SUCCEED, 'found', 'adc'),
    ('\\ba\\b', 'a-', SUCCEED, '"-"', '-'),
    ('\\ba\\b', '-a', SUCCEED, '"-"', '-'),
    ('\\ba\\b', '-a-', SUCCEED, '"-"', '-'),
    ('\\by\\b', 'xy', FAIL),
    ('\\by\\b', 'yz', FAIL),
    ('\\by\\b', 'xyz', FAIL),
    ('x\\b', 'xyz', FAIL),
    ('x\\B', 'xyz', SUCCEED, '"-"', '-'),
    ('\\Bz', 'xyz', SUCCEED, '"-"', '-'),
    ('z\\B', 'xyz', FAIL),
    ('\\Bx', 'xyz', FAIL),
    ('\\Ba\\B', 'a-', FAIL, '"-"', '-'),
    ('\\Ba\\B', '-a', FAIL, '"-"', '-'),
    ('\\Ba\\B', '-a-', FAIL, '"-"', '-'),
    ('\\By\\B', 'xy', FAIL),
    ('\\By\\B', 'yz', FAIL),
    ('\\By\\b', 'xy', SUCCEED, '"-"', '-'),
    ('\\by\\B', 'yz', SUCCEED, '"-"', '-'),
    ('\\By\\B', 'xyz', SUCCEED, '"-"', '-'),
    ('ab|cd', 'abc', SUCCEED, 'found', 'ab'),
    ('ab|cd', 'abcd', SUCCEED, 'found', 'ab'),
    ('()ef', 'def', SUCCEED, 'found+"-"+g1', 'ef-'),
    ('$b', 'b', FAIL),
    ('a\\(b', 'a(b', SUCCEED, 'found+"-"+g1', 'a(b-Error'),
    ('a\\(*b', 'ab', SUCCEED, 'found', 'ab'),
    ('a\\(*b', 'a((b', SUCCEED, 'found', 'a((b'),
    ('a\\\\b', 'a\\b', SUCCEED, 'found', 'a\\b'),
    ('((a))', 'abc', SUCCEED, 'found+"-"+g1+"-"+g2', 'a-a-a'),
    ('(a)b(c)', 'abc', SUCCEED, 'found+"-"+g1+"-"+g2', 'abc-a-c'),
    ('a+b+c', 'aabbabc', SUCCEED, 'found', 'abc'),
    ('(a+|b)*', 'ab', SUCCEED, 'found+"-"+g1', 'ab-b'),
    ('(a+|b)+', 'ab', SUCCEED, 'found+"-"+g1', 'ab-b'),
    ('(a+|b)?', 'ab', SUCCEED, 'found+"-"+g1', 'a-a'),
    (')(', '-', SYNTAX_ERROR),
    ('[^ab]*', 'cde', SUCCEED, 'found', 'cde'),
    ('abc', '', FAIL),
    ('a*', '', SUCCEED, 'found', ''),
    ('a|b|c|d|e', 'e', SUCCEED, 'found', 'e'),
    ('(a|b|c|d|e)f', 'ef', SUCCEED, 'found+"-"+g1', 'ef-e'),
    ('abcd*efg', 'abcdefg', SUCCEED, 'found', 'abcdefg'),
    ('ab*', 'xabyabbbz', SUCCEED, 'found', 'ab'),
    ('ab*', 'xayabbbz', SUCCEED, 'found', 'a'),
    ('(ab|cd)e', 'abcde', SUCCEED, 'found+"-"+g1', 'cde-cd'),
    ('[abhgefdc]ij', 'hij', SUCCEED, 'found', 'hij'),
    ('^(ab|cd)e', 'abcde', FAIL, 'xg1y', 'xy'),
    ('(abc|)ef', 'abcdef', SUCCEED, 'found+"-"+g1', 'ef-'),
    ('(a|b)c*d', 'abcd', SUCCEED, 'found+"-"+g1', 'bcd-b'),
    ('(ab|ab*)bc', 'abc', SUCCEED, 'found+"-"+g1', 'abc-a'),
    ('a([bc]*)c*', 'abc', SUCCEED, 'found+"-"+g1', 'abc-bc'),
    ('a([bc]*)(c*d)', 'abcd', SUCCEED, 'found+"-"+g1+"-"+g2', 'abcd-bc-d'),
    ('a([bc]+)(c*d)', 'abcd', SUCCEED, 'found+"-"+g1+"-"+g2', 'abcd-bc-d'),
    ('a([bc]*)(c+d)', 'abcd', SUCCEED, 'found+"-"+g1+"-"+g2', 'abcd-b-cd'),
    ('a[bcd]*dcdcde', 'adcdcde', SUCCEED, 'found', 'adcdcde'),
    ('a[bcd]+dcdcde', 'adcdcde', FAIL),
    ('(ab|a)b*c', 'abc', SUCCEED, 'found+"-"+g1', 'abc-ab'),
    ('((a)(b)c)(d)', 'abcd', SUCCEED, 'g1+"-"+g2+"-"+g3+"-"+g4', 'abc-a-b-d'),
    ('[a-zA-Z_][a-zA-Z0-9_]*', 'alpha', SUCCEED, 'found', 'alpha'),
    ('^a(bc+|b[eh])g|.h$', 'abh', SUCCEED, 'found+"-"+g1', 'bh-None'),
    ('(bc+d$|ef*g.|h?i(j|k))', 'effgz', SUCCEED, 'found+"-"+g1+"-"+g2', 'effgz-effgz-None'),
    ('(bc+d$|ef*g.|h?i(j|k))', 'ij', SUCCEED, 'found+"-"+g1+"-"+g2', 'ij-ij-j'),
    ('(bc+d$|ef*g.|h?i(j|k))', 'effg', FAIL),
    ('(bc+d$|ef*g.|h?i(j|k))', 'bcdd', FAIL),
    ('(bc+d$|ef*g.|h?i(j|k))', 'reffgz', SUCCEED, 'found+"-"+g1+"-"+g2', 'effgz-effgz-None'),
    ('(((((((((a)))))))))', 'a', SUCCEED, 'found', 'a'),
    ('multiple words of text', 'uh-uh', FAIL),
    ('multiple words', 'multiple words, yeah', SUCCEED, 'found', 'multiple words'),
    ('(.*)c(.*)', 'abcde', SUCCEED, 'found+"-"+g1+"-"+g2', 'abcde-ab-de'),
    ('\\((.*), (.*)\\)', '(a, b)', SUCCEED, 'g2+"-"+g1', 'b-a'),
    ('[k]', 'ab', FAIL),
    ('a[-]?c', 'ac', SUCCEED, 'found', 'ac'),
    ('(abc)\\1', 'abcabc', SUCCEED, 'g1', 'abc'),
    ('([a-c]*)\\1', 'abcabc', SUCCEED, 'g1', 'abc'),
    ('^(.+)?B', 'AB', SUCCEED, 'g1', 'A'),
    ('(a+).\\1$', 'aaaaa', SUCCEED, 'found+"-"+g1', 'aaaaa-aa'),
    ('^(a+).\\1$', 'aaaa', FAIL),
    ('(abc)\\1', 'abcabc', SUCCEED, 'found+"-"+g1', 'abcabc-abc'),
    ('([a-c]+)\\1', 'abcabc', SUCCEED, 'found+"-"+g1', 'abcabc-abc'),
    ('(a)\\1', 'aa', SUCCEED, 'found+"-"+g1', 'aa-a'),
    ('(a+)\\1', 'aa', SUCCEED, 'found+"-"+g1', 'aa-a'),
    ('(a+)+\\1', 'aa', SUCCEED, 'found+"-"+g1', 'aa-a'),
    ('(a).+\\1', 'aba', SUCCEED, 'found+"-"+g1', 'aba-a'),
    ('(a)ba*\\1', 'aba', SUCCEED, 'found+"-"+g1', 'aba-a'),
    ('(aa|a)a\\1$', 'aaa', SUCCEED, 'found+"-"+g1', 'aaa-a'),
    ('(a|aa)a\\1$', 'aaa', SUCCEED, 'found+"-"+g1', 'aaa-a'),
    ('(a+)a\\1$', 'aaa', SUCCEED, 'found+"-"+g1', 'aaa-a'),
    ('([abc]*)\\1', 'abcabc', SUCCEED, 'found+"-"+g1', 'abcabc-abc'),
    ('(a)(b)c|ab', 'ab', SUCCEED, 'found+"-"+g1+"-"+g2', 'ab-None-None'),
    ('(a)+x', 'aaax', SUCCEED, 'found+"-"+g1', 'aaax-a'),
    ('([ac])+x', 'aacx', SUCCEED, 'found+"-"+g1', 'aacx-c'),
    ('([^/]*/)*sub1/', 'd:msgs/tdir/sub1/trial/away.cpp', SUCCEED, 'found+"-"+g1', 'd:msgs/tdir/sub1/-tdir/'),
    ('([^.]*)\\.([^:]*):[T ]+(.*)', 'track1.title:TBlah blah blah', SUCCEED, 'found+"-"+g1+"-"+g2+"-"+g3', 'track1.title:TBlah blah blah-track1-title-Blah blah blah'),
    ('([^N]*N)+', 'abNNxyzN', SUCCEED, 'found+"-"+g1', 'abNNxyzN-xyzN'),
    ('([^N]*N)+', 'abNNxyz', SUCCEED, 'found+"-"+g1', 'abNN-N'),
    ('([abc]*)x', 'abcx', SUCCEED, 'found+"-"+g1', 'abcx-abc'),
    ('([abc]*)x', 'abc', FAIL),
    ('([xyz]*)x', 'abcx', SUCCEED, 'found+"-"+g1', 'x-'),
    ('(a)+b|aac', 'aac', SUCCEED, 'found+"-"+g1', 'aac-None'),

    # Test symbolic groups

    ('(?P<i d>aaa)a', 'aaaa', SYNTAX_ERROR),
    ('(?P<id>aaa)a', 'aaaa', SUCCEED, 'found+"-"+id', 'aaaa-aaa'),
    ('(?P<id>aa)(?P=id)', 'aaaa', SUCCEED, 'found+"-"+id', 'aaaa-aa'),
    ('(?P<id>aa)(?P=xd)', 'aaaa', SYNTAX_ERROR),

    # Test octal escapes/memory references

    ('\\1', 'a', SYNTAX_ERROR),

    # All tests from Perl

    ('ab{0,}bc', 'abbbbc', SUCCEED, 'found', 'abbbbc'),
    ('ab{1,}bc', 'abq', FAIL),
    ('ab{1,}bc', 'abbbbc', SUCCEED, 'found', 'abbbbc'),
    ('ab{1,3}bc', 'abbbbc', SUCCEED, 'found', 'abbbbc'),
    ('ab{3,4}bc', 'abbbbc', SUCCEED, 'found', 'abbbbc'),
    ('ab{4,5}bc', 'abbbbc', FAIL),
    ('ab{0,1}bc', 'abc', SUCCEED, 'found', 'abc'),
    ('ab{0,1}c', 'abc', SUCCEED, 'found', 'abc'),
    ('^', 'abc', SUCCEED, 'found', ''),
    ('$', 'abc', SUCCEED, 'found', ''),
    ('a[b-]', 'a-', SUCCEED, 'found', 'a-'),
    ('a[b-a]', '-', SYNTAX_ERROR),
    ('*a', '-', SYNTAX_ERROR),
    ('(*)b', '-', SYNTAX_ERROR),
    ('a{1,}b{1,}c', 'aabbabc', SUCCEED, 'found', 'abc'),
    ('a**', '-', SYNTAX_ERROR),
    ('a.+?c', 'abcabc', SUCCEED, 'found', 'abc'),
    ('(a+|b){0,}', 'ab', SUCCEED, 'found+"-"+g1', 'ab-b'),
    ('(a+|b){1,}', 'ab', SUCCEED, 'found+"-"+g1', 'ab-b'),
    ('(a+|b){0,1}', 'ab', SUCCEED, 'found+"-"+g1', 'a-a'),
    ('([abc])*d', 'abbbcd', SUCCEED, 'found+"-"+g1', 'abbbcd-c'),
    ('([abc])*bcd', 'abcd', SUCCEED, 'found+"-"+g1', 'abcd-a'),
    ('^(ab|cd)e', 'abcde', FAIL),
    ('((((((((((a))))))))))', 'a', SUCCEED, 'g10', 'a'),
    ('((((((((((a))))))))))\\10', 'aa', SUCCEED, 'found', 'aa'),
# Python does not have the same rules for \\41 so this is a syntax error
#    ('((((((((((a))))))))))\\41', 'aa', FAIL),
#    ('((((((((((a))))))))))\\41', 'a!', SUCCEED, 'found', 'a!'),
    ('((((((((((a))))))))))\\41', '', SYNTAX_ERROR),
    ('(?i)((((((((((a))))))))))\\41', '', SYNTAX_ERROR),
    ('(?i)abc', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)abc', 'XBC', FAIL),
    ('(?i)abc', 'AXC', FAIL),
    ('(?i)abc', 'ABX', FAIL),
    ('(?i)abc', 'XABCY', SUCCEED, 'found', 'ABC'),
    ('(?i)abc', 'ABABC', SUCCEED, 'found', 'ABC'),
    ('(?i)ab*c', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)ab*bc', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)ab*bc', 'ABBC', SUCCEED, 'found', 'ABBC'),
    ('(?i)ab*?bc', 'ABBBBC', SUCCEED, 'found', 'ABBBBC'),
    ('(?i)ab{0,}?bc', 'ABBBBC', SUCCEED, 'found', 'ABBBBC'),
    ('(?i)ab+?bc', 'ABBC', SUCCEED, 'found', 'ABBC'),
    ('(?i)ab+bc', 'ABC', FAIL),
    ('(?i)ab+bc', 'ABQ', FAIL),
    ('(?i)ab{1,}bc', 'ABQ', FAIL),
    ('(?i)ab+bc', 'ABBBBC', SUCCEED, 'found', 'ABBBBC'),
    ('(?i)ab{1,}?bc', 'ABBBBC', SUCCEED, 'found', 'ABBBBC'),
    ('(?i)ab{1,3}?bc', 'ABBBBC', SUCCEED, 'found', 'ABBBBC'),
    ('(?i)ab{3,4}?bc', 'ABBBBC', SUCCEED, 'found', 'ABBBBC'),
    ('(?i)ab{4,5}?bc', 'ABBBBC', FAIL),
    ('(?i)ab??bc', 'ABBC', SUCCEED, 'found', 'ABBC'),
    ('(?i)ab??bc', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)ab{0,1}?bc', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)ab??bc', 'ABBBBC', FAIL),
    ('(?i)ab??c', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)ab{0,1}?c', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)^abc$', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)^abc$', 'ABCC', FAIL),
    ('(?i)^abc', 'ABCC', SUCCEED, 'found', 'ABC'),
    ('(?i)^abc$', 'AABC', FAIL),
    ('(?i)abc$', 'AABC', SUCCEED, 'found', 'ABC'),
    ('(?i)^', 'ABC', SUCCEED, 'found', ''),
    ('(?i)$', 'ABC', SUCCEED, 'found', ''),
    ('(?i)a.c', 'ABC', SUCCEED, 'found', 'ABC'),
    ('(?i)a.c', 'AXC', SUCCEED, 'found', 'AXC'),
    ('(?i)a.*?c', 'AXYZC', SUCCEED, 'found', 'AXYZC'),
    ('(?i)a.*c', 'AXYZD', FAIL),
    ('(?i)a[bc]d', 'ABC', FAIL),
    ('(?i)a[bc]d', 'ABD', SUCCEED, 'found', 'ABD'),
    ('(?i)a[b-d]e', 'ABD', FAIL),
    ('(?i)a[b-d]e', 'ACE', SUCCEED, 'found', 'ACE'),
    ('(?i)a[b-d]', 'AAC', SUCCEED, 'found', 'AC'),
    ('(?i)a[-b]', 'A-', SUCCEED, 'found', 'A-'),
    ('(?i)a[b-]', 'A-', SUCCEED, 'found', 'A-'),
    ('(?i)a[b-a]', '-', SYNTAX_ERROR),
    ('(?i)a[]b', '-', SYNTAX_ERROR),
    ('(?i)a[', '-', SYNTAX_ERROR),
    ('(?i)a]', 'A]', SUCCEED, 'found', 'A]'),
    ('(?i)a[]]b', 'A]B', SUCCEED, 'found', 'A]B'),
    ('(?i)a[^bc]d', 'AED', SUCCEED, 'found', 'AED'),
    ('(?i)a[^bc]d', 'ABD', FAIL),
    ('(?i)a[^-b]c', 'ADC', SUCCEED, 'found', 'ADC'),
    ('(?i)a[^-b]c', 'A-C', FAIL),
    ('(?i)a[^]b]c', 'A]C', FAIL),
    ('(?i)a[^]b]c', 'ADC', SUCCEED, 'found', 'ADC'),
    ('(?i)ab|cd', 'ABC', SUCCEED, 'found', 'AB'),
    ('(?i)ab|cd', 'ABCD', SUCCEED, 'found', 'AB'),
    ('(?i)()ef', 'DEF', SUCCEED, 'found+"-"+g1', 'EF-'),
    ('(?i)*a', '-', SYNTAX_ERROR),
    ('(?i)(*)b', '-', SYNTAX_ERROR),
    ('(?i)$b', 'B', FAIL),
    ('(?i)a\\', '-', SYNTAX_ERROR),
    ('(?i)a\\(b', 'A(B', SUCCEED, 'found+"-"+g1', 'A(B-Error'),
    ('(?i)a\\(*b', 'AB', SUCCEED, 'found', 'AB'),
    ('(?i)a\\(*b', 'A((B', SUCCEED, 'found', 'A((B'),
    ('(?i)a\\\\b', 'A\\B', SUCCEED, 'found', 'A\\B'),
    ('(?i)abc)', '-', SYNTAX_ERROR),
    ('(?i)(abc', '-', SYNTAX_ERROR),
    ('(?i)((a))', 'ABC', SUCCEED, 'found+"-"+g1+"-"+g2', 'A-A-A'),
    ('(?i)(a)b(c)', 'ABC', SUCCEED, 'found+"-"+g1+"-"+g2', 'ABC-A-C'),
    ('(?i)a+b+c', 'AABBABC', SUCCEED, 'found', 'ABC'),
    ('(?i)a{1,}b{1,}c', 'AABBABC', SUCCEED, 'found', 'ABC'),
    ('(?i)a**', '-', SYNTAX_ERROR),
    ('(?i)a.+?c', 'ABCABC', SUCCEED, 'found', 'ABC'),
    ('(?i)a.*?c', 'ABCABC', SUCCEED, 'found', 'ABC'),
    ('(?i)a.{0,5}?c', 'ABCABC', SUCCEED, 'found', 'ABC'),
    ('(?i)(a+|b)*', 'AB', SUCCEED, 'found+"-"+g1', 'AB-B'),
    ('(?i)(a+|b){0,}', 'AB', SUCCEED, 'found+"-"+g1', 'AB-B'),
    ('(?i)(a+|b)+', 'AB', SUCCEED, 'found+"-"+g1', 'AB-B'),
    ('(?i)(a+|b){1,}', 'AB', SUCCEED, 'found+"-"+g1', 'AB-B'),
    ('(?i)(a+|b)?', 'AB', SUCCEED, 'found+"-"+g1', 'A-A'),
    ('(?i)(a+|b){0,1}', 'AB', SUCCEED, 'found+"-"+g1', 'A-A'),
    ('(?i)(a+|b){0,1}?', 'AB', SUCCEED, 'found+"-"+g1', '-None'),
    ('(?i))(', '-', SYNTAX_ERROR),
    ('(?i)[^ab]*', 'CDE', SUCCEED, 'found', 'CDE'),
    ('(?i)abc', '', FAIL),
    ('(?i)a*', '', SUCCEED, 'found', ''),
    ('(?i)([abc])*d', 'ABBBCD', SUCCEED, 'found+"-"+g1', 'ABBBCD-C'),
    ('(?i)([abc])*bcd', 'ABCD', SUCCEED, 'found+"-"+g1', 'ABCD-A'),
    ('(?i)a|b|c|d|e', 'E', SUCCEED, 'found', 'E'),
    ('(?i)(a|b|c|d|e)f', 'EF', SUCCEED, 'found+"-"+g1', 'EF-E'),
    ('(?i)abcd*efg', 'ABCDEFG', SUCCEED, 'found', 'ABCDEFG'),
    ('(?i)ab*', 'XABYABBBZ', SUCCEED, 'found', 'AB'),
    ('(?i)ab*', 'XAYABBBZ', SUCCEED, 'found', 'A'),
    ('(?i)(ab|cd)e', 'ABCDE', SUCCEED, 'found+"-"+g1', 'CDE-CD'),
    ('(?i)[abhgefdc]ij', 'HIJ', SUCCEED, 'found', 'HIJ'),
    ('(?i)^(ab|cd)e', 'ABCDE', FAIL),
    ('(?i)(abc|)ef', 'ABCDEF', SUCCEED, 'found+"-"+g1', 'EF-'),
    ('(?i)(a|b)c*d', 'ABCD', SUCCEED, 'found+"-"+g1', 'BCD-B'),
    ('(?i)(ab|ab*)bc', 'ABC', SUCCEED, 'found+"-"+g1', 'ABC-A'),
    ('(?i)a([bc]*)c*', 'ABC', SUCCEED, 'found+"-"+g1', 'ABC-BC'),
    ('(?i)a([bc]*)(c*d)', 'ABCD', SUCCEED, 'found+"-"+g1+"-"+g2', 'ABCD-BC-D'),
    ('(?i)a([bc]+)(c*d)', 'ABCD', SUCCEED, 'found+"-"+g1+"-"+g2', 'ABCD-BC-D'),
    ('(?i)a([bc]*)(c+d)', 'ABCD', SUCCEED, 'found+"-"+g1+"-"+g2', 'ABCD-B-CD'),
    ('(?i)a[bcd]*dcdcde', 'ADCDCDE', SUCCEED, 'found', 'ADCDCDE'),
    ('(?i)a[bcd]+dcdcde', 'ADCDCDE', FAIL),
    ('(?i)(ab|a)b*c', 'ABC', SUCCEED, 'found+"-"+g1', 'ABC-AB'),
    ('(?i)((a)(b)c)(d)', 'ABCD', SUCCEED, 'g1+"-"+g2+"-"+g3+"-"+g4', 'ABC-A-B-D'),
    ('(?i)[a-zA-Z_][a-zA-Z0-9_]*', 'ALPHA', SUCCEED, 'found', 'ALPHA'),
    ('(?i)^a(bc+|b[eh])g|.h$', 'ABH', SUCCEED, 'found+"-"+g1', 'BH-None'),
    ('(?i)(bc+d$|ef*g.|h?i(j|k))', 'EFFGZ', SUCCEED, 'found+"-"+g1+"-"+g2', 'EFFGZ-EFFGZ-None'),
    ('(?i)(bc+d$|ef*g.|h?i(j|k))', 'IJ', SUCCEED, 'found+"-"+g1+"-"+g2', 'IJ-IJ-J'),
    ('(?i)(bc+d$|ef*g.|h?i(j|k))', 'EFFG', FAIL),
    ('(?i)(bc+d$|ef*g.|h?i(j|k))', 'BCDD', FAIL),
    ('(?i)(bc+d$|ef*g.|h?i(j|k))', 'REFFGZ', SUCCEED, 'found+"-"+g1+"-"+g2', 'EFFGZ-EFFGZ-None'),
    ('(?i)((((((((((a))))))))))', 'A', SUCCEED, 'g10', 'A'),
    ('(?i)((((((((((a))))))))))\\10', 'AA', SUCCEED, 'found', 'AA'),
    #('(?i)((((((((((a))))))))))\\41', 'AA', FAIL),
    #('(?i)((((((((((a))))))))))\\41', 'A!', SUCCEED, 'found', 'A!'),
    ('(?i)(((((((((a)))))))))', 'A', SUCCEED, 'found', 'A'),
    ('(?i)(?:(?:(?:(?:(?:(?:(?:(?:(?:(a))))))))))', 'A', SUCCEED, 'g1', 'A'),
    ('(?i)(?:(?:(?:(?:(?:(?:(?:(?:(?:(a|b|c))))))))))', 'C', SUCCEED, 'g1', 'C'),
    ('(?i)multiple words of text', 'UH-UH', FAIL),
    ('(?i)multiple words', 'MULTIPLE WORDS, YEAH', SUCCEED, 'found', 'MULTIPLE WORDS'),
    ('(?i)(.*)c(.*)', 'ABCDE', SUCCEED, 'found+"-"+g1+"-"+g2', 'ABCDE-AB-DE'),
    ('(?i)\\((.*), (.*)\\)', '(A, B)', SUCCEED, 'g2+"-"+g1', 'B-A'),
    ('(?i)[k]', 'AB', FAIL),
#    ('(?i)abcd', 'ABCD', SUCCEED, 'found+"-"+\\found+"-"+\\\\found', 'ABCD-$&-\\ABCD'),
#    ('(?i)a(bc)d', 'ABCD', SUCCEED, 'g1+"-"+\\g1+"-"+\\\\g1', 'BC-$1-\\BC'),
    ('(?i)a[-]?c', 'AC', SUCCEED, 'found', 'AC'),
    ('(?i)(abc)\\1', 'ABCABC', SUCCEED, 'g1', 'ABC'),
    ('(?i)([a-c]*)\\1', 'ABCABC', SUCCEED, 'g1', 'ABC'),
    ('a(?!b).', 'abad', SUCCEED, 'found', 'ad'),
    ('a(?=d).', 'abad', SUCCEED, 'found', 'ad'),
    ('a(?=c|d).', 'abad', SUCCEED, 'found', 'ad'),
    ('a(?:b|c|d)(.)', 'ace', SUCCEED, 'g1', 'e'),
    ('a(?:b|c|d)*(.)', 'ace', SUCCEED, 'g1', 'e'),
    ('a(?:b|c|d)+?(.)', 'ace', SUCCEED, 'g1', 'e'),
    ('a(?:b|(c|e){1,2}?|d)+?(.)', 'ace', SUCCEED, 'g1 + g2', 'ce'),

    # lookbehind: split by : but not if it is escaped by -.
    ('(?<!-):(.*?)(?<!-):', 'a:bc-:de:f', SUCCEED, 'g1', 'bc-:de' ),
    # escaping with \ as we know it
    ('(?<!\\\\):(.*?)(?<!\\\\):', 'a:bc\\:de:f', SUCCEED, 'g1', 'bc\\:de' ),
    # terminating with ' and escaping with ? as in edifact
    ("(?<!\\?)'(.*?)(?<!\\?)'", "a'bc?'de'f", SUCCEED, 'g1', "bc?'de" ),

    # Comments using the (?#...) syntax

    ('w(?# comment', 'w', SYNTAX_ERROR),
    ('w(?# comment 1)xy(?# comment 2)z', 'wxyz', SUCCEED, 'found', 'wxyz'),

    # Check odd placement of embedded pattern modifiers

    # not an error under PCRE/PRE:
    ('(?i)w', 'W', SUCCEED, 'found', 'W'),
    # ('w(?i)', 'W', SYNTAX_ERROR),

    # Comments using the x embedded pattern modifier

    ("""(?x)w# comment 1
        x y
        # comment 2
        z""", 'wxyz', SUCCEED, 'found', 'wxyz'),

    # using the m embedded pattern modifier

    ('^abc', """jkl
abc
xyz""", FAIL),
    ('(?m)^abc', """jkl
abc
xyz""", SUCCEED, 'found', 'abc'),

    ('(?m)abc$', """jkl
xyzabc
123""", SUCCEED, 'found', 'abc'),



    # test \w, etc. both inside and outside character classes

    ('\\w+', '--ab_cd0123--', SUCCEED, 'found', 'ab_cd0123'),
    ('[\\w]+', '--ab_cd0123--', SUCCEED, 'found', 'ab_cd0123'),
    ('\\D+', '1234abc5678', SUCCEED, 'found', 'abc'),
    ('[\\D]+', '1234abc5678', SUCCEED, 'found', 'abc'),
    ('[\\da-fA-F]+', '123abc', SUCCEED, 'found', '123abc'),
    # not an error under PCRE/PRE:
    # ('[\\d-x]', '-', SYNTAX_ERROR),
    (r'([\s]*)([\S]*)([\s]*)', ' testing!1972', SUCCEED, 'g3+g2+g1', 'testing!1972 '),
    (r'(\s*)(\S*)(\s*)', ' testing!1972', SUCCEED, 'g3+g2+g1', 'testing!1972 '),

    (r'\xff', '\u00FF', SUCCEED, 'found', chr(255)),
    # new \x semantics
    (r'\x00ff', '\u00FF', FAIL),
    # (r'\x00ff', '\377', SUCCEED, 'found', chr(255)),
    (r'\t\n\v\r\f\a', '\t\n\v\r\f\a', SUCCEED, 'found', '\t\n\v\r\f\a'),
    ('\t\n\v\r\f\a', '\t\n\v\r\f\a', SUCCEED, 'found', '\t\n\v\r\f\a'),
    (r'\t\n\v\r\f\a', '\t\n\v\r\f\a', SUCCEED, 'found', chr(9)+chr(10)+chr(11)+chr(13)+chr(12)+chr(7)),
    (r'[\t][\n][\v][\r][\f][\b]', '\t\n\v\r\f\b', SUCCEED, 'found', '\t\n\v\r\f\b'),

    #
    # post-1.5.2 additions

    # xmllib problem
    (r'(([a-z]+):)?([a-z]+)$', 'smil', SUCCEED, 'g1+"-"+g2+"-"+g3', 'None-None-smil'),
    # bug 110866: reference to undefined group
    (r'((.)\1+)', '', SYNTAX_ERROR),
    # bug 111869: search (PRE/PCRE fails on this one, SRE doesn't)
    (r'.*d', 'abc\nabd', SUCCEED, 'found', 'abd'),
    # bug 112468: various expected syntax errors
    (r'(', '', SYNTAX_ERROR),
    (r'[\41]', '!', SUCCEED, 'found', '!'),
    # bug 114033: nothing to repeat
    (r'(x?)?', 'x', SUCCEED, 'found', 'x'),
    # bug 115040: rescan if flags are modified inside pattern
    (r'(?x) foo ', 'foo', SUCCEED, 'found', 'foo'),
    # bug 115618: negative lookahead
    (r'(?<!abc)(d.f)', 'abcdefdof', SUCCEED, 'found', 'dof'),
    # bug 116251: character class bug
    (r'[\w-]+', 'laser_beam', SUCCEED, 'found', 'laser_beam'),
    # bug 123769+127259: non-greedy backtracking bug
    (r'.*?\S *:', 'xx:', SUCCEED, 'found', 'xx:'),
    (r'a[ ]*?\ (\d+).*', 'a   10', SUCCEED, 'found', 'a   10'),
    (r'a[ ]*?\ (\d+).*', 'a    10', SUCCEED, 'found', 'a    10'),
    # bug 127259: \Z shouldn't depend on multiline mode
    (r'(?ms).*?x\s*\Z(.*)','xx\nx\n', SUCCEED, 'g1', ''),
    # bug 128899: uppercase literals under the ignorecase flag
    (r'(?i)M+', 'MMM', SUCCEED, 'found', 'MMM'),
    (r'(?i)m+', 'MMM', SUCCEED, 'found', 'MMM'),
    (r'(?i)[M]+', 'MMM', SUCCEED, 'found', 'MMM'),
    (r'(?i)[m]+', 'MMM', SUCCEED, 'found', 'MMM'),
    # bug 130748: ^* should be an error (nothing to repeat)
    (r'^*', '', SYNTAX_ERROR),
    # bug 133283: minimizing repeat problem
    (r'"(?:\\"|[^"])*?"', r'"\""', SUCCEED, 'found', r'"\""'),
    # bug 477728: minimizing repeat problem
    (r'^.*?$', 'one\ntwo\nthree\n', FAIL),
    # bug 483789: minimizing repeat problem
    (r'a[^>]*?b', 'a>b', FAIL),
    # bug 490573: minimizing repeat problem
    (r'^a*?$', 'foo', FAIL),
    # bug 470582: nested groups problem
    (r'^((a)c)?(ab)$', 'ab', SUCCEED, 'g1+"-"+g2+"-"+g3', 'None-None-ab'),
    # another minimizing repeat problem (capturing groups in assertions)
    ('^([ab]*?)(?=(b)?)c', 'abc', SUCCEED, 'g1+"-"+g2', 'ab-None'),
    ('^([ab]*?)(?!(b))c', 'abc', SUCCEED, 'g1+"-"+g2', 'ab-None'),
    ('^([ab]*?)(?<!(a))c', 'abc', SUCCEED, 'g1+"-"+g2', 'ab-None'),
]

u = '\u00C4'
tests.extend([
    # bug 410271: \b broken under locales
    # (r'\b.\b', 'a', SUCCEED, 'found', 'a'),
    # (r'(?u)\b.\b', u, SUCCEED, 'found', u),
    # (r'(?u)\w', u, SUCCEED, 'found', u),
])


def assertTypedEqual(actual, expect, msg=None):
    assertEqual(actual, expect, msg)
    def recurse(actual, expect):
        if type(expect) in ("tuple", "list"):
            for x, y in zip(actual, expect):
                recurse(x, y)
        else:
            assertIs(type(actual), type(expect), msg)
    recurse(actual, expect)

def checkPatternError(pattern, errmsg, pos=None):
    _, err = trycatch(lambda: re.compile(pattern))

    assertIsNotNone(err)
    assertTrue(err.startswith(errmsg), "%r does not starts with %r" % (err, errmsg))

    if pos != None:
        assertTrue(("%s at position %d" % (errmsg, pos)) in err,
                   "error %r not at position %d" % (err, pos))

def checkTemplateError(pattern, repl, string, errmsg, pos=None):
    _, err = trycatch(lambda: re.sub(pattern, repl, string))

    assertIsNotNone(err)
    assertTrue(err.startswith(errmsg), "%r does not starts with %r" % (err, errmsg))

    if pos != None:
        assertTrue(("%s at position %d" % (errmsg, pos)) in err,
                   "error %r not at position %d" % (err, pos))

def test_search_star_plus():
    assertEqual(re.search('x*', 'axx').span(0), (0, 0))
    assertEqual(re.search('x*', 'axx').span(), (0, 0))
    assertEqual(re.search('x+', 'axx').span(0), (1, 3))
    assertEqual(re.search('x+', 'axx').span(), (1, 3))
    assertIsNone(re.search('x', 'aaa'))
    assertEqual(re.match('a*', 'xxx').span(0), (0, 0))
    assertEqual(re.match('a*', 'xxx').span(), (0, 0))
    assertEqual(re.match('x*', 'xxxa').span(0), (0, 3))
    assertEqual(re.match('x*', 'xxxa').span(), (0, 3))
    assertIsNone(re.match('a+', 'xxx'))

def test_branching():
    """Test Branching
    Test expressions using the OR ('|') operator."""
    assertEqual(re.match('(ab|ba)', 'ab').span(), (0, 2))
    assertEqual(re.match('(ab|ba)', 'ba').span(), (0, 2))
    assertEqual(re.match('(abc|bac|ca|cb)', 'abc').span(),
                        (0, 3))
    assertEqual(re.match('(abc|bac|ca|cb)', 'bac').span(),
                        (0, 3))
    assertEqual(re.match('(abc|bac|ca|cb)', 'ca').span(),
                        (0, 2))
    assertEqual(re.match('(abc|bac|ca|cb)', 'cb').span(),
                        (0, 2))
    assertEqual(re.match('((a)|(b)|(c))', 'a').span(), (0, 1))
    assertEqual(re.match('((a)|(b)|(c))', 'b').span(), (0, 1))
    assertEqual(re.match('((a)|(b)|(c))', 'c').span(), (0, 1))

def bump_num(matchobj):
    int_value = int(matchobj.group(0))
    return str(int_value + 1)

def test_basic_re_sub():
    assertTypedEqual(re.sub('y', 'a', 'xyz'), 'xaz')
    assertTypedEqual(re.sub(b'y', b'a', b'xyz'), b'xaz')
    for y in ("\u00E0", "\u0430", "\U0001d49c"):
        assertEqual(re.sub(y, 'a', 'x%sz' % y), 'xaz')

    assertEqual(re.sub("(?i)b+", "x", "bbbb BBBB"), 'x x')
    assertEqual(re.sub(r'\d+', bump_num, '08.2 -2 23x99y'),
                '9.3 -3 24x100y')
    assertEqual(re.sub(r'\d+', bump_num, '08.2 -2 23x99y', 3),
                '9.3 -3 23x99y')
    assertEqual(re.sub(r'\d+', bump_num, '08.2 -2 23x99y', count=3),
                '9.3 -3 23x99y')

    assertEqual(re.sub('.', lambda m: r"\n", 'x'), '\\n')
    assertEqual(re.sub('.', r"\n", 'x'), '\n')

    s = r"\1\1"
    assertEqual(re.sub('(.)', s, 'x'), 'xx')
    assertEqual(re.sub('(.)', s.replace('\\', r'\\'), 'x'), s)
    assertEqual(re.sub('(.)', lambda m: s, 'x'), s)

    assertEqual(re.sub('(?P<a>x)', r'\g<a>\g<a>', 'xx'), 'xxxx')
    assertEqual(re.sub('(?P<a>x)', r'\g<a>\g<1>', 'xx'), 'xxxx')
    assertEqual(re.sub('(?P<unk>x)', r'\g<unk>\g<unk>', 'xx'), 'xxxx')
    assertEqual(re.sub('(?P<unk>x)', r'\g<1>\g<1>', 'xx'), 'xxxx')
    assertEqual(re.sub('()x', r'\g<0>\g<0>', 'xx'), 'xxxx')

    assertEqual(re.sub('a', r'\t\n\v\r\f\a\b', 'a'), '\t\n\v\r\f\a\b')
    assertEqual(re.sub('a', '\t\n\v\r\f\a\b', 'a'), '\t\n\v\r\f\a\b')
    assertEqual(re.sub('a', '\t\n\v\r\f\a\b', 'a'),
                        (chr(9)+chr(10)+chr(11)+chr(13)+chr(12)+chr(7)+chr(8)))
    for c in 'cdehijklmopqsuwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'.elems():
        assertRaises(lambda: re.sub('a', '\\' + c, 'a'))

    assertEqual(re.sub(r'^\s*', 'X', 'test'), 'Xtest')

def test_bug_449964():
    # fails for group followed by other escape
    assertEqual(re.sub(r'(?P<unk>x)', r'\g<1>\g<1>\b', 'xx'),
                        'xx\bxx\b')

def test_bug_449000():
    # Test for sub() on escaped characters
    assertEqual(re.sub(r'\r\n', r'\n', 'abc\r\ndef\r\n'),
                        'abc\ndef\n')
    assertEqual(re.sub('\r\n', r'\n', 'abc\r\ndef\r\n'),
                        'abc\ndef\n')
    assertEqual(re.sub(r'\r\n', '\n', 'abc\r\ndef\r\n'),
                        'abc\ndef\n')
    assertEqual(re.sub('\r\n', '\n', 'abc\r\ndef\r\n'),
                        'abc\ndef\n')

def test_bug_1661():
    # Verify that flags do not get silently ignored with compiled patterns
    pattern = re.compile('.')
    assertRaises(lambda: re.match(pattern, 'A', re.I))
    assertRaises(lambda: re.search(pattern, 'A', re.I))
    assertRaises(lambda: re.findall(pattern, 'A', re.I))
    assertRaises(lambda: re.compile(pattern, re.I))

def test_bug_3629():
    # A regex that triggered a bug in the sre-code validator
    re.compile("(?P<quote>)(?(quote))")

def test_sub_template_numeric_escape():
    # bug 776311 and friends
    assertEqual(re.sub('x', r'\0', 'x'), '\0')
    assertEqual(re.sub('x', r'\000', 'x'), '\000')
    assertEqual(re.sub('x', r'\001', 'x'), '\001')
    assertEqual(re.sub('x', r'\008', 'x'), '\0' + '8')
    assertEqual(re.sub('x', r'\009', 'x'), '\0' + '9')
    assertEqual(re.sub('x', r'\111', 'x'), '\111')
    assertEqual(re.sub('x', r'\117', 'x'), '\117')
    assertEqual(re.sub('x', r'\377', 'x'), '\u00FF')

    assertEqual(re.sub('x', r'\1111', 'x'), '\1111')
    assertEqual(re.sub('x', r'\1111', 'x'), '\111' + '1')

    assertEqual(re.sub('x', r'\00', 'x'), '\x00')
    assertEqual(re.sub('x', r'\07', 'x'), '\x07')
    assertEqual(re.sub('x', r'\08', 'x'), '\0' + '8')
    assertEqual(re.sub('x', r'\09', 'x'), '\0' + '9')
    assertEqual(re.sub('x', r'\0a', 'x'), '\0' + 'a')

    checkTemplateError('x', r'\400', 'x',
                       r'octal escape value \400 outside of ' +
                       r'range 0-0o377', 0)
    checkTemplateError('x', r'\777', 'x',
                       r'octal escape value \777 outside of ' +
                       r'range 0-0o377', 0)

    checkTemplateError('x', r'\1', 'x', 'invalid group reference 1', 1)
    checkTemplateError('x', r'\8', 'x', 'invalid group reference 8', 1)
    checkTemplateError('x', r'\9', 'x', 'invalid group reference 9', 1)
    checkTemplateError('x', r'\11', 'x', 'invalid group reference 11', 1)
    checkTemplateError('x', r'\18', 'x', 'invalid group reference 18', 1)
    checkTemplateError('x', r'\1a', 'x', 'invalid group reference 1', 1)
    checkTemplateError('x', r'\90', 'x', 'invalid group reference 90', 1)
    checkTemplateError('x', r'\99', 'x', 'invalid group reference 99', 1)
    checkTemplateError('x', r'\118', 'x', 'invalid group reference 11', 1)
    checkTemplateError('x', r'\11a', 'x', 'invalid group reference 11', 1)
    checkTemplateError('x', r'\181', 'x', 'invalid group reference 18', 1)
    checkTemplateError('x', r'\800', 'x', 'invalid group reference 80', 1)
    checkTemplateError('x', r'\8', '', 'invalid group reference 8', 1)

    # in python2.3 (etc), these loop endlessly in sre_parser.py
    assertEqual(re.sub('(((((((((((x)))))))))))', r'\11', 'x'), 'x')
    assertEqual(re.sub('((((((((((y))))))))))(.)', r'\118', 'xyz'),
                        'xz8')
    assertEqual(re.sub('((((((((((y))))))))))(.)', r'\11a', 'xyz'),
                        'xza')

def test_qualified_re_sub():
    assertEqual(re.sub('a', 'b', 'aaaaa'), 'bbbbb')
    assertEqual(re.sub('a', 'b', 'aaaaa', 1), 'baaaa')
    assertEqual(re.sub('a', 'b', 'aaaaa', count=1), 'baaaa')

    assertRaisesRegex(lambda: re.sub('a', 'b', 'aaaaa', 1, count=1),
                      r'sub: got multiple values for keyword argument "count"')
    assertRaisesRegex(lambda: re.sub('a', 'b', 'aaaaa', 1, 0, flags=0),
                      r'sub: got multiple values for keyword argument "flags"')
    assertRaisesRegex(lambda: re.sub('a', 'b', 'aaaaa', 1, 0, 0),
                      r'sub: got 6 arguments, want at most 5')

def test_misuse_flags():
    assertEqual(re.sub('a', 'b', 'aaaaa', re.I),
                re.sub('a', 'b', 'aaaaa', count=int(re.I)))
    assertEqual(re.subn("b*", "x", "xyz", re.I),
                re.subn("b*", "x", "xyz", count=int(re.I)))
    assertEqual(re.split(":", ":a:b::c", re.I),
                re.split(":", ":a:b::c", maxsplit=int(re.I)))

def test_bug_114660():
    assertEqual(re.sub(r'(\S)\s+(\S)', r'\1 \2', 'hello  there'),
                        'hello there')

def test_symbolic_groups():
    re.compile(r'(?P<a>x)(?P=a)(?(a)y)')
    re.compile(r'(?P<a1>x)(?P=a1)(?(a1)y)')
    re.compile(r'(?P<a1>x)\1(?(1)y)')
    re.compile(b'(?P<a1>x)(?P=a1)(?(a1)y)')
    # New valid identifiers in Python 3
    re.compile('(?P<µ>x)(?P=µ)(?(µ)y)')
    re.compile('(?P<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>x)(?P=𝔘𝔫𝔦𝔠𝔬𝔡𝔢)(?(𝔘𝔫𝔦𝔠𝔬𝔡𝔢)y)')
    # Support > 100 groups.
    pat = '|'.join(['x(?P<a%d>%x)y' % (i, i) for i in range(1, 200 + 1)])
    pat = '(?:%s)(?(200)z|t)' % pat
    assertEqual(re.match(pat, 'xc8yz').span(), (0, 5))

def test_symbolic_groups_errors():
    checkPatternError(r'(?P<a>)(?P<a>)',
                      "redefinition of group name 'a' as group 2; " +
                      "was group 1")
    checkPatternError(r'(?P<a>(?P=a))',
                      "cannot refer to an open group", 10)
    checkPatternError(r'(?Pxy)', 'unknown extension ?Px')
    checkPatternError(r'(?P<a>)(?P=a', 'missing ), unterminated name', 11)
    checkPatternError(r'(?P=', 'missing group name', 4)
    checkPatternError(r'(?P=)', 'missing group name', 4)
    checkPatternError(r'(?P=1)', "bad character in group name '1'", 4)
    checkPatternError(r'(?P=a)', "unknown group name 'a'")
    checkPatternError(r'(?P=a1)', "unknown group name 'a1'")
    checkPatternError(r'(?P=a.)', "bad character in group name 'a.'", 4)
    checkPatternError(r'(?P<)', 'missing >, unterminated name', 4)
    checkPatternError(r'(?P<a', 'missing >, unterminated name', 4)
    checkPatternError(r'(?P<', 'missing group name', 4)
    checkPatternError(r'(?P<>)', 'missing group name', 4)
    checkPatternError(r'(?P<1>)', "bad character in group name '1'", 4)
    checkPatternError(r'(?P<a.>)', "bad character in group name 'a.'", 4)
    checkPatternError(r'(?(', 'missing group name', 3)
    checkPatternError(r'(?())', 'missing group name', 3)
    checkPatternError(r'(?(a))', "unknown group name 'a'", 3)
    checkPatternError(r'(?(-1))', "bad character in group name '-1'", 3)
    checkPatternError(r'(?(1a))', "bad character in group name '1a'", 3)
    checkPatternError(r'(?(a.))', "bad character in group name 'a.'", 3)
    checkPatternError('(?P<©>x)', "bad character in group name '©'", 4)
    checkPatternError('(?P=©)', "bad character in group name '©'", 4)
    checkPatternError('(?(©)y)', "bad character in group name '©'", 3)
    checkPatternError(b'(?P<\xc2\xb5>x)',
                      r"bad character in group name '\xc2\xb5'", 4)
    checkPatternError(b'(?P=\xc2\xb5)',
                      r"bad character in group name '\xc2\xb5'", 4)
    checkPatternError(b'(?(\xc2\xb5)y)',
                      r"bad character in group name '\xc2\xb5'", 3)

def test_symbolic_refs():
    assertEqual(re.sub('(?P<a>x)|(?P<b>y)', r'\g<b>', 'xx'), '')
    assertEqual(re.sub('(?P<a>x)|(?P<b>y)', r'\2', 'xx'), '')
    assertEqual(re.sub(b'(?P<a1>x)', b'\\g<a1>', b'xx'), b'xx')
    # New valid identifiers in Python 3
    assertEqual(re.sub('(?P<µ>x)', r'\g<µ>', 'xx'), 'xx')
    assertEqual(re.sub('(?P<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>x)', r'\g<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>', 'xx'), 'xx')
    # Support > 100 groups.
    pat = '|'.join(['x(?P<a%d>%x)y' % (i, i) for i in range(1, 200 + 1)])
    assertEqual(re.sub(pat, r'\g<200>', 'xc8yzxc8y'), 'c8zc8')

def test_symbolic_refs_errors():
    checkTemplateError('(?P<a>x)', r'\g<a', 'xx',
                       'missing >, unterminated name', 3)
    checkTemplateError('(?P<a>x)', r'\g<', 'xx',
                       'missing group name', 3)
    checkTemplateError('(?P<a>x)', r'\g', 'xx', 'missing <', 2)
    checkTemplateError('(?P<a>x)', r'\g<a a>', 'xx',
                       "bad character in group name 'a a'", 3)
    checkTemplateError('(?P<a>x)', r'\g<>', 'xx',
                       'missing group name', 3)
    checkTemplateError('(?P<a>x)', r'\g<1a1>', 'xx',
                       "bad character in group name '1a1'", 3)
    checkTemplateError('(?P<a>x)', r'\g<2>', 'xx',
                       'invalid group reference 2', 3)
    checkTemplateError('(?P<a>x)', r'\2', 'xx',
                       'invalid group reference 2', 1)
    assertRaisesRegex(lambda: re.sub('(?P<a>x)', r'\g<ab>', 'xx'),
                      "unknown group name 'ab'")
    checkTemplateError('(?P<a>x)', r'\g<-1>', 'xx',
                       "bad character in group name '-1'", 3)
    checkTemplateError('(?P<a>x)', r'\g<+1>', 'xx',
                       "bad character in group name '+1'", 3)
    checkTemplateError('()'*10, r'\g<1_0>', 'xx',
                       "bad character in group name '1_0'", 3)
    checkTemplateError('(?P<a>x)', r'\g< 1 >', 'xx',
                       "bad character in group name ' 1 '", 3)
    checkTemplateError('(?P<a>x)', r'\g<©>', 'xx',
                       "bad character in group name '©'", 3)
    checkTemplateError(b'(?P<a>x)', b'\\g<\xc2\xb5>', b'xx',
                       r"bad character in group name '\xc2\xb5'", 3)
    checkTemplateError('(?P<a>x)', r'\g<㊀>', 'xx',
                       "bad character in group name '㊀'", 3)
    checkTemplateError('(?P<a>x)', r'\g<¹>', 'xx',
                       "bad character in group name '¹'", 3)
    checkTemplateError('(?P<a>x)', r'\g<१>', 'xx',
                       "bad character in group name '१'", 3)

def test_re_subn():
    assertEqual(re.subn("(?i)b+", "x", "bbbb BBBB"), ('x x', 2))
    assertEqual(re.subn("b+", "x", "bbbb BBBB"), ('x BBBB', 1))
    assertEqual(re.subn("b+", "x", "xyz"), ('xyz', 0))
    assertEqual(re.subn("b*", "x", "xyz"), ('xxxyxzx', 4))
    assertEqual(re.subn("b*", "x", "xyz", 2), ('xxxyz', 2))
    assertEqual(re.subn("b*", "x", "xyz", count=2), ('xxxyz', 2))

    assertRaisesRegex(lambda: re.subn('a', 'b', 'aaaaa', 1, count=1),
                      r'subn: got multiple values for keyword argument "count"')
    assertRaisesRegex(lambda: re.subn('a', 'b', 'aaaaa', 1, 0, flags=0),
                      r'subn: got multiple values for keyword argument "flags"')
    assertRaisesRegex(lambda: re.subn('a', 'b', 'aaaaa', 1, 0, 0),
                      r'subn: got 6 arguments, want at most 5')

def test_re_split():
    string = ":a:b::c"
    assertTypedEqual(re.split(":", string),
                            ['', 'a', 'b', '', 'c'])
    assertTypedEqual(re.split(":+", string),
                            ['', 'a', 'b', 'c'])
    assertTypedEqual(re.split("(:+)", string),
                            ['', ':', 'a', ':', 'b', '::', 'c'])

    string = b":a:b::c"
    assertTypedEqual(re.split(b":", string),
                            [b'', b'a', b'b', b'', b'c'])
    assertTypedEqual(re.split(b":+", string),
                            [b'', b'a', b'b', b'c'])
    assertTypedEqual(re.split(b"(:+)", string),
                            [b'', b':', b'a', b':', b'b', b'::', b'c'])

    for v in ("\u00E0\u00DF\u00E7", "\u0430\u0431\u0432",
                    "\U0001d49c\U0001d49e\U0001d4b5"):
        a, b, c = v.codepoints() # starlark fix
        string = ":%s:%s::%s" % (a, b, c)
        assertEqual(re.split(":", string), ['', a, b, '', c])
        assertEqual(re.split(":+", string), ['', a, b, c])
        assertEqual(re.split("(:+)", string),
                            ['', ':', a, ':', b, '::', c])

    assertEqual(re.split("(?::+)", ":a:b::c"), ['', 'a', 'b', 'c'])
    assertEqual(re.split("(:)+", ":a:b::c"),
                        ['', ':', 'a', ':', 'b', ':', 'c'])
    assertEqual(re.split("([b:]+)", ":a:b::c"),
                        ['', ':', 'a', ':b::', 'c'])
    assertEqual(re.split("(b)|(:+)", ":a:b::c"),
                        ['', None, ':', 'a', None, ':', '', 'b', None, '',
                        None, '::', 'c'])
    assertEqual(re.split("(?:b)|(?::+)", ":a:b::c"),
                        ['', 'a', '', '', 'c'])

    for sep, expected in [
        (':*', ['', '', 'a', '', 'b', '', 'c', '']),
        ('(?::*)', ['', '', 'a', '', 'b', '', 'c', '']),
        ('(:*)', ['', ':', '', '', 'a', ':', '', '', 'b', '::', '', '', 'c', '', '']),
        ('(:)*', ['', ':', '', None, 'a', ':', '', None, 'b', ':', '', None, 'c', None, '']),
    ]:
        assertTypedEqual(re.split(sep, ':a:b::c'), expected)

    for sep, expected in [
        ('', ['', ':', 'a', ':', 'b', ':', ':', 'c', '']),
        (r'\b', [':', 'a', ':', 'b', '::', 'c', '']),
        (r'(?=:)', ['', ':a', ':b', ':', ':c']),
        (r'(?<=:)', [':', 'a:', 'b:', ':', 'c']),
    ]:
        assertTypedEqual(re.split(sep, ':a:b::c'), expected)

def test_qualified_re_split():
    assertEqual(re.split(":", ":a:b::c", 2), ['', 'a', 'b::c'])
    assertEqual(re.split(":", ":a:b::c", maxsplit=2), ['', 'a', 'b::c'])
    assertEqual(re.split(':', 'a:b:c:d', maxsplit=2), ['a', 'b', 'c:d'])
    assertEqual(re.split("(:)", ":a:b::c", maxsplit=2),
                        ['', ':', 'a', ':', 'b::c'])
    assertEqual(re.split("(:+)", ":a:b::c", maxsplit=2),
                        ['', ':', 'a', ':', 'b::c'])
    assertEqual(re.split("(:*)", ":a:b::c", maxsplit=2),
                        ['', ':', '', '', 'a:b::c'])

    assertRaisesRegex(lambda: re.split(":", ":a:b::c", 2, maxsplit=2),
                      r'split: got multiple values for keyword argument "maxsplit"')
    assertRaisesRegex(lambda: re.split(":", ":a:b::c", 2, 0, flags=0),
                      r'split: got multiple values for keyword argument "flags"')
    assertRaisesRegex(lambda: re.split(":", ":a:b::c", 2, 0, 0),
                      r'split: got 5 arguments, want at most 4')

def test_re_findall():
    assertEqual(re.findall(":+", "abc"), [])
    string = "a:b::c:::d"
    assertTypedEqual(re.findall(":+", string),
                            [":", "::", ":::"])
    assertTypedEqual(re.findall("(:+)", string),
                            [":", "::", ":::"])
    assertTypedEqual(re.findall("(:)(:*)", string),
                            [(":", ""), (":", ":"), (":", "::")])
    string = b"a:b::c:::d"
    assertTypedEqual(re.findall(b":+", string),
                            [b":", b"::", b":::"])
    assertTypedEqual(re.findall(b"(:+)", string),
                            [b":", b"::", b":::"])
    assertTypedEqual(re.findall(b"(:)(:*)", string),
                            [(b":", b""), (b":", b":"), (b":", b"::")])
    for x in ("\u00E0", "\u0430", "\U0001d49c"):
        xx = x * 2
        xxx = x * 3
        string = "a%sb%sc%sd" % (x, xx, xxx)
        assertEqual(re.findall("%s+" % x, string), [x, xx, xxx])
        assertEqual(re.findall("(%s+)" % x, string), [x, xx, xxx])
        assertEqual(re.findall("(%s)(%s*)" % (x, x), string),
                            [(x, ""), (x, x), (x, xx)])

def test_bug_117612():
    assertEqual(re.findall(r"(a|(b))", "aba"),
                        [("a", ""),("b", "b"),("a", "")])

def test_re_match():
    string = 'a'
    assertEqual(re.match('a', string).groups(), ())
    assertEqual(re.match('(a)', string).groups(), ('a',))
    assertEqual(re.match('(a)', string).group(0), 'a')
    assertEqual(re.match('(a)', string).group(1), 'a')
    assertEqual(re.match('(a)', string).group(1, 1), ('a', 'a'))

    string = b'a'
    assertEqual(re.match(b'a', string).groups(), ())
    assertEqual(re.match(b'(a)', string).groups(), (b'a',))
    assertEqual(re.match(b'(a)', string).group(0), b'a')
    assertEqual(re.match(b'(a)', string).group(1), b'a')
    assertEqual(re.match(b'(a)', string).group(1, 1), (b'a', b'a'))

    for a in ("\u00E0", "\u0430", "\U0001d49c"):
        assertEqual(re.match(a, a).groups(), ())
        assertEqual(re.match('(%s)' % a, a).groups(), (a,))
        assertEqual(re.match('(%s)' % a, a).group(0), a)
        assertEqual(re.match('(%s)' % a, a).group(1), a)
        assertEqual(re.match('(%s)' % a, a).group(1, 1), (a, a))

    pat = re.compile('((a)|(b))(c)?')
    assertEqual(pat.match('a').groups(), ('a', 'a', None, None))
    assertEqual(pat.match('b').groups(), ('b', None, 'b', None))
    assertEqual(pat.match('ac').groups(), ('a', 'a', None, 'c'))
    assertEqual(pat.match('bc').groups(), ('b', None, 'b', 'c'))
    assertEqual(pat.match('bc').groups(""), ('b', "", 'b', 'c'))

    pat = re.compile('(?:(?P<a1>a)|(?P<b2>b))(?P<c3>c)?')
    assertEqual(pat.match('a').group(1, 2, 3), ('a', None, None))
    assertEqual(pat.match('b').group('a1', 'b2', 'c3'),
                        (None, 'b', None))
    assertEqual(pat.match('ac').group(1, 'b2', 3), ('a', None, 'c'))

def test_group():
    # A single group
    m = re.match('(a)(b)', 'ab')
    assertEqual(m.group(), 'ab')
    assertEqual(m.group(0), 'ab')
    assertEqual(m.group(1), 'a')
    assertRaises(lambda: m.group(-1))
    assertRaises(lambda: m.group(3))
    assertRaises(lambda: m.group(1<<1000))
    assertRaises(lambda: m.group('x'))
    # Multiple groups
    assertEqual(m.group(2, 1), ('b', 'a'))

def test_match_getitem():
    pat = re.compile('(?:(?P<a1>a)|(?P<b2>b))(?P<c3>c)?')

    m = pat.match('a')
    assertEqual(m['a1'], 'a')
    assertEqual(m['b2'], None)
    assertEqual(m['c3'], None)
    assertEqual('a1={a1} b2={b2} c3={c3}'.format(**m.groupdict()), 'a1=a b2=None c3=None')
    assertEqual(m[0], 'a')
    assertEqual(m[1], 'a')
    assertEqual(m[2], None)
    assertEqual(m[3], None)
    assertRaisesRegex(lambda: m['X'], 'no such group')
    assertRaisesRegex(lambda: m[-1], 'no such group')
    assertRaisesRegex(lambda: m[4], 'no such group')
    assertRaisesRegex(lambda: m[0, 1], 'no such group')
    assertRaisesRegex(lambda: m[(0,)], 'no such group')
    assertRaisesRegex(lambda: m[(0, 1)], 'no such group')
    assertRaisesRegex(lambda: 'a1={a2}'.format(m), 'format: keyword a2 not found')

    m = pat.match('ac')
    assertEqual(m['a1'], 'a')
    assertEqual(m['b2'], None)
    assertEqual(m['c3'], 'c')
    assertEqual('a1={a1} b2={b2} c3={c3}'.format(**m.groupdict()), 'a1=a b2=None c3=c')
    assertEqual(m[0], 'ac')
    assertEqual(m[1], 'a')
    assertEqual(m[2], None)
    assertEqual(m[3], 'c')

    # Cannot assign.
    def cb(): m[0] = 1
    assertRaises(cb)

    # No len().
    assertRaises(lambda: len(m))

def test_re_fullmatch():
    # Issue 16203: Proposal: add re.fullmatch() method.
    assertEqual(re.fullmatch(r"a", "a").span(), (0, 1))
    assertEqual(re.fullmatch(r"a|ab", "ab").span(), (0, 2))
    assertEqual(re.fullmatch(b"a|ab", b"ab").span(), (0, 2))
    for v, l in zip(("\u00E0\u00DF", "\u0430\u0431", "\U0001d49c\U0001d49e"), (4, 4, 8)):
        a, b = v.codepoints() # starlark fix
        r = r"%s|%s" % (a, a + b)
        assertEqual(re.fullmatch(r, a + b).span(), (0, l))
    assertEqual(re.fullmatch(r".*?$", "abc").span(), (0, 3))
    assertEqual(re.fullmatch(r".*?", "abc").span(), (0, 3))
    assertEqual(re.fullmatch(r"a.*?b", "ab").span(), (0, 2))
    assertEqual(re.fullmatch(r"a.*?b", "abb").span(), (0, 3))
    assertEqual(re.fullmatch(r"a.*?b", "axxb").span(), (0, 4))
    assertIsNone(re.fullmatch(r"a+", "ab"))
    assertIsNone(re.fullmatch(r"abc$", "abc\n"))
    assertIsNone(re.fullmatch(r"abc\Z", "abc\n"))
    assertIsNone(re.fullmatch(r"(?m)abc$", "abc\n"))
    assertEqual(re.fullmatch(r"ab(?=c)cd", "abcd").span(), (0, 4))
    assertEqual(re.fullmatch(r"ab(?<=b)cd", "abcd").span(), (0, 4))
    assertEqual(re.fullmatch(r"(?=a|ab)ab", "ab").span(), (0, 2))

    assertEqual(
        re.compile(r"bc").fullmatch("abcd", pos=1, endpos=3).span(), (1, 3))
    assertEqual(
        re.compile(r".*?$").fullmatch("abcd", pos=1, endpos=3).span(), (1, 3))
    assertEqual(
        re.compile(r".*?").fullmatch("abcd", pos=1, endpos=3).span(), (1, 3))

def test_re_groupref_exists():
    assertEqual(re.match(r'^(\()?([^()]+)(?(1)\))$', '(a)').groups(),
                        ('(', 'a'))
    assertEqual(re.match(r'^(\()?([^()]+)(?(1)\))$', 'a').groups(),
                        (None, 'a'))
    assertIsNone(re.match(r'^(\()?([^()]+)(?(1)\))$', 'a)'))
    assertIsNone(re.match(r'^(\()?([^()]+)(?(1)\))$', '(a'))
    assertEqual(re.match('^(?:(a)|c)((?(1)b|d))$', 'ab').groups(),
                        ('a', 'b'))
    assertEqual(re.match(r'^(?:(a)|c)((?(1)b|d))$', 'cd').groups(),
                        (None, 'd'))
    assertEqual(re.match(r'^(?:(a)|c)((?(1)|d))$', 'cd').groups(),
                        (None, 'd'))
    assertEqual(re.match(r'^(?:(a)|c)((?(1)|d))$', 'a').groups(),
                        ('a', ''))

    # Tests for bug #1177831: exercise groups other than the first group
    p = re.compile('(?P<g1>a)(?P<g2>b)?((?(g2)c|d))')
    assertEqual(p.match('abc').groups(),
                        ('a', 'b', 'c'))
    assertEqual(p.match('ad').groups(),
                        ('a', None, 'd'))
    assertIsNone(p.match('abd'))
    assertIsNone(p.match('ac'))

    # Support > 100 groups.
    pat = '|'.join(['x(?P<a%d>%x)y' % (i, i) for i in range(1, 200 + 1)])
    pat = '(?:%s)(?(200)z)' % pat
    assertEqual(re.match(pat, 'xc8yz').span(), (0, 5))

def test_re_groupref_exists_errors():
    checkPatternError(r'(?P<a>)(?(0)a|b)', 'bad group number', 10)
    checkPatternError(r'()(?(-1)a|b)',
                      "bad character in group name '-1'", 5)
    checkPatternError(r'()(?(+1)a|b)',
                      "bad character in group name '+1'", 5)
    checkPatternError(r'()'*10 + r'(?(1_0)a|b)',
                      "bad character in group name '1_0'", 23)
    checkPatternError(r'()(?( 1 )a|b)',
                      "bad character in group name ' 1 '", 5)
    checkPatternError(r'()(?(㊀)a|b)',
                      "bad character in group name '㊀'", 5)
    checkPatternError(r'()(?(¹)a|b)',
                      "bad character in group name '¹'", 5)
    checkPatternError(r'()(?(१)a|b)',
                      "bad character in group name '१'", 5)
    checkPatternError(r'()(?(1',
                      "missing ), unterminated name", 5)
    checkPatternError(r'()(?(1)a',
                      "missing ), unterminated subpattern", 2)
    checkPatternError(r'()(?(1)a|b',
                      'missing ), unterminated subpattern', 2)
    checkPatternError(r'()(?(1)a|b|c',
                      'conditional backref with more than ' +
                      'two branches', 10)
    checkPatternError(r'()(?(1)a|b|c)',
                      'conditional backref with more than ' +
                      'two branches', 10)
    checkPatternError(r'()(?(2)a)',
                      "invalid group reference 2", 5)

def test_re_groupref_exists_validation_bug():
    for i in range(256):
        re.compile(format(r'()(?(1)\x%02x?)', i))

def test_re_groupref_overflow():
    MAXGROUPS = 1000
    checkTemplateError('()', r'\g<%s>' % MAXGROUPS, 'xx',
                       'invalid group reference %d' % MAXGROUPS, 3)
    checkPatternError(r'(?P<a>)(?(%d))' % MAXGROUPS,
                      'invalid group reference %d' % MAXGROUPS, 10)

def test_re_groupref():
    assertEqual(re.match(r'^(\|)?([^()]+)\1$', '|a|').groups(),
                        ('|', 'a'))
    assertEqual(re.match(r'^(\|)?([^()]+)\1?$', 'a').groups(),
                        (None, 'a'))
    assertIsNone(re.match(r'^(\|)?([^()]+)\1$', 'a|'))
    assertIsNone(re.match(r'^(\|)?([^()]+)\1$', '|a'))
    assertEqual(re.match(r'^(?:(a)|c)(\1)$', 'aa').groups(),
                        ('a', 'a'))
    assertEqual(re.match(r'^(?:(a)|c)(\1)?$', 'c').groups(),
                        (None, None))

    checkPatternError(r'(abc\1)', 'cannot refer to an open group', 4)

def test_groupdict():
    assertEqual(re.match('(?P<first>first) (?P<second>second)',
                                'first second').groupdict(),
                        {'first':'first', 'second':'second'})

def test_expand():
    assertEqual(re.match("(?P<first>first) (?P<second>second)",
                                "first second")
                                .expand(r"\2 \1 \g<second> \g<first>"),
                        "second first second first")
    assertEqual(re.match("(?P<first>first)|(?P<second>second)",
                                "first")
                                .expand(r"\2 \g<second>"),
                        " ")

def test_repeat_minmax():
    assertIsNone(re.match(r"^(\w){1}$", "abc"))
    assertIsNone(re.match(r"^(\w){1}?$", "abc"))
    assertIsNone(re.match(r"^(\w){1,2}$", "abc"))
    assertIsNone(re.match(r"^(\w){1,2}?$", "abc"))

    assertEqual(re.match(r"^(\w){3}$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){1,3}$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){1,4}$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){3,4}?$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){3}?$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){1,3}?$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){1,4}?$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){3,4}?$", "abc").group(1), "c")

    assertIsNone(re.match(r"^x{1}$", "xxx"))
    assertIsNone(re.match(r"^x{1}?$", "xxx"))
    assertIsNone(re.match(r"^x{1,2}$", "xxx"))
    assertIsNone(re.match(r"^x{1,2}?$", "xxx"))

    assertTrue(re.match(r"^x{3}$", "xxx"))
    assertTrue(re.match(r"^x{1,3}$", "xxx"))
    assertTrue(re.match(r"^x{3,3}$", "xxx"))
    assertTrue(re.match(r"^x{1,4}$", "xxx"))
    assertTrue(re.match(r"^x{3,4}?$", "xxx"))
    assertTrue(re.match(r"^x{3}?$", "xxx"))
    assertTrue(re.match(r"^x{1,3}?$", "xxx"))
    assertTrue(re.match(r"^x{1,4}?$", "xxx"))
    assertTrue(re.match(r"^x{3,4}?$", "xxx"))

    assertIsNone(re.match(r"^x{}$", "xxx"))
    assertTrue(re.match(r"^x{}$", "x{}"))

    checkPatternError(r'x{2,1}',
                      'min repeat greater than max repeat', 2)

def test_getattr():
    assertEqual(re.compile("(?i)(a)(b)").pattern, "(?i)(a)(b)")
    assertEqual(re.compile("(?i)(a)(b)").flags, re.I | re.U)
    assertEqual(re.compile("(?i)(a)(b)").groups, 2)
    assertEqual(re.compile("(?i)(a)(b)").groupindex, {})
    assertEqual(re.compile("(?i)(?P<first>a)(?P<other>b)").groupindex,
                        {'first': 1, 'other': 2})

    assertEqual(re.match("(a)", "a").pos, 0)
    assertEqual(re.match("(a)", "a").endpos, 1)
    assertEqual(re.match("(a)", "a").string, "a")
    assertEqual(re.match("(a)", "a").regs, ((0, 1), (0, 1)))
    assertTrue(re.match("(a)", "a").re)

    # Issue 14260. groupindex should be non-modifiable mapping.
    p = re.compile(r'(?i)(?P<first>a)(?P<other>b)')
    assertEqual(sorted(p.groupindex), ['first', 'other'])
    assertEqual(p.groupindex['other'], 2)

    def cb(): p.groupindex['other'] = 0
    assertRaises(cb)
    assertEqual(p.groupindex['other'], 2)

def test_special_escapes():
    assertEqual(re.search(r"\b(b.)\b",
                          "abcd abc bcd bx").group(1), "bx")
    assertEqual(re.search(r"\B(b.)\B",
                          "abc bcd bc abxd").group(1), "bx")
    assertEqual(re.search(r"\b(b.)\b",
                          "abcd abc bcd bx", re.ASCII).group(1), "bx")
    assertEqual(re.search(r"\B(b.)\B",
                          "abc bcd bc abxd", re.ASCII).group(1), "bx")
    assertEqual(re.search(r"^abc$", "\nabc\n", re.M).group(0), "abc")
    assertEqual(re.search(r"^\Aabc\Z$", "abc", re.M).group(0), "abc")
    assertIsNone(re.search(r"^\Aabc\Z$", "\nabc\n", re.M))
    assertEqual(re.search(b"\\b(b.)\\b",
                          b"abcd abc bcd bx").group(1), b"bx")
    assertEqual(re.search(b"\\B(b.)\\B",
                          b"abc bcd bc abxd").group(1), b"bx")
    assertEqual(re.search(b"\\b(b.)\\b",
                          b"abcd abc bcd bx", re.LOCALE).group(1), b"bx")
    assertEqual(re.search(b"\\B(b.)\\B",
                          b"abc bcd bc abxd", re.LOCALE).group(1), b"bx")
    assertEqual(re.search(b"^abc$", b"\nabc\n", re.M).group(0), b"abc")
    assertEqual(re.search(b"^\\Aabc\\Z$", b"abc", re.M).group(0), b"abc")
    assertIsNone(re.search(b"^\\Aabc\\Z$", b"\nabc\n", re.M))
    assertEqual(re.search(r"\d\D\w\W\s\S",
                          "1aa! a").group(0), "1aa! a")
    assertEqual(re.search(b"\\d\\D\\w\\W\\s\\S",
                          b"1aa! a").group(0), b"1aa! a")
    assertEqual(re.search(r"\d\D\w\W\s\S",
                          "1aa! a", re.ASCII).group(0), "1aa! a")
    assertEqual(re.search(b"\\d\\D\\w\\W\\s\\S",
                          b"1aa! a", re.LOCALE).group(0), b"1aa! a")

def test_other_escapes():
    checkPatternError("\\", 'bad escape (end of pattern)', 0)
    assertEqual(re.match(r"\(", '(').group(), '(')
    assertIsNone(re.match(r"\(", ')'))
    assertEqual(re.match(r"\\", '\\').group(), '\\')
    assertEqual(re.match(r"[\]]", ']').group(), ']')
    assertIsNone(re.match(r"[\]]", '['))
    assertEqual(re.match(r"[a\-c]", '-').group(), '-')
    assertIsNone(re.match(r"[a\-c]", 'b'))
    assertEqual(re.match(r"[\^a]+", 'a^').group(), 'a^')
    assertIsNone(re.match(r"[\^a]+", 'b'))
    re.purge()  # for warnings
    for c in 'ceghijklmopqyzCEFGHIJKLMNOPQRTVXY'.elems():
        assertRaises(lambda: re.compile('\\%c' % c))
    for c in 'ceghijklmopqyzABCEFGHIJKLMNOPQRTVXYZ'.elems():
        assertRaises(lambda: re.compile('[\\%c]' % c))

def test_named_unicode_escapes():
    # test individual Unicode named escapes
    assertTrue(re.match(r'\N{LESS-THAN SIGN}', '<'))
    assertTrue(re.match(r'\N{less-than sign}', '<'))
    assertIsNone(re.match(r'\N{LESS-THAN SIGN}', '>'))
    assertTrue(re.match(r'\N{SNAKE}', '\U0001f40d'))
    assertTrue(re.match(r'\N{ARABIC LIGATURE UIGHUR KIRGHIZ YEH WITH ' +
                        r'HAMZA ABOVE WITH ALEF MAKSURA ISOLATED FORM}',
                        '\ufbf9'))
    assertTrue(re.match(r'[\N{LESS-THAN SIGN}-\N{GREATER-THAN SIGN}]',
                        '='))
    assertIsNone(re.match(r'[\N{LESS-THAN SIGN}-\N{GREATER-THAN SIGN}]',
                        ';'))

    # test errors in \N{name} handling - only valid names should pass
    checkPatternError(r'\N', 'missing {', 2)
    checkPatternError(r'[\N]', 'missing {', 3)
    checkPatternError(r'\N{', 'missing character name', 3)
    checkPatternError(r'[\N{', 'missing character name', 4)
    checkPatternError(r'\N{}', 'missing character name', 3)
    checkPatternError(r'[\N{}]', 'missing character name', 4)
    checkPatternError(r'\NSNAKE}', 'missing {', 2)
    checkPatternError(r'[\NSNAKE}]', 'missing {', 3)
    checkPatternError(r'\N{SNAKE',
                            'missing }, unterminated name', 3)
    checkPatternError(r'[\N{SNAKE]',
                            'missing }, unterminated name', 4)
    checkPatternError(r'[\N{SNAKE]}',
                            "undefined character name 'SNAKE]'", 1)
    checkPatternError(r'\N{SPAM}',
                            "undefined character name 'SPAM'", 0)
    checkPatternError(r'[\N{SPAM}]',
                            "undefined character name 'SPAM'", 1)
    checkPatternError(r'\N{KEYCAP NUMBER SIGN}',
                        "undefined character name 'KEYCAP NUMBER SIGN'", 0)
    checkPatternError(r'[\N{KEYCAP NUMBER SIGN}]',
                        "undefined character name 'KEYCAP NUMBER SIGN'", 1)
    checkPatternError(b'\\N{LESS-THAN SIGN}', r'bad escape \N', 0)
    checkPatternError(b'[\\N{LESS-THAN SIGN}]', r'bad escape \N', 1)

def test_string_boundaries():
    # See http://bugs.python.org/issue10713
    assertEqual(re.search(r"\b(abc)\b", "abc").group(1),
                        "abc")
    # There's a word boundary at the start of a string.
    assertTrue(re.match(r"\b", "abc"))
    # A non-empty string includes a non-boundary zero-length match.
    assertTrue(re.search(r"\B", "abc"))
    # There is no non-boundary match at the start of a string.
    assertFalse(re.match(r"\B", "abc"))
    # However, an empty string contains no word boundaries, and also no
    # non-boundaries.
    # SKIP, is not fixable: assertIsNone(re.search(r"\B", ""))
    # This one is questionable and different from the perlre behaviour,
    # but describes current behavior.
    assertIsNone(re.search(r"\b", ""))
    # A single word-character string has two boundaries, but no
    # non-boundary gaps.
    assertEqual(len(re.findall(r"\b", "a")), 2)
    assertEqual(len(re.findall(r"\B", "a")), 0)
    # If there are no words, there are no boundaries
    assertEqual(len(re.findall(r"\b", " ")), 0)
    assertEqual(len(re.findall(r"\b", "   ")), 0)
    # Can match around the whitespace.
    assertEqual(len(re.findall(r"\B", " ")), 2)

def test_bigcharset():
    assertEqual(re.match("([\u2222\u2223])",
                                "\u2222").group(1), "\u2222")
    r = '[%s]' % ''.join([chr(c) for c in range(256, 1<<16, 255)])
    assertEqual(re.match(r, "\uff01").group(), "\uff01")

def test_big_codesize():
    # Issue #1160
    r = re.compile('|'.join(['%d'%x for x in range(10000)]))
    assertTrue(r.match('1000'))
    assertTrue(r.match('9999'))

def test_anyall():
    assertEqual(re.match("a.b", "a\nb", re.DOTALL).group(0),
                        "a\nb")
    assertEqual(re.match("a.*b", "a\n\nb", re.DOTALL).group(0),
                        "a\n\nb")

def test_lookahead():
    assertEqual(re.match(r"(a(?=\s[^a]))", "a b").group(1), "a")
    assertEqual(re.match(r"(a(?=\s[^a]*))", "a b").group(1), "a")
    assertEqual(re.match(r"(a(?=\s[abc]))", "a b").group(1), "a")
    assertEqual(re.match(r"(a(?=\s[abc]*))", "a bc").group(1), "a")
    assertEqual(re.match(r"(a)(?=\s\1)", "a a").group(1), "a")
    assertEqual(re.match(r"(a)(?=\s\1*)", "a aa").group(1), "a")
    assertEqual(re.match(r"(a)(?=\s(abc|a))", "a a").group(1), "a")

    assertEqual(re.match(r"(a(?!\s[^a]))", "a a").group(1), "a")
    assertEqual(re.match(r"(a(?!\s[abc]))", "a d").group(1), "a")
    assertEqual(re.match(r"(a)(?!\s\1)", "a b").group(1), "a")
    assertEqual(re.match(r"(a)(?!\s(abc|a))", "a b").group(1), "a")

    # Group reference.
    assertTrue(re.match(r'(a)b(?=\1)a', 'aba'))
    assertIsNone(re.match(r'(a)b(?=\1)c', 'abac'))
    # Conditional group reference.
    assertTrue(re.match(r'(?:(a)|(x))b(?=(?(2)x|c))c', 'abc'))
    assertIsNone(re.match(r'(?:(a)|(x))b(?=(?(2)c|x))c', 'abc'))
    assertTrue(re.match(r'(?:(a)|(x))b(?=(?(2)x|c))c', 'abc'))
    assertIsNone(re.match(r'(?:(a)|(x))b(?=(?(1)b|x))c', 'abc'))
    assertTrue(re.match(r'(?:(a)|(x))b(?=(?(1)c|x))c', 'abc'))
    # Group used before defined.
    assertTrue(re.match(r'(a)b(?=(?(2)x|c))(c)', 'abc'))
    assertIsNone(re.match(r'(a)b(?=(?(2)b|x))(c)', 'abc'))
    assertTrue(re.match(r'(a)b(?=(?(1)c|x))(c)', 'abc'))

def test_lookbehind():
    assertTrue(re.match(r'ab(?<=b)c', 'abc'))
    assertIsNone(re.match(r'ab(?<=c)c', 'abc'))
    assertIsNone(re.match(r'ab(?<!b)c', 'abc'))
    assertTrue(re.match(r'ab(?<!c)c', 'abc'))
    # Group reference.
    assertTrue(re.match(r'(a)a(?<=\1)c', 'aac'))
    assertIsNone(re.match(r'(a)b(?<=\1)a', 'abaa'))
    assertIsNone(re.match(r'(a)a(?<!\1)c', 'aac'))
    assertTrue(re.match(r'(a)b(?<!\1)a', 'abaa'))
    # Conditional group reference.
    assertIsNone(re.match(r'(?:(a)|(x))b(?<=(?(2)x|c))c', 'abc'))
    assertIsNone(re.match(r'(?:(a)|(x))b(?<=(?(2)b|x))c', 'abc'))
    assertTrue(re.match(r'(?:(a)|(x))b(?<=(?(2)x|b))c', 'abc'))
    assertIsNone(re.match(r'(?:(a)|(x))b(?<=(?(1)c|x))c', 'abc'))
    assertTrue(re.match(r'(?:(a)|(x))b(?<=(?(1)b|x))c', 'abc'))
    # Group used before defined.
    assertRaises(lambda: re.compile(r'(a)b(?<=(?(2)b|x))(c)'))
    assertIsNone(re.match(r'(a)b(?<=(?(1)c|x))(c)', 'abc'))
    assertTrue(re.match(r'(a)b(?<=(?(1)b|x))(c)', 'abc'))
    # Group defined in the same lookbehind pattern
    assertRaises(lambda: re.compile(r'(a)b(?<=(.)\2)(c)'))
    assertRaises(lambda: re.compile(r'(a)b(?<=(?P<a>.)(?P=a))(c)'))
    assertRaises(lambda: re.compile(r'(a)b(?<=(a)(?(2)b|x))(c)'))
    assertRaises(lambda: re.compile(r'(a)b(?<=(.)(?<=\2))(c)'))

def test_ignore_case():
    assertEqual(re.match("abc", "ABC", re.I).group(0), "ABC")
    assertEqual(re.match(b"abc", b"ABC", re.I).group(0), b"ABC")
    assertEqual(re.match(r"(a\s[^a])", "a b", re.I).group(1), "a b")
    assertEqual(re.match(r"(a\s[^a]*)", "a bb", re.I).group(1), "a bb")
    assertEqual(re.match(r"(a\s[abc])", "a b", re.I).group(1), "a b")
    assertEqual(re.match(r"(a\s[abc]*)", "a bb", re.I).group(1), "a bb")
    assertEqual(re.match(r"((a)\s\2)", "a a", re.I).group(1), "a a")
    assertEqual(re.match(r"((a)\s\2*)", "a aa", re.I).group(1), "a aa")
    assertEqual(re.match(r"((a)\s(abc|a))", "a a", re.I).group(1), "a a")
    assertEqual(re.match(r"((a)\s(abc|a)*)", "a aa", re.I).group(1), "a aa")

    # Two different characters have the same lowercase.
    # assert 'K'.lower() == '\u212a'.lower() == 'k' # 'K'
    assertTrue(re.match(r'K', '\u212a', re.I))
    assertTrue(re.match(r'k', '\u212a', re.I))
    assertTrue(re.match(r'\u212a', 'K', re.I))
    assertTrue(re.match(r'\u212a', 'k', re.I))

    # Two different characters have the same uppercase.
    # assert 's'.upper() == '\u017f'.upper() == 'S' # 'ſ'
    assertTrue(re.match(r'S', '\u017f', re.I))
    assertTrue(re.match(r's', '\u017f', re.I))
    assertTrue(re.match(r'\u017f', 'S', re.I))
    assertTrue(re.match(r'\u017f', 's', re.I))

    # Two different characters have the same uppercase. Unicode 9.0+.
    # assert '\u0432'.upper() == '\u1c80'.upper() == '\u0412' # 'в', 'ᲀ', 'В'
    assertTrue(re.match(r'\u0412', '\u0432', re.I))
    assertTrue(re.match(r'\u0412', '\u1c80', re.I))
    assertTrue(re.match(r'\u0432', '\u0412', re.I))
    assertTrue(re.match(r'\u0432', '\u1c80', re.I))
    assertTrue(re.match(r'\u1c80', '\u0412', re.I))
    assertTrue(re.match(r'\u1c80', '\u0432', re.I))

    # Two different characters have the same multicharacter uppercase.
    # assert '\ufb05'.upper() == '\ufb06'.upper() == 'ST' # 'ﬅ', 'ﬆ'
    assertTrue(re.match(r'\ufb05', '\ufb06', re.I))
    assertTrue(re.match(r'\ufb06', '\ufb05', re.I))

def test_ignore_case_set():
    assertTrue(re.match(r'[19A]', 'A', re.I))
    assertTrue(re.match(r'[19a]', 'a', re.I))
    assertTrue(re.match(r'[19a]', 'A', re.I))
    assertTrue(re.match(r'[19A]', 'a', re.I))
    assertTrue(re.match(b'[19A]', b'A', re.I))
    assertTrue(re.match(b'[19a]', b'a', re.I))
    assertTrue(re.match(b'[19a]', b'A', re.I))
    assertTrue(re.match(b'[19A]', b'a', re.I))

    # Two different characters have the same lowercase.
    # assert 'K'.lower() == '\u212a'.lower() == 'k' # 'K'
    assertTrue(re.match(r'[19K]', '\u212a', re.I))
    assertTrue(re.match(r'[19k]', '\u212a', re.I))
    assertTrue(re.match(r'[19\u212a]', 'K', re.I))
    assertTrue(re.match(r'[19\u212a]', 'k', re.I))

    # Two different characters have the same uppercase.
    # assert 's'.upper() == '\u017f'.upper() == 'S' # 'ſ'
    assertTrue(re.match(r'[19S]', '\u017f', re.I))
    assertTrue(re.match(r'[19s]', '\u017f', re.I))
    assertTrue(re.match(r'[19\u017f]', 'S', re.I))
    assertTrue(re.match(r'[19\u017f]', 's', re.I))

    # Two different characters have the same uppercase. Unicode 9.0+.
    # assert '\u0432'.upper() == '\u1c80'.upper() == '\u0412' # 'в', 'ᲀ', 'В'
    assertTrue(re.match(r'[19\u0412]', '\u0432', re.I))
    assertTrue(re.match(r'[19\u0412]', '\u1c80', re.I))
    assertTrue(re.match(r'[19\u0432]', '\u0412', re.I))
    assertTrue(re.match(r'[19\u0432]', '\u1c80', re.I))
    assertTrue(re.match(r'[19\u1c80]', '\u0412', re.I))
    assertTrue(re.match(r'[19\u1c80]', '\u0432', re.I))

    # Two different characters have the same multicharacter uppercase.
    # assert '\ufb05'.upper() == '\ufb06'.upper() == 'ST' # 'ﬅ', 'ﬆ'
    assertTrue(re.match(r'[19\ufb05]', '\ufb06', re.I))
    assertTrue(re.match(r'[19\ufb06]', '\ufb05', re.I))

def test_ignore_case_range():
    # Issues #3511, #17381.
    assertTrue(re.match(r'[9-a]', '_', re.I))
    assertIsNone(re.match(r'[9-A]', '_', re.I))
    assertTrue(re.match(b'[9-a]', b'_', re.I))
    assertIsNone(re.match(b'[9-A]', b'_', re.I))
    assertTrue(re.match(r'[\xc0-\xde]', '\u00D7', re.I))
    assertIsNone(re.match(r'[\xc0-\xde]', '\u00F7', re.I))
    assertTrue(re.match(r'[\xe0-\xfe]', '\u00F7', re.I))
    assertIsNone(re.match(r'[\xe0-\xfe]', '\u00D7', re.I))
    assertTrue(re.match(r'[\u0430-\u045f]', '\u0450', re.I))
    assertTrue(re.match(r'[\u0430-\u045f]', '\u0400', re.I))
    assertTrue(re.match(r'[\u0400-\u042f]', '\u0450', re.I))
    assertTrue(re.match(r'[\u0400-\u042f]', '\u0400', re.I))
    assertTrue(re.match(r'[\U00010428-\U0001044f]', '\U00010428', re.I))
    assertTrue(re.match(r'[\U00010428-\U0001044f]', '\U00010400', re.I))
    assertTrue(re.match(r'[\U00010400-\U00010427]', '\U00010428', re.I))
    assertTrue(re.match(r'[\U00010400-\U00010427]', '\U00010400', re.I))

    # Two different characters have the same lowercase.
    # assert 'K'.lower() == '\u212a'.lower() == 'k' # 'K'
    assertTrue(re.match(r'[J-M]', '\u212a', re.I))
    assertTrue(re.match(r'[j-m]', '\u212a', re.I))
    assertTrue(re.match(r'[\u2129-\u212b]', 'K', re.I))
    assertTrue(re.match(r'[\u2129-\u212b]', 'k', re.I))

    # Two different characters have the same uppercase.
    # assert 's'.upper() == '\u017f'.upper() == 'S' # 'ſ'
    assertTrue(re.match(r'[R-T]', '\u017f', re.I))
    assertTrue(re.match(r'[r-t]', '\u017f', re.I))
    assertTrue(re.match(r'[\u017e-\u0180]', 'S', re.I))
    assertTrue(re.match(r'[\u017e-\u0180]', 's', re.I))

    # Two different characters have the same uppercase. Unicode 9.0+.
    # assert '\u0432'.upper() == '\u1c80'.upper() == '\u0412' # 'в', 'ᲀ', 'В'
    assertTrue(re.match(r'[\u0411-\u0413]', '\u0432', re.I))
    assertTrue(re.match(r'[\u0411-\u0413]', '\u1c80', re.I))
    assertTrue(re.match(r'[\u0431-\u0433]', '\u0412', re.I))
    assertTrue(re.match(r'[\u0431-\u0433]', '\u1c80', re.I))
    assertTrue(re.match(r'[\u1c80-\u1c82]', '\u0412', re.I))
    assertTrue(re.match(r'[\u1c80-\u1c82]', '\u0432', re.I))

    # Two different characters have the same multicharacter uppercase.
    # assert '\ufb05'.upper() == '\ufb06'.upper() == 'ST' # 'ﬅ', 'ﬆ'
    assertTrue(re.match(r'[\ufb04-\ufb05]', '\ufb06', re.I))
    assertTrue(re.match(r'[\ufb06-\ufb07]', '\ufb05', re.I))

def test_category():
    assertEqual(re.match(r"(\s)", " ").group(1), " ")

def test_not_literal():
    assertEqual(re.search(r"\s([^a])", " b").group(1), "b")
    assertEqual(re.search(r"\s([^a]*)", " bb").group(1), "bb")

def test_possible_set_operations():
    s = str(bytes(range(128)))
    assertEqual(re.findall(r'[0-9--1]', s), list('-./0123456789'.elems()))
    assertEqual(re.findall(r'[--1]', s), list('-./01'.elems()))
    assertEqual(re.findall(r'[%--1]', s), list("%&'()*+,-1".elems()))
    assertEqual(re.findall(r'[%--]', s), list("%&'()*+,-".elems()))

    assertEqual(re.findall(r'[0-9&&1]', s), list('&0123456789'.elems()))
    assertEqual(re.findall(r'[0-8&&1]', s), list('&012345678'.elems()))
    assertEqual(re.findall(r'[\d&&1]', s), list('&0123456789'.elems()))
    assertEqual(re.findall(r'[&&1]', s), list('&1'.elems()))

    assertEqual(re.findall(r'[0-9||a]', s), list('0123456789a|'.elems()))
    assertEqual(re.findall(r'[\d||a]', s), list('0123456789a|'.elems()))
    assertEqual(re.findall(r'[||1]', s), list('1|'.elems()))

    assertEqual(re.findall(r'[0-9~~1]', s), list('0123456789~'.elems()))
    assertEqual(re.findall(r'[\d~~1]', s), list('0123456789~'.elems()))
    assertEqual(re.findall(r'[~~1]', s), list('1~'.elems()))

    assertEqual(re.findall(r'[[0-9]|]', s), list('0123456789[]'.elems()))
    assertEqual(re.findall(r'[[0-8]|]', s), list('012345678[]'.elems()))

    assertEqual(re.findall(r'[[:digit:]|]', s), list(':[]dgit'.elems()))

def test_search_coverage():
    assertEqual(re.search(r"\s(b)", " b").group(1), "b")
    assertEqual(re.search(r"a\s", "a ").group(0), "a ")

def assertMatch(pattern, text, match=None, span=None,
                matcher=re.fullmatch):
    if match == None and span == None:
        # the pattern matches the whole text
        match = text
        span = (0, len(text))
    elif match == None or span == None:
        fail('If match is not None, span should be specified ' +
             '(and vice versa).')
    m = matcher(pattern, text)
    assertTrue(m)
    assertEqual(m.group(), match)
    assertEqual(m.span(), span)

LITERAL_CHARS = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!"%\',/:;<=>@_`'

def test_re_escape():
    p = ''.join([chr(i) for i in range(256)])
    for c in p.codepoints():
        assertMatch(re.escape(c), c)
        assertMatch('[' + re.escape(c) + ']', c)
        assertMatch('(?x)' + re.escape(c), c)
    assertMatch(re.escape(p), p)
    for c in '-.]{}'.elems():
        assertEqual(re.escape(c)[:1], '\\')
    literal_chars = LITERAL_CHARS
    assertEqual(re.escape(literal_chars), literal_chars)

def test_re_escape_bytes():
    p = bytes(range(256))
    for i in p.elems():
        b = bytes([i])
        assertMatch(re.escape(b), b)
        assertMatch(bcat(b'[', re.escape(b), b']'), b)
        assertMatch(bcat(b'(?x)', re.escape(b)), b)
    assertMatch(re.escape(p), p)
    for i in b'-.]{}'.elems():
        b = bytes([i])
        assertEqual(re.escape(b)[:1], b'\\')
    literal_chars = bytes(LITERAL_CHARS)
    assertEqual(re.escape(literal_chars), literal_chars)

def test_re_escape_non_ascii():
    s = 'xxx\u2620\u2620\u2620xxx'
    s_escaped = re.escape(s)
    assertEqual(s_escaped, s)
    assertMatch(s_escaped, s)
    assertMatch('.%s+.' % re.escape('\u2620'), s,
                'x\u2620\u2620\u2620x', (2, 13), re.search) # absolute byte positions

def test_re_escape_non_ascii_bytes():
    b = bytes('y\u2620y\u2620y')
    b_escaped = re.escape(b)
    assertEqual(b_escaped, b)
    assertMatch(b_escaped, b)
    res = re.findall(re.escape(bytes('\u2620')), b)
    assertEqual(len(res), 2)

def test_constants():
    assertEqual(re.I, re.IGNORECASE)
    assertEqual(re.L, re.LOCALE)
    assertEqual(re.M, re.MULTILINE)
    assertEqual(re.S, re.DOTALL)
    assertEqual(re.X, re.VERBOSE)

def test_flags():
    for flag in [re.I, re.M, re.X, re.S, re.A, re.U]:
        assertTrue(re.compile('^pattern$', flag))
    for flag in [re.I, re.M, re.X, re.S, re.A, re.L]:
        assertTrue(re.compile(b'^pattern$', flag))

def test_sre_character_literals():
    for i in [0, 8, 16, 32, 64, 127, 128, 255, 256, 0xFFFF, 0x10000, 0x10FFFF]:
        if i < 256:
            assertTrue(re.match(format(r"\%03o", i), chr(i)))
            assertTrue(re.match(format(r"\%03o0", i), chr(i)+"0"))
            assertTrue(re.match(format(r"\%03o8", i), chr(i)+"8"))
            assertTrue(re.match(format(r"\x%02x", i), chr(i)))
            assertTrue(re.match(format(r"\x%02x0", i), chr(i)+"0"))
            assertTrue(re.match(format(r"\x%02xz", i), chr(i)+"z"))
        if i < 0x10000:
            assertTrue(re.match(format(r"\u%04x", i), chr(i)))
            assertTrue(re.match(format(r"\u%04x0", i), chr(i)+"0"))
            assertTrue(re.match(format(r"\u%04xz", i), chr(i)+"z"))
        assertTrue(re.match(format(r"\U%08x", i), chr(i)))
        assertTrue(re.match(format(r"\U%08x0", i), chr(i)+"0"))
        assertTrue(re.match(format(r"\U%08xz", i), chr(i)+"z"))
    assertTrue(re.match(r"\0", "\000"))
    assertTrue(re.match(r"\08", "\0008"))
    assertTrue(re.match(r"\01", "\001"))
    assertTrue(re.match(r"\018", "\0018"))
    checkPatternError(r"\567",
                      r'octal escape value \567 outside of ' +
                      r'range 0-0o377', 0)
    checkPatternError(r"\911", 'invalid group reference 91', 1)
    checkPatternError(r"\x1", r'incomplete escape \x1', 0)
    checkPatternError(r"\x1z", r'incomplete escape \x1', 0)
    checkPatternError(r"\u123", r'incomplete escape \u123', 0)
    checkPatternError(r"\u123z", r'incomplete escape \u123', 0)
    checkPatternError(r"\U0001234", r'incomplete escape \U0001234', 0)
    checkPatternError(r"\U0001234z", r'incomplete escape \U0001234', 0)
    checkPatternError(r"\U00110000", r'bad escape \U00110000', 0)

def test_sre_character_class_literals():
    for i in [0, 8, 16, 32, 64, 127, 128, 255, 256, 0xFFFF, 0x10000, 0x10FFFF]:
        if i < 256:
            assertTrue(re.match(format(r"[\%o]", i), chr(i)))
            assertTrue(re.match(format(r"[\%o8]", i), chr(i)))
            assertTrue(re.match(format(r"[\%03o]", i), chr(i)))
            assertTrue(re.match(format(r"[\%03o0]", i), chr(i)))
            assertTrue(re.match(format(r"[\%03o8]", i), chr(i)))
            assertTrue(re.match(format(r"[\x%02x]", i), chr(i)))
            assertTrue(re.match(format(r"[\x%02x0]", i), chr(i)))
            assertTrue(re.match(format(r"[\x%02xz]", i), chr(i)))
        if i < 0x10000:
            assertTrue(re.match(format(r"[\u%04x]", i), chr(i)))
            assertTrue(re.match(format(r"[\u%04x0]", i), chr(i)))
            assertTrue(re.match(format(r"[\u%04xz]", i), chr(i)))
        assertTrue(re.match(format(r"[\U%08x]", i), chr(i)))
        assertTrue(re.match(format(r"[\U%08x0]", i), chr(i)+"0"))
        assertTrue(re.match(format(r"[\U%08xz]", i), chr(i)+"z"))
    checkPatternError(r"[\567]",
                      r'octal escape value \567 outside of ' +
                      r'range 0-0o377', 1)
    checkPatternError(r"[\911]", r'bad escape \9', 1)
    checkPatternError(r"[\x1z]", r'incomplete escape \x1', 1)
    checkPatternError(r"[\u123z]", r'incomplete escape \u123', 1)
    checkPatternError(r"[\U0001234z]", r'incomplete escape \U0001234', 1)
    checkPatternError(r"[\U00110000]", r'bad escape \U00110000', 1)
    assertTrue(re.match(r"[\U0001d49c-\U0001d4b5]", "\U0001d49e"))

def test_sre_byte_literals():
    for i in [0, 8, 16, 32, 64, 127, 128, 255]:
        assertTrue(re.match(bytes(format(r"\%03o", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"\%03o0", i)), bcat(bytes([i]),b"0")))
        assertTrue(re.match(bytes(format(r"\%03o8", i)), bcat(bytes([i]), b"8")))
        assertTrue(re.match(bytes(format(r"\x%02x", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"\x%02x0", i)), bcat(bytes([i]), b"0")))
        assertTrue(re.match(bytes(format(r"\x%02xz", i)), bcat(bytes([i]), b"z")))
    assertRaises(lambda: re.compile(b"\\u1234"))
    assertRaises(lambda: re.compile(b"\\U00012345"))
    assertTrue(re.match(b"\\0", b"\000"))
    assertTrue(re.match(b"\\08", b"\0008"))
    assertTrue(re.match(b"\\01", b"\001"))
    assertTrue(re.match(b"\\018", b"\0018"))
    checkPatternError(b"\\567",
                      r'octal escape value \567 outside of ' +
                      r'range 0-0o377', 0)
    checkPatternError(b"\\911", 'invalid group reference 91', 1)
    checkPatternError(b"\\x1", r'incomplete escape \x1', 0)
    checkPatternError(b"\\x1z", r'incomplete escape \x1', 0)

def test_sre_byte_class_literals():
    for i in [0, 8, 16, 32, 64, 127, 128, 255]:
        assertTrue(re.match(bytes(format(r"[\%o]", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"[\%o8]", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"[\%03o]", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"[\%03o0]", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"[\%03o8]", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"[\x%02x]", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"[\x%02x0]", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"[\x%02xz]", i)), bytes([i])))
    assertRaises(lambda: re.compile(b"[\\u1234]"))
    assertRaises(lambda: re.compile(b"[\\U00012345]"))
    checkPatternError(b"[\\567]",
                      r'octal escape value \567 outside of ' +
                      r'range 0-0o377', 1)
    checkPatternError(b"[\\911]", r'bad escape \9', 1)
    checkPatternError(b"[\\x1z]", r'incomplete escape \x1', 1)

def test_character_set_errors():
    checkPatternError(r'[', 'unterminated character set', 0)
    checkPatternError(r'[^', 'unterminated character set', 0)
    checkPatternError(r'[a', 'unterminated character set', 0)
    # bug 545855 -- This pattern failed to cause a compile error as it
    # should, instead provoking a TypeError.
    checkPatternError(r"[a-", 'unterminated character set', 0)
    checkPatternError(r"[\w-b]", r'bad character range \w-b', 1)
    checkPatternError(r"[a-\w]", r'bad character range a-\w', 1)
    checkPatternError(r"[b-a]", 'bad character range b-a', 1)

def test_bug_113254():
    assertEqual(re.match(r'(a)|(b)', 'b').start(1), -1)
    assertEqual(re.match(r'(a)|(b)', 'b').end(1), -1)
    assertEqual(re.match(r'(a)|(b)', 'b').span(1), (-1, -1))

def test_bug_527371():
    # bug described in patches 527371/672491
    assertIsNone(re.match(r'(a)?a','a').lastindex)
    assertEqual(re.match(r'(a)(b)?b','ab').lastindex, 1)
    assertEqual(re.match(r'(?P<a>a)(?P<b>b)?b','ab').lastgroup, 'a')
    assertEqual(re.match(r"(?P<a>a(b))", "ab").lastgroup, 'a')
    assertEqual(re.match(r"((a))", "a").lastindex, 1)

def test_bug_418626():
    # bugs 418626 at al. -- Testing Greg Chapman's addition of op code
    # SRE_OP_MIN_REPEAT_ONE for eliminating recursion on simple uses of
    # pattern '*?' on a long string.
    assertEqual(re.match('.*?c', 10000*'ab'+'cd').end(0), 20001)
    assertEqual(re.match('.*?cd', 5000*'ab'+'c'+5000*'ab'+'cde').end(0),
                        20003)
    assertEqual(re.match('.*?cd', 20000*'abc'+'de').end(0), 60001)
    # non-simple '*?' still used to hit the recursion limit, before the
    # non-recursive scheme was implemented.
    assertEqual(re.search('(a|b)*?c', 10000*'ab'+'cd').end(0), 20001)

def test_bug_612074():
    pat="["+re.escape("\u2039")+"]"
    assertEqual(re.compile(pat) and 1, 1)

def test_stack_overflow():
    # nasty cases that used to overflow the straightforward recursive
    # implementation of repeated groups.
    assertEqual(re.match('(x)*', 50000*'x').group(1), 'x')
    assertEqual(re.match('(x)*y', 50000*'x'+'y').group(1), 'x')
    assertEqual(re.match('(x)*?y', 50000*'x'+'y').group(1), 'x')

def test_nothing_to_repeat():
    for reps in '*', '+', '?', '{1,2}':
        for mod in '', '?':
            checkPatternError('%s%s' % (reps, mod),
                              'nothing to repeat', 0)
            checkPatternError('(?:%s%s)' % (reps, mod),
                              'nothing to repeat', 3)

def test_multiple_repeat():
    for outer_reps in '*', '+', '?', '{1,2}':
        for outer_mod in '', '?', '+':
            outer_op = outer_reps + outer_mod
            for inner_reps in '*', '+', '?', '{1,2}':
                for inner_mod in '', '?', '+':
                    if inner_mod + outer_reps in ('?', '+'):
                        continue
                    inner_op = inner_reps + inner_mod
                    checkPatternError(r'x%s%s' % (inner_op, outer_op),
                            'multiple repeat', 1 + len(inner_op))

def test_unlimited_zero_width_repeat():
    # Issue #9669
    assertIsNone(re.match(r'(?:a?)*y', 'z'))
    assertIsNone(re.match(r'(?:a?)+y', 'z'))
    assertIsNone(re.match(r'(?:a?){2,}y', 'z'))
    assertIsNone(re.match(r'(?:a?)*?y', 'z'))
    assertIsNone(re.match(r'(?:a?)+?y', 'z'))
    assertIsNone(re.match(r'(?:a?){2,}?y', 'z'))

def test_bug_448951():
    # bug 448951 (similar to 429357, but with single char match)
    # (Also test greedy matches.)
    for op in ('','?','*'):
        assertEqual(re.match(r'((.%s):)?z'%op, 'z').groups(),
                            (None, None))
        assertEqual(re.match(r'((.%s):)?z'%op, 'a:z').groups(),
                            ('a:', 'a'))

def test_bug_725106():
    # capturing groups in alternatives in repeats
    assertEqual(re.match('^((a)|b)*', 'abc').groups(),
                        ('b', 'a'))
    assertEqual(re.match('^(([ab])|c)*', 'abc').groups(),
                        ('c', 'b'))
    assertEqual(re.match('^((d)|[ab])*', 'abc').groups(),
                        ('b', None))
    assertEqual(re.match('^((a)c|[ab])*', 'abc').groups(),
                        ('b', None))
    assertEqual(re.match('^((a)|b)*?c', 'abc').groups(),
                        ('b', 'a'))
    assertEqual(re.match('^(([ab])|c)*?d', 'abcd').groups(),
                        ('c', 'b'))
    assertEqual(re.match('^((d)|[ab])*?c', 'abc').groups(),
                        ('b', None))
    assertEqual(re.match('^((a)c|[ab])*?c', 'abc').groups(),
                        ('b', None))

def test_bug_725149():
    # mark_stack_base restoring before restoring marks
    assertEqual(re.match('(a)(?:(?=(b)*)c)*', 'abb').groups(),
                        ('a', None))
    assertEqual(re.match('(a)((?!(b)*))*', 'abb').groups(),
                        ('a', None, None))

def test_finditer():
    iter = re.finditer(r":+", "a:b::c:::d")
    assertEqual([item.group(0) for item in iter],
                        [":", "::", ":::"])

    pat = re.compile(r":+")
    iter = pat.finditer("a:b::c:::d", 1, 10)
    assertEqual([item.group(0) for item in iter],
                        [":", "::", ":::"])

    pat = re.compile(r":+")
    iter = pat.finditer("a:b::c:::d", pos=1, endpos=10)
    assertEqual([item.group(0) for item in iter],
                        [":", "::", ":::"])

    pat = re.compile(r":+")
    iter = pat.finditer("a:b::c:::d", endpos=10, pos=1)
    assertEqual([item.group(0) for item in iter],
                        [":", "::", ":::"])

    pat = re.compile(r":+")
    iter = pat.finditer("a:b::c:::d", pos=3, endpos=8)
    assertEqual([item.group(0) for item in iter],
                        ["::", "::"])

def test_bug_926075():
    assertIsNot(re.compile('bug_926075'),
                        re.compile(b'bug_926075'))

def test_bug_931848():
    pattern = "[\u002E\u3002\uFF0E\uFF61]"
    assertEqual(re.compile(pattern).split("a.b.c"),
                        ['a','b','c'])

def test_bug_581080():
    find = [m.span() for m in re.finditer(r"\s", "a b")]
    expect = [(1,2)]
    assertEqual(find, expect)

def test_bug_817234():
    find = [m.span() for m in re.finditer(r".*", "asdf")]
    expect = [(0, 4), (4, 4)]
    assertEqual(find, expect)

def test_bug_6561():
    # '\d' should match characters in Unicode category 'Nd'
    # (Number, Decimal Digit), but not those in 'Nl' (Number,
    # Letter) or 'No' (Number, Other).
    decimal_digits = [
        '\u0037', # '\N{DIGIT SEVEN}', category 'Nd'
        '\u0e58', # '\N{THAI DIGIT SIX}', category 'Nd'
        '\uff10', # '\N{FULLWIDTH DIGIT ZERO}', category 'Nd'
        ]
    for x in decimal_digits:
        assertEqual(re.match(r'^\d$', x).group(0), x)

    not_decimal_digits = [
        '\u2165', # '\N{ROMAN NUMERAL SIX}', category 'Nl'
        '\u3039', # '\N{HANGZHOU NUMERAL TWENTY}', category 'Nl'
        '\u2082', # '\N{SUBSCRIPT TWO}', category 'No'
        '\u32b4', # '\N{CIRCLED NUMBER THIRTY NINE}', category 'No'
        ]
    for x in not_decimal_digits:
        assertIsNone(re.match(r'^\d$', x))

def test_inline_flags():
    # Bug #1700
    upper_char = '\u1ea0' # Latin Capital Letter A with Dot Below
    lower_char = '\u1ea1' # Latin Small Letter A with Dot Below

    p = re.compile('.' + upper_char, re.I | re.S)
    q = p.match('\n' + lower_char)
    assertTrue(q)

    p = re.compile('.' + lower_char, re.I | re.S)
    q = p.match('\n' + upper_char)
    assertTrue(q)

    p = re.compile('(?i).' + upper_char, re.S)
    q = p.match('\n' + lower_char)
    assertTrue(q)

    p = re.compile('(?i).' + lower_char, re.S)
    q = p.match('\n' + upper_char)
    assertTrue(q)

    p = re.compile('(?is).' + upper_char)
    q = p.match('\n' + lower_char)
    assertTrue(q)

    p = re.compile('(?is).' + lower_char)
    q = p.match('\n' + upper_char)
    assertTrue(q)

    p = re.compile('(?s)(?i).' + upper_char)
    q = p.match('\n' + lower_char)
    assertTrue(q)

    p = re.compile('(?s)(?i).' + lower_char)
    q = p.match('\n' + upper_char)
    assertTrue(q)

    assertTrue(re.match('(?ix) ' + upper_char, lower_char))
    assertTrue(re.match('(?ix) ' + lower_char, upper_char))
    assertTrue(re.match(' (?i) ' + upper_char, lower_char, re.X))
    assertTrue(re.match('(?x) (?i) ' + upper_char, lower_char))
    assertTrue(re.match(' (?x) (?i) ' + upper_char, lower_char, re.X))

    msg = "global flags not at the start of the expression"
    checkPatternError(upper_char + '(?i)', msg, 3)

    checkPatternError('(?s).(?i)' + upper_char, msg, 5)
    checkPatternError('(?i) ' + upper_char + ' (?x)', msg, 9)
    checkPatternError(' (?x) (?i) ' + upper_char, msg, 1)
    checkPatternError('^(?i)' + upper_char, msg, 1)
    checkPatternError('$|(?i)' + upper_char, msg, 2)
    checkPatternError('(?:(?i)' + upper_char + ')', msg, 3)
    checkPatternError('(^)?(?(1)(?i)' + upper_char + ')', msg, 9)
    checkPatternError('($)?(?(1)|(?i)' + upper_char + ')', msg, 10)

def test_dollar_matches_twice():
    r"""Test that $ does not include \n
    $ matches the end of string, and just before the terminating \n"""
    pattern = re.compile('$')
    # Not supported: assertEqual(pattern.sub('#', 'a\nb\n'), 'a\nb#\n#')
    assertEqual(pattern.sub('#', 'a\nb\nc'), 'a\nb\nc#')
    # Not supported: assertEqual(pattern.sub('#', '\n'), '#\n#')

    pattern = re.compile('$', re.MULTILINE)
    assertEqual(pattern.sub('#', 'a\nb\n' ), 'a#\nb#\n#' )
    assertEqual(pattern.sub('#', 'a\nb\nc'), 'a#\nb#\nc#')
    assertEqual(pattern.sub('#', '\n'), '#\n#')

def test_bytes_str_mixing():
    # Mixing str and bytes is disallowed
    pat = re.compile('.')
    bpat = re.compile(b'.')
    assertRaises(lambda: pat.match(b'b'))
    assertRaises(lambda: bpat.match('b'))
    assertRaises(lambda: pat.sub(b'b', 'c'))
    assertRaises(lambda: pat.sub('b', b'c'))
    assertRaises(lambda: pat.sub(b'b', b'c'))
    assertRaises(lambda: bpat.sub(b'b', 'c'))
    assertRaises(lambda: bpat.sub('b', b'c'))
    assertRaises(lambda: bpat.sub('b', 'c'))

def test_ascii_and_unicode_flag():
    # String patterns
    for flags in (0, re.UNICODE):
        pat = re.compile('\u00C0', flags | re.IGNORECASE)
        assertTrue(pat.match('\u00E0'))
        pat = re.compile(r'\w', flags)
        assertTrue(pat.match('\u00E0'))
    pat = re.compile('\u00C0', re.ASCII | re.IGNORECASE)
    assertIsNone(pat.match('\u00E0'))
    pat = re.compile('(?a)\u00C0', re.IGNORECASE)
    assertIsNone(pat.match('\u00E0'))
    pat = re.compile(r'\w', re.ASCII)
    assertIsNone(pat.match('\u00E0'))
    pat = re.compile(r'(?a)\w')
    assertIsNone(pat.match('\u00E0'))
    # Bytes patterns
    for flags in (0, re.ASCII):
        pat = re.compile(b'\xc0', flags | re.IGNORECASE)
        assertIsNone(pat.match(b'\xe0'))
        pat = re.compile(b'\\w', flags)
        assertIsNone(pat.match(b'\xe0'))
    # Incompatibilities
    assertRaises(lambda: re.compile(b'\\w', re.UNICODE))
    assertRaises(lambda: re.compile(b'(?u)\\w'))
    assertRaises(lambda: re.compile(r'\w', re.UNICODE | re.ASCII))
    assertRaises(lambda: re.compile(r'(?u)\w', re.ASCII))
    assertRaises(lambda: re.compile(r'(?a)\w', re.UNICODE))
    assertRaises(lambda: re.compile(r'(?au)\w'))

def test_scoped_flags():
    assertTrue(re.match(r'(?i:a)b', 'Ab'))
    assertIsNone(re.match(r'(?i:a)b', 'aB'))
    assertIsNone(re.match(r'(?-i:a)b', 'Ab', re.IGNORECASE))
    assertTrue(re.match(r'(?-i:a)b', 'aB', re.IGNORECASE))
    assertIsNone(re.match(r'(?i:(?-i:a)b)', 'Ab'))
    assertTrue(re.match(r'(?i:(?-i:a)b)', 'aB'))

    assertTrue(re.match(r'\w(?a:\W)\w', '\u00E0\u00E0\u00E0'))
    assertTrue(re.match(r'(?a:\W(?u:\w)\W)', '\u00E0\u00E0\u00E0'))
    assertTrue(re.match(r'\W(?u:\w)\W', '\u00E0\u00E0\u00E0', re.ASCII))

    checkPatternError(r'(?a)(?-a:\w)',
            "bad inline flags: cannot turn off flags 'a', 'u' and 'L'", 8)
    checkPatternError(r'(?i-i:a)',
            'bad inline flags: flag turned on and off', 5)
    checkPatternError(r'(?au:a)',
            "bad inline flags: flags 'a', 'u' and 'L' are incompatible", 4)
    checkPatternError(b'(?aL:a)',
            "bad inline flags: flags 'a', 'u' and 'L' are incompatible", 4)

    checkPatternError(r'(?-', 'missing flag', 3)
    checkPatternError(r'(?-+', 'missing flag', 3)
    checkPatternError(r'(?-z', 'unknown flag', 3)
    checkPatternError(r'(?-i', 'missing :', 4)
    checkPatternError(r'(?-i)', 'missing :', 4)
    checkPatternError(r'(?-i+', 'missing :', 4)
    checkPatternError(r'(?-iz', 'unknown flag', 4)
    checkPatternError(r'(?i:', 'missing ), unterminated subpattern', 0)
    checkPatternError(r'(?i', 'missing -, : or )', 3)
    checkPatternError(r'(?i+', 'missing -, : or )', 3)
    checkPatternError(r'(?iz', 'unknown flag', 3)

def test_ignore_spaces():
    for space in " \t\n\r\v\f".elems():
        assertTrue(re.fullmatch(space + 'a', 'a', re.VERBOSE))
    for space in (b" ", b"\t", b"\n", b"\r", b"\v", b"\f"):
        assertTrue(re.fullmatch(bcat(space, b'a'), b'a', re.VERBOSE))
    assertTrue(re.fullmatch('(?x) a', 'a'))
    assertTrue(re.fullmatch(' (?x) a', 'a', re.VERBOSE))
    assertTrue(re.fullmatch('(?x) (?x) a', 'a'))
    assertTrue(re.fullmatch(' a(?x: b) c', ' ab c'))
    assertTrue(re.fullmatch(' a(?-x: b) c', 'a bc', re.VERBOSE))
    assertTrue(re.fullmatch('(?x) a(?-x: b) c', 'a bc'))
    assertTrue(re.fullmatch('(?x) a| b', 'a'))
    assertTrue(re.fullmatch('(?x) a| b', 'b'))

def test_comments():
    assertTrue(re.fullmatch('#x\na', 'a', re.VERBOSE))
    assertTrue(re.fullmatch(b'#x\na', b'a', re.VERBOSE))
    assertTrue(re.fullmatch('(?x)#x\na', 'a'))
    assertTrue(re.fullmatch('#x\n(?x)#y\na', 'a', re.VERBOSE))
    assertTrue(re.fullmatch('(?x)#x\n(?x)#y\na', 'a'))
    assertTrue(re.fullmatch('#x\na(?x:#y\nb)#z\nc', '#x\nab#z\nc'))
    assertTrue(re.fullmatch('#x\na(?-x:#y\nb)#z\nc', 'a#y\nbc',
                                    re.VERBOSE))
    assertTrue(re.fullmatch('(?x)#x\na(?-x:#y\nb)#z\nc', 'a#y\nbc'))
    assertTrue(re.fullmatch('(?x)#x\na|#y\nb', 'a'))
    assertTrue(re.fullmatch('(?x)#x\na|#y\nb', 'b'))

def test_bug_6509():
    # Replacement strings of both types must parse properly.
    # all strings
    pat = re.compile(r'a(\w)')
    assertEqual(pat.sub('b\\1', 'ac'), 'bc')
    pat = re.compile('a(.)')
    assertEqual(pat.sub('b\\1', 'a\u1234'), 'b\u1234')
    pat = re.compile('..')
    assertEqual(pat.sub(lambda m: 'str', 'a5'), 'str')

    # all bytes
    pat = re.compile(b'a(\\w)')
    assertEqual(pat.sub(b'b\\1', b'ac'), b'bc')
    pat = re.compile(b'a(.)')
    assertEqual(pat.sub(b'b\\1', b'a\xCD'), b'b\xCD')
    pat = re.compile(b'..')
    assertEqual(pat.sub(lambda m: b'bytes', b'a5'), b'bytes')

def test_search_dot_unicode():
    assertTrue(re.search("123.*-", '123abc-'))
    assertTrue(re.search("123.*-", '123\u00E9-'))
    assertTrue(re.search("123.*-", '123\u20ac-'))
    assertTrue(re.search("123.*-", '123\U0010ffff-'))
    assertTrue(re.search("123.*-", '123\u00E9\u20ac\U0010ffff-'))

def test_compile():
    # Test return value when given string and pattern as parameter
    pattern = re.compile('random pattern')
    assertIsInstance(pattern, "Pattern")
    same_pattern = re.compile(pattern)
    assertIsInstance(same_pattern, "Pattern")
    assertIs(same_pattern, pattern)
    # Test behaviour when not given a string or pattern as parameter
    assertRaises(lambda: re.compile(0))

def test_bug_16688():
    # Issue 16688: Backreferences make case-insensitive regex fail on
    # non-ASCII strings.
    assertEqual(re.findall(r"(?i)(a)\1", "aa \u0100"), ['a'])
    assertEqual(re.match(r"(?s).{1,3}", "\u0100\u0100").span(), (0, 4))

def test_repeat_minmax_overflow():
    # Issue #13169
    string = "x" * 100000
    assertEqual(re.match(r".{65535}", string).span(), (0, 65535))
    assertEqual(re.match(r".{,65535}", string).span(), (0, 65535))
    assertEqual(re.match(r".{65535,}?", string).span(), (0, 65535))
    assertEqual(re.match(r".{65536}", string).span(), (0, 65536))
    assertEqual(re.match(r".{,65536}", string).span(), (0, 65536))
    assertEqual(re.match(r".{65536,}?", string).span(), (0, 65536))
    # 1<<128 should be big enough to overflow both SRE_CODE and Py_ssize_t.
    assertRaises(lambda: re.compile(r".{%d}" % 1<<128))
    assertRaises(lambda: re.compile(r".{,%d}" % 1<<128))
    assertRaises(lambda: re.compile(r".{%d,}?" % 1<<128))
    assertRaises(lambda: re.compile(r".{%d,%d}" % (1<<129, 1<<128)))

def test_look_behind_overflow():
    string = "x" * 2500000
    p1 = r"(?<=((.{%d}){%d}){%d})"
    p2 = r"(?<!((.{%d}){%d}){%d})"
    # Test that the templates are valid and look-behind with width 2**21
    # (larger than sys.maxunicode) are supported.
    assertEqual(re.search(p1 % (1<<7, 1<<7, 1<<7), string).span(),
                        (1<<21, 1<<21))
    assertEqual(re.search(p2 % (1<<7, 1<<7, 1<<7), string).span(),
                        (0, 0))
    # Test that 2**22 is accepted as a repetition number and look-behind
    # width.
    re.compile(p1 % (1<<22, 1, 1))
    re.compile(p1 % (1, 1<<22, 1))
    re.compile(p1 % (1, 1, 1<<22))
    re.compile(p2 % (1<<22, 1, 1))
    re.compile(p2 % (1, 1<<22, 1))
    re.compile(p2 % (1, 1, 1<<22))
    # But 2**66 is too large for look-behind width.
    errmsg = "looks too much behind"
    assertRaisesRegex(lambda: re.compile(p1 % (1<<22, 1<<22, 1<<22)), errmsg)
    assertRaisesRegex(lambda: re.compile(p2 % (1<<22, 1<<22, 1<<22)), errmsg)

def test_backref_group_name_in_exception():
    # Issue 17341: Poor error message when compiling invalid regex
    checkPatternError('(?P=<foo>)',
                      "bad character in group name '<foo>'", 4)

def test_group_name_in_exception():
    # Issue 17341: Poor error message when compiling invalid regex
    checkPatternError('(?P<?foo>)',
                      "bad character in group name '?foo'", 4)

def test_issue17998():
    for reps in '*', '+', '?', '{1}':
        for mod in '', '?':
            pattern = '.' + reps + mod + 'yz'
            assertEqual(re.compile(pattern, re.S).findall('xyz'),
                                ['xyz'], msg=pattern)
            pattern = bytes(pattern)
            assertEqual(re.compile(pattern, re.S).findall(b'xyz'),
                                [b'xyz'], msg=pattern)

def test_match_repr():
    string = '[abracadabra]'
    m = re.search(r'(.+)(.*?)\1', string)
    pattern = r"<re\.Match object; span=\(1, 12\), match='abracadabra'>"
    assertRegex(repr(m), pattern)

    string = b'[abracadabra]'
    m = re.search(b'(.+)(.*?)\\1', string)
    pattern = r"<re\.Match object; span=\(1, 12\), match=b'abracadabra'>"
    assertRegex(repr(m), pattern)

    first, second = list(re.finditer("(aa)|(bb)", "aa bb"))
    pattern = r"<re\.Match object; span=\(0, 2\), match='aa'>"
    assertRegex(repr(first), pattern)

    pattern = r"<re\.Match object; span=\(3, 5\), match='bb'>"
    assertRegex(repr(second), pattern)

def test_zerowidth():
    # Issues 852532, 1647489, 3262, 25054.
    assertEqual(re.split(r"\b", "a::bc"), ['', 'a', '::', 'bc', ''])
    assertEqual(re.split(r"\b|:+", "a::bc"), ['', 'a', '', '', 'bc', ''])
    assertEqual(re.split(r"(?<!\w)(?=\w)|:+", "a::bc"), ['', 'a', '', 'bc'])
    # SKIP because regexp2 does not support longest match search:
    # assertEqual(re.split(r"(?<=\w)(?!\w)|:+", "a::bc"), ['a', '', 'bc', ''])

    assertEqual(re.sub(r"\b", "-", "a::bc"), '-a-::-bc-')
    assertEqual(re.sub(r"\b|:+", "-", "a::bc"), '-a---bc-')
    assertEqual(re.sub(r"(\b|:+)", r"[\1]", "a::bc"), '[]a[][::][]bc[]')

    assertEqual(re.findall(r"\b|:+", "a::bc"), ['', '', '::', '', ''])
    assertEqual(re.findall(r"\b|\w+", "a::bc"),
                        ['', 'a', '', '', 'bc', ''])

    assertEqual([m.span() for m in re.finditer(r"\b|:+", "a::bc")],
                        [(0, 0), (1, 1), (1, 3), (3, 3), (5, 5)])
    assertEqual([m.span() for m in re.finditer(r"\b|\w+", "a::bc")],
                        [(0, 0), (0, 1), (1, 1), (3, 3), (3, 5), (5, 5)])

def test_bug_2537():
    # issue 2537: empty submatches
    # Note: the Python and Go regex engines work differently,
    # so this test has to be skipped.
    return
    for outer_op in ('{0,}', '*', '+', '{1,187}'):
        for inner_op in ('{0,}', '*', '?'):
            r = re.compile("^((x|y)%s)%s" % (inner_op, outer_op))
            m = r.match("xyyzy")
            assertEqual(m.group(0), "xyy")
            assertEqual(m.group(1), "")
            assertEqual(m.group(2), "y")

def test_keyword_parameters():
    # Issue #20283: Accepting the string keyword parameter.
    pat = re.compile(r'(ab)')
    assertEqual(
        pat.match(string='abracadabra', pos=7, endpos=10).span(), (7, 9))
    assertEqual(
        pat.fullmatch(string='abracadabra', pos=7, endpos=9).span(), (7, 9))
    assertEqual(
        pat.search(string='abracadabra', pos=3, endpos=10).span(), (7, 9))
    assertEqual(
        pat.findall(string='abracadabra', pos=3, endpos=10), ['ab'])
    assertEqual(
        pat.split(string='abracadabra', maxsplit=1),
        ['', 'ab', 'racadabra'])

def test_bug_20998():
    # Issue #20998: Fullmatch of repeated single character pattern
    # with ignore case.
    assertEqual(re.fullmatch('[a-c]+', 'ABC', re.I).span(), (0, 3))

def check_en_US_iso88591():
    # locale.setlocale(locale.LC_CTYPE, 'en_US.iso88591')
    assertTrue(re.match(b'\xc5\xe5', b'\xc5\xe5', re.L|re.I))
    assertTrue(re.match(b'\xc5', b'\xe5', re.L|re.I))
    assertTrue(re.match(b'\xe5', b'\xc5', re.L|re.I))
    assertTrue(re.match(b'(?Li)\xc5\xe5', b'\xc5\xe5'))
    assertTrue(re.match(b'(?Li)\xc5', b'\xe5'))
    assertTrue(re.match(b'(?Li)\xe5', b'\xc5'))

def check_en_US_utf8():
    # locale.setlocale(locale.LC_CTYPE, 'en_US.utf8')
    assertTrue(re.match(b'\xc5\xe5', b'\xc5\xe5', re.L|re.I))
    assertIsNone(re.match(b'\xc5', b'\xe5', re.L|re.I))
    assertIsNone(re.match(b'\xe5', b'\xc5', re.L|re.I))
    assertTrue(re.match(b'(?Li)\xc5\xe5', b'\xc5\xe5'))
    assertIsNone(re.match(b'(?Li)\xc5', b'\xe5'))
    assertIsNone(re.match(b'(?Li)\xe5', b'\xc5'))

def test_error():
    _, err = trycatch(lambda: re.compile('(\u20ac))'))
    assertTrue(' at position 5' in err)

    # Bytes pattern
    _, err = trycatch(lambda: re.compile(b'(\xa4))'))
    assertTrue(' at position 3' in err)

    # Multiline pattern
    _, err = trycatch(lambda: re.compile("""
            (
                abc
            )
            )
            (
            """, re.VERBOSE))
    assertTrue(' at position 61' in err)
    assertTrue('(line 5, column 13)' in err)

def test_misc_errors():
    checkPatternError(r'(', 'missing ), unterminated subpattern', 0)
    checkPatternError(r'((a|b)', 'missing ), unterminated subpattern', 0)
    checkPatternError(r'(a|b))', 'unbalanced parenthesis', 5)
    checkPatternError(r'(?P', 'unexpected end of pattern', 3)
    checkPatternError(r'(?z)', 'unknown extension ?z', 1)
    checkPatternError(r'(?iz)', 'unknown flag', 3)
    checkPatternError(r'(?i', 'missing -, : or )', 3)
    checkPatternError(r'(?#abc', 'missing ), unterminated comment', 0)
    checkPatternError(r'(?<', 'unexpected end of pattern', 3)
    checkPatternError(r'(?<>)', 'unknown extension ?<>', 1)
    checkPatternError(r'(?', 'unexpected end of pattern', 2)

def test_pattern_compare():
    pattern1 = re.compile('abc', re.IGNORECASE)

    # equal to itself
    assertEqual(pattern1, pattern1)
    assertFalse(pattern1 != pattern1)

    # equal
    re.purge()
    pattern2 = re.compile('abc', re.IGNORECASE)
    assertEqual(pattern2, pattern1)

    # not equal: different pattern
    re.purge()
    pattern3 = re.compile('XYZ', re.IGNORECASE)
    # Don't test hash(pattern3) != hash(pattern1) because there is no
    # warranty that hash values are different
    assertNotEqual(pattern3, pattern1)

    # not equal: different flag (flags=0)
    re.purge()
    pattern4 = re.compile('abc')
    assertNotEqual(pattern4, pattern1)

    # only == and != comparison operators are supported
    assertRaises(lambda: pattern1 < pattern2)

def test_pattern_compare_bytes():
    pattern1 = re.compile(b'abc')

    # equal: test bytes patterns
    re.purge()
    pattern2 = re.compile(b'abc')
    assertEqual(pattern2, pattern1)

    # not equal: pattern of a different types (str vs bytes),
    # comparison must not raise a BytesWarning
    re.purge()
    pattern3 = re.compile('abc')
    assertNotEqual(pattern3, pattern1)

def test_bug_34294():
    # Issue 34294: wrong capturing groups

    # exists since Python 2
    s = "a\tx"
    p = r"\b(?=(\t)|(x))x"
    assertEqual(re.search(p, s).groups(), (None, 'x'))

    # introduced in Python 3.7.0
    s = "ab"
    p = r"(?=(.)(.)?)"
    assertEqual(re.findall(p, s),
                        [('a', 'b'), ('b', '')])
    assertEqual([m.groups() for m in re.finditer(p, s)],
                        [('a', 'b'), ('b', None)])

    # test-cases provided by issue34294, introduced in Python 3.7.0
    p = r"(?=<(?P<tag>\w+)/?>(?:(?P<text>.+?)</(?P=tag)>)?)"
    s = "<test><foo2/></test>"
    assertEqual(re.findall(p, s),
                        [('test', '<foo2/>'), ('foo2', '')])
    assertEqual([m.groupdict() for m in re.finditer(p, s)],
                        [{'tag': 'test', 'text': '<foo2/>'},
                        {'tag': 'foo2', 'text': None}])
    s = "<test>Hello</test><foo/>"
    assertEqual([m.groupdict() for m in re.finditer(p, s)],
                        [{'tag': 'test', 'text': 'Hello'},
                        {'tag': 'foo', 'text': None}])
    s = "<test>Hello</test><foo/><foo/>"
    assertEqual([m.groupdict() for m in re.finditer(p, s)],
                        [{'tag': 'test', 'text': 'Hello'},
                        {'tag': 'foo', 'text': None},
                        {'tag': 'foo', 'text': None}])

def test_MARK_PUSH_macro_bug():
    # issue35859, MARK_PUSH() macro didn't protect MARK-0 if it
    # was the only available mark.
    assertEqual(re.match(r'(ab|a)*?b', 'ab').groups(), ('a',))
    assertEqual(re.match(r'(ab|a)+?b', 'ab').groups(), ('a',))
    assertEqual(re.match(r'(ab|a){0,2}?b', 'ab').groups(), ('a',))
    assertEqual(re.match(r'(.b|a)*?b', 'ab').groups(), ('a',))

def test_MIN_UNTIL_mark_bug():
    # Fixed in issue35859, reported in issue9134.
    # JUMP_MIN_UNTIL_2 should MARK_PUSH() if in a repeat
    s = 'axxzbcz'
    p = r'(?:(?:a|bc)*?(xx)??z)*'
    assertEqual(re.match(p, s).groups(), ('xx',))

    # test-case provided by issue9134
    s = 'xtcxyzxc'
    p = r'((x|yz)+?(t)??c)*'
    m = re.match(p, s)
    assertEqual(m.span(), (0, 8))
    assertEqual(m.span(2), (6, 7))
    assertEqual(m.groups(), ('xyzxc', 'x', 't'))

def test_REPEAT_ONE_mark_bug():
    # issue35859
    # JUMP_REPEAT_ONE_1 should MARK_PUSH() if in a repeat
    s = 'aabaab'
    p = r'(?:[^b]*a(?=(b)|(a))ab)*'
    m = re.match(p, s)
    assertEqual(m.span(), (0, 6))
    assertEqual(m.span(2), (4, 5))
    assertEqual(m.groups(), (None, 'a'))

    # JUMP_REPEAT_ONE_2 should MARK_PUSH() if in a repeat
    s = 'abab'
    p = r'(?:[^b]*(?=(b)|(a))ab)*'
    m = re.match(p, s)
    assertEqual(m.span(), (0, 4))
    assertEqual(m.span(2), (2, 3))
    assertEqual(m.groups(), (None, 'a'))

    assertEqual(re.match(r'(ab?)*?b', 'ab').groups(), ('a',))

def test_MIN_REPEAT_ONE_mark_bug():
    # issue35859
    # JUMP_MIN_REPEAT_ONE should MARK_PUSH() if in a repeat
    s = 'abab'
    p = r'(?:.*?(?=(a)|(b))b)*'
    m = re.match(p, s)
    assertEqual(m.span(), (0, 4))
    assertEqual(m.span(2), (3, 4))
    assertEqual(m.groups(), (None, 'b'))

    s = 'axxzaz'
    p = r'(?:a*?(xx)??z)*'
    assertEqual(re.match(p, s).groups(), ('xx',))

def test_ASSERT_NOT_mark_bug():
    # Fixed in issue35859, reported in issue725149.
    # JUMP_ASSERT_NOT should LASTMARK_SAVE()
    assertEqual(re.match(r'(?!(..)c)', 'ab').groups(), (None,))

    # JUMP_ASSERT_NOT should MARK_PUSH() if in a repeat
    m = re.match(r'((?!(ab)c)(.))*', 'abab')
    assertEqual(m.span(), (0, 4))
    assertEqual(m.span(1), (3, 4))
    assertEqual(m.span(3), (3, 4))
    assertEqual(m.groups(), ('b', None, 'b'))

def test_bug_40736():
    assertRaisesRegex(lambda: re.search("x*", 5), "got int")
    assertRaisesRegex(lambda: re.search("x*", type), "got builtin_function_or_method")

def test_search_anchor_at_beginning():
    s = 'x'*10000000 # 10**7
    def fn():
        for p in r'\Ay', r'^y':
            assertIsNone(re.search(p, s))
            assertEqual(re.split(p, s), [s])
            assertEqual(re.findall(p, s), [])
            assertEqual(list(re.finditer(p, s)), [])
            assertEqual(re.sub(p, '', s), s)
    # Without optimization it takes 1 second on my computer.
    # With optimization -- 0.0003 seconds.
    assertLess(measure(fn), 0.25)

def test_possessive_quantifiers():
    """Test Possessive Quantifiers
    Test quantifiers of the form @+ for some repetition operator @,
    e.g. x{3,5}+ meaning match from 3 to 5 greadily and proceed
    without creating a stack frame for rolling the stack back and
    trying 1 or more fewer matches."""
    # Not supported by either Go regex engine; skip.
    return
    assertIsNone(re.match('e*+e', 'eeee'))
    assertEqual(re.match('e++a', 'eeea').group(0), 'eeea')
    assertEqual(re.match('e?+a', 'ea').group(0), 'ea')
    assertEqual(re.match('e{2,4}+a', 'eeea').group(0), 'eeea')
    assertIsNone(re.match('(.)++.', 'ee'))
    assertEqual(re.match('(ae)*+a', 'aea').groups(), ('ae',))
    assertEqual(re.match('([ae][ae])?+a', 'aea').groups(),
                        ('ae',))
    assertEqual(re.match('(e?){2,4}+a', 'eeea').groups(),
                        ('',))
    assertEqual(re.match('()*+a', 'a').groups(), ('',))
    assertEqual(re.search('x*+', 'axx').span(), (0, 0))
    assertEqual(re.search('x++', 'axx').span(), (1, 3))
    assertEqual(re.match('a*+', 'xxx').span(), (0, 0))
    assertEqual(re.match('x*+', 'xxxa').span(), (0, 3))
    assertIsNone(re.match('a++', 'xxx'))
    assertIsNone(re.match(r"^(\w){1}+$", "abc"))
    assertIsNone(re.match(r"^(\w){1,2}+$", "abc"))

    assertEqual(re.match(r"^(\w){3}+$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){1,3}+$", "abc").group(1), "c")
    assertEqual(re.match(r"^(\w){1,4}+$", "abc").group(1), "c")

    assertIsNone(re.match("^x{1}+$", "xxx"))
    assertIsNone(re.match("^x{1,2}+$", "xxx"))

    assertTrue(re.match("^x{3}+$", "xxx"))
    assertTrue(re.match("^x{1,3}+$", "xxx"))
    assertTrue(re.match("^x{1,4}+$", "xxx"))

    assertIsNone(re.match("^x{}+$", "xxx"))
    assertTrue(re.match("^x{}+$", "x{}"))

def test_fullmatch_possessive_quantifiers():
    # Not supported by either Go regex engine; skip.
    return
    assertTrue(re.fullmatch(r'a++', 'a'))
    assertTrue(re.fullmatch(r'a*+', 'a'))
    assertTrue(re.fullmatch(r'a?+', 'a'))
    assertTrue(re.fullmatch(r'a{1,3}+', 'a'))
    assertIsNone(re.fullmatch(r'a++', 'ab'))
    assertIsNone(re.fullmatch(r'a*+', 'ab'))
    assertIsNone(re.fullmatch(r'a?+', 'ab'))
    assertIsNone(re.fullmatch(r'a{1,3}+', 'ab'))
    assertTrue(re.fullmatch(r'a++b', 'ab'))
    assertTrue(re.fullmatch(r'a*+b', 'ab'))
    assertTrue(re.fullmatch(r'a?+b', 'ab'))
    assertTrue(re.fullmatch(r'a{1,3}+b', 'ab'))

    assertTrue(re.fullmatch(r'(?:ab)++', 'ab'))
    assertTrue(re.fullmatch(r'(?:ab)*+', 'ab'))
    assertTrue(re.fullmatch(r'(?:ab)?+', 'ab'))
    assertTrue(re.fullmatch(r'(?:ab){1,3}+', 'ab'))
    assertIsNone(re.fullmatch(r'(?:ab)++', 'abc'))
    assertIsNone(re.fullmatch(r'(?:ab)*+', 'abc'))
    assertIsNone(re.fullmatch(r'(?:ab)?+', 'abc'))
    assertIsNone(re.fullmatch(r'(?:ab){1,3}+', 'abc'))
    assertTrue(re.fullmatch(r'(?:ab)++c', 'abc'))
    assertTrue(re.fullmatch(r'(?:ab)*+c', 'abc'))
    assertTrue(re.fullmatch(r'(?:ab)?+c', 'abc'))
    assertTrue(re.fullmatch(r'(?:ab){1,3}+c', 'abc'))

def test_findall_possessive_quantifiers():
    # Not supported by either Go regex engine; skip.
    return
    assertEqual(re.findall(r'a++', 'aab'), ['aa'])
    assertEqual(re.findall(r'a*+', 'aab'), ['aa', '', ''])
    assertEqual(re.findall(r'a?+', 'aab'), ['a', 'a', '', ''])
    assertEqual(re.findall(r'a{1,3}+', 'aab'), ['aa'])

    assertEqual(re.findall(r'(?:ab)++', 'ababc'), ['abab'])
    assertEqual(re.findall(r'(?:ab)*+', 'ababc'), ['abab', '', ''])
    assertEqual(re.findall(r'(?:ab)?+', 'ababc'), ['ab', 'ab', '', ''])
    assertEqual(re.findall(r'(?:ab){1,3}+', 'ababc'), ['abab'])

def test_atomic_grouping():
    """Test Atomic Grouping
    Test non-capturing groups of the form (?>...), which does
    not maintain any stack point created within the group once the
    group is finished being evaluated."""
    pattern1 = re.compile(r'a(?>bc|b)c')
    assertIsNone(pattern1.match('abc'))
    assertTrue(pattern1.match('abcc'))
    assertIsNone(re.match(r'(?>.*).', 'abc'))
    # SKIP: assertTrue(re.match(r'(?>x)++', 'xxx'))
    # SKIP: assertTrue(re.match(r'(?>x++)', 'xxx'))
    # SKIP: assertIsNone(re.match(r'(?>x)++x', 'xxx'))
    # SKIP: assertIsNone(re.match(r'(?>x++)x', 'xxx'))

def test_fullmatch_atomic_grouping():
    assertTrue(re.fullmatch(r'(?>a+)', 'a'))
    assertTrue(re.fullmatch(r'(?>a*)', 'a'))
    assertTrue(re.fullmatch(r'(?>a?)', 'a'))
    assertTrue(re.fullmatch(r'(?>a{1,3})', 'a'))
    assertIsNone(re.fullmatch(r'(?>a+)', 'ab'))
    assertIsNone(re.fullmatch(r'(?>a*)', 'ab'))
    assertIsNone(re.fullmatch(r'(?>a?)', 'ab'))
    assertIsNone(re.fullmatch(r'(?>a{1,3})', 'ab'))
    assertTrue(re.fullmatch(r'(?>a+)b', 'ab'))
    assertTrue(re.fullmatch(r'(?>a*)b', 'ab'))
    assertTrue(re.fullmatch(r'(?>a?)b', 'ab'))
    assertTrue(re.fullmatch(r'(?>a{1,3})b', 'ab'))

    assertTrue(re.fullmatch(r'(?>(?:ab)+)', 'ab'))
    assertTrue(re.fullmatch(r'(?>(?:ab)*)', 'ab'))
    assertTrue(re.fullmatch(r'(?>(?:ab)?)', 'ab'))
    assertTrue(re.fullmatch(r'(?>(?:ab){1,3})', 'ab'))
    assertIsNone(re.fullmatch(r'(?>(?:ab)+)', 'abc'))
    assertIsNone(re.fullmatch(r'(?>(?:ab)*)', 'abc'))
    assertIsNone(re.fullmatch(r'(?>(?:ab)?)', 'abc'))
    assertIsNone(re.fullmatch(r'(?>(?:ab){1,3})', 'abc'))
    assertTrue(re.fullmatch(r'(?>(?:ab)+)c', 'abc'))
    assertTrue(re.fullmatch(r'(?>(?:ab)*)c', 'abc'))
    assertTrue(re.fullmatch(r'(?>(?:ab)?)c', 'abc'))
    assertTrue(re.fullmatch(r'(?>(?:ab){1,3})c', 'abc'))

def test_findall_atomic_grouping():
    assertEqual(re.findall(r'(?>a+)', 'aab'), ['aa'])
    assertEqual(re.findall(r'(?>a*)', 'aab'), ['aa', '', ''])
    assertEqual(re.findall(r'(?>a?)', 'aab'), ['a', 'a', '', ''])
    assertEqual(re.findall(r'(?>a{1,3})', 'aab'), ['aa'])

    assertEqual(re.findall(r'(?>(?:ab)+)', 'ababc'), ['abab'])
    assertEqual(re.findall(r'(?>(?:ab)*)', 'ababc'), ['abab', '', ''])
    assertEqual(re.findall(r'(?>(?:ab)?)', 'ababc'), ['ab', 'ab', '', ''])
    assertEqual(re.findall(r'(?>(?:ab){1,3})', 'ababc'), ['abab'])

def test_bug_gh91616():
    assertTrue(re.fullmatch(r'(?s:(?>.*?\.).*)\Z', "a.txt")) # reproducer
    assertTrue(re.fullmatch(r'(?s:(?=(?P<g0>.*?\.))(?P=g0).*)\Z', "a.txt"))

def test_bug_gh100061():
    # gh-100061
    assertEqual(re.match('(?>(?:.(?!D))+)', 'ABCDE').span(), (0, 2))
    # SKIP: assertEqual(re.match('(?:.(?!D))++', 'ABCDE').span(), (0, 2))
    assertEqual(re.match('(?>(?:.(?!D))*)', 'ABCDE').span(), (0, 2))
    # SKIP: assertEqual(re.match('(?:.(?!D))*+', 'ABCDE').span(), (0, 2))
    assertEqual(re.match('(?>(?:.(?!D))?)', 'CDE').span(), (0, 0))
    # SKIP: assertEqual(re.match('(?:.(?!D))?+', 'CDE').span(), (0, 0))
    assertEqual(re.match('(?>(?:.(?!D)){1,3})', 'ABCDE').span(), (0, 2))
    # SKIP: assertEqual(re.match('(?:.(?!D)){1,3}+', 'ABCDE').span(), (0, 2))
    # gh-106052
    assertEqual(re.match("(?>(?:ab?c)+)", "aca").span(), (0, 2))
    # SKIP: assertEqual(re.match("(?:ab?c)++", "aca").span(), (0, 2))
    assertEqual(re.match("(?>(?:ab?c)*)", "aca").span(), (0, 2))
    # SKIP: assertEqual(re.match("(?:ab?c)*+", "aca").span(), (0, 2))
    assertEqual(re.match("(?>(?:ab?c)?)", "a").span(), (0, 0))
    # SKIP: assertEqual(re.match("(?:ab?c)?+", "a").span(), (0, 0))
    assertEqual(re.match("(?>(?:ab?c){1,3})", "aca").span(), (0, 2))
    # SKIP: assertEqual(re.match("(?:ab?c){1,3}+", "aca").span(), (0, 2))

def test_fail():
    assertEqual(re.search(r'12(?!)|3', '123')[0], '3')

def test_character_set_any():
    # The union of complementary character sets mathes any character
    # and is equivalent to "(?s:.)".
    s = '1x\n'
    for p in (r'[\s\S]', r'[\d\D]', r'[\w\W]', r'[\S\s]', r'\s|\S'):
        assertEqual(re.findall(p, s), list(s.codepoints()))
        assertEqual(re.fullmatch('(?:' + p + ')+', s).group(), s)

def test_character_set_none():
    # Negation of the union of complementary character sets does not match
    # any character.
    s = '1x\n'
    for p in (r'[^\s\S]', r'[^\d\D]', r'[^\w\W]', r'[^\S\s]'):
        assertIsNone(re.search(p, s))
        assertIsNone(re.search('(?s:.)' + p, s))

def get_debug_out(pat):
    return capture_output(lambda: re.compile(pat, re.DEBUG))[1]

def test_debug_flag():
    pat = r'(\.)(?:[ch]|py)(?(1)$|: )'
    dump = '''\
SUBPATTERN 1 0 0
  LITERAL 46
BRANCH
  IN
    LITERAL 99
    LITERAL 104
OR
  LITERAL 112
  LITERAL 121
GROUPREF_EXISTS 1
  AT AT_END
ELSE
  LITERAL 58
  LITERAL 32
'''
    assertEqual(get_debug_out(pat), dump)
    # Debug output is output again even a second time (bypassing
    # the cache -- issue #20426).
    assertEqual(get_debug_out(pat), dump)

def test_atomic_group():
    assertEqual(get_debug_out(r'(?>ab?)'), '''\
ATOMIC_GROUP
  LITERAL 97
  MAX_REPEAT 0 1
    LITERAL 98
''')

def check(pattern, expected):
    assertEqual(repr(re.compile(pattern)), expected)

def check_flags(pattern, flags, expected):
    assertEqual(repr(re.compile(pattern, flags)), expected)

def test_without_flags():
    check('random pattern',
                "re.compile('random pattern')")

def test_single_flag():
    check_flags('random pattern', re.IGNORECASE,
        "re.compile('random pattern', re.IGNORECASE)")

def test_multiple_flags():
    check_flags('random pattern', re.I|re.S|re.X,
        "re.compile('random pattern', " +
        "re.IGNORECASE|re.DOTALL|re.VERBOSE)")

def test_unicode_flag():
    check_flags('random pattern', re.U,
                        "re.compile('random pattern')")
    check_flags('random pattern', re.I|re.S|re.U,
                        "re.compile('random pattern', " +
                        "re.IGNORECASE|re.DOTALL)")

def test_inline_flags_2():
    check('(?i)pattern',
                "re.compile('(?i)pattern', re.IGNORECASE)")

def test_unknown_flags():
    check_flags('random pattern', 0x123000,
                        "re.compile('random pattern', 0x123000)")
    check_flags('random pattern', 0x123000|re.I,
        "re.compile('random pattern', re.IGNORECASE|0x123000)")

def test_bytes():
    check(b'bytes pattern',
                "re.compile(b'bytes pattern')")
    check_flags(b'bytes pattern', re.A,
                        "re.compile(b'bytes pattern', re.ASCII)")

def test_locale():
    check_flags(b'bytes pattern', re.L,
                        "re.compile(b'bytes pattern', re.LOCALE)")

def test_quotes():
    check('random "double quoted" pattern',
        '''re.compile('random "double quoted" pattern')''')
    check("random 'single quoted' pattern",
        '''re.compile("random 'single quoted' pattern")''')
    check('''both 'single' and "double" quotes''',
        '''re.compile('both \\'single\\' and "double" quotes')''')

def test_long_pattern():
    pattern = 'Very %spattern' % ('long ' * 1000)
    r = repr(re.compile(pattern))
    assertLess(len(r), 300)
    assertEqual(r[:30], "re.compile('Very long long lon")
    r = repr(re.compile(pattern, re.I))
    assertLess(len(r), 300)
    assertEqual(r[:30], "re.compile('Very long long lon")
    assertEqual(r[-16:], ", re.IGNORECASE)")

def test_flags_repr():
    assertEqual(repr(re.I), "2")
    assertEqual(repr(re.I|re.S|re.X), "82")
    assertEqual(repr(re.I|re.S|re.X|(1<<20)), "1048658")
    assertEqual(repr(~re.I), "-3")
    assertEqual(repr(~(re.I|re.S|re.X)), "-83")
    assertEqual(repr(~(re.I|re.S|re.X|(1<<20))), "-1048659")


def test_immutable():
    # bpo-43908: check that re types are immutable
    def cb1():
        re.Match.foo = 1
    assertRaises(cb1)
    def cb2():
        re.Pattern.foo = 1
    assertRaises(cb2)

def test_repeat_minmax_overflow_maxrepeat():
    # MAXREPEAT provided as a global variable
    string = "x" * 100000
    assertIsNone(re.match(r".{%d}" % (MAXREPEAT - 1), string))
    assertEqual(re.match(r".{,%d}" % (MAXREPEAT - 1), string).span(),
                (0, 100000))
    assertIsNone(re.match(r".{%d,}?" % (MAXREPEAT - 1), string))
    assertRaises(lambda: re.compile(r".{%d}" % MAXREPEAT))
    assertRaises(lambda: re.compile(r".{,%d}" % MAXREPEAT))
    assertRaises(lambda: re.compile(r".{%d,}?" % MAXREPEAT))

def test_re_benchmarks():
    're_tests benchmarks'
    for pattern, s in benchmarks:
        p = re.compile(pattern)
        assertTrue(p.search(s))
        assertTrue(p.match(s))
        assertTrue(p.fullmatch(s))
        s2 = ' '*10000 + s + ' '*10000
        assertTrue(p.search(s2))
        assertTrue(p.match(s2, 10000))
        assertTrue(p.match(s2, 10000, 10000 + len(s)))
        assertTrue(p.fullmatch(s2, 10000, 10000 + len(s)))

# decode a string from ascii
def decode_ascii(s):
    b = []
    for c in s.codepoint_ords():
        if c >= 128:
            return None
        b.append(c)

    return bytes(b)

def test_re_tests():
    're_tests test suite'
    for t in tests:
        [pattern, s, outcome, repl, expected] = [None] * 5
        if len(t) == 5:
            pattern, s, outcome, repl, expected = t
        elif len(t) == 3:
            pattern, s, outcome = t
        else:
            fail('Test tuples should have 3 or 5 fields: {}'.format(t))

        if outcome == SYNTAX_ERROR:  # Expected a syntax error
            assertRaises(lambda: re.compile(pattern))
            continue

        obj = re.compile(pattern)
        result = obj.search(s)
        if outcome == FAIL:
            assertIsNone(result, 'Succeeded incorrectly')
            continue

        assertTrue(result, 'Failed incorrectly')
        # Matched, as expected, so now we compute the
        # result string and compare it to our expected result.
        start, end = result.span(0)
        vardict = {'found': result.group(0),
                    'groups': result.group(),
                    'flags': result.re.flags}
        
        def getgroup(i):
            gi = result.group(i)
            # Special hack because else the string concat fails:
            if gi == None:
                gi = "None"
            return gi

        for i in range(1, 100):
            gi, e = trycatch(getgroup, i)
            if e != None:
                gi = "Error"
            vardict['g%d' % i] = gi
        for i in result.re.groupindex.keys():
            gi, e = trycatch(getgroup, i)
            if e != None:
                gi = "Error"
            vardict[i] = gi
        assertEqual(eval(repl, vardict), expected, 'grouping error')

        # Try the match with both pattern and string converted to
        # bytes, and check that it still succeeds.
        bpat = decode_ascii(pattern)
        bs = decode_ascii(s)

        if bpat != None and bs != None:
            obj = re.compile(bpat)
            assertTrue(obj.search(bs))

        # Try the match with the search area limited to the extent
        # of the match and see if it still succeeds.  \B will
        # break (because it won't match at the end or start of a
        # string), so we'll ignore patterns that feature it.
        if (pattern[:2] != r'\B' and pattern[-2:] != r'\B' and result != None):
            obj = re.compile(pattern)
            assertTrue(obj.search(s, start, end + 1))

        # Try the match with IGNORECASE enabled, and check that it
        # still succeeds.
        obj = re.compile(pattern, re.IGNORECASE)
        assertTrue(obj.search(s))

        # Try the match with UNICODE locale enabled, and check
        # that it still succeeds.
        obj = re.compile(pattern, re.UNICODE)
        assertTrue(obj.search(s))


# Extra tests for Starlark re

def test_interface():
    assertEqual(str(re), '<module re>')
    assertEqual(type(re), 'module')
    assertEqual(bool(re), True)
    assertRaisesRegex(lambda: {re: 0}, 'unhashable')
    assertRaisesRegex(lambda: re.unknown,
                      'has no .unknown field or method')

    p = re.compile(r'.')
    assertEqual(str(p), "re.compile('.')")
    assertEqual(type(p), 'Pattern')
    assertEqual(bool(p), True)
    assertEqual(len({p: 0}), 1)
    assertRaisesRegex(lambda: p.unknown,
                      'has no .unknown field or method')

    m = p.match('x')
    assertEqual(str(m), "<re.Match object; span=(0, 1), match='x'>")
    assertEqual(type(m), 'Match')
    assertEqual(bool(m), True)
    assertEqual(len({m: 0}), 1)
    assertRaisesRegex(lambda: m.unknown,
                      'has no .unknown field or method')

    i = p.finditer('abc')
    assertRegex(str(i), 'match_iterator object at')
    assertEqual(type(i), 'match_iterator')
    assertEqual(bool(i), True)
    assertRaisesRegex(lambda: {i: 0}, 'unhashable')
    assertRaisesRegex(lambda: i.unknown,
                      'has no .unknown field or method')

def test_compare():
    assertEqual(re, re)

    p = re.compile(r'.')
    assertEqual(p, re.compile(r'.'))
    assertEqual(p, re.compile(r'.', 0))
    assertNotEqual(p, re.compile(r'.', re.IGNORECASE))
    assertNotEqual(p, re.compile(b'.'))
    assertRaises(lambda: p < re.compile(r'.'),
                 'Pattern < Pattern not implemented')

    assertEqual(p.match('xyz'), re.match(r'.', 'xyz'))
    assertEqual(p.match('xyz'), re.match(r'.', 'xyz', flags=0))
    assertNotEqual(p.match('xyz'), p.match('x'))
    assertNotEqual(p.match('xyz'), re.match(b'.', b'xyz'))
    assertRaises(lambda: p.match('xyz') < re.match(r'.', 'xyz'),
                 'Match < Match not implemented')

def test_invalid_args():
    assertRaises(lambda: re.compile())
    assertRaises(lambda: re.purge(0))
    assertRaises(lambda: re.escape())

    assertRaises(lambda: re.search())
    assertRaises(lambda: re.search(r'*'))
    assertRaises(lambda: re.search(r'.', b'x'))

    assertRaises(lambda: re.match())
    assertRaises(lambda: re.match(r'*', ''))
    assertRaises(lambda: re.match(r'.', b'x'))

    assertRaises(lambda: re.fullmatch())
    assertRaises(lambda: re.fullmatch(r'*', ''))
    assertRaises(lambda: re.fullmatch(r'.', b'x'))

    assertRaises(lambda: re.split())
    assertRaises(lambda: re.split(r'*', ''))
    assertRaises(lambda: re.split(r'.', b'x'))

    assertRaises(lambda: re.findall())
    assertRaises(lambda: re.findall(r'*', ''))
    assertRaises(lambda: re.findall(r'.', b'x'))

    assertRaises(lambda: re.finditer())
    assertRaises(lambda: re.finditer(r'*', ''))
    assertRaises(lambda: re.finditer(r'.', b'x'))

    assertRaises(lambda: re.sub())
    assertRaises(lambda: re.sub(r'*', '', ''))
    assertRaises(lambda: re.sub(r'.', b'x', ''))

    assertRaises(lambda: re.subn())
    assertRaises(lambda: re.subn(r'*', '', ''))
    assertRaises(lambda: re.subn(r'.', b'x', ''))

def test_invalid_args_pattern():
    p = re.compile(r'.')

    assertRaises(lambda: p.search())
    assertRaises(lambda: p.search(b'x'))

    assertRaises(lambda: p.match())
    assertRaises(lambda: p.match(b'x'))

    assertRaises(lambda: p.fullmatch())
    assertRaises(lambda: p.fullmatch(b'x'))

    assertRaises(lambda: p.split())
    assertRaises(lambda: p.split(b'x'))

    assertRaises(lambda: p.findall())
    assertRaises(lambda: p.findall(b'x'))

    assertRaises(lambda: p.finditer())
    assertRaises(lambda: p.finditer(b'x'))

    assertRaises(lambda: p.sub())
    assertRaises(lambda: p.sub(b'x', ''))

    assertRaises(lambda: p.subn())
    assertRaises(lambda: p.subn(b'x', ''))

def test_invalid_args_match():
    m = re.match(r'.', 'x')

    assertRaises(lambda: m.expand())
    assertRaises(lambda: m.expand(b'x'))
    assertRaises(lambda: m.expand('\\'))

    assertRaises(lambda: m.group(key=None))
    assertRaises(lambda: m.group(None))
    assertRaises(lambda: m.group(0, None))

    assertRaises(lambda: m.groups(none=None))
    assertRaises(lambda: m.groupdict(none=None))

    assertRaises(lambda: m.start(None))
    assertRaises(lambda: m.start(none=None))

    assertRaises(lambda: m.end(None))
    assertRaises(lambda: m.end(none=None))

    assertRaises(lambda: m.span(None))
    assertRaises(lambda: m.span(none=None))

load("re.star", re_compile="compile", re_match="match") # type: ignore
re_compile = re_compile # type: ignore
re_match = re_match # type: ignore

def test_re_load():
    p = re_compile(r'x')
    assertTrue(p.match('xyz'))
    assertEqual(p.match('xyz').span(), (0, 1))
    assertTrue(re_match(r'x', 'xyz'))
    assertEqual(re_match(r'x', 'xyz').span(), (0, 1))
    
def test_pattern_members():
    p = re.compile(r'(?P<num>\d+)-(?P<outer>(?P<word>\w+))', re.IGNORECASE | re.ASCII)

    assertEqual(p.flags, re.IGNORECASE | re.ASCII)
    assertEqual(p.groups, 3)
    assertEqual(p.groupindex, {'num': 1, 'outer': 2, 'word': 3})
    assertEqual(p.pattern, r'(?P<num>\d+)-(?P<outer>(?P<word>\w+))')

def test_match_nogroups():
    p = re.compile(r'\d+-\w+')
    m = p.search('123-abc')

    assertEqual(m.pos, 0)
    assertEqual(m.endpos, 7)
    assertEqual(m.lastindex, None)
    assertEqual(m.re, p)
    assertEqual(m.string, '123-abc')
    assertEqual(m.group(), '123-abc')
    assertEqual(m.group(0), '123-abc')
    assertEqual(m.group(0, 0), ('123-abc', '123-abc'))
    assertRaises(lambda: m.group(1))
    assertRaises(lambda: m.group('0'))
    assertRaises(lambda: m.group(0, '0'))
    assertEqual(m.groups(), ())
    assertEqual(m.groupdict(), {})
    assertEqual(m.start(), 0)
    assertEqual(m.start(0), 0)
    assertRaises(lambda: m.start(1))
    assertRaises(lambda: m.start('0'))
    assertEqual(m.end(), 7)
    assertEqual(m.end(0), 7)
    assertRaises(lambda: m.end(1))
    assertRaises(lambda: m.end('0'))
    assertEqual(m.span(), (0, 7))
    assertEqual(m.span(0), (0, 7))
    assertRaises(lambda: m.span(1))
    assertRaises(lambda: m.span('0'))

    p = re.compile(b'\\d+-\\w+')
    m = p.search(b'123-abc')

    assertEqual(m.string, b'123-abc')
    assertEqual(m.group(), b'123-abc')

def test_match_groups():
    p = re.compile(r'(?P<num>\d+)-(?P<outer>(?P<word>\w+))')
    m = p.search('-123-abc-', pos=1, endpos=8)

    assertEqual(m.pos, 1)
    assertEqual(m.endpos, 8)
    assertEqual(m.lastindex, 2)
    assertEqual(m.re, p)
    assertEqual(m.string, '-123-abc-')
    assertEqual(m.expand(r'[\2]-[\1]'), '[abc]-[123]')
    assertEqual(m.group(), '123-abc')
    assertEqual(m.group(1), '123')
    assertEqual(m.group('word'), 'abc')
    assertEqual(m.group(1, 'outer'), ('123', 'abc'))
    assertRaises(lambda: m.group(4))
    assertRaises(lambda: m.group('0'))
    assertEqual(m.groups(), ('123', 'abc', 'abc'))
    assertEqual(m.groupdict(), {'num': '123', 'outer': 'abc', 'word': 'abc'})
    assertEqual(m.start(), 1)
    assertEqual(m.start(1), 1)
    assertEqual(m.start('word'), 5)
    assertRaises(lambda: m.start(4))
    assertRaises(lambda: m.start('0'))
    assertEqual(m.end(), 8)
    assertEqual(m.end(1), 4)
    assertEqual(m.end('word'), 8)
    assertRaises(lambda: m.end(4))
    assertRaises(lambda: m.end('0'))
    assertEqual(m.span(), (1, 8))
    assertEqual(m.span(1), (1, 4))
    assertEqual(m.span('word'), (5, 8))
    assertRaises(lambda: m.span(4))
    assertRaises(lambda: m.span('0'))

    p = re.compile(b'(?P<num>\\d+)-(?P<outer>(?P<word>\\w+))')
    m = p.search(b'-123-abc-', pos=1, endpos=8)

    assertEqual(m.expand(b'[\\2]-[\\1]'), b'[abc]-[123]')
    assertEqual(m.group('word'), b'abc')
    assertRaises(lambda: m.group(b'word'))

def test_match_lastindex():
    m = re.match(r'.', 'x')
    assertEqual(m.lastindex, None)
    assertEqual(m.lastgroup, None)

    m = re.match(r'(.)', 'x')
    assertEqual(m.lastindex, 1)
    assertEqual(m.lastgroup, None)

    m = re.match(r'(?P<name>.)', 'x')
    assertEqual(m.lastindex, 1)
    assertEqual(m.lastgroup, 'name')

    m = re.match(r'(?P<name>a)?', 'x')
    assertEqual(m.lastindex, None)
    assertEqual(m.lastgroup, None)

    m = re.match(r'(x)(?P<name>a)?', 'x')
    assertEqual(m.lastindex, 1)
    assertEqual(m.lastgroup, None)

    m = re.match(r'(?P<matched>x)(?P<name>a)?', 'x')
    assertEqual(m.lastindex, 1)
    assertEqual(m.lastgroup, 'matched')

    m = re.match(r'(?P<first>(?P<second>x))?', 'x')
    assertEqual(m.lastindex, 1)
    assertEqual(m.lastgroup, 'first')

    m = re.match(r'(?P<first>(?P<second>x))?(P<third>x)?', 'x')
    assertEqual(m.lastindex, 1)
    assertEqual(m.lastgroup, 'first')

    m = re.match(r'(\bx)?(x)', 'x')
    assertEqual(m.lastindex, 2)
    assertEqual(m.lastgroup, None)

    m = re.match(b'(?P<name>.)', b'x')
    assertEqual(m.lastindex, 1)
    assertEqual(m.lastgroup, 'name')

def test_possessive_repeat_err():
    assertRaises(lambda: re.compile(r'.?+'))
    assertRaises(lambda: re.compile(r'.*+'))
    assertRaises(lambda: re.compile(r'.++'))
    assertRaises(lambda: re.compile(r'.{0,}+'))

def test_debug_flag_2():
    pat = r'(?!)(?<=\d)(?<!\d)(.+)\1[ab-c\d]{2,}(?i:x)'
    dump = '''\
FAILURE
ASSERT -1
  IN
    CATEGORY CATEGORY_DIGIT
ASSERT_NOT -1
  IN
    CATEGORY CATEGORY_DIGIT
SUBPATTERN 1 0 0
  MAX_REPEAT 1 MAXREPEAT
    ANY None
GROUPREF 1
MAX_REPEAT 2 MAXREPEAT
  IN
    LITERAL 97
    RANGE (98, 99)
    CATEGORY CATEGORY_DIGIT
SUBPATTERN None 2 0
  LITERAL 120
'''
    assertEqual(get_debug_out(pat), dump)

def test_sub_err():
    p = re.compile(r'\w+')
    assertRaises(lambda: p.sub(0, 'word-word'))
    assertRaises(lambda: p.sub(b'x', 'word-word'))
    assertRaises(lambda: p.sub(lambda: 'x', 'word-word'))
    assertRaises(lambda: p.sub(lambda m: 0, 'word-word'))
    assertRaises(lambda: p.sub(lambda m: b'x', 'word-word'))
    assertRaises(lambda: p.sub(lambda m: fail('failure'), 'word-word'))

def test_repr_ascii():
    # repr()
    assertEqual(repr(re.compile(r'\d')), r"re.compile('\\d')")
    assertEqual(repr(re.compile(b'\\d')), r"re.compile(b'\\d')")
    assertEqual(repr(re.compile(r"'")), 're.compile("\'")')
    assertEqual(repr(re.compile(b"'")), 're.compile(b"\'")')

    assertEqual(repr(re.compile('\a\b\f\n\r\t\v')),
                r"re.compile('\x07\x08\x0c\n\r\t\x0b')")
    assertEqual(repr(re.compile(b'\a\b\f\n\r\t\v')),
                r"re.compile(b'\x07\x08\x0c\n\r\t\x0b')")
    assertEqual(repr(re.compile('\x00a\x7fb\ufeffc')),
                r"re.compile('\x00a\x7fb\ufeffc')")

    b = re.escape(bytes(range(128, 170)))
    r = (r"re.compile(b'\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f" +
         r"\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2" +
         r"\xa3\xa4\xa5\xa6\xa7\xa8\xa9')")
    assertEqual(repr(re.compile(b)), r)

def test_template_escape():
    # special characters should not be escaped
    p = re.compile(r'[\w]+')

    assertEqual(p.sub(r'.', 'ab-abc'), '.-.')
    assertEqual(p.sub(r'\.', 'ab-abc'), r'\.-\.')
    assertEqual(p.sub(r'\.', 'ab-abc'), r'\.-\.')
    assertEqual(p.sub(r'\.\\', 'ab-abc'), '\\.\\-\\.\\')

def test_max_rune():
    re.compile(r'\U0010FFFF')
    assertRaises(lambda: re.compile(r'\U00110000'))

def test_curly_braces():
    assertTrue(re.match(r'{', '{'))
    assertTrue(re.match(r'{}', '{}'))
    assertTrue(re.match(r'{abc', '{abc'))
    assertTrue(re.match(r'{1', '{1'))
    assertTrue(re.match(r'{0,1', '{0,1'))
    assertTrue(re.match(r'{0,1,0}', '{0,1,0}'))

    assertTrue(re.match(r'{', '{', re.FALLBACK))
    assertTrue(re.match(r'{}', '{}', re.FALLBACK))
    assertTrue(re.match(r'{abc', '{abc', re.FALLBACK))
    assertTrue(re.match(r'{1', '{1', re.FALLBACK))
    assertTrue(re.match(r'{0,1', '{0,1', re.FALLBACK))
    assertTrue(re.match(r'{0,1,0}', '{0,1,0}', re.FALLBACK))

    # extra test for coverage
    assertRaises(lambda: re.compile(r'.{%d,%d}' % (0, 1<<128)))

def test_flag_errors():
    assertRaises(lambda: re.compile('.', re.LOCALE),
                 "cannot use LOCALE flag with a str pattern")
    assertRaises(lambda: re.compile('.', re.ASCII | re.UNICODE),
                 "ASCII and UNICODE flags are incompatible")
    assertRaises(lambda: re.compile(b'.', re.UNICODE),
                 "cannot use UNICODE flag with a bytes pattern")
    assertRaises(lambda: re.compile(b'.', re.ASCII | re.LOCALE),
                 "ASCII and LOCALE flags are incompatible")

    checkPatternError(r'(?L)', "bad inline flags: cannot use 'L' flag with a str pattern", 3)
    checkPatternError(b'(?u)', "bad inline flags: cannot use 'u' flag with a bytes pattern", 3)
    checkPatternError(r'(?au)', "bad inline flags: flags 'a', 'u' and 'L' are incompatible", 4)
    checkPatternError(b'(?aL)', "bad inline flags: flags 'a', 'u' and 'L' are incompatible", 4)
    checkPatternError(r'(?u', "missing -, : or )", 3)
    checkPatternError(r'(?uf', "unknown flag", 3)
    checkPatternError(r'(?u0', "missing -, : or )", 3)
    checkPatternError(r'.(?-', "missing flag", 4)
    checkPatternError(r'.(?-f)', "unknown flag", 4)
    checkPatternError(r'.(?-0)', "missing flag", 4)
    checkPatternError(r'.(?-u)', "bad inline flags: cannot turn off flags 'a', 'u' and 'L'", 5)
    checkPatternError(r'.(?-i', "missing :", 5)
    checkPatternError(r'.(?-if)', "unknown flag", 5)
    checkPatternError(r'.(?-i0)', "missing :", 5)
    checkPatternError(r'.(?i-i:)', "bad inline flags: flag turned on and off", 6)

def test_invalid_group():
    assertRaises(lambda: re.compile('(?P<group>x)(?P<group>y)'))
    assertRaises(lambda: re.match('.', 'x').group('unknown'))
    assertRaises(lambda: re.match('.', 'x', re.FALLBACK).group('unknown'))

def test_fallback():
    assertTrue(re.match(r'a*', 'aaa', re.FALLBACK))
    assertTrue(re.match(r'a+', 'aaa', re.FALLBACK))
    assertTrue(re.match(r'a{2,}', 'aaa', re.FALLBACK))
    assertTrue(re.match(b'a{2,}', b'aaa', re.FALLBACK))
    assertTrue(re.match(r'[\w]', '\u00F6', re.FALLBACK))
    assertTrue(re.match(r'xx', 'Xxx', re.IGNORECASE|re.FALLBACK))
    assertTrue(re.match(r'xx', 'Xxx', re.IGNORECASE|re.ASCII|re.FALLBACK))
    assertTrue(re.match(r'\u00d6', '\u00F6', re.IGNORECASE|re.FALLBACK))
    assertIsNone(re.match(r'\u00d6', '\u00F6', re.IGNORECASE|re.ASCII|re.FALLBACK))
    assertIsNone(re.match(b'[\\w]', b'\u00F6', re.FALLBACK))
    assertIsNone(re.match(b'\\xd6', b'\u00F6', re.IGNORECASE|re.FALLBACK))

    FALLBACK = 0x200
    p = re.compile('x', re.IGNORECASE|FALLBACK)
    assertEqual(repr(p), r"re.compile('x', re.IGNORECASE|re.FALLBACK)")

def test_fallback_groups():
    # check if the order of groups is correct

    def groupindex(p):
        index = {i: n for n, i in p.groupindex.items()}
        return [index[i] if i in index else i for i in range(1, p.groups + 1)]

    p = re.compile('(.)(?P<x>.)(.)', re.FALLBACK)
    g = groupindex(p)
    assertEqual(g, [1, 'x', 3])
    assertEqual(p.match('xyz').group(*g), ('x', 'y', 'z'))

    p = re.compile('(.)(?P<x>(.))(.)', re.FALLBACK)
    g = groupindex(p)
    assertEqual(g, [1, 'x', 3, 4])
    assertEqual(p.match('xyz').group(*g), ('x', 'y', 'y', 'z'))

    p = re.compile('(.)(?P<x>.)(?P<a>.)', re.FALLBACK)
    g = groupindex(p)
    assertEqual(g, [1, 'x', 'a'])
    assertEqual(p.match('xyz').group(*g), ('x', 'y', 'z'))

    p = re.compile('(.)(?P<x>(?P<b>.))(?P<a>.)', re.FALLBACK)
    g = groupindex(p)
    assertEqual(g, [1, 'x', 'b', 'a'])
    assertEqual(p.match('xyz').group(*g), ('x', 'y', 'y', 'z'))

def test_ascii_and_unicode_flag_fallback():
    # String patterns
    for flags in (0, re.UNICODE):
        pat = re.compile('\u00C0', flags | re.IGNORECASE | re.FALLBACK)
        assertTrue(pat.match('\u00E0'))
        pat = re.compile(r'\w', flags)
        assertTrue(pat.match('\u00E0'))
    pat = re.compile('\u00C0', re.ASCII | re.IGNORECASE | re.FALLBACK)
    assertIsNone(pat.match('\u00E0'))
    pat = re.compile('(?a)\u00C0', re.IGNORECASE | re.FALLBACK)
    assertIsNone(pat.match('\u00E0'))
    pat = re.compile(r'\w', re.ASCII)
    assertIsNone(pat.match('\u00E0'))
    pat = re.compile(r'(?a)\w')
    assertIsNone(pat.match('\u00E0'))
    # Bytes patterns
    for flags in (0, re.ASCII):
        pat = re.compile(b'\xc0', flags | re.IGNORECASE | re.FALLBACK)
        assertIsNone(pat.match(b'\xe0'))
        pat = re.compile(b'\\w', flags)
        assertIsNone(pat.match(b'\xe0'))

def test_ignore_case_fallback():
    assertEqual(re.match("abc", "ABC", re.I | re.FALLBACK).group(0), "ABC")
    assertEqual(re.match(b"abc", b"ABC", re.I | re.FALLBACK).group(0), b"ABC")
    assertEqual(re.match(r"(a\s[^a])", "a b", re.I | re.FALLBACK).group(1), "a b")
    assertEqual(re.match(r"(a\s[^a]*)", "a bb", re.I | re.FALLBACK).group(1), "a bb")
    assertEqual(re.match(r"(a\s[abc])", "a b", re.I | re.FALLBACK).group(1), "a b")
    assertEqual(re.match(r"(a\s[abc]*)", "a bb", re.I | re.FALLBACK).group(1), "a bb")
    assertEqual(re.match(r"((a)\s\2)", "a a", re.I | re.FALLBACK).group(1), "a a")
    assertEqual(re.match(r"((a)\s\2*)", "a aa", re.I | re.FALLBACK).group(1), "a aa")
    assertEqual(re.match(r"((a)\s(abc|a))", "a a", re.I | re.FALLBACK).group(1), "a a")
    assertEqual(re.match(r"((a)\s(abc|a)*)", "a aa", re.I | re.FALLBACK).group(1), "a aa")

    # Two different characters have the same lowercase.
    # assert 'K'.lower() == '\u212a'.lower() == 'k' # 'K'
    assertTrue(re.match(r'K', '\u212a', re.I | re.FALLBACK))
    assertTrue(re.match(r'k', '\u212a', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u212a', 'K', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u212a', 'k', re.I | re.FALLBACK))

    # Two different characters have the same uppercase.
    # assert 's'.upper() == '\u017f'.upper() == 'S' # 'ſ'
    assertTrue(re.match(r'S', '\u017f', re.I | re.FALLBACK))
    assertTrue(re.match(r's', '\u017f', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u017f', 'S', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u017f', 's', re.I | re.FALLBACK))

    # Two different characters have the same uppercase. Unicode 9.0+.
    # assert '\u0432'.upper() == '\u1c80'.upper() == '\u0412' # 'в', 'ᲀ', 'В'
    assertTrue(re.match(r'\u0412', '\u0432', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u0412', '\u1c80', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u0432', '\u0412', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u0432', '\u1c80', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u1c80', '\u0412', re.I | re.FALLBACK))
    assertTrue(re.match(r'\u1c80', '\u0432', re.I | re.FALLBACK))

    # Two different characters have the same multicharacter uppercase.
    # assert '\ufb05'.upper() == '\ufb06'.upper() == 'ST' # 'ﬅ', 'ﬆ'
    assertTrue(re.match(r'\ufb05', '\ufb06', re.I | re.FALLBACK))
    assertTrue(re.match(r'\ufb06', '\ufb05', re.I | re.FALLBACK))

def test_ignore_case_set_fallback():
    assertTrue(re.match(r'[19A]', 'A', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19a]', 'a', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19a]', 'A', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19A]', 'a', re.I | re.FALLBACK))
    assertTrue(re.match(b'[19A]', b'A', re.I | re.FALLBACK))
    assertTrue(re.match(b'[19a]', b'a', re.I | re.FALLBACK))
    assertTrue(re.match(b'[19a]', b'A', re.I | re.FALLBACK))
    assertTrue(re.match(b'[19A]', b'a', re.I | re.FALLBACK))

    # Two different characters have the same lowercase.
    # assert 'K'.lower() == '\u212a'.lower() == 'k' # 'K'
    assertTrue(re.match(r'[19K]', '\u212a', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19k]', '\u212a', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u212a]', 'K', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u212a]', 'k', re.I | re.FALLBACK))

    # Two different characters have the same uppercase.
    # assert 's'.upper() == '\u017f'.upper() == 'S' # 'ſ'
    assertTrue(re.match(r'[19S]', '\u017f', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19s]', '\u017f', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u017f]', 'S', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u017f]', 's', re.I | re.FALLBACK))

    # Two different characters have the same uppercase. Unicode 9.0+.
    # assert '\u0432'.upper() == '\u1c80'.upper() == '\u0412' # 'в', 'ᲀ', 'В'
    assertTrue(re.match(r'[19\u0412]', '\u0432', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u0412]', '\u1c80', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u0432]', '\u0412', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u0432]', '\u1c80', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u1c80]', '\u0412', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\u1c80]', '\u0432', re.I | re.FALLBACK))

    # Two different characters have the same multicharacter uppercase.
    # assert '\ufb05'.upper() == '\ufb06'.upper() == 'ST' # 'ﬅ', 'ﬆ'
    assertTrue(re.match(r'[19\ufb05]', '\ufb06', re.I | re.FALLBACK))
    assertTrue(re.match(r'[19\ufb06]', '\ufb05', re.I | re.FALLBACK))

def test_ignore_case_range_fallback():
    # Issues #3511, #17381.
    assertTrue(re.match(r'[9-a]', '_', re.I | re.FALLBACK))
    assertIsNone(re.match(r'[9-A]', '_', re.I | re.FALLBACK))
    assertTrue(re.match(b'[9-a]', b'_', re.I | re.FALLBACK))
    assertIsNone(re.match(b'[9-A]', b'_', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\xc0-\xde]', '\u00D7', re.I | re.FALLBACK))
    assertIsNone(re.match(r'[\xc0-\xde]', '\u00F7', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\xe0-\xfe]', '\u00F7', re.I | re.FALLBACK))
    assertIsNone(re.match(r'[\xe0-\xfe]', '\u00D7', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u0430-\u045f]', '\u0450', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u0430-\u045f]', '\u0400', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u0400-\u042f]', '\u0450', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u0400-\u042f]', '\u0400', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\U00010428-\U0001044f]', '\U00010428', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\U00010428-\U0001044f]', '\U00010400', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\U00010400-\U00010427]', '\U00010428', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\U00010400-\U00010427]', '\U00010400', re.I | re.FALLBACK))

    # Two different characters have the same lowercase.
    # assert 'K'.lower() == '\u212a'.lower() == 'k' # 'K'
    assertTrue(re.match(r'[J-M]', '\u212a', re.I | re.FALLBACK))
    assertTrue(re.match(r'[j-m]', '\u212a', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u2129-\u212b]', 'K', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u2129-\u212b]', 'k', re.I | re.FALLBACK))

    # Two different characters have the same uppercase.
    # assert 's'.upper() == '\u017f'.upper() == 'S' # 'ſ'
    assertTrue(re.match(r'[R-T]', '\u017f', re.I | re.FALLBACK))
    assertTrue(re.match(r'[r-t]', '\u017f', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u017e-\u0180]', 'S', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u017e-\u0180]', 's', re.I | re.FALLBACK))

    # Two different characters have the same uppercase. Unicode 9.0+.
    # assert '\u0432'.upper() == '\u1c80'.upper() == '\u0412' # 'в', 'ᲀ', 'В'
    assertTrue(re.match(r'[\u0411-\u0413]', '\u0432', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u0411-\u0413]', '\u1c80', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u0431-\u0433]', '\u0412', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u0431-\u0433]', '\u1c80', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u1c80-\u1c82]', '\u0412', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\u1c80-\u1c82]', '\u0432', re.I | re.FALLBACK))

    # Two different characters have the same multicharacter uppercase.
    # assert '\ufb05'.upper() == '\ufb06'.upper() == 'ST' # 'ﬅ', 'ﬆ'
    assertTrue(re.match(r'[\ufb04-\ufb05]', '\ufb06', re.I | re.FALLBACK))
    assertTrue(re.match(r'[\ufb06-\ufb07]', '\ufb05', re.I | re.FALLBACK))

def test_unicode_categories():
    for flag in (0, re.FALLBACK):
        assertTrue(re.match(r'[\w]', 'x', flag))
        assertTrue(re.match(r'[\w]', '\u00E4', flag))
        assertIsNone(re.match(r'[\w]', '!', flag))
        assertTrue(re.match(r'[\w!]', 'x', flag))
        assertTrue(re.match(r'[\w!]', '!', flag))
        assertIsNone(re.match(r'[\w!]', '?', flag))
        assertTrue(re.match(r'[\w!]', '\u00E4', flag))

        assertTrue(re.match(r'[\w]', 'x', flag | re.ASCII))
        assertIsNone(re.match(r'[\w]', '!', flag | re.ASCII))
        assertIsNone(re.match(r'[\w]', '\u00E4', flag | re.ASCII))
        assertTrue(re.match(r'[\w!]', 'x', flag | re.ASCII))
        assertIsNone(re.match(r'[\w!]', '\u00E4', flag | re.ASCII))

        assertIsNone(re.match(r'[\W]', 'x', flag))
        assertIsNone(re.match(r'[\W]', '\u00E4', flag))
        assertTrue(re.match(r'[\W]', '!', flag))
        assertIsNone(re.match(r'[\W!]', 'x', flag))
        assertTrue(re.match(r'[\W!]', '!', flag))
        assertTrue(re.match(r'[\W!]', '?', flag))
        assertIsNone(re.match(r'[\W!]', '\u00E4', flag))

        assertIsNone(re.match(r'[\W]', 'x', flag | re.ASCII))
        assertTrue(re.match(r'[\W]', '!', flag | re.ASCII))
        assertTrue(re.match(r'[\W]', '\u00E4', flag | re.ASCII))
        assertIsNone(re.match(r'[\W!]', 'x', flag | re.ASCII))
        assertTrue(re.match(r'[\W!]', '\u00E4', flag | re.ASCII))

        assertTrue(re.match(r'[\d]', '0', flag))
        assertIsNone(re.match(r'[\d]', 'x', flag))
        assertTrue(re.match(r'[\D]', 'x', flag))
        assertIsNone(re.match(r'[\D]', '0', flag))

        assertTrue(re.match(r'[\s]', ' ', flag))
        assertIsNone(re.match(r'[\s]', 'x', flag))
        assertTrue(re.match(r'[\S]', 'x', flag))
        assertIsNone(re.match(r'[\S]', ' ', flag))

def ascii_range_flags():
    return [flag | re.ASCII | re.IGNORECASE for flag in (0, re.FALLBACK)]

def test_ascii_range_literals():
    for flag in ascii_range_flags():
        # ascii literals
        assertTrue(re.match(r'xy', 'XY', flag))
        assertTrue(re.match(r'xY', 'Xy', flag))
        assertTrue(re.match(r'xy\u00E4', 'XY\u00E4', flag))
        assertIsNone(re.match(r'xy\u00E4', 'XY\u00C4', flag))

        # ranges as ascii literals
        assertTrue(re.match(r'[x-x][y-y]', 'XY', flag))
        assertTrue(re.match(r'[x-x][Y-Y]', 'Xy', flag))
        assertTrue(re.match(r'[x-x][y-y][\u00E4-\u00E4]', 'XY\u00E4', flag))
        assertIsNone(re.match(r'[x-x][y-y][\u00E4-\u00E4]', 'XY\u00C4', flag))

def test_ascii_full_range():
    for flag in ascii_range_flags():

        # full range
        for s in ['+', '0', '9', '=', 'A', 'Z', '^', 'a', 'z', '|']:
            assertTrue(re.match(r'[!-}]', s, flag))

def test_ascii_same_subrange():
    for flag in ascii_range_flags():
        # same subrange index
        # 1
        for s in ['+', '0', '9', '=', '@']:
            assertTrue(re.match(r'[!-@]', s, flag))
        assertIsNone(re.match(r'[!-@]', 'A', flag))
        assertIsNone(re.match(r'[!-@]', 'a', flag))
        # 2
        for s in ['A', 'F', 'Z', 'a', 'f', 'z']:
            assertTrue(re.match(r'[A-Z]', s, flag))
        assertIsNone(re.match(r'[A-Z]', '0', flag))
        # 3
        for s in ['[', ']', '_', '`']:
            assertTrue(re.match(r'[\x5b-\x60]', s, flag))
        assertIsNone(re.match(r'[\x5b-\x60]', 'A', flag))
        assertIsNone(re.match(r'[\x5b-\x60]', 'a', flag))
        # 4
        for s in ['A', 'F', 'Z', 'a', 'f', 'z']:
            assertTrue(re.match(r'[a-z]', s, flag))
        assertIsNone(re.match(r'[a-z]', '0', flag))
        # 5
        for s in ['{', '|', '}']:
            assertTrue(re.match(r'[\x7b-\xff]', s, flag))
        assertIsNone(re.match(r'[\x7b-\xff]', 'A', flag))
        assertIsNone(re.match(r'[\x7b-\xff]', 'a', flag))
        assertIsNone(re.match(r'[\x7b-\xff]', '0', flag))

def test_ascii_overlapping():
    for flag in ascii_range_flags():
        # subrange 1-2
        for s in ['0', '9', 'A', 'F', 'a', 'f']:
            assertTrue(re.match(r'[0-F]', s, flag))
        for s in ['G', 'Z', 'g', 'z']:
            assertIsNone(re.match(r'[0-F]', s, flag))

        # subrange 1-3
        for s in ['0', '9', 'A', 'F', 'Z', 'a', 'f', 'z', '^', '_']:
            assertTrue(re.match(r'[0-_]', s, flag))
        assertIsNone(re.match(r'[0-_]', '|', flag))

        # subrange 2-3
        for s in ['F', 'Z', 'f', 'z', '^', '_']:
            assertTrue(re.match(r'[F-_]', s, flag))
        for s in ['/', 'A', 'E', 'a', 'e', '`', '|']:
            assertIsNone(re.match(r'[F-_]', s, flag))

        # subrange 3-4
        for s in ['_', 'a', 'z', 'A', 'Z', '|']:
            assertTrue(re.match(r'[_-|]', s, flag))
        for s in ['0', '~']:
            assertIsNone(re.match(r'[_-|]', s, flag))

        # subrange 4-5
        for s in ['m', 'z', 'M', 'Z', '|']:
            assertTrue(re.match(r'[m-|]', s, flag))
        for s in ['0', 'a', 'l', 'A', 'L', '~']:
            assertIsNone(re.match(r'[m-|]', s, flag))

def test_ascii_subrange_letters():
    # subranges 2 and 4 partially in the range
    for flag in ascii_range_flags():
        # overlapping
        for s in [chr(c) for c in range(ord('A'), ord('z') + 1)]:
            assertTrue(re.match(r'[A-a]', s, flag))
            assertTrue(re.match(r'[B-f]', s, flag))
            assertTrue(re.match(r'[F-h]', s, flag))
            assertTrue(re.match(r'[N-m]', s, flag))
            assertTrue(re.match(r'[X-z]', s, flag))

        # non-overlapping
        for s in ['H', 'I', 'Z', 'A', 'C', 'h', 'i', 'z', 'a', 'c', '_']:
            assertTrue(re.match(r'[H-c]', s, flag))
        for s in ['D', 'E', 'G', 'd', 'e', 'g']:
            assertIsNone(re.match(r'[H-c]', s, flag))

def test_regex_node_equals():
    # test the equals function of regex nodes by compiling regex patterns
    # of type a|b|c|... and then letting the parser simplify the parsed expressions.

    assertIsNone(re.match(r'(?!)x|(?!)y|(?!)z', 'x'))
    assertTrue(re.match(r'.x|.y|.z', '!y'))
    assertTrue(re.search(r'(?<=a)x|(?<=a)y|(?<=a)z', 'ay'))
    assertTrue(re.match(r'^x|^y|^z', 'y'))
    assertTrue(re.match(r'(?:a|.)x|(?:a|.)y|(?:a|.)z', 'ay'))
    assertTrue(re.match(r'(a)(\1x|\1y|\1z)', 'aay'))
    assertTrue(re.match(r'(a)((?(1)b|c)x|(?(1)b|c)y|(?(1)b|c)z)', 'aby'))
    assertTrue(re.match(r'\dx|\dy|\dz', '0y'))
    assertTrue(re.match(r'ax|ay|az', 'ay'))
    assertTrue(re.match(r'a+x|a+y|a+z', 'aay'))
    assertTrue(re.match(r'(a)x|(a)y|(a)z', 'ay'))
    assertTrue(re.match(r'(?>a)x|(?>a)y|(?>a)z', 'ay'))

def test_ascii_cases():
    for flag in (0, re.FALLBACK):
        ranges = ['[a-z]', '[A-Z]']
        literals = {
            'K': ['K', 'k', '\u212A'],
            'I': ['I', 'i', '\u0130', '\u0131'],
            'S': ['S', 's', '\u017F'],
        }

        for lit in literals.values():
            for pattern in ranges + lit:
                p = re.compile(pattern, re.IGNORECASE | flag)
                for s in lit:
                    assertTrue(p.match(s))

        # Special case
        for pattern in [r'[\u0130-\u0131]', r'[\u0131-\u0132]']:
            p = re.compile(pattern, re.IGNORECASE | flag)
            for s in literals['I']:
                assertTrue(p.match(s))

def test_special_pos():
    for flag in (0, re.FALLBACK):
        p = re.compile(r'\w+', flag)

        assertTrue(p.search('abcd', pos=2))
        assertIsNone(p.search('abcd', pos=2, endpos=2))
        assertIsNone(p.search('abcd', pos=2, endpos=1))
        assertIsNone(p.search('abcd', pos=3, endpos=1))
        assertTrue(p.search('abcd', pos=-1))
        assertIsNone(p.search('abcd', pos=2, endpos=-1))

        # the character gets removed, because pos and endpos are inside of its utf8 representation
        assertTrue(p.search('-\u30C4-', pos=1))
        assertIsNone(p.search('-\u30C4-', pos=2, endpos=4))
        assertIsNone(p.search('-\u30C4-', pos=2, endpos=3))
        assertIsNone(p.search('-\u30C4-', pos=3, endpos=4))
        assertIsNone(p.search('-\u30C4\u30C4-', pos=2, endpos=6))
        assertIsNone(p.search('-\u30C4\u30C4-', pos=3, endpos=5))
        assertIsNone(p.search('-\u30C4\u30C4-', pos=4, endpos=5))

def test_span_ascii():
    for flag in (0, re.FALLBACK):
        p = re.compile(r'\w+', flag)
        assertEqual(p.search('--x-').span(), (2, 3))
        assertEqual(p.search('--x-', pos=2).span(), (2, 3))
        assertEqual(p.search('--xx-', pos=2).span(), (2, 4))
        assertEqual(p.search('--xx\x79-', pos=3).span(), (3, 5))

        p = re.compile(b'\\w+', flag)
        assertEqual(p.search(b'--x-').span(), (2, 3))
        assertEqual(p.search(b'--x-', pos=2).span(), (2, 3))
        assertEqual(p.search(b'--xx-', pos=2).span(), (2, 4))
        assertEqual(p.search(b'--xx\x79-', pos=3).span(), (3, 5))
        assertEqual(p.search(b'--xx\x79-', pos=3, endpos=4).span(), (3, 4))
        assertEqual(p.search(b'--xx\x79-', pos=3, endpos=5).span(), (3, 5))

def test_span_bytes_nonascii():
    for flag in (0, re.FALLBACK):
        p = re.compile(b'\\w+', flag)

        assertEqual(p.search(b'--\x41\xc3--').span(), (2, 3))
        assertEqual(p.search(b'--\x41\xc3--', pos=2).span(), (2, 3))
        assertIsNone(p.search(b'--\x41\xc3--', pos=3))

        assertEqual(p.search(b'--\x41\xc3\x41--', pos=2).span(), (2, 3))
        assertEqual(p.search(b'--\x41\xc3\x41--', pos=3).span(), (4, 5))
        assertEqual(p.search(b'--\x41\xc3\x41--', pos=4).span(), (4, 5))
        assertIsNone(p.search(b'--\x41\xc3\x41--', pos=3, endpos=4))
        assertIsNone(p.search(b'--\x41\xc3\x41--', pos=5))

        assertEqual(p.search(b'\x41\xc3\xc3\x41\x41', pos=2).span(), (3, 5))
        assertEqual(p.search(b'\x41\xc3\xc3\x41\x41', pos=2, endpos=4).span(), (3, 4))

def test_span_unicode():
    for flag in (0, re.FALLBACK):
        p = re.compile(r'\w+', flag)

        # unicode
        assertEqual(p.search('--\u00DF--').span(), (2, 4))
        assertEqual(p.search('--\u00DF--', pos=2).span(), (2, 4))
        assertEqual(p.search('--\u00DF--', pos=2, endpos=4).span(), (2, 4))
        assertEqual(p.search('--\u30C4--').span(), (2, 5))
        assertIsNone(p.search('--\u00DF--', pos=4))

        # multiple unicode characters
        assertEqual(p.search('--\u00DF\u00DF--', pos=2).span(), (2, 6))
        assertEqual(p.search('--\u00DF\u00DF--', pos=2, endpos=6).span(), (2, 6))
        assertEqual(p.search('--\u00DF\u00DF--', pos=4, endpos=6).span(), (4, 6))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=4).span(), (4, 9))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=4, endpos=7).span(), (4, 7))

        # not starting at full characters
        assertIsNone(p.search('--\u00DF--', pos=3))
        assertIsNone(p.search('--\u30C4--', pos=4))
        assertEqual(p.search('--\u00DF\u30C4--', pos=3).span(), (4, 7))
        assertEqual(p.search('--\u00DF\u30C4--', pos=4).span(), (4, 7))
        assertIsNone(p.search('--\u00DF\u30C4--', pos=5))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=3).span(), (4, 9))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=4).span(), (4, 9))

        # not ending at full characters
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=3, endpos=9).span(), (4, 9))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=5, endpos=9).span(), (7, 9))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=6, endpos=9).span(), (7, 9))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=3, endpos=8).span(), (4, 7))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=4, endpos=8).span(), (4, 7))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=4, endpos=7).span(), (4, 7))
        assertEqual(p.search('--\u00DF\u30C4\u00DF--', pos=4, endpos=7).span(), (4, 7))
        assertIsNone(p.search('--\u00DF\u30C4\u00DF--', pos=4, endpos=6))
        assertIsNone(p.search('--\u00DF\u30C4\u00DF--', pos=5, endpos=6))
        assertIsNone(p.search('--\u00DF\u30C4\u00DF--', pos=6, endpos=7))

def test_span_unicode_invalid():
    inv = '\u00f9'[1] # invalid str

    # the only way to create a invalid utf8 string is with slicing
    for flag in (0, re.FALLBACK):
        p = re.compile(r'\w+', flag)

        s = '-\u00DF' + inv + '\u00DF-'
        assertEqual(p.search(s).span(), (1, 6))
        assertEqual(p.search(s, pos=1).span(), (1, 6))
        assertEqual(p.search(s, pos=2).span(), (3, 6))
        assertEqual(p.search(s, pos=2, endpos=5).span(), (3, 4))
        assertEqual(p.search(s, pos=4, endpos=6).span(), (4, 6))
        assertIsNone(p.search(s, pos=4, endpos=5))
        assertEqual([m.span() for m in p.finditer(s)], [(1, 6)])

        s = '-\u00DF-' + inv + '-\u00DF-'
        assertEqual(p.search(s).span(), (1, 3))
        assertEqual(p.search(s, pos=2).span(), (4, 5))
        assertEqual(p.search(s, pos=4, endpos=6).span(), (4, 5))
        assertEqual(p.search(s, pos=5, endpos=8).span(), (6, 8))
        assertIsNone(p.search(s, pos=5, endpos=6))
        assertEqual([m.span() for m in p.finditer(s)], [(1, 3), (4, 5), (6, 8)])

def test_bits_optimized():
    inv = 'x' + '\u00f9'[1] + '\u00DF' # invalid str

    # the only way to create a invalid utf8 string is with slicing
    for flag in (0, re.FALLBACK):
        p = re.compile(r'\w+', flag)

        s = "-".join([inv for _ in range(16)]) # no optimization yet

        assertEqual(p.search(s).span(), (0, 4))
        assertEqual(p.search(s, pos=2).span(), (2, 4))
        assertEqual(p.search(s, pos=3).span(), (5, 9))
        assertEqual(p.search(s, pos=31).span(), (31, 34))
        assertIsNone(p.search(s, pos=79))

        s = "-".join([inv for _ in range(64)]) # with optimization

        assertEqual(p.search(s).span(), (0, 4))
        assertEqual(p.search(s, pos=2).span(), (2, 4))
        assertEqual(p.search(s, pos=3).span(), (5, 9))
        assertEqual(p.search(s, pos=31).span(), (31, 34))
        assertIsNone(p.search(s, pos=319))

def test_no_fallback():
    assertRaises(lambda: re.FALLBACK)
    assertRaises(lambda: re.compile(r'(x)(?!y)'))
    assertRaises(lambda: re.compile(r'(x)\1'))
    assertRaises(lambda: re.compile(r'(x){1024}'))

    FALLBACK = 0x200
    p = re.compile('x', re.IGNORECASE|FALLBACK)
    assertEqual(repr(p), r"re.compile('x', re.IGNORECASE|0x200)")

def test_cache():
    s = r'abc'

    re.purge()
    p = re.compile(s)
    assertIs(p, re.compile(s))
    assertIs(p, re.compile(p))

    patI = re.compile(s, re.IGNORECASE)
    assertIsNot(p, patI)
    assertIs(patI, re.compile(s, re.IGNORECASE))

    patb = re.compile(bytes(s))
    assertIsNot(p, patb)
    assertIs(patb, re.compile(bytes(s)))

    assertIs(p, re.compile(s))
    re.purge()
    assertIsNot(p, re.compile(s))

    # no cache with DEBUG flag
    re.purge()

    def compile_debug(p): # discard output
        return capture_output(lambda: re.compile(p, re.DEBUG))[0]

    p = compile_debug(s)
    assertIsNot(p, re.compile(s))
    assertIsNot(p, compile_debug(s))

def test_max_cache_size():
    s = r'abc'

    re.purge()
    p = re.compile(s)
    assertIs(p, re.compile(s))

    # add many patterns to overfill the cache
    for i in range(250):
        re.compile(str(i))

    assertIsNot(p, re.compile(s))

def test_no_cache():
    s = r'abc'

    p = re.compile(s)
    assertIsNot(p, re.compile(s))
    re.purge() # should have no effect
    assertIsNot(p, re.compile(s))


# Run all tests:

if WITH_FALLBACK:
    test_search_star_plus()
    test_branching()
    test_basic_re_sub()
    test_bug_449964()
    test_bug_449000()
    test_bug_1661()
    test_bug_3629()
    test_sub_template_numeric_escape()
    test_qualified_re_sub()
    test_misuse_flags()
    test_bug_114660()
    test_symbolic_groups()
    test_symbolic_groups_errors()
    test_symbolic_refs()
    test_symbolic_refs_errors()
    test_re_subn()
    test_re_split()
    test_qualified_re_split()
    test_re_findall()
    test_bug_117612()
    test_re_match()
    test_group()
    test_match_getitem()
    test_re_fullmatch()
    test_re_groupref_exists()
    test_re_groupref_exists_errors()
    test_re_groupref_exists_validation_bug()
    test_re_groupref_overflow()
    test_re_groupref()
    test_groupdict()
    test_expand()
    test_repeat_minmax()
    test_getattr()
    test_special_escapes()
    test_other_escapes()
    test_named_unicode_escapes()
    test_string_boundaries()
    test_bigcharset()
    test_big_codesize()
    test_anyall()
    test_lookahead()
    test_lookbehind()
    test_ignore_case()
    test_ignore_case_set()
    test_ignore_case_range()
    test_category()
    test_not_literal()
    test_possible_set_operations()
    test_search_coverage()
    test_re_escape()
    test_re_escape_bytes()
    test_re_escape_non_ascii()
    test_re_escape_non_ascii_bytes()
    test_constants()
    test_flags()
    test_sre_character_literals()
    test_sre_character_class_literals()
    test_sre_byte_literals()
    test_sre_byte_class_literals()
    test_character_set_errors()
    test_bug_113254()
    test_bug_527371()
    test_bug_418626()
    test_bug_612074()
    test_stack_overflow()
    test_nothing_to_repeat()
    test_multiple_repeat()
    test_unlimited_zero_width_repeat()
    test_bug_448951()
    test_bug_725106()
    test_bug_725149()
    test_finditer()
    test_bug_926075()
    test_bug_931848()
    test_bug_581080()
    test_bug_817234()
    test_bug_6561()
    test_inline_flags()
    test_dollar_matches_twice()
    test_bytes_str_mixing()
    test_ascii_and_unicode_flag()
    test_scoped_flags()
    test_ignore_spaces()
    test_comments()
    test_bug_6509()
    test_search_dot_unicode()
    test_compile()
    test_bug_16688()
    test_repeat_minmax_overflow()
    test_backref_group_name_in_exception()
    test_group_name_in_exception()
    test_issue17998()
    test_match_repr()
    test_zerowidth()
    test_bug_2537()
    test_keyword_parameters()
    test_bug_20998()
    test_error()
    test_misc_errors()
    test_pattern_compare()
    test_pattern_compare_bytes()
    test_bug_34294()
    test_MARK_PUSH_macro_bug()
    test_MIN_UNTIL_mark_bug()
    test_REPEAT_ONE_mark_bug()
    test_MIN_REPEAT_ONE_mark_bug()
    test_ASSERT_NOT_mark_bug()
    test_bug_40736()
    test_search_anchor_at_beginning()
    test_possessive_quantifiers()
    test_fullmatch_possessive_quantifiers()
    test_findall_possessive_quantifiers()
    test_atomic_grouping()
    test_fullmatch_atomic_grouping()
    test_findall_atomic_grouping()
    test_bug_gh91616()
    test_bug_gh100061()
    test_debug_flag()
    test_atomic_group()
    test_fail()
    test_character_set_any()
    test_character_set_none()
    test_without_flags()
    test_single_flag()
    test_multiple_flags()
    test_unicode_flag()
    test_inline_flags_2()
    test_unknown_flags()
    test_bytes()
    test_locale()
    test_quotes()
    test_long_pattern()
    test_flags_repr()
    test_immutable()
    test_repeat_minmax_overflow_maxrepeat()
    test_re_benchmarks()
    test_re_tests()

    test_interface()
    test_compare()
    test_invalid_args()
    test_invalid_args_pattern()
    test_invalid_args_match()
    test_re_load()
    test_pattern_members()
    test_match_nogroups()
    test_match_groups()
    test_match_lastindex()
    test_possessive_repeat_err()
    test_debug_flag_2()
    test_sub_err()
    test_repr_ascii()
    test_template_escape()
    test_max_rune()
    test_curly_braces()
    test_flag_errors()
    test_invalid_group()
    test_fallback()
    test_fallback_groups()
    test_ascii_and_unicode_flag_fallback()
    test_ignore_case_fallback()
    test_ignore_case_set_fallback()
    test_ignore_case_range_fallback()
    test_unicode_categories()
    test_ascii_range_literals()
    test_ascii_full_range()
    test_ascii_same_subrange()
    test_ascii_overlapping()
    test_ascii_subrange_letters()
    test_regex_node_equals()
    test_ascii_cases()
    test_special_pos()
    test_span_ascii()
    test_span_bytes_nonascii()
    test_span_unicode()
    test_span_unicode_invalid()
    test_bits_optimized()
else:
    test_no_fallback()

if WITH_CACHE:
    test_cache()
    test_max_cache_size()
else:
    test_no_cache()

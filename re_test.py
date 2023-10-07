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

    # ('(Python)\\1', 'PythonPython'),    # Backreference
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
    # ('(?P<foo_123>a)(?P=foo_123)', 'aa', SUCCEED, 'g1', 'a'),

    # Test octal escapes
    ('\\1', 'a', SYNTAX_ERROR),    # Backreference
    ('[\\1]', '\1', SUCCEED, 'found', '\1'),  # Character
    ('\\09', chr(0) + '9', SUCCEED, 'found', chr(0) + '9'),
    ('\\141', 'a', SUCCEED, 'found', 'a'),
    # ('(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)(k)(l)\\119', 'abcdefghijklk9', SUCCEED, 'found+"-"+g11', 'abcdefghijklk9-k'),

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
    # (r'\x00ffffffffffffff', '\u00FF', SUCCEED, 'found', chr(255)),
    # (r'\x00f', '\017', SUCCEED, 'found', chr(15)),
    # (r'\x00fe', '\u00FE', SUCCEED, 'found', chr(254)),

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
    # ('(abc)\\1', 'abcabc', SUCCEED, 'g1', 'abc'),
    # ('([a-c]*)\\1', 'abcabc', SUCCEED, 'g1', 'abc'),
    ('^(.+)?B', 'AB', SUCCEED, 'g1', 'A'),
    # ('(a+).\\1$', 'aaaaa', SUCCEED, 'found+"-"+g1', 'aaaaa-aa'),
    # ('^(a+).\\1$', 'aaaa', FAIL),
    # ('(abc)\\1', 'abcabc', SUCCEED, 'found+"-"+g1', 'abcabc-abc'),
    # ('([a-c]+)\\1', 'abcabc', SUCCEED, 'found+"-"+g1', 'abcabc-abc'),
    # ('(a)\\1', 'aa', SUCCEED, 'found+"-"+g1', 'aa-a'),
    # ('(a+)\\1', 'aa', SUCCEED, 'found+"-"+g1', 'aa-a'),
    # ('(a+)+\\1', 'aa', SUCCEED, 'found+"-"+g1', 'aa-a'),
    # ('(a).+\\1', 'aba', SUCCEED, 'found+"-"+g1', 'aba-a'),
    # ('(a)ba*\\1', 'aba', SUCCEED, 'found+"-"+g1', 'aba-a'),
    # ('(aa|a)a\\1$', 'aaa', SUCCEED, 'found+"-"+g1', 'aaa-a'),
    # ('(a|aa)a\\1$', 'aaa', SUCCEED, 'found+"-"+g1', 'aaa-a'),
    # ('(a+)a\\1$', 'aaa', SUCCEED, 'found+"-"+g1', 'aaa-a'),
    # ('([abc]*)\\1', 'abcabc', SUCCEED, 'found+"-"+g1', 'abcabc-abc'),
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
    # ('(?P<id>aa)(?P=id)', 'aaaa', SUCCEED, 'found+"-"+id', 'aaaa-aa'),
    # ('(?P<id>aa)(?P=xd)', 'aaaa', SYNTAX_ERROR),

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
    # ('((((((((((a))))))))))\\10', 'aa', SUCCEED, 'found', 'aa'),
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
    #('(?i)((((((((((a))))))))))\\10', 'AA', SUCCEED, 'found', 'AA'),
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
    #('(?i)(abc)\\1', 'ABCABC', SUCCEED, 'g1', 'ABC'),
    #('(?i)([a-c]*)\\1', 'ABCABC', SUCCEED, 'g1', 'ABC'),
    #('a(?!b).', 'abad', SUCCEED, 'found', 'ad'),
    #('a(?=d).', 'abad', SUCCEED, 'found', 'ad'),
    #('a(?=c|d).', 'abad', SUCCEED, 'found', 'ad'),
    ('a(?:b|c|d)(.)', 'ace', SUCCEED, 'g1', 'e'),
    ('a(?:b|c|d)*(.)', 'ace', SUCCEED, 'g1', 'e'),
    ('a(?:b|c|d)+?(.)', 'ace', SUCCEED, 'g1', 'e'),
    ('a(?:b|(c|e){1,2}?|d)+?(.)', 'ace', SUCCEED, 'g1 + g2', 'ce'),

    # lookbehind: split by : but not if it is escaped by -.
    #('(?<!-):(.*?)(?<!-):', 'a:bc-:de:f', SUCCEED, 'g1', 'bc-:de' ),
    # escaping with \ as we know it
    #('(?<!\\\\):(.*?)(?<!\\\\):', 'a:bc\\:de:f', SUCCEED, 'g1', 'bc\\:de' ),
    # terminating with ' and escaping with ? as in edifact
    #("(?<!\\?)'(.*?)(?<!\\?)'", "a'bc?'de'f", SUCCEED, 'g1', "bc?'de" ),

    # Comments using the (?#...) syntax

    #('w(?# comment', 'w', SYNTAX_ERROR),
    #('w(?# comment 1)xy(?# comment 2)z', 'wxyz', SUCCEED, 'found', 'wxyz'),

    # Check odd placement of embedded pattern modifiers

    # not an error under PCRE/PRE:
    ('(?i)w', 'W', SUCCEED, 'found', 'W'),
    # ('w(?i)', 'W', SYNTAX_ERROR),

    # Comments using the x embedded pattern modifier

    #("""(?x)w# comment 1
    #    x y
    #    # comment 2
    #    z""", 'wxyz', SUCCEED, 'found', 'wxyz'),

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
    # (r'\x00ff', '\u00FF', SUCCEED, 'found', chr(255)),
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
    (r'(x?)?', 'x', SUCCEED, 'found', 'x'),
    # bug 115040: rescan if flags are modified inside pattern
    #(r'(?x) foo ', 'foo', SUCCEED, 'found', 'foo'),
    # bug 115618: negative lookahead
    #(r'(?<!abc)(d.f)', 'abcdefdof', SUCCEED, 'found', 'dof'),
    # bug 116251: character class bug
    (r'[\w-]+', 'laser_beam', SUCCEED, 'found', 'laser_beam'),
    # bug 123769+127259: non-greedy backtracking bug
    (r'.*?\S *:', 'xx:', SUCCEED, 'found', 'xx:'),
    (r'a[ ]*?\ (\d+).*', 'a   10', SUCCEED, 'found', 'a   10'),
    (r'a[ ]*?\ (\d+).*', 'a    10', SUCCEED, 'found', 'a    10'),
    # bug 127259: \Z shouldn't depend on multiline mode
    #(r'(?ms).*?x\s*\Z(.*)','xx\nx\n', SUCCEED, 'g1', ''),
    # bug 128899: uppercase literals under the ignorecase flag
    (r'(?i)M+', 'MMM', SUCCEED, 'found', 'MMM'),
    (r'(?i)m+', 'MMM', SUCCEED, 'found', 'MMM'),
    (r'(?i)[M]+', 'MMM', SUCCEED, 'found', 'MMM'),
    (r'(?i)[m]+', 'MMM', SUCCEED, 'found', 'MMM'),
    # bug 130748: ^* should be an error (nothing to repeat)
    #(r'^*', '', SYNTAX_ERROR),
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
    #('^([ab]*?)(?=(b)?)c', 'abc', SUCCEED, 'g1+"-"+g2', 'ab-None'),
    #('^([ab]*?)(?!(b))c', 'abc', SUCCEED, 'g1+"-"+g2', 'ab-None'),
    #('^([ab]*?)(?<!(a))c', 'abc', SUCCEED, 'g1+"-"+g2', 'ab-None'),
]

u = 'Ä'
tests.extend([
    # bug 410271: \b broken under locales
    (r'\b.\b', 'a', SUCCEED, 'found', 'a'),
    #(r'(?u)\b.\b', u, SUCCEED, 'found', u),
    #(r'(?u)\w', u, SUCCEED, 'found', u),
])


# Dummies to fix Python warnings
re = re
assertEqual = assertEqual
assertNotEqual = assertNotEqual
assertIs = assertIs
assertIsNot = assertIsNot
assertIsNone = assertIsNone
assertIsNotNone = assertIsNotNone
assertFalse = assertFalse
assertTrue = assertTrue
assertIn = assertIn
# assertNotIn = assertNotIn
assertLess = assertLess
assertIsInstance = assertIsInstance
assertRegex = assertRegex
assertRaises = assertRaises
assertRaisesRegex = assertRaisesRegex
measure = measure
trycatch = trycatch
fail = fail

# Add some helper functions to replace missing types
def bytearray(b): return b
def memoryview(b): return b
def pow(x, n):
    r = 1
    while n:
        if n & 1:
            r *= x
        x *= x
        n //= 2
    return r

# Change the former classes S und B to identify functions
def S(s): return s
def B(b): return b

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
    assertRaises(lambda: re.compile(pattern), errmsg)

def checkTemplateError(pattern, repl, string, errmsg, pos=None):
    assertRaises(lambda: re.sub(pattern, repl, string), errmsg)

def checkSyntaxError(pattern, syntax):
    checkPatternError(pattern, 'invalid or unsupported Perl syntax: `%s`' % syntax)

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
    assertTypedEqual(re.sub('y', S('a'), S('xyz')), 'xaz')
    assertTypedEqual(re.sub(b'y', b'a', b'xyz'), b'xaz')
    assertTypedEqual(re.sub(b'y', B(b'a'), B(b'xyz')), b'xaz')
    assertTypedEqual(re.sub(b'y', bytearray(b'a'), bytearray(b'xyz')), b'xaz')
    assertTypedEqual(re.sub(b'y', memoryview(b'a'), memoryview(b'xyz')), b'xaz')
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
    assertRaises(lambda: re.compile("(?P<quote>)(?(quote))"))

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
                      r"sub: got multiple values for keyword argument \"count\"")
    assertRaisesRegex(lambda: re.sub('a', 'b', 'aaaaa', 1, 0, flags=0),
                      r"sub: got multiple values for keyword argument \"flags\"")
    assertRaisesRegex(lambda: re.sub('a', 'b', 'aaaaa', 1, 0, 0),
                      r"sub: got 6 arguments, want at most 5")

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
    err = 'invalid or unsupported Perl syntax: `(?P`'
    assertRaises(lambda: re.compile(r'(?P<a>x)(?P=a)(?(a)y)'), err)
    assertRaises(lambda: re.compile(r'(?P<a1>x)(?P=a1)(?(a1)y)'), err)
    assertRaises(lambda: re.compile(r'(?P<a1>x)\1(?(1)y)'), 'invalid group reference 1')
    assertRaises(lambda: re.compile(b'(?P<a1>x)(?P=a1)(?(a1)y)'), err)
    # New valid identifiers in Python 3
    assertRaises(lambda: re.compile('(?P<µ>x)(?P=µ)(?(µ)y)'), 'invalid named capture: `(?P<µ>`')
    assertRaises(lambda: re.compile('(?P<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>x)(?P=𝔘𝔫𝔦𝔠𝔬𝔡𝔢)(?(𝔘𝔫𝔦𝔠𝔬𝔡𝔢)y)'), 'invalid named capture: `(?P<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>`')
    # Support > 100 groups.
    pat = '|'.join(['x(?P<a%d>%x)y' % (i, i) for i in range(1, 200 + 1)])
    pat = '(?:%s)(?(200)z|t)' % pat
    assertRaises(lambda: (re.match(pat, 'xc8yz').span(), (0, 5)), 'invalid or unsupported Perl syntax: `(?(`')

def test_symbolic_groups_errors():
    checkPatternError(r'(?P<a>)(?P<a>)',
                            "redefinition of group name 'a' as group 2; " +
                            "was group 1")

    err = 'invalid or unsupported Perl syntax: `(?P`'
    checkPatternError(r'(?P<a>(?P=a))', err, 10)
    checkPatternError(r'(?Pxy)', err)
    checkPatternError(r'(?P<a>)(?P=a', err, 11)
    checkPatternError(r'(?P=', err, 4)
    checkPatternError(r'(?P=)', err, 4)
    checkPatternError(r'(?P=1)', err, 4)
    checkPatternError(r'(?P=a)', err)
    checkPatternError(r'(?P=a1)', err)
    checkPatternError(r'(?P=a.)', err, 4)
    checkPatternError(r'(?P<)', 'invalid named capture: `(?P<)`', 4)
    checkPatternError(r'(?P<a', 'invalid named capture: `(?P<a`', 4)
    checkPatternError(r'(?P<', err, 4)
    checkPatternError(r'(?P<>)', 'invalid named capture: `(?P<>`', 4)
    checkPatternError(r'(?P<1>)', "bad character in group name '1'", 4)
    checkPatternError(r'(?P<a.>)', "invalid named capture: `(?P<a.>`", 4)
    err = 'invalid or unsupported Perl syntax: `(?(`'
    checkPatternError(r'(?(', err, 3)
    checkPatternError(r'(?())', err, 3)
    checkPatternError(r'(?(a))', err, 3)
    checkPatternError(r'(?(-1))', err, 3)
    checkPatternError(r'(?(1a))', err, 3)
    checkPatternError(r'(?(a.))', err, 3)
    checkPatternError('(?P<©>x)', 'invalid named capture: `(?P<©>`', 4)
    checkPatternError('(?P=©)', 'invalid or unsupported Perl syntax: `(?P`', 4)
    checkPatternError('(?(©)y)', err, 3)
    checkPatternError(b'(?P<\xc2\xb5>x)',
                      'invalid named capture: `(?P<µ>`', 4)
    checkPatternError(b'(?P=\xc2\xb5)',
                      'invalid or unsupported Perl syntax: `(?P`', 4)
    checkPatternError(b'(?(\xc2\xb5)y)', err, 3)

def test_symbolic_refs():
    assertEqual(re.sub('(?P<a>x)|(?P<b>y)', r'\g<b>', 'xx'), '')
    assertEqual(re.sub('(?P<a>x)|(?P<b>y)', r'\2', 'xx'), '')
    assertEqual(re.sub(b'(?P<a1>x)', b'\\g<a1>', b'xx'), b'xx')
    # New valid identifiers in Python 3 (but do not work in Go)
    assertRaises(lambda: re.sub('(?P<µ>x)', r'\g<µ>', 'xx'), 'invalid named capture: `(?P<µ>`')
    assertRaises(lambda: re.sub('(?P<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>x)', r'\g<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>', 'xx'), 'invalid named capture: `(?P<𝔘𝔫𝔦𝔠𝔬𝔡𝔢>`')
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
    for string in ":a:b::c", S(":a:b::c"):
        assertTypedEqual(re.split(":", string),
                                ['', 'a', 'b', '', 'c'])
        assertTypedEqual(re.split(":+", string),
                                ['', 'a', 'b', 'c'])
        assertTypedEqual(re.split("(:+)", string),
                                ['', ':', 'a', ':', 'b', '::', 'c'])
    for string in (b":a:b::c", B(b":a:b::c")):
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
        # (r'(?=:)', ['', ':a', ':b', ':', ':c']),
        # (r'(?<=:)', [':', 'a:', 'b:', ':', 'c']),
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
    for string in "a:b::c:::d", S("a:b::c:::d"):
        assertTypedEqual(re.findall(":+", string),
                                [":", "::", ":::"])
        assertTypedEqual(re.findall("(:+)", string),
                                [":", "::", ":::"])
        assertTypedEqual(re.findall("(:)(:*)", string),
                                [(":", ""), (":", ":"), (":", "::")])
    for string in (b"a:b::c:::d", B(b"a:b::c:::d"), bytearray(b"a:b::c:::d"),
                    memoryview(b"a:b::c:::d")):
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
    for string in ('a', S('a')):
        assertEqual(re.match('a', string).groups(), ())
        assertEqual(re.match('(a)', string).groups(), ('a',))
        assertEqual(re.match('(a)', string).group(0), 'a')
        assertEqual(re.match('(a)', string).group(1), 'a')
        assertEqual(re.match('(a)', string).group(1, 1), ('a', 'a'))
    for string in (b'a', B(b'a'), bytearray(b'a'), memoryview(b'a')):
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
    def Index(i):
        return i
    # A single group
    m = re.match('(a)(b)', 'ab')
    assertEqual(m.group(), 'ab')
    assertEqual(m.group(0), 'ab')
    assertEqual(m.group(1), 'a')
    assertEqual(m.group(Index(1)), 'a')
    assertRaises(lambda: m.group(-1))
    assertRaises(lambda: m.group(3))
    assertRaises(lambda: m.group(1<<1000))
    assertRaises(lambda: m.group(Index(1<<1000)))
    assertRaises(lambda: m.group('x'))
    # Multiple groups
    assertEqual(m.group(2, 1), ('b', 'a'))
    assertEqual(m.group(Index(2), Index(1)), ('b', 'a'))

def test_match_getitem():
    pat = re.compile('(?:(?P<a1>a)|(?P<b2>b))(?P<c3>c)?')

    m = pat.match('a')
    assertEqual(m['a1'], 'a')
    assertEqual(m['b2'], None)
    assertEqual(m['c3'], None)
    # assertEqual('a1={a1} b2={b2} c3={c3}'.format_map(m), 'a1=a b2=None c3=None')
    assertEqual(m[0], 'a')
    assertEqual(m[1], 'a')
    assertEqual(m[2], None)
    assertEqual(m[3], None)
    assertRaisesRegex(lambda: m['X'], 'no such group')
    assertRaisesRegex(lambda: m[-1], 'no such group')
    assertRaisesRegex(lambda: m[4], 'no such group')
    assertRaisesRegex(lambda: m[0, 1], 'no such group')
    assertRaisesRegex(lambda: m[(0,)], 'no such group')
    assertRaisesRegex(lambda: m[m[(0, 1)]], 'no such group')
    # assertRaisesRegex(lambda: 'a1={a2}'.format_map(m), 'no such group')

    m = pat.match('ac')
    assertEqual(m['a1'], 'a')
    assertEqual(m['b2'], None)
    assertEqual(m['c3'], 'c')
    # assertEqual('a1={a1} b2={b2} c3={c3}'.format_map(m), 'a1=a b2=None c3=c')
    assertEqual(m[0], 'ac')
    assertEqual(m[1], 'a')
    assertEqual(m[2], None)
    assertEqual(m[3], 'c')

    # Cannot assign.
    def cb():
        m[0] = 1
    assertRaises(cb)

    # No len().
    assertRaises(lambda: len(m))

def test_re_fullmatch():
    # Issue 16203: Proposal: add re.fullmatch() method.
    assertEqual(re.fullmatch(r"a", "a").span(), (0, 1))
    for string in ("ab", S("ab")):
        assertEqual(re.fullmatch(r"a|ab", string).span(), (0, 2))
    for string in (b"ab", B(b"ab")):
        assertEqual(re.fullmatch(b"a|ab", string).span(), (0, 2))
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
    assertRaises(lambda: re.fullmatch(r"abc\Z", "abc\n"), 'invalid escape sequence: `\\Z`')
    assertIsNone(re.fullmatch(r"(?m)abc$", "abc\n"))
    err = 'invalid or unsupported Perl syntax: `(?=`'
    assertRaises(lambda: re.fullmatch(r"ab(?=c)cd", "abcd").span(), err)
    assertRaises(lambda: re.fullmatch(r"(?=a|ab)ab", "ab").span(), err)
    err = 'invalid or unsupported Perl syntax: `(?<`'
    assertRaises(lambda: re.fullmatch(r"ab(?<=b)cd", "abcd").span(), err)

    assertEqual(
        re.compile(r"bc").fullmatch("abcd", pos=1, endpos=3).span(), (1, 3))
    assertEqual(
        re.compile(r".*?$").fullmatch("abcd", pos=1, endpos=3).span(), (1, 3))
    assertEqual(
        re.compile(r".*?").fullmatch("abcd", pos=1, endpos=3).span(), (1, 3))

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
                      'invalid repeat count: `{2,1}`', 2)

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

    def cb():
        p.groupindex['other'] = 0
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
    err = r'invalid escape sequence: `\Z`'
    assertRaises(lambda: re.search(r"^\Aabc\Z$", "abc", re.M).group(0), err)
    assertRaises(lambda: re.search(r"^\Aabc\Z$", "\nabc\n", re.M), err)
    assertEqual(re.search(b"\\b(b.)\\b",
                                b"abcd abc bcd bx").group(1), b"bx")
    assertEqual(re.search(b"\\B(b.)\\B",
                                b"abc bcd bc abxd").group(1), b"bx")
    assertEqual(re.search(b"\\b(b.)\\b",
                                b"abcd abc bcd bx", re.LOCALE).group(1), b"bx")
    assertEqual(re.search(b"\\B(b.)\\B",
                                b"abc bcd bc abxd", re.LOCALE).group(1), b"bx")
    assertEqual(re.search(b"^abc$", b"\nabc\n", re.M).group(0), b"abc")
    assertRaises(lambda: re.search(b"^\\Aabc\\Z$", b"abc", re.M).group(0), err)
    assertRaises(lambda: re.search(b"^\\Aabc\\Z$", b"\nabc\n", re.M), err)
    assertEqual(re.search(r"\d\D\w\W\s\S",
                                "1aa! a").group(0), "1aa! a")
    assertEqual(re.search(b"\\d\\D\\w\\W\\s\\S",
                                b"1aa! a").group(0), b"1aa! a")
    assertEqual(re.search(r"\d\D\w\W\s\S",
                                "1aa! a", re.ASCII).group(0), "1aa! a")
    assertEqual(re.search(b"\\d\\D\\w\\W\\s\\S",
                                b"1aa! a", re.LOCALE).group(0), b"1aa! a")

def test_other_escapes():
    checkPatternError("\\", r'trailing backslash at end of expression: ``', 0)
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
    for c in 'ceghijklmopquxyCEFGHIJKLMNOPRTUVXYZ'.elems():
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
    # SKIP: assertIsNone(re.search(r"\B", ""))
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
    r = '[%s]' % ''.join([chr(c) for c in range(256, pow(2,16), 255)])
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

def test_ignore_case():
    assertEqual(re.match("abc", "ABC", re.I).group(0), "ABC")
    assertEqual(re.match(b"abc", b"ABC", re.I).group(0), b"ABC")
    assertEqual(re.match(r"(a\s[^a])", "a b", re.I).group(1), "a b")
    assertEqual(re.match(r"(a\s[^a]*)", "a bb", re.I).group(1), "a bb")
    assertEqual(re.match(r"(a\s[abc])", "a b", re.I).group(1), "a b")
    assertEqual(re.match(r"(a\s[abc]*)", "a bb", re.I).group(1), "a bb")
    # assertEqual(re.match(r"((a)\s\2)", "a a", re.I).group(1), "a a")
    # assertEqual(re.match(r"((a)\s\2*)", "a aa", re.I).group(1), "a aa")
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
    # SKIP in Go: assertTrue(re.match(r'\ufb05', '\ufb06', re.I))
    # SKIP in Go: assertTrue(re.match(r'\ufb06', '\ufb05', re.I))

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
    # SKIP in Go: assertTrue(re.match(r'[19\ufb05]', '\ufb06', re.I))
    # SKIP in Go: assertTrue(re.match(r'[19\ufb06]', '\ufb05', re.I))

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
    # SKIP in Go: assertTrue(re.match(r'[\ufb04-\ufb05]', '\ufb06', re.I))
    # SKIP in Go: assertTrue(re.match(r'[\ufb06-\ufb07]', '\ufb05', re.I))

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
    assertEqual(re.findall(r'[\d&&1]', s), list('&0123456789'.elems()))
    assertEqual(re.findall(r'[&&1]', s), list('&1'.elems()))


    assertEqual(re.findall(r'[0-9||a]', s), list('0123456789a|'.elems()))
    assertEqual(re.findall(r'[\d||a]', s), list('0123456789a|'.elems()))
    assertEqual(re.findall(r'[||1]', s), list('1|'.elems()))

    assertEqual(re.findall(r'[0-9~~1]', s), list('0123456789~'.elems()))
    assertEqual(re.findall(r'[\d~~1]', s), list('0123456789~'.elems()))
    assertEqual(re.findall(r'[~~1]', s), list('1~'.elems()))

    assertEqual(re.findall(r'[[0-9]|]', s), list('0123456789[]'.elems()))

    # Does not work in Go: assertEqual(re.findall(r'[[:digit:]|]', s), list(':[]dgit'.elems()))

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
        # assertMatch('(?x)' + re.escape(c), c)
    assertMatch(re.escape(p), p)
    for c in '-.]{}'.elems():
        assertEqual(re.escape(c)[:1], '\\')
    literal_chars = LITERAL_CHARS
    assertEqual(re.escape(literal_chars), literal_chars)

def test_re_escape_bytes():
    p = bytes(range(256))
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

# fix for starlark; catb = concat bytes
def catb(*args):
    l = []
    for arg in args:
        l += list(arg.elems())
    return bytes(l)

def test_sre_byte_literals():
    for i in [0, 8, 16, 32, 64, 127, 128, 255]:
        assertTrue(re.match(bytes(format(r"\%03o", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"\%03o0", i)), catb(bytes([i]),b"0")))
        assertTrue(re.match(bytes(format(r"\%03o8", i)), catb(bytes([i]), b"8")))
        assertTrue(re.match(bytes(format(r"\x%02x", i)), bytes([i])))
        assertTrue(re.match(bytes(format(r"\x%02x0", i)), catb(bytes([i]), b"0")))
        assertTrue(re.match(bytes(format(r"\x%02xz", i)), catb(bytes([i]), b"z")))
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
    checkPatternError(r'[', 'missing closing ]: `[`', 0)
    checkPatternError(r'[^', 'missing closing ]: `[^`', 0)
    checkPatternError(r'[a', 'missing closing ]: `[a`', 0)
    # bug 545855 -- This pattern failed to cause a compile error as it
    # should, instead provoking a TypeError.
    checkPatternError(r"[a-", 'missing closing ]: `[a-`', 0)
    # Works in Go: checkPatternError(r"[\w-b]", r'bad character range \w-b', 1)
    # Works in Go: checkPatternError(r"[a-\w]", r'bad character range a-\w', 1)
    checkPatternError(r"[b-a]", 'invalid character class range: `b-a`', 1)

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
            err = 'missing argument to repetition operator: `%s%s`' % (reps, mod)
            checkPatternError('%s%s' % (reps, mod), err, 0)
            checkPatternError('(?:%s%s)' % (reps, mod), err, 3)

def test_multiple_repeat():
    for outer_reps in '*', '+', '?', '{1,2}':
        for outer_mod in '', '?', '+':
            outer_op = outer_reps + outer_mod
            for inner_reps in '*', '+', '?', '{1,2}':
                for inner_mod in '', '?', '+':
                    if inner_mod + outer_reps in ('?', '+'):
                        continue
                    inner_op = inner_reps + inner_mod
                    assertRaisesRegex(lambda: re.compile(r'x%s%s' % (inner_op, outer_op)),
                                 'invalid nested repetition operator')

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

def test_bug_764548():
    # bug 764548, re.compile() barfs on str/unicode subclasses
    def my_unicode(str): return str
    pat = re.compile(my_unicode("abc"))
    assertIsNone(pat.match("xyz"))

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
    # Does not work: assertIsNone(pat.match('\u00E0'))
    # Not supported: pat = re.compile('(?a)\u00C0', re.IGNORECASE)
    # Not supported: assertIsNone(pat.match('\u00E0'))
    pat = re.compile(r'\w', re.ASCII)
    assertIsNone(pat.match('\u00E0'))
    # Not supported: pat = re.compile(r'(?a)\w')
    # Not supported: assertIsNone(pat.match('\u00E0'))
    # Bytes patterns
    for flags in (0, re.ASCII):
        # Not supported: pat = re.compile(b'\u00C0', flags | re.IGNORECASE)
        # Not supported: assertIsNone(pat.match(b'\u00E0'))
        pat = re.compile(b'\\w', flags)
        assertIsNone(pat.match(b'\u00E0'))
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

    checkPatternError(r'(?-', 'invalid or unsupported Perl syntax: `(?-`', 3)
    checkPatternError(r'(?-+', 'invalid or unsupported Perl syntax: `(?-+`', 3)
    checkPatternError(r'(?-z', 'invalid or unsupported Perl syntax: `(?-z`', 3)
    checkPatternError(r'(?-i', 'invalid or unsupported Perl syntax: `(?-i`', 4)
    # Compiles without errors: checkPatternError(r'(?-i)', 'missing :', 4)
    checkPatternError(r'(?-i+', 'invalid or unsupported Perl syntax: `(?-i+`', 4)
    checkPatternError(r'(?-iz', 'invalid or unsupported Perl syntax: `(?-iz`', 4)
    checkPatternError(r'(?i:', 'missing closing ): `(?i:`', 0)
    checkPatternError(r'(?i', 'invalid or unsupported Perl syntax: `(?i`', 3)
    checkPatternError(r'(?i+', 'invalid or unsupported Perl syntax: `(?i+`', 3)
    checkPatternError(r'(?iz', 'invalid or unsupported Perl syntax: `(?iz`', 3)

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
    assertIsInstance(pattern, "pattern")
    same_pattern = re.compile(pattern)
    assertIsInstance(same_pattern, "pattern")
    assertIs(same_pattern, pattern)
    # Test behaviour when not given a string or pattern as parameter
    assertRaises(lambda: re.compile(0))

def test_bug_16688():
    # Issue 16688: Backreferences make case-insensitive regex fail on
    # non-ASCII strings.
    assertEqual(re.match(r"(?s).{1,3}", "\u0100\u0100").span(), (0, 4))

def test_repeat_minmax_overflow():
    # Issue #13169
    # Note: the maximum repeat count of Go is 1000
    string = "x" * 100000
    assertEqual(re.match(r".{1000}", string).span(), (0, 1000))
    assertEqual(re.match(r".{,1000}", string).span(), (0, 1000))
    assertEqual(re.match(r".{1000,}?", string).span(), (0, 1000))
    # 2**128 should be big enough to overflow both SRE_CODE and Py_ssize_t.
    assertRaises(lambda: re.compile(r".{%d}" % pow(2,128)))
    assertRaises(lambda: re.compile(r".{,%d}" % pow(2,128)))
    assertRaises(lambda: re.compile(r".{%d,}?" % pow(2,128)))
    assertRaises(lambda: re.compile(r".{%d,%d}" % (pow(2,129), pow(2,128))))

def test_group_name_in_exception():
    # Issue 17341: Poor error message when compiling invalid regex
    checkPatternError('(?P<?foo>)',
                      "invalid named capture: `(?P<?foo>`", 4)

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
    string = 'abracadabra'
    m = re.search(r'(.+)(.*?)', string)
    pattern = r"<re\.match object; span=\(0, 11\), match='abracadabra'>"
    assertRegex(repr(m), pattern)

    string = b'abracadabra'
    m = re.search(b'(.+)(.*?)', string)
    pattern = r"<re\.match object; span=\(0, 11\), match=b'abracadabra'>"
    assertRegex(repr(m), pattern)

    first, second = list(re.finditer("(aa)|(bb)", "aa bb"))
    pattern = r"<re\.match object; span=\(0, 2\), match='aa'>"
    assertRegex(repr(first), pattern)

    pattern = r"<re\.match object; span=\(3, 5\), match='bb'>"
    assertRegex(repr(second), pattern)

def test_zerowidth():
    # Issues 852532, 1647489, 3262, 25054.
    assertEqual(re.split(r"\b", "a::bc"), ['', 'a', '::', 'bc', ''])
    assertEqual(re.split(r"\b|:+", "a::bc"), ['', 'a', '', '', 'bc', ''])
    # assertEqual(re.split(r"(?<!\w)(?=\w)|:+", "a::bc"), ['', 'a', '', 'bc'])
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

# TODO: remove this or next
def check_en_US_iso88591():
    assertTrue(re.match(b'\xc5\xe5', b'\xc5\xe5', re.L|re.I))
    assertTrue(re.match(b'\xc5', b'\xe5', re.L|re.I))
    assertTrue(re.match(b'\xe5', b'\xc5', re.L|re.I))
    assertTrue(re.match(b'(?Li)\xc5\xe5', b'\xc5\xe5'))
    assertTrue(re.match(b'(?Li)\xc5', b'\xe5'))
    assertTrue(re.match(b'(?Li)\xe5', b'\xc5'))

def check_en_US_utf8():
    assertTrue(re.match(b'\xc5\xe5', b'\xc5\xe5', re.L|re.I))
    assertIsNone(re.match(b'\xc5', b'\xe5', re.L|re.I))
    assertIsNone(re.match(b'\xe5', b'\xc5', re.L|re.I))
    assertTrue(re.match(b'(?Li)\xc5\xe5', b'\xc5\xe5'))
    assertIsNone(re.match(b'(?Li)\xc5', b'\xe5'))
    assertIsNone(re.match(b'(?Li)\xe5', b'\xc5'))

def test_error():
    assertRaisesRegex(lambda: re.compile('(\u20ac))'), r'unexpected \):')
    # Bytes pattern
    assertRaises(lambda: re.compile(b'(\xa4))'))

def test_misc_errors():
    checkPatternError(r'(', 'missing closing ): `(`', 0)
    checkPatternError(r'((a|b)', 'missing closing ): `((a|b)`', 0)
    checkPatternError(r'(a|b))', 'unexpected ): `(a|b))`', 5)
    checkSyntaxError(r'(?z)', '(?z')
    checkSyntaxError(r'(?iz)', '(?iz')
    checkSyntaxError(r'(?i', '(?i')
    checkSyntaxError(r'(?#abc', '(?#')
    checkSyntaxError(r'(?<', '(?<')
    checkSyntaxError(r'(?<>)', '(?<')
    checkSyntaxError(r'(?', '(?')

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
    # Removed unsupported tests
    assertEqual(re.match(r'(ab?)*?b', 'ab').groups(), ('a',))

def test_MIN_REPEAT_ONE_mark_bug():
    # Removed unsupported tests
    s = 'axxzaz'
    p = r'(?:a*?(xx)??z)*'
    assertEqual(re.match(p, s).groups(), ('xx',))

def test_bug_40736():
    assertRaisesRegex(lambda: re.search("x*", 5), "got int")
    assertRaisesRegex(lambda: re.search("x*", type), "got builtin_function_or_method")

def test_search_anchor_at_beginning():
    s = 'x'*pow(10,7)
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

def test_immutable():
    # bpo-43908: check that re types are immutable
    def fn1(): re.Match.foo = 1
    def fn2(): re.Pattern.foo = 1
    def fn3():
        pat = re.compile("")
        tp = type(pat.scanner(""))
        tp.foo = 1

    assertRaises(fn1)
    assertRaises(fn2)
    assertRaises(fn3)

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
        assertEqual(eval(repl, vardict), expected,
                            'grouping error')

        # Try the match with both pattern and string converted to
        # bytes, and check that it still succeeds.
        bpat = bytes(pattern)
        bs = bytes(s)

        obj = re.compile(bpat)
        assertTrue(obj.search(bs))

        # Try the match with the search area limited to the extent
        # of the match and see if it still succeeds.  \B will
        # break (because it won't match at the end or start of a
        # string), so we'll ignore patterns that feature it.
        if (pattern[:2] != r'\B' and pattern[-2:] != r'\B'
                    and result != None):
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


# Run all tests:

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
test_bug_764548()
test_finditer()
test_bug_926075()
test_bug_931848()
test_bug_817234()
test_bug_6561()
test_inline_flags()
test_dollar_matches_twice()
test_bytes_str_mixing()
test_ascii_and_unicode_flag()
test_scoped_flags()
test_bug_6509()
test_search_dot_unicode()
test_compile()
test_bug_16688()
test_repeat_minmax_overflow()
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
test_MARK_PUSH_macro_bug()
test_MIN_UNTIL_mark_bug()
test_REPEAT_ONE_mark_bug()
test_MIN_REPEAT_ONE_mark_bug()
test_bug_40736()
test_search_anchor_at_beginning()
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
test_immutable()
test_re_benchmarks()
test_re_tests()

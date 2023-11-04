package regex

import (
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"

	// Necessary for go:linkname
	_ "unsafe"
)

const (
	// minimum and maximum characters involved in folding.
	minFold        = 0x0041
	maxFoldASCII   = 0x007a
	maxFoldUnicode = 0x1e943
)

// preprocessor is a type, that converts a Python-compatible regex pattern to regex pattern,
// that is compatible with either the `regexp.Regexp` or `regexp2.Regexp` engines.
type preprocessor struct {
	isStr bool
	p     *subPattern
}

// newPreprocessor creates a new regex preprocessor by parsing the regex pattern.
// If the regex pattern where passed as a bytes object, the `isStr` parameter should be set to false.
// The `flags` parameter should contain flags compatible with Python.
func newPreprocessor(s string, isStr bool, flags uint32) (*preprocessor, error) {
	sp, err := parse(s, isStr, flags)
	if err != nil {
		return nil, err
	}

	p := &preprocessor{
		isStr: isStr,
		p:     sp,
	}

	return p, nil
}

// flags returns the flags, that where parsed from the regex pattern
// or where set at the beginning of the regex pattern.
func (p *preprocessor) flags() uint32 {
	return p.p.state.flags
}

// groupNames returns a mapping of group names to its indices.
func (p *preprocessor) groupNames() map[string]int {
	return p.p.state.groupdict
}

// isSupported checks, if the current pattern is supported by the regexp engine `regexp.Regexp`
// (regex engine of the Go standard library).
func (p *preprocessor) isSupported() bool {
	if p.p.isUnsupported() {
		return false
	}

	for group := range p.p.state.groupdict {
		if !isGoIdentifer(group) {
			return false
		}
	}

	return true
}

// isGoIdentifer checks, if name is a valid Go identifier.
func isGoIdentifer(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		if c > unicode.MaxASCII {
			return false
		}

		if !isASCIILetterByte(c) && c != '_' && (i == 0 || !isDigitByte(c)) {
			return false
		}
	}

	return true
}

// stdPattern returns the preprocessed regex pattern, that is functionally equivalent
// to the Python regex engine but is still compatible with the `regexp.Regexp' engine.
func (p *preprocessor) stdPattern() string {
	var b strings.Builder

	p.writeFlags(&b, true)

	// Create the pattern string with the default replacer.
	p.p.write(&b, p.isStr, func(w *subPatternWriter, n *regexNode, ctx *subPatternContext) bool {
		return p.defaultReplacer(w, n, ctx, true)
	})

	return b.String()
}

// writeFlags writes a group of global regex flags into a string builder.
// If the `std` parameter is true, then the current regex pattern should be
// compatible with the default regex engine. If this is not the case or if
// the current regex should only ignore cases for ASCII characters, then the
// preprocessor will do the case ignoring.
func (p *preprocessor) writeFlags(w *strings.Builder, std bool) {
	flags := p.flags()

	// Remove the IGNORECASE flag if the case ignoring should be done by the preprocessor:
	// If the flag is enabled together with the ASCII flag or if the current pattern is of type bytes.
	if !std || flags&FlagASCII != 0 || !p.isStr {
		flags &= ^FlagIgnoreCase
	}
	if std {
		flags &= ^FlagUnicode
	}

	if flags&(supportedFlags|FlagUnicode) != 0 {
		w.WriteString("(?")

		if flags&FlagIgnoreCase != 0 {
			w.WriteByte('i')
		}
		if flags&FlagMultiline != 0 {
			w.WriteByte('m')
		}
		if flags&FlagDotAll != 0 {
			w.WriteByte('s')
		}
		if flags&FlagUnicode != 0 {
			w.WriteByte('u')
		}

		w.WriteByte(')')
	}
}

// unicodeRanges contains the equivalent unicode character sets of the character classes \d, \D, \s, and so on.
// These sets are compatible with the default regex engine `regexp.Regexp`.
// For other engines, they can be converted to character sets by using the parser of the default regex engine
// `syntax.Parse`.
var unicodeRanges = map[catcode]string{
	categoryDigit:    `[\p{Nd}]`,
	categoryNotDigit: `[^\p{Nd}]`,
	categorySpace:    `[ \t\n\r\f\v\p{Z}]`,
	categoryNotSpace: `[^ \t\n\r\f\v\p{Z}]`,
	categoryWord:     `[\p{L}\p{N}_]`,
	categoryNotWord:  `[^\p{L}\p{N}_]`,
}

// defaultReplacer is the default replacer for creating a preprocessed regex pattern.
// It rewrites three types of regex nodes:
// (1) If the unicode flag is present for the current regex node of type "CATEGORY", then the category is
// replaced by a character set that includes or excludes all unicode variants belonging to the regex category.
// The simplest case is to replace the category with the corresponding unicode character classes (`\p{}...`).
// This is possible if the default regular expression engine is used. Also, if the category is not negated or
// is the only element in the character set, it can be used without modification. Otherwise, the category must
// be replaced with the ranges of characters included in the character class.
// The two other types of regex nodes which need to be rewritten are only affected if both the ASCII and
// IGNORECASE flags are enabled:
// (2) Regex nodes of type LITERAL are rewritten to a character set if they represent an ASCII letter.
// Let c be its corresponding ASCII letter. The literal is then rewritten to the character set [cC], where C is
// the opposite case of the ASCII letter c. The result is that the literal matches the same letter while ignoring
// the case. This preprocessing is necessary because when both IGNORECASE and ASCII are enabled, only the case
// of ASCII letters should be ignored.
// (3) The same applies to regex nodes of type RANGE. If the range contains ASCII letters, the range needs to be
// replaced with at least two ranges. For any subranges that exclusively contain ASCII letters, other ranges must
// be added to include the opposite cases of these letters. Additionally, any subranges without ASCII letters must
// also be included.
func (p *preprocessor) defaultReplacer(w *subPatternWriter, n *regexNode, ctx *subPatternContext, std bool) bool {
	flags := p.flags()
	if ctx.group != nil {
		addFlags := ctx.group.addFlags
		delFlags := ctx.group.delFlags
		flags = combineFlags(flags, addFlags, delFlags)
	}

	isUnicode := flags&FlagUnicode != 0
	ignorecase := flags&FlagIgnoreCase != 0
	ascii := flags&FlagASCII != 0 || !p.isStr

	switch n.opcode {
	case opCategory:
		// If the current pattern is a string and the ASCII mode is not enabled,
		// some patterns must be substituted with equivalent unicode counterparts,
		// as `regexp.Regexp` only matches ASCII patterns by default.
		// Categories are always inside of character sets.

		if !isUnicode {
			return false
		}

		category := n.params.(catcode)

		// Check if the shorter unicode character sets can be added to the regex, which are only fully
		// supported by the default regex engine. Also, they can only be used if the current category does
		// not negate the set or if the current category regex node is the only element in the character set.
		// Although including the negated character class of `\d` in a character set (by using `\P{Nd}`) is
		// straightforward, it is not trivial to include the negated character of `\w` (which has an unicode
		// equivalent of `\p{Z}\v`). This is because it is not possible to exclude one of the sets `\p{Z}`
		// AND `\v` at the same time. Therefore, it is necessary to include ranges that contain ALL characters
		// not present in `\p{Z}\v`.

		if std {
			// The following switch statement is equivalent with the following if clause:
			// "if non-negated category or negated category and not hasSiblings"
			switch category {
			case categoryNotDigit, categoryNotSpace, categoryNotWord:
				if ctx.hasSiblings {
					break
				}

				fallthrough
			default:
				if r, ok := unicodeRanges[category]; ok {
					r = strings.TrimSuffix(strings.TrimPrefix(r, "["), "]") // remove the character set chars
					w.writeString(r)                                        // write the character set
					return true
				}
			}
		}

		// Add the character class by adding all ranges.

		r, err := buildUnicodeRanges(category)
		if err != nil {
			return false
		}

		writeRanges(w, r)

		return true
	case opLiteral:
		if !ignorecase {
			return false
		}

		// If the IGNORECASE flag is set, the preprocessor needs to handle case ignoring in many cases to match
		// the behavior from Python. The preprocessor always needs to handle case ignoring for the fallback
		// engine since it compares characters differently compared to the default engine (and also to Python).
		// For the default engine, it is only necessary if the ASCII mode is also enabled, or the literal is a
		// folded character of 'i'.

		// Check, if the literal does not need to be folded by the preprocessor.
		if std && !(ascii || needsFoldedLiteral(n.c)) {
			return false
		}

		// Create all cases for `n.c` by creating a folded range `c-c`.
		r := createFoldedRanges(n.c, n.c, ascii)

		if len(r) == 2 && r[0] == r[1] {
			// If the folded range only contains one element, then write it as a literal.
			w.writeLiteral(r[0])
		} else if ctx.inSet {
			writeRanges(w, r)
		} else {
			w.writeByte('[')
			writeRanges(w, r)
			w.writeByte(']')
		}

		return true
	case opRange:
		if !ignorecase {
			return false
		}

		// See the comment at case `opLiteral`.

		p := n.params.(rangeParams)

		if std && !(ascii || needsFoldedRange(p.lo, p.hi)) {
			return false
		}

		r := createFoldedRanges(p.lo, p.hi, ascii)
		writeRanges(w, r)

		return true
	}

	return false
}

// combineFlags determines the flags of the current subpattern by combining the global flags
// with the added and deleted flags of this subpattern.
func combineFlags(flags, addFlags, delFlags uint32) uint32 {
	if addFlags&typeFlags != 0 {
		flags &= ^typeFlags
	}
	return (flags | addFlags) & ^delFlags
}

// buildUnicodeRanges creates a slice of ranges, that includes all unicode characters matching the regex category.
func buildUnicodeRanges(c catcode) ([]rune, error) {
	r, ok := unicodeRanges[c]
	if !ok {
		return nil, fmt.Errorf("unknown category %d", c)
	}

	re, err := syntax.Parse(r, syntax.Perl)
	if err != nil {
		return nil, err
	}

	if re.Op != syntax.OpCharClass {
		return nil, fmt.Errorf("expected regex syntax type %s, got %s", syntax.OpCharClass, re.Op)
	}

	return re.Rune, nil
}

// writeRanges writes the slice of ranges to the subpattern writer.
func writeRanges(w *subPatternWriter, r []rune) {
	for i := 0; i < len(r); i += 2 {
		lo, hi := r[i], r[i+1]

		w.writeLiteral(lo)
		if lo != hi {
			w.writeByte('-')
			w.writeLiteral(hi)
		}
	}
}

// needsFoldedLiteral reports, whether the preprocessor needs to handle the case folding of character `c`.
func needsFoldedLiteral(c rune) bool {
	switch c {
	case 'I', 'i', '\u0130', '\u0131', '\ufb05', '\ufb06':
		return true
	default:
		return false
	}
}

// needsFoldedRange reports, whether the preprocessor needs to handle the case folding of range `[lo-hi]`.
func needsFoldedRange(lo, hi rune) bool {
	if inRange(lo, hi, 'I') || inRange(lo, hi, 'i') {
		return true
	}
	if inRange(lo, hi, '\u0130') || inRange(lo, hi, '\u0131') {
		return true
	}
	if inRange(lo, hi, '\ufb05') || inRange(lo, hi, '\ufb06') {
		return true
	}
	return false
}

// inRange reports whether `c“ is in the range `[lo-hi]`.
func inRange(lo, hi, c rune) bool {
	return lo <= c && c <= hi
}

// createFoldedRanges creates a slice of ranges, which contains all cases of the characters from `lo` to `hi`.
// If `ascii` is set to true, this function only determines different cases of ASCII characters.
// The resulting slice of ranges is sorted and any overlapping ranges are merged together.
// See also `appendFoldedRange` of package `regexp/syntax`.
func createFoldedRanges(lo, hi rune, ascii bool) []rune {
	var r []rune

	var maxFold rune
	if ascii {
		maxFold = maxFoldASCII
	} else {
		maxFold = maxFoldUnicode
	}

	// Optimizations.
	if lo <= minFold && hi >= maxFold {
		// Range is full: folding can't add more.
		return appendRange(r, lo, hi)
	}
	if hi < minFold || lo > maxFold {
		// Range is outside folding possibilities.
		return appendRange(r, lo, hi)
	}
	if lo < minFold {
		// [lo, minFold-1] needs no folding.
		r = appendRange(r, lo, minFold-1)
		lo = minFold
	}
	if hi > maxFold {
		// [maxFold+1, hi] needs no folding.
		r = appendRange(r, maxFold+1, hi)
		hi = maxFold
	}

	// Determine the folding function.
	var fold func(c rune) rune
	if ascii {
		fold = simpleFoldASCII
	} else {
		fold = simpleFold
	}

	// Brute force. Depend on appendRange to coalesce ranges on the fly.
	for c := lo; c <= hi; c++ {
		r = appendRange(r, c, c)
		for f := fold(c); f != c; f = fold(f) {
			r = appendRange(r, f, f)
		}
	}

	// Sort and simplify ranges.
	return cleanClass(&r)
}

// simpleFold is the equivalent function of `unicode.SimpleFold`
// with support for 'U+0130' and 'U+0131' for 'I' and 'i'
// and for 'U+FB05' and 'U+FB06'
func simpleFold(c rune) rune {
	switch c {
	case 'I':
		return 'i'
	case 'i':
		return '\u0130'
	case '\u0130':
		return '\u0131'
	case '\u0131':
		return 'I'
	case '\ufb05':
		return '\ufb06'
	case '\ufb06':
		return '\ufb05'
	default:
		return unicode.SimpleFold(c)
	}
}

// simpleFoldASCII is the equivalent function of `unicode.SimpleFold` limited to ASCII characters.
func simpleFoldASCII(c rune) rune {
	if inRange('A', 'Z', c) {
		return c - 'A' + 'a'
	} else if inRange('a', 'z', c) {
		return c - 'a' + 'A'
	} else {
		return c
	}
}

// Link function from package `regexp/syntax` instead of copy and paste them:

//go:linkname appendRange regexp/syntax.appendRange
func appendRange(r []rune, lo, hi rune) []rune

//go:linkname cleanClass regexp/syntax.cleanClass
func cleanClass(rp *[]rune) []rune

// fallbackPattern builds a preprocessed regex pattern compatible with the `regexp2.Regexp`.
// This pattern is almost identical to the one produced by `stdPattern`, with the exception of not
// using any unicode classes (`\p{...}`) and not naming any captured groups to preserve their order.
// This is required because `regexp2.Regexp` (and also .NET) orders capture groups from left to right
// based on the order of the opening parentheses. However, named capture groups are always ordered
// last, after the non-named capture groups. This results in a different order of capture groups
// between Python and `regexp.Regexp2`. So, all capture group names must be omitted from the regex
// pattern.
func (p *preprocessor) fallbackPattern() string {
	var b strings.Builder

	p.writeFlags(&b, false)

	p.p.write(&b, p.isStr, func(w *subPatternWriter, n *regexNode, ctx *subPatternContext) bool {
		if p.defaultReplacer(w, n, ctx, false) {
			return true
		}

		if n.opcode == opSubpattern {
			// The preprocessor must only write subpatterns differently,
			// that have a group number.

			p := n.params.(subPatternParam)

			if p.group < 0 {
				return false
			}

			w.writeByte('(')
			if p.p.len() > 0 {
				w.writePattern(p.p, &p)
			}
			w.writeByte(')')

			return true
		}

		return false
	})

	return b.String()
}

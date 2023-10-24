package regex

import (
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"
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

	flags := p.flags()
	if flags&FlagIgnoreCase != 0 && flags&FlagASCII != 0 {
		// Remove the IGNORECASE flag if it is enabled together with the ASCII flag,
		// because the preprocessor handles the case ignoring for ASCII characters.
		flags &= ^FlagIgnoreCase
	}

	if flags&supportedFlags != 0 {
		b.WriteString("(?")

		if flags&FlagIgnoreCase != 0 && flags&FlagASCII == 0 {
			b.WriteByte('i')
		}
		if flags&FlagMultiline != 0 {
			b.WriteByte('m')
		}
		if flags&FlagDotAll != 0 {
			b.WriteByte('s')
		}

		b.WriteByte(')')
	}

	// Create the pattern string with the default replacer.
	b.WriteString(p.p.toString(p.isStr, func(w *subPatternWriter, n *regexNode, ctx *subPatternContext) bool {
		return p.defaultReplacer(w, n, ctx, true)
	}))

	return b.String()
}

// unicodeRanges contains the equivalent unicode character sets of the character classes \d, \D, \s, and so on.
// These sets are compatible with the default regex engine `regexp.Regexp`.
// For other engines, they can be converted to character sets by using the parser of the default regex engine
// `syntax.Parse`.
var unicodeRanges = map[catcode]string{
	categoryDigit:    `[\p{Nd}]`,
	categoryNotDigit: `[^\p{Nd}]`,
	categorySpace:    `[\p{Z}\v]`,
	categoryNotSpace: `[^\p{Z}\v]`,
	categoryWord:     `[\p{L}\p{N}_]`,
	categoryNotWord:  `[^\p{L}\p{N}_]`,
}

// defaultReplacer is the default replacer for creating a preprocessed regex pattern.
// It rewrites three types of regex nodes:
// (1) If the unicode flag is present for the current regex node of type "CATEGORY", then the category is
// replaced by a character set that includes or excludes all unicode variants belonging to the regex category.
// The simplest case is to replace the category with the corresponding unicode character classes (`\p{}...`).
// This is possible if the standard regular expression engine is used. Also, if the category is not negated or
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

		if addFlags&FlagUnicode != 0 || delFlags&FlagASCII != 0 {
			flags |= FlagUnicode
			flags &= ^FlagASCII
		}
		if addFlags&FlagASCII != 0 || delFlags&FlagUnicode != 0 {
			flags |= FlagASCII
			flags &= ^FlagUnicode
		}
	}

	unicode := false
	asciiCase := false

	if flags&FlagUnicode != 0 {
		unicode = true
	} else if flags&FlagIgnoreCase != 0 && flags&FlagASCII != 0 {
		asciiCase = true
	}

	switch n.opcode {
	case opCategory:
		// If the current pattern is a string and the ASCII mode is not enabled,
		// some patterns must be substituted with equivalent unicode counterparts,
		// as `regexp.Regexp` only matches ASCII patterns by default.
		// Categories are always inside of character sets.

		if !p.isStr || !unicode {
			return false
		}

		category := n.params.(catcode)

		// Check if the shorter unicode character sets can be added to the regex, which are only fully
		// supported by the standard regex engine. Also, they can only be used if the current category does
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
					w.WriteString(r)                                        // write the character set
					return true
				}
			}
		}

		// Add the character class by adding all ranges.

		r, err := buildUnicodeRange(category)
		if err != nil {
			return false
		}

		for i := 0; i < len(r); i += 2 {
			lo, hi := r[i], r[i+1]

			w.writeLiteral(lo)
			if lo != hi {
				w.WriteByte('-')
				w.writeLiteral(hi)
			}
		}
	case opLiteral:
		if !asciiCase {
			return false
		}

		return writeLiteralCases(w, n.c, ctx.inSet)
	case opRange:
		if !asciiCase {
			return false
		}

		p := n.params.(rangeParams)

		// If the first and last character of the range are the same,
		// the range is be written as a literal matching both cases.
		if p.lo == p.hi {
			return writeLiteralCases(w, p.lo, ctx.inSet)
		}

		writeSubranges(w, p.lo, p.hi)
	}

	return false
}

// buildRange creates a range, that includes all unicode characters matching the regex category.
func buildUnicodeRange(c catcode) ([]rune, error) {
	r, ok := unicodeRanges[c]
	if ok {
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

// writeLiteralCases writes both cases if the literal `c` to the pattern, if it is an ASCII letter.
// If the literal is not inside a set, it is replaced with a set containing both cases.
// Otherwise, if the literal is inside a set, both cases are added to the set.
func writeLiteralCases(w *subPatternWriter, c rune, inSet bool) bool {
	o := oppositeASCIICase(c)
	if c == o { // no other ASCII case exists
		return false
	}

	if !inSet {
		w.WriteByte('[')
		w.writeLiteral(c)
		w.writeLiteral(o)
		w.WriteByte(']')
	} else {
		w.writeLiteral(c)
		w.writeLiteral(o)
	}

	return true
}

// oppositeASCIICase returns the opposite case of character 'c'.
// If 'c' is not an ASCII character or has no opposite case, it is returned without change.
func oppositeASCIICase(c rune) rune {
	if 'A' <= c && c <= 'Z' {
		return lower(c)
	}
	if 'a' <= c && c <= 'z' {
		return upper(c)
	}
	return c
}

// lower returns the corresponding lowercase letter for any ASCII letter between 'A' and 'Z'.
func lower(c rune) rune {
	return c - 'A' + 'a'
}

// upper returns the corresponding uppercase letter for any ASCII letter between 'a' and 'z'.
func upper(c rune) rune {
	return c - 'a' + 'A'
}

// writeSubranges writes the specified range, together with an range that matches the ASCII
// letters of opposite case in this range. This is done by splitting the range into up to
// five subranges:
// (1) characters from \x00 to \x40 ('\0' to 'A' - 1)
// (2) characters from \x41 to \x5a ('A' to 'Z')
// (3) characters from \x5b to \x60 ('Z' + 1 to 'a' - 1)
// (4) characters from \x61 to \x7a ('a' to 'z')
// (5) characters from \x7b         ('z' + 1)
//
// If subranges (2) or (4) are not present, the range does not needs to be modified.
// If both subranges exist and are fully covered, the range also does not need to be modified.
// Otherwise, the subranges (1) to (5) are appended as normal, if they exist.
// If both subranges (2) and (4) exist, then the corresponding subrange with the
// opposite case are appended. Whenever possible, the subranges (2), (4) and their
// equivalent cases are simplified.
func writeSubranges(w *subPatternWriter, lo, hi rune) bool {
	if lo <= subr2Start && subr4End <= hi {
		// If the subranges (2) and (4) are fully covered by the range,
		// it does not need to be modified.
		return false
	}

	subLo := subrangeIndex(lo)
	subHi := subrangeIndex(hi)

	if subLo == subHi {
		// Simplest case: If lo and hi are within the same subrange, it means
		// that they are either both ASCII letters or not.
		switch subLo {
		case subr2, subr4: // both are ASCII letters
			writeRangeCases(w, lo, hi)
			return true
		default: // do not handle the other range
			return false
		}
	}

	// At this case, the values of lo and hi are inside of different subranges and not fully
	// covered. This means that ASCII letters are included in the range, because each subrange
	// that does not represent ASCII letters is adjacent to a subrange that does represent
	// ASCII letters.

	// At first, append all subranges, that do not represent ASCII letters, so (1), (3) and (5).

	if subLo == subr1 { // append subrange (1), if it is included
		writeRange(w, lo, min(hi, subr1End))
	}
	if subLo <= subr3 && subr3 <= subHi { // append subrange (3), if it is included
		writeRange(w, max(lo, subr3Start), min(hi, subr3End))
	}
	if subHi == subr5 { // append subrange (5), if it is included
		writeRange(w, max(lo, subr5Start), hi)
	}

	// Now the subranges containing ASCII characters should be considered.
	// First, consider the case where only one of the ASCII subranges is included in the range.

	if subHi <= subr3 { // Only the subrange (2) still needs to be appended with its other cases.
		// Determine the lower and upper limit within subrange (2).
		lo = max(lo, subr2Start)
		hi = min(hi, subr2End)

		// Write both subranges
		writeRangeCases(w, lo, hi)
		return true
	}

	if subLo >= subr3 { // Only the subrange (4) still needs to be appended with its other cases.
		// Determine the lower and upper limit within subrange (4).
		lo = max(lo, subr4Start)
		hi = max(hi, subr4End)

		// Write both subranges
		writeRangeCases(w, lo, hi)
		return true
	}

	// Both the subrange (2) and (4) exist in the range.

	// Determine the lower limit within subrange (2)
	// and the upper limit within subrange (4):
	lo2 := max(lo, subr2Start)
	hi4 := min(hi, subr4End)

	// If the two subranges (2) and (4) overlap, ignoring the case, then the
	// ranges 'A' - 'Z' and 'a' - 'z' must be added to the regex pattern.
	if lo2 <= upper(hi4) {
		writeRange(w, 'A', 'Z')
		writeRange(w, 'a', 'z')
		return true
	}

	// Both ranges are not overlapping, so it is necessary to manually
	// add the range covering the opposite case for both ranges.

	writeRangeCases(w, lo2, subr2End)
	writeRangeCases(w, subr4Start, hi4)

	return true
}

// writeRange writes the range in format "l-h", where l represents
// the character `lo` and h represents character hi.
func writeRange(w *subPatternWriter, lo, hi rune) {
	w.writeLiteral(lo)
	w.WriteByte('-')
	w.writeLiteral(hi)
}

// writeRangeCases writes a range for `lo` to `hi` and a range covering the opposite case.
// The characters `lo` and `hi` must be ASCII letters from the same subrange.
func writeRangeCases(w *subPatternWriter, lo, hi rune) {
	writeRange(w, lo, hi)
	writeRange(w, oppositeASCIICase(lo), oppositeASCIICase(hi))
}

// Constants for subranges
const (
	subr1 = iota
	subr2
	subr3
	subr4
	subr5

	subr1End   = 'A' - 1
	subr2Start = 'A'
	subr2End   = 'Z'
	subr3Start = 'Z' + 1
	subr3End   = 'a' - 1
	subr4Start = 'a'
	subr4End   = 'z'
	subr5Start = 'z' + 1
)

// subrangeIndex returns the index of the subrange in which the character c is included.
func subrangeIndex(c rune) int {
	if c <= subr1End {
		return subr1
	} else if c <= subr2End {
		return subr2
	} else if c <= subr3End {
		return subr3
	} else if c <= subr4End {
		return subr4
	} else {
		return subr5
	}
}

// fallbackPattern builds a preprocessed regex pattern compatible with the `regexp2.Regexp`.
// This pattern is almost identical to the one produced by `stdPattern`, with the exception of not
// using any unicode classes (`\p{...}`) and renaming all captured groups to preserve their order.
// Also, a mapping of new group names to group indices is returned.
func (p *preprocessor) fallbackPattern() (string, map[string]int) {
	groupMapping := make(map[string]int)

	s := p.p.toString(p.isStr, func(w *subPatternWriter, n *regexNode, ctx *subPatternContext) bool {
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

			g := fmt.Sprintf("g%02d", p.group) // every group gets a unique group name to keep its order
			groupMapping[g] = p.group          // store the group position (determined from the parser) together with its new name

			w.WriteString("(?<") // No ? before P
			w.WriteString(g)
			w.WriteByte('>')
			if p.p.len() > 0 {
				w.writePattern(p.p, &p)
			}
			w.WriteByte(')')

			return true
		}

		return false
	})

	return s, groupMapping
}

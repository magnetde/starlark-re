package re

import "go.starlark.net/starlark"

// split splits `str` at all occurrences of pattern `p`. See also `reSplit`.
func split(p *Pattern, str strOrBytes, maxSplit int) (*starlark.List, error) {
	s := str.value

	var list []starlark.Value

	beg := 0
	end := 0

	err := findMatches(p.re, s, 0, len(s), maxSplit, func(match []int) error {
		end = match[0]

		list = append(list, p.pattern.asType(s[beg:end]))

		// Add all groups
		for i := 1; 2*i < len(match); i++ {
			s := match[2*i]
			e := match[2*i+1]

			if s >= 0 && e >= 0 {
				list = append(list, p.pattern.asType(str.value[s:e]))
			} else {
				list = append(list, starlark.None)

			}
		}

		beg = match[1]
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Append even if empty
	list = append(list, p.pattern.asType(s[beg:]))

	return starlark.NewList(list), nil
}

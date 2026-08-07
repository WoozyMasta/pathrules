// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"fmt"
	"slices"
	"strings"
)

// maxBraceExpansions is the hard, non-configurable cap
// on the number of alternatives one pattern may expand into.
// It exists purely as a safety limit against combinatorial explosion
// from adjacent brace groups.
const maxBraceExpansions = 256

// braceGroup is one top-level "{a,b,c}" alternation group found in a pattern.
type braceGroup struct {
	alts  []string // alts holds the group's comma-separated alternatives, in source order.
	start int      // start is the index of the group's "{" in the source pattern.
	end   int      // end is the index of the group's matching "}" in the source pattern.
}

// expandBraces expands top-level brace alternation groups
// in pattern into the cartesian product of alternatives.
//
// Every unescaped "{" outside a "[...]" character class starts an alternation group and must form a complete,
// non-nested group with at least two comma-separated alternatives that are not all empty;
// anything else (unterminated, single alternative, nested, all-empty)
// is ErrInvalidPattern - there is no "no comma means literal" fallback.
// When escaping is true, "\{", "\}", "\," and "\\" are literal and do not participate in group syntax;
// a literal "{" otherwise requires escaping.
// Expansion is capped at maxBraceExpansions alternatives per pattern.
func expandBraces(pattern string, escaping bool) ([]string, error) {
	groups, err := findBraceGroups(pattern, escaping)
	if err != nil {
		return nil, err
	}

	if len(groups) == 0 {
		return []string{pattern}, nil
	}

	total := 1
	for _, g := range groups {
		total *= len(g.alts)
		if total > maxBraceExpansions {
			return nil, fmt.Errorf("%w: brace expansion exceeds limit of %d alternatives",
				ErrInvalidPattern, maxBraceExpansions)
		}
	}

	return cartesianExpand(pattern, groups), nil
}

// findBraceGroups scans pattern left to right for top-level brace groups,
// skipping "[...]" character classes and, when escaping is true,
// escaped bytes, so neither is mistaken for group syntax.
func findBraceGroups(pattern string, escaping bool) ([]braceGroup, error) {
	var groups []braceGroup

	i := 0
	for i < len(pattern) {
		switch {
		case escaping && pattern[i] == '\\':
			i = skipEscaped(pattern, i)
		case pattern[i] == '[':
			i = skipCharClass(pattern, i)
		case pattern[i] == '{':
			group, next, err := parseBraceGroup(pattern, i, escaping)
			if err != nil {
				return nil, err
			}

			groups = append(groups, group)
			i = next
		default:
			i++
		}
	}

	return groups, nil
}

// parseBraceGroup parses one brace group starting at pattern[start] ("{"),
// requiring a matching, non-nested "}" with at least two comma-separated alternatives that are not all empty.
// It returns the parsed group and the index right after the closing "}".
func parseBraceGroup(pattern string, start int, escaping bool) (braceGroup, int, error) {
	j := start + 1
	altStart := j
	var alts []string

	for j < len(pattern) {
		switch {
		case escaping && pattern[j] == '\\':
			j = skipEscaped(pattern, j)
		case pattern[j] == '[':
			j = skipCharClass(pattern, j)
		case pattern[j] == '{':
			return braceGroup{}, 0, fmt.Errorf("%w: nested brace groups are not supported (%q)",
				ErrInvalidPattern, pattern)
		case pattern[j] == ',':
			alts = append(alts, pattern[altStart:j])
			j++
			altStart = j
		case pattern[j] == '}':
			alts = append(alts, pattern[altStart:j])
			return validateBraceGroup(pattern, start, j, alts)
		default:
			j++
		}
	}

	return braceGroup{}, 0, fmt.Errorf("%w: unterminated brace expression (%q)", ErrInvalidPattern, pattern)
}

// validateBraceGroup enforces the "at least two alternatives, not all empty"
// grammar rule and builds the resulting braceGroup.
func validateBraceGroup(pattern string, start, end int, alts []string) (braceGroup, int, error) {
	if len(alts) < 2 {
		return braceGroup{}, 0, fmt.Errorf("%w: brace expression must contain at least two alternatives (%q)",
			ErrInvalidPattern, pattern)
	}

	allEmpty := true
	for _, a := range alts {
		if a != "" {
			allEmpty = false
			break
		}
	}

	if allEmpty {
		return braceGroup{}, 0, fmt.Errorf("%w: brace expression alternatives must not all be empty (%q)",
			ErrInvalidPattern, pattern)
	}

	return braceGroup{start: start, end: end, alts: alts}, end + 1, nil
}

// skipEscaped returns the index right after the escaped byte pair
// starting at pattern[i] ("\"), or i+1 for a trailing lone backslash.
func skipEscaped(pattern string, i int) int {
	if i+1 < len(pattern) {
		return i + 2
	}

	return i + 1
}

// skipCharClass returns the index right after the "[...]" character class starting at pattern[i],
// or i+1 when pattern[i] does not start a valid one
// (a lone "[" is then just an ordinary character to the caller).
func skipCharClass(pattern string, i int) int {
	if end := findCharClassEnd(pattern, i); end >= 0 {
		return end + 1
	}

	return i + 1
}

// cartesianExpand builds the cartesian product of groups' alternatives,
// splicing each combination back into pattern's surrounding literal text.
// The leftmost group varies slowest, matching shell brace-expansion order.
func cartesianExpand(pattern string, groups []braceGroup) []string {
	total := 1
	for _, g := range groups {
		total *= len(g.alts)
	}

	results := make([]string, total)
	indices := make([]int, len(groups))

	for n := 0; n < total; n++ {
		var b strings.Builder

		prev := 0
		for gi, g := range groups {
			b.WriteString(pattern[prev:g.start])
			b.WriteString(g.alts[indices[gi]])
			prev = g.end + 1
		}
		b.WriteString(pattern[prev:])
		results[n] = b.String()

		// Advance the mixed-radix counter, rightmost group first.
		for gi := range slices.Backward(groups) {
			indices[gi]++
			if indices[gi] < len(groups[gi].alts) {
				break
			}
			indices[gi] = 0
		}
	}

	return results
}

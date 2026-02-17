// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

// Matcher evaluates path decisions against compiled ordered rules.
type Matcher struct {
	compiled        []compiledRule
	defaultAction   Action
	caseInsensitive bool
}

// NewMatcher compiles ordered rules into matcher.
func NewMatcher(rules []Rule, opts MatcherOptions) (*Matcher, error) {
	opts.applyDefaults()

	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		cr, err := compileRule(rule, opts.CaseInsensitive)
		if err != nil {
			return nil, err
		}

		compiled = append(compiled, *cr)
	}

	return &Matcher{
		compiled:        compiled,
		defaultAction:   opts.DefaultAction,
		caseInsensitive: opts.CaseInsensitive,
	}, nil
}

// Decide returns deterministic include/exclude decision for one path.
//
// Decision policy:
// - last matched rule wins
// - if no rule matched, default action is used
func (m *Matcher) Decide(path string, isDir bool) MatchResult {
	candidate := normalizePath(path)
	if m.caseInsensitive {
		candidate = asciiLower(candidate)
	}

	res := MatchResult{
		Included:  m.defaultAction == ActionInclude,
		Matched:   false,
		RuleIndex: -1,
	}

	for i := range m.compiled {
		if !m.compiled[i].matches(candidate, isDir) {
			continue
		}

		res.Matched = true
		res.RuleIndex = i
		res.Included = m.compiled[i].source.Action == ActionInclude
	}

	return res
}

// Included reports whether path is included by decision policy.
func (m *Matcher) Included(path string, isDir bool) bool {
	return m.Decide(path, isDir).Included
}

// Excluded reports whether path is excluded by decision policy.
func (m *Matcher) Excluded(path string, isDir bool) bool {
	return !m.Decide(path, isDir).Included
}

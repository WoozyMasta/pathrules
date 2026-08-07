// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

// Action represents a decision action of one rule.
type Action uint8

const (
	// ActionUnknown is unset/invalid action placeholder.
	ActionUnknown Action = iota
	// ActionExclude means matching path should be excluded.
	ActionExclude
	// ActionInclude means matching path should be included.
	ActionInclude
)

// Rule is one user-visible path rule.
type Rule struct {
	// Pattern is a gitignore-like pattern.
	Pattern string `json:"pattern" yaml:"pattern"`
	// Action is a decision action applied when the rule matches.
	Action Action `json:"action" yaml:"action"`
}

// MatcherOptions controls matcher behavior.
type MatcherOptions struct {
	// CaseInsensitive enables ASCII case-insensitive matching.
	CaseInsensitive bool `json:"case_insensitive,omitempty" yaml:"case_insensitive,omitempty"`

	// DefaultAction is applied when no rule matched.
	DefaultAction Action `json:"default_action,omitempty" yaml:"default_action,omitempty"`

	// EnableBraceExpansion enables gitignore-like brace alternation "{a,b,c}" in patterns,
	// expanded into a cartesian product of alternatives at compile time.
	// Disabled by default, so "{" and "}" stay literal for existing rule sets.
	//
	// When enabled, "{" always starts an alternation group and must form a complete,
	// non-nested group with at least two comma-separated alternatives (not all empty);
	// anything else is ErrInvalidPattern. A literal "{" then requires EnableEscaping.
	EnableBraceExpansion bool `json:"enable_brace_expansion,omitempty" yaml:"enable_brace_expansion,omitempty"`

	// EnableEscaping enables backslash-escaping of pattern metacharacters:
	// "\*", "\?", "\[", "\]", "\{", "\}", "\,", "\\",
	// and generically, "\X" for any other character X.
	// Independent of EnableBraceExpansion: useful on its own to match a literal "*" or "?" in a filename.
	//
	// Disabled by default, so a pattern's backslashes keep being normalized to "/"
	// (Windows-style path input), same as when this option does not exist.
	// When enabled, that normalization stops: use "/" for path separators and "\X" for a literal X.
	EnableEscaping bool `json:"enable_escaping,omitempty" yaml:"enable_escaping,omitempty"`
}

// MatchResult is a deterministic decision produced by matcher.
type MatchResult struct {
	// Included reports final include decision.
	Included bool `json:"included" yaml:"included"`
	// Matched reports whether at least one rule matched.
	Matched bool `json:"matched" yaml:"matched"`
	// RuleIndex is the matched rule index in matcher input order, -1 when no match.
	RuleIndex int `json:"rule_index" yaml:"rule_index"`
}

// applyDefaults fills zero-valued options with defaults.
func (opts *MatcherOptions) applyDefaults() {
	if !opts.DefaultAction.valid() {
		opts.DefaultAction = ActionInclude
	}
}

// valid reports whether action value is supported.
func (a Action) valid() bool {
	return a == ActionExclude || a == ActionInclude
}

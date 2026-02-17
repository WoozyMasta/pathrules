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

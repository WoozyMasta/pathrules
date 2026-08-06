// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ParseOptions controls ParseRules line-parsing semantics.
type ParseOptions struct {
	// CommentPrefix is the line-comment prefix. Empty defaults to "#".
	// The comment-prefix check always runs before the negation-prefix check,
	// so a line matching both is treated as a comment;
	// do not configure one prefix as a strict prefix of the other (e.g. "#" and "#!").
	CommentPrefix string `json:"comment_prefix,omitempty" yaml:"comment_prefix,omitempty"`
	// NegationPrefix is the negation token. Empty defaults to "!".
	// Only consulted when DisableNegation is false. See CommentPrefix caveat above.
	NegationPrefix string `json:"negation_prefix,omitempty" yaml:"negation_prefix,omitempty"`
	// PlainAction is applied to a plain (non-negated) pattern line.
	// Zero value defaults to ActionExclude.
	PlainAction Action `json:"plain_action,omitempty" yaml:"plain_action,omitempty"`
	// NegatedAction is applied to a negated pattern line.
	// Zero value defaults to ActionInclude. Unused when DisableNegation is true.
	NegatedAction Action `json:"negated_action,omitempty" yaml:"negated_action,omitempty"`
	// DisableNegation disables negation-prefix handling entirely:
	// every line uses PlainAction, and a leading NegationPrefix token
	// (escaped or not) is left in the pattern verbatim.
	DisableNegation bool `json:"disable_negation,omitempty" yaml:"disable_negation,omitempty"`
	// KeepTrailingSpaces skips trailing-space trimming; only trailing "\r" is stripped.
	// Default false preserves current trimming behavior.
	KeepTrailingSpaces bool `json:"keep_trailing_spaces,omitempty" yaml:"keep_trailing_spaces,omitempty"`
}

// applyDefaults fills zero-valued options with defaults.
func (opts *ParseOptions) applyDefaults() {
	if !opts.PlainAction.valid() {
		opts.PlainAction = ActionExclude
	}

	if !opts.NegatedAction.valid() {
		opts.NegatedAction = ActionInclude
	}

	if opts.CommentPrefix == "" {
		opts.CommentPrefix = "#"
	}

	if opts.NegationPrefix == "" {
		opts.NegationPrefix = "!"
	}
}

// validate reports configuration conflicts that would make parsing ambiguous.
func (opts ParseOptions) validate() error {
	if !opts.DisableNegation && opts.CommentPrefix == opts.NegationPrefix {
		return fmt.Errorf("%w: comment prefix and negation prefix are both %q",
			ErrInvalidParseOptions, opts.CommentPrefix)
	}

	return nil
}

// ParseRules parses gitignore-like rules from reader according to opts.
//
// Semantics (defaults, i.e. opts == ParseOptions{}):
//   - blank lines are ignored
//   - lines starting with the comment prefix ("#" by default) are ignored
//   - lines starting with the negation prefix ("!" by default)
//     create an include rule and have the prefix stripped;
//     other lines create an exclude rule
//   - "\" + comment prefix and "\" + negation prefix
//     escape a literal leading comment/negation token
//
// See ParseOptions for reconfiguring prefixes and actions.
func ParseRules(r io.Reader, opts ParseOptions) ([]Rule, error) {
	opts.applyDefaults()
	if err := opts.validate(); err != nil {
		return nil, err
	}

	s := bufio.NewScanner(r)
	rules := make([]Rule, 0, 16)
	escapedComment := `\` + opts.CommentPrefix
	escapedNegation := `\` + opts.NegationPrefix

	for s.Scan() {
		line := strings.TrimRight(s.Text(), "\r")
		if line == "" {
			continue
		}

		if !opts.KeepTrailingSpaces {
			line = trimTrailingSpaces(line)
			if line == "" {
				continue
			}
		}

		if strings.HasPrefix(line, opts.CommentPrefix) {
			continue
		}

		if strings.HasPrefix(line, escapedComment) {
			line = line[1:]
		}

		action := opts.PlainAction
		if !opts.DisableNegation {
			if strings.HasPrefix(line, opts.NegationPrefix) {
				action = opts.NegatedAction
				line = line[len(opts.NegationPrefix):]
			} else if strings.HasPrefix(line, escapedNegation) {
				line = line[1:]
			}
		}

		if line == "" {
			continue
		}

		rules = append(rules, Rule{
			Action:  action,
			Pattern: line,
		})
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("scan rules: %w", err)
	}

	return rules, nil
}

// ParseRulesString parses rules from string input according to opts.
func ParseRulesString(src string, opts ParseOptions) ([]Rule, error) {
	return ParseRules(strings.NewReader(src), opts)
}

// trimTrailingSpaces removes trailing spaces unless escaped by "\".
func trimTrailingSpaces(s string) string {
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		if len(s) >= 2 && s[len(s)-2] == '\\' {
			s = s[:len(s)-2] + s[len(s)-1:]
			break
		}

		s = s[:len(s)-1]
	}

	return s
}

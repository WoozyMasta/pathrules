// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRules(t *testing.T) {
	t.Parallel()

	// Built via explicit "\n"-joined lines rather than a raw string literal:
	// the trailing "\ " escape on the last line is significant test input,
	// and editors/formatters that trim trailing whitespace on save
	// would silently strip it from a raw string literal's source line.
	rules, err := ParseRulesString(strings.Join([]string{
		"",
		"# comment",
		"*.tmp",
		"!keep.tmp",
		`\#literal`,
		`\!bang`,
		`name\ `,
		"",
	}, "\n"), ParseOptions{})
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	if len(rules) != 5 {
		t.Fatalf("len(rules)=%d, want 5", len(rules))
	}

	if rules[0].Action != ActionExclude || rules[0].Pattern != "*.tmp" {
		t.Fatalf("rule[0]=%+v", rules[0])
	}

	if rules[1].Action != ActionInclude || rules[1].Pattern != "keep.tmp" {
		t.Fatalf("rule[1]=%+v", rules[1])
	}

	if rules[2].Action != ActionExclude || rules[2].Pattern != "#literal" {
		t.Fatalf("rule[2]=%+v", rules[2])
	}

	if rules[3].Action != ActionExclude || rules[3].Pattern != "!bang" {
		t.Fatalf("rule[3]=%+v", rules[3])
	}

	if rules[4].Action != ActionExclude || rules[4].Pattern != "name " {
		t.Fatalf("rule[4]=%+v", rules[4])
	}
}

func TestParseRulesRejectsOverlongLine(t *testing.T) {
	t.Parallel()

	// bufio.Scanner's default max token size is 64KiB;
	// a single line (no newline) past that must surface as a normal error,
	// not a panic, and must not silently truncate the input.
	src := strings.Repeat("a", 128*1024)

	_, err := ParseRulesString(src, ParseOptions{})
	if err == nil {
		t.Fatalf("ParseRulesString(overlong line): want error, got nil")
	}
}

func TestParseRulesInvertedActions(t *testing.T) {
	t.Parallel()

	// Allow-list style: plain lines include, "!" excludes -
	// the inverse of the gitignore-style defaults, and the motivating use case for ParseOptions.
	rules, err := ParseRulesString(strings.Join([]string{
		"*.c",
		"!*.tmp",
	}, "\n"), ParseOptions{
		PlainAction:   ActionInclude,
		NegatedAction: ActionExclude,
	})
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("len(rules)=%d, want 2", len(rules))
	}

	if rules[0].Action != ActionInclude || rules[0].Pattern != "*.c" {
		t.Fatalf("rule[0]=%+v", rules[0])
	}

	if rules[1].Action != ActionExclude || rules[1].Pattern != "*.tmp" {
		t.Fatalf("rule[1]=%+v", rules[1])
	}
}

func TestParseRulesDisableNegation(t *testing.T) {
	t.Parallel()

	rules, err := ParseRulesString(strings.Join([]string{
		"!bang",
		`\!bang`,
	}, "\n"), ParseOptions{DisableNegation: true})
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("len(rules)=%d, want 2", len(rules))
	}

	// Negation is not special-cased at all: the leading "!" is left untouched,
	// and the escape backslash (which has nothing to escape) is left untouched too.
	if rules[0].Action != ActionExclude || rules[0].Pattern != "!bang" {
		t.Fatalf("rule[0]=%+v", rules[0])
	}

	if rules[1].Action != ActionExclude || rules[1].Pattern != `\!bang` {
		t.Fatalf("rule[1]=%+v", rules[1])
	}
}

func TestParseRulesCustomPrefixes(t *testing.T) {
	t.Parallel()

	rules, err := ParseRulesString(strings.Join([]string{
		";; comment",
		"*.tmp",
		"~keep.tmp",
	}, "\n"), ParseOptions{
		CommentPrefix:  ";;",
		NegationPrefix: "~",
	})
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("len(rules)=%d, want 2", len(rules))
	}

	if rules[0].Action != ActionExclude || rules[0].Pattern != "*.tmp" {
		t.Fatalf("rule[0]=%+v", rules[0])
	}

	if rules[1].Action != ActionInclude || rules[1].Pattern != "keep.tmp" {
		t.Fatalf("rule[1]=%+v", rules[1])
	}
}

func TestParseRulesConflictingPrefixesRejected(t *testing.T) {
	t.Parallel()

	_, err := ParseRulesString("*.tmp", ParseOptions{
		CommentPrefix:  "#",
		NegationPrefix: "#",
	})
	if !errors.Is(err, ErrInvalidParseOptions) {
		t.Fatalf("ParseRulesString: err=%v, want ErrInvalidParseOptions", err)
	}

	// The conflict only matters when negation is enabled:
	// with negation disabled, NegationPrefix is never consulted, so equal prefixes are moot.
	rules, err := ParseRulesString("*.tmp", ParseOptions{
		CommentPrefix:   "#",
		NegationPrefix:  "#",
		DisableNegation: true,
	})
	if err != nil {
		t.Fatalf("ParseRulesString(DisableNegation): %v", err)
	}

	if len(rules) != 1 || rules[0].Pattern != "*.tmp" {
		t.Fatalf("rules=%+v", rules)
	}
}

func TestParseRulesKeepTrailingSpaces(t *testing.T) {
	t.Parallel()

	rules, err := ParseRulesString("name   ", ParseOptions{KeepTrailingSpaces: true})
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	if len(rules) != 1 || rules[0].Pattern != "name   " {
		t.Fatalf("rules=%+v", rules)
	}
}

func TestParseRulesPlainEqualsNegatedActionAllowed(t *testing.T) {
	t.Parallel()

	rules, err := ParseRulesString(strings.Join([]string{
		"*.tmp",
		"!keep.tmp",
	}, "\n"), ParseOptions{
		PlainAction:   ActionExclude,
		NegatedAction: ActionExclude,
	})
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("len(rules)=%d, want 2", len(rules))
	}

	// The negation prefix is still recognized and stripped from the pattern
	// even though it happens to produce the same action as a plain line.
	if rules[1].Action != ActionExclude || rules[1].Pattern != "keep.tmp" {
		t.Fatalf("rule[1]=%+v", rules[1])
	}
}

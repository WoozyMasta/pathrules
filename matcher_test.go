// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import "testing"

func TestMatcherIgnoreMode(t *testing.T) {
	t.Parallel()

	rules, err := ParseRulesString(`
*.tmp
!keep.tmp
build/
!build/keep.txt
`)
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	m, err := NewMatcher(rules, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if m.Included("a.tmp", false) {
		t.Fatalf("a.tmp must be excluded")
	}

	if !m.Included("keep.tmp", false) {
		t.Fatalf("keep.tmp must be included")
	}

	if m.Included("build/a.txt", false) {
		t.Fatalf("build/a.txt must be excluded")
	}

	if !m.Included("build/keep.txt", false) {
		t.Fatalf("build/keep.txt must be included by last matching rule")
	}
}

func TestMatcherAllowListMode(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{Action: ActionInclude, Pattern: "*.paa"},
		{Action: ActionInclude, Pattern: "textures/**"},
	}

	m, err := NewMatcher(rules, MatcherOptions{
		DefaultAction: ActionExclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Included("image.paa", false) {
		t.Fatalf("image.paa must be included")
	}

	if !m.Included("textures/ui/a.png", false) {
		t.Fatalf("textures/ui/a.png must be included")
	}

	if m.Included("scripts/main.c", false) {
		t.Fatalf("scripts/main.c must be excluded by default")
	}
}

func TestMatcherAnchoredPattern(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{Action: ActionExclude, Pattern: "/config/*.cpp"},
	}

	m, err := NewMatcher(rules, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("config/server.cpp", false) {
		t.Fatalf("config/server.cpp must be excluded")
	}

	if m.Excluded("addons/config/server.cpp", false) {
		t.Fatalf("addons/config/server.cpp must not match anchored pattern")
	}
}

func TestMatcherCharClass(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "file[0-2].txt"},
	}, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("file1.txt", false) {
		t.Fatalf("file1.txt must be excluded")
	}

	if m.Excluded("file9.txt", false) {
		t.Fatalf("file9.txt must not match char class pattern")
	}
}

func TestMatcherCaseInsensitive(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "*.CPP"},
	}, MatcherOptions{
		CaseInsensitive: true,
		DefaultAction:   ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded(`src\MAIN.cpp`, false) {
		t.Fatalf("src\\MAIN.cpp must be excluded in case-insensitive mode")
	}
}

func TestMatcherDefaultActionFallback(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher(nil, MatcherOptions{})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	got := m.Decide("file.txt", false)
	if !got.Included || got.Matched || got.RuleIndex != -1 {
		t.Fatalf("unexpected fallback decision: %+v", got)
	}
}

func TestMatcherTrailingDoubleStar(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "assets/group/**"},
	}, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("assets/group/file.paa", false) {
		t.Fatalf("assets/group/file.paa must be excluded")
	}

	if !m.Excluded("mods/assets/group/file.paa", false) {
		t.Fatalf("mods/assets/group/file.paa must be excluded by unanchored rule")
	}

	if m.Excluded("assets/group", true) {
		t.Fatalf("assets/group must not match trailing /** without descendant component")
	}
}

func TestMatcherUnanchoredPathWildcard(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "scripts/module_010/*.c"},
	}, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("scripts/module_010/main.c", false) {
		t.Fatalf("scripts/module_010/main.c must be excluded")
	}

	if !m.Excluded("addons/scripts/module_010/main.c", false) {
		t.Fatalf("addons/scripts/module_010/main.c must be excluded by unanchored rule")
	}

	if m.Excluded("scripts/module_010/sub/main.c", false) {
		t.Fatalf("scripts/module_010/sub/main.c must not match single-segment wildcard")
	}
}

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"errors"
	"strings"
	"testing"
)

func TestMatcherIgnoreMode(t *testing.T) {
	t.Parallel()

	rules, err := ParseRulesString(`
*.tmp
!keep.tmp
build/
!build/keep.txt
`, ParseOptions{})
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

func TestMatcherAnchoredComponentPattern(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{Action: ActionExclude, Pattern: "/root.txt"},
	}

	m, err := NewMatcher(rules, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("root.txt", false) {
		t.Fatalf("root.txt must be excluded")
	}

	if m.Excluded("dir/root.txt", false) {
		t.Fatalf("dir/root.txt must not match anchored component pattern")
	}
}

func TestMatcherAnchoredDirOnlyComponentPattern(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{Action: ActionExclude, Pattern: "/build/"},
	}

	m, err := NewMatcher(rules, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("build/a.txt", false) {
		t.Fatalf("build/a.txt must be excluded")
	}

	if m.Excluded("src/build/a.txt", false) {
		t.Fatalf("src/build/a.txt must not match anchored dir-only component pattern")
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

func TestMatcherDecideNormalizesEdgeCasePaths(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "etc/passwd"},
	}, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	// Matcher.Decide has no error return:
	// it normalizes whatever it is given and never panics.
	// Path-safety validation is Provider's job, not Matcher's.
	//
	// "/etc/passwd", the Windows-separated form, and "./etc/passwd"
	// all normalize to the candidate "etc/passwd" and hit the exclude rule.
	matchCases := []string{"/etc/passwd", `\etc\passwd`, "./etc/passwd"}
	for _, path := range matchCases {
		if !m.Excluded(path, false) {
			t.Fatalf("Excluded(%q)=false, want true (normalizes to etc/passwd)", path)
		}
	}

	// "", ".", and ".." all normalize to an empty candidate,
	// which never matches any rule, so the default action (Include) applies.
	emptyCases := []string{"", ".", ".."}
	for _, path := range emptyCases {
		if m.Excluded(path, false) {
			t.Fatalf("Excluded(%q)=true, want false (normalizes to empty candidate)", path)
		}
	}
}

func TestMatcherDecideHandlesInvalidUTF8Path(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "*.tmp"},
	}, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	invalid := string([]byte{0xff, 0xfe, '/', 0x80, 0x81, '.', 't', 'm', 'p'})

	// Matching is byte-oriented (no rune decoding),
	// so invalid UTF-8 must neither panic nor be silently dropped: it still matches "*.tmp".
	if !m.Excluded(invalid, false) {
		t.Fatalf("Excluded(invalid UTF-8 path)=false, want true")
	}
}

func TestMatcherDecideHandlesVeryLongPatternAndPath(t *testing.T) {
	t.Parallel()

	longName := strings.Repeat("a", 20000) + ".bin"

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: longName},
	}, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded(longName, false) {
		t.Fatalf("Excluded(longName)=false, want true")
	}

	almostSame := strings.Repeat("a", 19999) + "b.bin"
	if m.Excluded(almostSame, false) {
		t.Fatalf("Excluded(almostSame)=true, want false")
	}

	veryLongUnrelatedPath := strings.Repeat("dir/", 5000) + "file.txt"
	if m.Excluded(veryLongUnrelatedPath, false) {
		t.Fatalf("Excluded(veryLongUnrelatedPath)=true, want false")
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

func TestMatcherDecideRuleIndexIsLastMatch(t *testing.T) {
	t.Parallel()

	rules := []Rule{
		{Action: ActionExclude, Pattern: "*.tmp"},
		{Action: ActionInclude, Pattern: "*.tmp"},
		{Action: ActionExclude, Pattern: "keep.tmp"},
		{Action: ActionInclude, Pattern: "keep.tmp"},
	}

	m, err := NewMatcher(rules, MatcherOptions{
		DefaultAction: ActionExclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	got := m.Decide("keep.tmp", false)
	if !got.Matched || got.RuleIndex != 3 || !got.Included {
		t.Fatalf("expected last matching rule (index 3, included) to win, got %+v", got)
	}

	got = m.Decide("other.tmp", false)
	if !got.Matched || got.RuleIndex != 1 || !got.Included {
		t.Fatalf("expected rule index 1 (include) to win over rule 0, got %+v", got)
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

func TestMatcherBraceExpansionDisabledByDefault(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: "*.{md,txt}"},
	}, MatcherOptions{
		DefaultAction: ActionExclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	// EnableBraceExpansion defaults to false: "{" and "}" stay literal characters.
	if m.Included("readme.md", false) {
		t.Fatalf("readme.md must not be included when brace expansion is disabled")
	}

	if !m.Included("readme.{md,txt}", false) {
		t.Fatalf("readme.{md,txt} (literal) must be included when brace expansion is disabled")
	}
}

func TestMatcherBraceExpansionEnabled(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: "*.{md,txt}"},
	}, MatcherOptions{
		DefaultAction:        ActionExclude,
		EnableBraceExpansion: true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Included("readme.md", false) {
		t.Fatalf("readme.md must be included")
	}

	if !m.Included("readme.txt", false) {
		t.Fatalf("readme.txt must be included")
	}

	if m.Included("readme.rst", false) {
		t.Fatalf("readme.rst must not be included")
	}
}

func TestMatcherBraceExpansionCaseInsensitive(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: "*.{MD,TXT}"},
	}, MatcherOptions{
		DefaultAction:        ActionExclude,
		CaseInsensitive:      true,
		EnableBraceExpansion: true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Included("README.MD", false) {
		t.Fatalf("README.MD must be included in case-insensitive mode")
	}
}

func TestMatcherBraceExpansionAnchoredDirOnly(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "/{docs,doc}/"},
	}, MatcherOptions{
		DefaultAction:        ActionInclude,
		EnableBraceExpansion: true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("docs/readme.txt", false) {
		t.Fatalf("docs/readme.txt must be excluded")
	}

	if !m.Excluded("doc/readme.txt", false) {
		t.Fatalf("doc/readme.txt must be excluded")
	}

	if m.Excluded("other/readme.txt", false) {
		t.Fatalf("other/readme.txt must not be excluded")
	}

	if m.Excluded("sub/docs/readme.txt", false) {
		t.Fatalf("sub/docs/readme.txt must not match anchored brace group")
	}
}

func TestMatcherBraceExpansionSlashInsideGroup(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionExclude, Pattern: "a/{b,c}/d"},
	}, MatcherOptions{
		DefaultAction:        ActionInclude,
		EnableBraceExpansion: true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Excluded("a/b/d", false) {
		t.Fatalf("a/b/d must be excluded")
	}

	if !m.Excluded("a/c/d", false) {
		t.Fatalf("a/c/d must be excluded")
	}

	if m.Excluded("a/e/d", false) {
		t.Fatalf("a/e/d must not be excluded")
	}
}

func TestMatcherBraceExpansionNestedRejected(t *testing.T) {
	t.Parallel()

	_, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: "foo.{md,{adoc,asciidoc}}"},
	}, MatcherOptions{
		EnableBraceExpansion: true,
	})
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("NewMatcher: got err=%v, want ErrInvalidPattern", err)
	}
}

func TestMatcherEscapingDisabledByDefault(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: `file\*.txt`},
	}, MatcherOptions{
		DefaultAction: ActionExclude,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	// EnableEscaping defaults to false:
	// "\" is still normalized to "/" (Windows-style path input),
	// so this pattern means the path "file/*.txt", not a literal "*".
	if !m.Included("file/anything.txt", false) {
		t.Fatalf("file/anything.txt must be included")
	}

	if m.Included("file*.txt", false) {
		t.Fatalf("file*.txt must not be included when escaping is disabled")
	}
}

func TestMatcherEscapingLiteralWildcard(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: `file\*.txt`},
	}, MatcherOptions{
		DefaultAction:  ActionExclude,
		EnableEscaping: true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Included("file*.txt", false) {
		t.Fatalf("file*.txt (literal) must be included")
	}

	if m.Included("fileA.txt", false) {
		t.Fatalf("fileA.txt must not be included: \\* is a literal star, not a wildcard")
	}
}

func TestMatcherEscapingLiteralCharClassBracket(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: `data\[0\].bin`},
	}, MatcherOptions{
		DefaultAction:  ActionExclude,
		EnableEscaping: true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Included("data[0].bin", false) {
		t.Fatalf("data[0].bin (literal) must be included")
	}

	if m.Included("data0.bin", false) {
		t.Fatalf("data0.bin must not be included: \\[ \\] are literal brackets, not a char class")
	}
}

func TestMatcherEscapingCaseInsensitive(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: `FILE\*.TXT`},
	}, MatcherOptions{
		DefaultAction:   ActionExclude,
		CaseInsensitive: true,
		EnableEscaping:  true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Included("file*.txt", false) {
		t.Fatalf("file*.txt must be included in case-insensitive mode")
	}
}

func TestMatcherEscapingWithBraceExpansion(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: `file\{v1\}.{md,txt}`},
	}, MatcherOptions{
		DefaultAction:        ActionExclude,
		EnableBraceExpansion: true,
		EnableEscaping:       true,
	})
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}

	if !m.Included("file{v1}.md", false) {
		t.Fatalf("file{v1}.md must be included")
	}

	if !m.Included("file{v1}.txt", false) {
		t.Fatalf("file{v1}.txt must be included")
	}

	if m.Included("fileXv1X.md", false) {
		t.Fatalf("fileXv1X.md must not be included: \\{ \\} are literal braces")
	}
}

// Without EnableEscaping, EnableBraceExpansion's strict grammar leaves no way
// to write a literal "{": any "{" must form a valid >=2-alternative group or compilation fails.
func TestMatcherLiteralBraceRequiresEscapingWhenExpansionEnabled(t *testing.T) {
	t.Parallel()

	_, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: "notes{draft}.txt"},
	}, MatcherOptions{
		EnableBraceExpansion: true,
	})
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("NewMatcher: got err=%v, want ErrInvalidPattern", err)
	}
}

func TestMatcherBraceExpansionLimitExceeded(t *testing.T) {
	t.Parallel()

	group := "{a,b,c,d,e,f,g,h,i,j}"

	_, err := NewMatcher([]Rule{
		{Action: ActionInclude, Pattern: group + group + group},
	}, MatcherOptions{
		EnableBraceExpansion: true,
	})
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("NewMatcher: got err=%v, want ErrInvalidPattern", err)
	}
}

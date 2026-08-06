// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fuzzMaxRuleTextLen = 32 * 1024
	fuzzMaxPathLen     = 8 * 1024
	fuzzMaxRuleCount   = 2000
)

// fuzzGarbageSeeds are adversarial rule-file contents simulating
// a corrupted or binary file being mistaken for a rules file:
// raw binary bytes, invalid UTF-8 sequences, an embedded NUL byte, a UTF-8 BOM,
// control characters (ESC/BEL/vertical tab), lone CR without LF,
// and one very long line with no newline at all
// (which should hit bufio.Scanner's line-too-long path).
var fuzzGarbageSeeds = []string{
	string([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0xff, 0xfe, 0xfd, 0x7f}),
	"\xef\xbb\xbf*.tmp\n",
	"\x80\x81\x82\xc0\xaf\n",
	"file\x00name.txt\n*.tmp\x00\n",
	"*.tmp\r\r\r!keep.tmp",
	"\x1b[31m*.tmp\x07\x0b\x0c\n",
	strings.Repeat("a", 70*1024),
}

// fuzzGarbagePath is an adversarial candidate path: embedded NUL,
// an escape control character, and an invalid UTF-8 byte.
const fuzzGarbagePath = "\x00weird\x1bpath\xff"

// FuzzParseRules feeds arbitrary bytes through ParseRules and makes sure the parser never panics,
// and that every successfully parsed rule set either compiles or fails compilation with a normal error.
func FuzzParseRules(f *testing.F) {
	seeds := []string{
		"",
		"# comment only\n",
		"*.tmp\n!keep.tmp\n",
		"\\#not-a-comment\n\\!not-a-negation\n",
		"build/\n/config/*.cpp\n",
		"file[0-2].txt\n",
		"trailing space \n",
		"assets/group/**\n!assets/group/keep_*.paa\n",
	}
	seeds = append(seeds, fuzzGarbageSeeds...)
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// No size skip here: ParseRules is a single O(n) scan
		// and must handle oversized/garbage input
		// (including bufio.Scanner's line-too-long path)
		// as a normal error, not a panic.
		rules, err := ParseRules(bytes.NewReader(data), ParseOptions{})
		if err != nil {
			return
		}

		if len(rules) > fuzzMaxRuleCount {
			t.Skip("too many rules for fuzz budget")
		}

		// Compile errors are expected for some parsed patterns
		// (e.g. empty after normalization); the invariant here is only "never panics".
		_, _ = NewMatcher(rules, MatcherOptions{DefaultAction: ActionInclude})
	})
}

// FuzzNewMatcher feeds arbitrary rule-file text through the parser
// and compiler and makes sure compilation never panics and never hangs.
func FuzzNewMatcher(f *testing.F) {
	seeds := []string{
		"*.tmp\n!keep.tmp\nbuild/\n!build/keep.txt\n",
		"/config/*.cpp\n",
		"file[0-2].txt\n",
		"assets/group/**\n",
		"scripts/module_010/*.c\n",
		"data/file_[0-9].bin\n",
		"!docs/section/**/*.md\n",
	}
	seeds = append(seeds, fuzzGarbageSeeds...)
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > fuzzMaxRuleTextLen {
			t.Skip("input too large for fuzz budget")
		}

		rules, err := ParseRulesString(src, ParseOptions{})
		if err != nil {
			return
		}

		if len(rules) > fuzzMaxRuleCount {
			t.Skip("too many rules for fuzz budget")
		}

		m, err := NewMatcher(rules, MatcherOptions{DefaultAction: ActionInclude})
		if err != nil {
			return
		}

		if m == nil {
			t.Fatal("NewMatcher returned nil matcher with nil error")
		}
	})
}

// FuzzMatcherDecide feeds arbitrary rule text, path, and isDir combinations through Matcher.Decide
// and checks the documented MatchResult invariants.
func FuzzMatcherDecide(f *testing.F) {
	f.Add("*.tmp\n!keep.tmp\n", "keep.tmp", false)
	f.Add("build/\n!build/keep.txt\n", "build/keep.txt", false)
	f.Add("/config/*.cpp\n", "config/server.cpp", false)
	f.Add("", "any/path.txt", true)
	f.Add("file[0-2].txt\n", `src\MAIN.CPP`, false)
	for _, s := range fuzzGarbageSeeds {
		f.Add(s, fuzzGarbagePath, false)
	}

	f.Fuzz(func(t *testing.T, ruleText string, path string, isDir bool) {
		if len(ruleText) > fuzzMaxRuleTextLen || len(path) > fuzzMaxPathLen {
			t.Skip("input too large for fuzz budget")
		}

		rules, err := ParseRulesString(ruleText, ParseOptions{})
		if err != nil {
			return
		}

		if len(rules) > fuzzMaxRuleCount {
			t.Skip("too many rules for fuzz budget")
		}

		m, err := NewMatcher(rules, MatcherOptions{DefaultAction: ActionInclude})
		if err != nil {
			return
		}

		res := m.Decide(path, isDir)

		if res.RuleIndex < -1 {
			t.Fatalf("RuleIndex=%d, want >= -1", res.RuleIndex)
		}

		if !res.Matched && res.RuleIndex != -1 {
			t.Fatalf("Matched=false but RuleIndex=%d, want -1", res.RuleIndex)
		}

		if res.Matched && (res.RuleIndex < 0 || res.RuleIndex >= len(rules)) {
			t.Fatalf("Matched=true but RuleIndex=%d out of range [0,%d)", res.RuleIndex, len(rules))
		}
	})
}

// FuzzProviderDecide feeds arbitrary root/subdirectory rules file content
// and an arbitrary relative path through Provider.Decide,
// and checks it never panics and respects the same MatchResult invariants as Matcher.Decide.
func FuzzProviderDecide(f *testing.F) {
	f.Add("*.tmp\n", "!keep.tmp\n", "keep.tmp")
	f.Add("build/\n", "", "build/a.txt")
	f.Add("", "", "../escape.txt")
	f.Add("", "", "/etc/passwd")
	f.Add("", "", "")
	for _, s := range fuzzGarbageSeeds {
		f.Add(s, s, fuzzGarbagePath)
	}

	f.Fuzz(func(t *testing.T, rootRules string, subRules string, relPath string) {
		if len(rootRules) > fuzzMaxRuleTextLen ||
			len(subRules) > fuzzMaxRuleTextLen ||
			len(relPath) > fuzzMaxPathLen {
			t.Skip("input too large for fuzz budget")
		}

		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, defaultRulesFileName), []byte(rootRules), 0o600); err != nil {
			t.Fatalf("WriteFile root rules: %v", err)
		}

		subDir := filepath.Join(root, "sub")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(subDir, defaultRulesFileName), []byte(subRules), 0o600); err != nil {
			t.Fatalf("WriteFile sub rules: %v", err)
		}

		p, err := NewProvider(root, ProviderOptions{
			MatcherOptions: MatcherOptions{DefaultAction: ActionInclude},
		})
		if err != nil {
			return
		}

		res, err := p.Decide(relPath, false)
		if err != nil {
			// Malformed rule content or an invalid/outside-root path
			// are expected normal errors, not panics.
			return
		}

		if res.RuleIndex < -1 {
			t.Fatalf("RuleIndex=%d, want >= -1", res.RuleIndex)
		}

		if !res.Matched && res.RuleIndex != -1 {
			t.Fatalf("Matched=false but RuleIndex=%d, want -1", res.RuleIndex)
		}
	})
}

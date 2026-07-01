// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProviderRecursiveOverrides(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRulesFile(t, filepath.Join(root, ".pboignore"), "*.tmp\n")
	if err := os.MkdirAll(filepath.Join(root, "textures"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeRulesFile(t, filepath.Join(root, "textures", ".pboignore"), "!*.tmp\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".pboignore",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	if included, err := p.Included("a.tmp", false); err != nil || included {
		t.Fatalf("Included(a.tmp)=%v err=%v, want excluded", included, err)
	}

	if included, err := p.Included("textures/a.tmp", false); err != nil || !included {
		t.Fatalf("Included(textures/a.tmp)=%v err=%v, want included", included, err)
	}
}

func TestProviderMixesBaseAndFileRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRulesFile(t, filepath.Join(root, ".rules"), "scripts/**\n!scripts/keep.c\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".rules",
		BaseRules: []Rule{
			{Action: ActionInclude, Pattern: "*.c"},
		},
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionExclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	if included, err := p.Included("core/main.c", false); err != nil || !included {
		t.Fatalf("Included(core/main.c)=%v err=%v, want included", included, err)
	}

	if included, err := p.Included("scripts/main.c", false); err != nil || included {
		t.Fatalf("Included(scripts/main.c)=%v err=%v, want excluded", included, err)
	}

	if included, err := p.Included("scripts/keep.c", false); err != nil || !included {
		t.Fatalf("Included(scripts/keep.c)=%v err=%v, want included", included, err)
	}

	if included, err := p.Included("README.md", false); err != nil || included {
		t.Fatalf("Included(README.md)=%v err=%v, want excluded", included, err)
	}
}

func TestProviderDeepChainCumulativeMerge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRulesFile(t, filepath.Join(root, ".pboignore"), "*.tmp\n")
	writeRulesFile(t, filepath.Join(root, "a", ".pboignore"), "*.bak\n")
	writeRulesFile(t, filepath.Join(root, "a", "b", ".pboignore"), "!keep.tmp\n")
	writeRulesFile(t, filepath.Join(root, "a", "b", "c", ".pboignore"), "*.log\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".pboignore",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	// "a/b" re-includes keep.tmp; neither "a" nor "a/b/c" touch it,
	// so the mid-chain override must survive the intervening and trailing levels.
	if included, err := p.Included("a/b/c/keep.tmp", false); err != nil || !included {
		t.Fatalf("Included(a/b/c/keep.tmp)=%v err=%v, want included by mid-chain override", included, err)
	}

	// "other.tmp" matches none of the deeper overrides,
	// so the root-level exclude must still apply even though three more directory levels
	// of rules sit between the root and the file.
	if included, err := p.Included("a/b/c/other.tmp", false); err != nil || included {
		t.Fatalf("Included(a/b/c/other.tmp)=%v err=%v, want excluded by root rule", included, err)
	}
}

func TestProviderDeepestOverlappingRuleWins(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRulesFile(t, filepath.Join(root, ".pboignore"), "file.txt\n")
	writeRulesFile(t, filepath.Join(root, "a", ".pboignore"), "!file.txt\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".pboignore",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	if included, err := p.Included("a/file.txt", false); err != nil || !included {
		t.Fatalf("Included(a/file.txt)=%v err=%v, want included by deeper override", included, err)
	}

	if included, err := p.Included("file.txt", false); err != nil || included {
		t.Fatalf("Included(file.txt)=%v err=%v, want excluded by root rule", included, err)
	}
}

func TestProviderBaseRulesPersistThroughMultipleDirectoryLevels(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRulesFile(t, filepath.Join(root, ".rules"), "*.tmp\n")
	writeRulesFile(t, filepath.Join(root, "sub", ".rules"), "!keep.tmp\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".rules",
		BaseRules: []Rule{
			{Action: ActionExclude, Pattern: "*.log"},
		},
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	// BaseRules exclude *.log; neither directory level's rules touch it,
	// so the base decision must survive through the "sub" directory's rules file.
	if included, err := p.Included("sub/app.log", false); err != nil || included {
		t.Fatalf("Included(sub/app.log)=%v err=%v, want excluded by base rules", included, err)
	}

	// Root excludes *.tmp, "sub" re-includes keep.tmp specifically:
	// deepest directory override still applies on top of BaseRules.
	if included, err := p.Included("sub/keep.tmp", false); err != nil || !included {
		t.Fatalf("Included(sub/keep.tmp)=%v err=%v, want included by deepest override", included, err)
	}
}

func TestProviderWithoutBaseRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRulesFile(t, filepath.Join(root, ".rules"), "*.tmp\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".rules",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	if included, err := p.Included("a.tmp", false); err != nil || included {
		t.Fatalf("Included(a.tmp)=%v err=%v, want excluded", included, err)
	}

	if included, err := p.Included("a.txt", false); err != nil || !included {
		t.Fatalf("Included(a.txt)=%v err=%v, want included", included, err)
	}

	results, err := p.DecideInDir("", []DirEntry{{Name: "a.tmp"}, {Name: "a.txt"}})
	if err != nil {
		t.Fatalf("DecideInDir: %v", err)
	}

	if results[0].Included || !results[1].Included {
		t.Fatalf("unexpected DecideInDir results without base rules: %+v", results)
	}
}

func TestProviderRejectsTraversalPaths(t *testing.T) {
	t.Parallel()

	p, err := NewProvider(t.TempDir(), ProviderOptions{})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	cases := []string{
		"",
		"../a.txt",
		"/etc/passwd",
		"a/../b.txt",
	}

	for _, path := range cases {
		_, err := p.Decide(path, false)
		if !errors.Is(err, ErrPathOutsideRoot) {
			t.Fatalf("Decide(%q) err=%v, want ErrPathOutsideRoot", path, err)
		}
	}
}

func TestProviderCachesDirectoryMatchers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rulesPath := filepath.Join(root, ".rules")
	writeRulesFile(t, rulesPath, "*.tmp\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".rules",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	if included, err := p.Included("a.tmp", false); err != nil || included {
		t.Fatalf("Included(a.tmp)=%v err=%v, want excluded", included, err)
	}

	if err := os.Remove(rulesPath); err != nil {
		t.Fatalf("Remove rules file: %v", err)
	}

	// Cached matcher should still be used after the source file is removed.
	if included, err := p.Included("b.tmp", false); err != nil || included {
		t.Fatalf("Included(b.tmp)=%v err=%v, want excluded", included, err)
	}
}

func TestProviderDecideInDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRulesFile(t, filepath.Join(root, ".pboignore"), "*.tmp\n")
	writeRulesFile(t, filepath.Join(root, "textures", ".pboignore"), "!*.tmp\n")

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".pboignore",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	results, err := p.DecideInDir("textures", []DirEntry{
		{Name: "a.tmp", IsDir: false},
		{Name: "b.txt", IsDir: false},
	})
	if err != nil {
		t.Fatalf("DecideInDir: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("len(results)=%d, want 2", len(results))
	}

	if !results[0].Included {
		t.Fatalf("textures/a.tmp must be included by local override")
	}

	if !results[1].Included {
		t.Fatalf("textures/b.txt must stay included by default")
	}
}

func TestProviderDecideInDirRejectsInvalidEntry(t *testing.T) {
	t.Parallel()

	p, err := NewProvider(t.TempDir(), ProviderOptions{})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	_, err = p.DecideInDir("", []DirEntry{
		{Name: "ok.txt"},
		{Name: "../bad.txt"},
	})
	if !errors.Is(err, ErrInvalidEntryName) {
		t.Fatalf("DecideInDir err=%v, want ErrInvalidEntryName", err)
	}
}

func TestProviderRejectsInvalidRulesFileName(t *testing.T) {
	t.Parallel()

	_, err := NewProvider(t.TempDir(), ProviderOptions{
		RulesFileName: "../outside.rules",
	})
	if !errors.Is(err, ErrInvalidRulesFileName) {
		t.Fatalf("NewProvider err=%v, want ErrInvalidRulesFileName", err)
	}
}

func TestProviderAllowsRulesSymlinkEscapeByDefault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	writeRulesFile(t, filepath.Join(outside, ".rules"), "*.tmp\n")

	linkPath := filepath.Join(root, "linked")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".rules",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	included, err := p.Included("linked/file.tmp", false)
	if err != nil {
		t.Fatalf("Included err=%v, want nil", err)
	}

	if included {
		t.Fatalf("linked/file.tmp must be excluded by linked rules when check is disabled")
	}
}

func TestProviderRejectsRulesSymlinkEscapeWhenEnabled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	writeRulesFile(t, filepath.Join(outside, ".rules"), "*.tmp\n")

	linkPath := filepath.Join(root, "linked")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName:            ".rules",
		EnableSymlinkEscapeCheck: true,
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	_, err = p.Decide("linked/file.tmp", false)
	if !errors.Is(err, ErrRulesPathOutsideRoot) {
		t.Fatalf("Decide err=%v, want ErrRulesPathOutsideRoot", err)
	}
}

func writeRulesFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	benchRuleCount  = 96
	benchPathCount  = 512
	benchDirEntries = 256
)

var (
	benchDecisionSink MatchResult
	benchCountSink    int
)

func BenchmarkParseRules(b *testing.B) {
	src := buildBenchmarkRulesSource(benchRuleCount)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rules, err := ParseRulesString(src)
		if err != nil {
			b.Fatal(err)
		}

		if len(rules) == 0 {
			b.Fatal("empty rules")
		}
	}
}

func BenchmarkNewMatcher(b *testing.B) {
	rules, err := ParseRulesString(buildBenchmarkRulesSource(benchRuleCount))
	if err != nil {
		b.Fatal(err)
	}

	opts := MatcherOptions{
		DefaultAction: ActionInclude,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := NewMatcher(rules, opts)
		if err != nil {
			b.Fatal(err)
		}

		if m == nil {
			b.Fatal("nil matcher")
		}
	}
}

func BenchmarkMatcherDecide(b *testing.B) {
	rules, err := ParseRulesString(buildBenchmarkRulesSource(benchRuleCount))
	if err != nil {
		b.Fatal(err)
	}

	m, err := NewMatcher(rules, MatcherOptions{
		DefaultAction: ActionInclude,
	})
	if err != nil {
		b.Fatal(err)
	}

	paths := benchmarkPaths(benchPathCount)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchDecisionSink = m.Decide(paths[i%len(paths)], false)
	}
}

func BenchmarkNewMatcherRuleStyles(b *testing.B) {
	const styleRuleCount = 100

	variants := []struct {
		name  string
		rules []Rule
	}{
		{"exact", buildExactRules(styleRuleCount)},
		{"glob", buildGlobRules(styleRuleCount)},
		{"regexp_heavy", buildRegexpHeavyRules(styleRuleCount)},
		{"extension_list", ParseExtensions(buildExtensionNames(styleRuleCount))},
	}

	for _, v := range variants {
		b.Run(v.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m, err := NewMatcher(v.rules, MatcherOptions{
					DefaultAction: ActionInclude,
				})
				if err != nil {
					b.Fatal(err)
				}

				if m == nil {
					b.Fatal("nil matcher")
				}
			}
		})
	}
}

func BenchmarkMatcherDecideByPosition(b *testing.B) {
	scenarios := []struct {
		name string
		path string
	}{
		{"match_first", "match_first.bin"},
		{"match_middle", "match_middle.bin"},
		{"match_last", "match_last.bin"},
		{"no_match", "no_match.bin"},
	}

	for _, ruleCount := range []int{10, 100, 1000} {
		m, err := NewMatcher(buildPositionalDecideRules(ruleCount), MatcherOptions{
			DefaultAction: ActionInclude,
		})
		if err != nil {
			b.Fatal(err)
		}

		for _, sc := range scenarios {
			b.Run(fmt.Sprintf("rules_%d/%s", ruleCount, sc.name), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchDecisionSink = m.Decide(sc.path, false)
				}
			})
		}
	}
}

func BenchmarkProviderDecideCached(b *testing.B) {
	root := b.TempDir()
	prepareProviderBenchTree(b, root)

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".pboignore",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		b.Fatal(err)
	}

	paths := benchmarkPaths(benchPathCount)

	// Warm provider cache before timed loop.
	for i := 0; i < len(paths) && i < 64; i++ {
		_, _ = p.Decide(paths[i], false)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := p.Decide(paths[i%len(paths)], false)
		if err != nil {
			b.Fatal(err)
		}

		benchDecisionSink = res
	}
}

func BenchmarkProviderDecideCold(b *testing.B) {
	root := b.TempDir()
	prepareProviderBenchTree(b, root)
	paths := benchmarkPaths(benchPathCount)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := NewProvider(root, ProviderOptions{
			RulesFileName: ".pboignore",
			MatcherOptions: MatcherOptions{
				DefaultAction: ActionInclude,
			},
		})
		if err != nil {
			b.Fatal(err)
		}

		res, err := p.Decide(paths[i%len(paths)], false)
		if err != nil {
			b.Fatal(err)
		}

		benchDecisionSink = res
	}
}

func BenchmarkProviderDecideInDirBatch(b *testing.B) {
	root := b.TempDir()
	prepareProviderBenchTree(b, root)

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".pboignore",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		b.Fatal(err)
	}

	entries := benchmarkDirEntriesList(benchDirEntries)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := p.DecideInDir("assets/group_007", entries)
		if err != nil {
			b.Fatal(err)
		}

		included := 0
		for j := range results {
			if results[j].Included {
				included++
			}
		}

		benchCountSink = included
	}
}

func BenchmarkProviderDecideInDirLoop(b *testing.B) {
	root := b.TempDir()
	prepareProviderBenchTree(b, root)

	p, err := NewProvider(root, ProviderOptions{
		RulesFileName: ".pboignore",
		MatcherOptions: MatcherOptions{
			DefaultAction: ActionInclude,
		},
	})
	if err != nil {
		b.Fatal(err)
	}

	entries := benchmarkDirEntriesList(benchDirEntries)
	paths := make([]string, len(entries))
	for i := range entries {
		paths[i] = "assets/group_007/" + entries[i].Name
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		included := 0
		for j := range paths {
			res, err := p.Decide(paths[j], entries[j].IsDir)
			if err != nil {
				b.Fatal(err)
			}

			if res.Included {
				included++
			}
		}

		benchCountSink = included
	}
}

// BenchmarkProviderDecideDeepChain measures Decide cost
// for a path buried under a chain of nested directories that each carry their own rules file,
// like a large tree of layered .gitignore-style files.
// Roughly half the levels contribute a rule that actually matches on the way down,
// so the final decision is a genuine cumulative merge across the chain
// rather than only the deepest file mattering.
func BenchmarkProviderDecideDeepChain(b *testing.B) {
	for _, depth := range []int{8, 16, 32} {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			root := b.TempDir()
			leafRelDir, leafName := buildDeepChainTree(b, root, depth)
			leafRel := leafRelDir + "/" + leafName

			p, err := NewProvider(root, ProviderOptions{
				MatcherOptions: MatcherOptions{
					DefaultAction: ActionInclude,
				},
			})
			if err != nil {
				b.Fatal(err)
			}

			// Warm directory matcher cache before timed loops.
			if _, err := p.Decide(leafRel, false); err != nil {
				b.Fatal(err)
			}

			b.Run("Decide", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					res, err := p.Decide(leafRel, false)
					if err != nil {
						b.Fatal(err)
					}

					benchDecisionSink = res
				}
			})

			b.Run("DecideInDir", func(b *testing.B) {
				entries := []DirEntry{{Name: leafName}}

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					results, err := p.DecideInDir(leafRelDir, entries)
					if err != nil {
						b.Fatal(err)
					}

					benchDecisionSink = results[0]
				}
			})
		})
	}
}

// buildDeepChainTree creates a chain of depth nested directories,
// each with its own rules file, and returns the leaf's relative directory and file name.
// Even levels contribute a filler exclude rule that never matches the leaf;
// odd levels contribute a filler include rule;
// the deepest level also excludes the leaf file itself so the final decision depends
// on that last directory's rules file being reached and merged with its ancestors.
func buildDeepChainTree(b *testing.B, root string, depth int) (relDir string, leafName string) {
	b.Helper()

	const leaf = "leaf.bin"

	dir := root
	names := make([]string, 0, depth)
	for i := 0; i < depth; i++ {
		name := fmt.Sprintf("lvl%03d", i)
		names = append(names, name)
		dir = filepath.Join(dir, name)

		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}

		var rules string
		if i%2 == 0 {
			rules = fmt.Sprintf("*.tmp_%03d\n", i)
		} else {
			rules = fmt.Sprintf("!keep_%03d.bin\n*.bak_%03d\n", i, i)
		}

		if i == depth-1 {
			rules += leaf + "\n"
		}

		if err := os.WriteFile(filepath.Join(dir, defaultRulesFileName), []byte(rules), 0o600); err != nil {
			b.Fatal(err)
		}
	}

	return strings.Join(names, "/"), leaf
}

func BenchmarkNewProviderBaseRules(b *testing.B) {
	root := b.TempDir()
	prepareProviderBenchTree(b, root)

	b.Run("empty", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p, err := NewProvider(root, ProviderOptions{
				RulesFileName: ".pboignore",
				MatcherOptions: MatcherOptions{
					DefaultAction: ActionInclude,
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			if p == nil {
				b.Fatal("nil provider")
			}
		}
	})

	b.Run("non_empty", func(b *testing.B) {
		baseRules := []Rule{
			{Action: ActionInclude, Pattern: "*.paa"},
			{Action: ActionExclude, Pattern: "*.tmp"},
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p, err := NewProvider(root, ProviderOptions{
				RulesFileName: ".pboignore",
				BaseRules:     baseRules,
				MatcherOptions: MatcherOptions{
					DefaultAction: ActionInclude,
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			if p == nil {
				b.Fatal("nil provider")
			}
		}
	})
}

func buildExactRules(ruleCount int) []Rule {
	rules := make([]Rule, ruleCount)
	for i := range rules {
		rules[i] = Rule{Action: ActionExclude, Pattern: fmt.Sprintf("path/to/file_%04d.bin", i)}
	}

	return rules
}

func buildGlobRules(ruleCount int) []Rule {
	rules := make([]Rule, ruleCount)
	for i := range rules {
		rules[i] = Rule{Action: ActionExclude, Pattern: fmt.Sprintf("assets/group_%03d/*.paa", i%37)}
	}

	return rules
}

func buildRegexpHeavyRules(ruleCount int) []Rule {
	rules := make([]Rule, ruleCount)
	for i := range rules {
		if i%2 == 0 {
			rules[i] = Rule{Action: ActionExclude, Pattern: fmt.Sprintf("data/file_%03d_[0-9].bin", i%53)}
		} else {
			rules[i] = Rule{Action: ActionInclude, Pattern: fmt.Sprintf("assets/**/group_%03d/*.paa", i%29)}
		}
	}

	return rules
}

func buildExtensionNames(count int) []string {
	exts := make([]string, count)
	for i := range exts {
		exts[i] = fmt.Sprintf("ext%03d", i)
	}

	return exts
}

// buildPositionalDecideRules builds ruleCount exact-match filler rules
// plus three distinct target rules at the first, middle, and last index,
// so Decide benchmarks can isolate the cost of matching near either end
// of the compiled rule slice.
func buildPositionalDecideRules(ruleCount int) []Rule {
	rules := make([]Rule, ruleCount)
	for i := range rules {
		rules[i] = Rule{Action: ActionExclude, Pattern: fmt.Sprintf("filler_%04d.bin", i)}
	}

	rules[0] = Rule{Action: ActionExclude, Pattern: "match_first.bin"}
	rules[ruleCount/2] = Rule{Action: ActionExclude, Pattern: "match_middle.bin"}
	rules[ruleCount-1] = Rule{Action: ActionExclude, Pattern: "match_last.bin"}

	return rules
}

func buildBenchmarkRulesSource(ruleCount int) string {
	var sb strings.Builder
	sb.Grow(ruleCount * 18)

	sb.WriteString("# bench rules\n")
	sb.WriteString("*.tmp\n")
	sb.WriteString("!keep.tmp\n")

	for i := 0; i < ruleCount; i++ {
		switch i % 6 {
		case 0:
			_, _ = fmt.Fprintf(&sb, "assets/group_%03d/**\n", i%37)
		case 1:
			_, _ = fmt.Fprintf(&sb, "!assets/group_%03d/keep_*.paa\n", i%37)
		case 2:
			_, _ = fmt.Fprintf(&sb, "/scripts/module_%03d/*.c\n", i%71)
		case 3:
			_, _ = fmt.Fprintf(&sb, "build_%03d/\n", i%29)
		case 4:
			_, _ = fmt.Fprintf(&sb, "data/file_%03d_[0-9].bin\n", i%53)
		default:
			_, _ = fmt.Fprintf(&sb, "!docs/section_%03d/**/*.md\n", i%41)
		}
	}

	return sb.String()
}

func benchmarkPaths(pathCount int) []string {
	paths := make([]string, 0, pathCount)
	for i := 0; i < pathCount; i++ {
		switch i % 7 {
		case 0:
			paths = append(paths, fmt.Sprintf("assets/group_%03d/tex_%05d.paa", i%37, i))
		case 1:
			paths = append(paths, fmt.Sprintf("assets/group_%03d/keep_%05d.paa", i%37, i))
		case 2:
			paths = append(paths, fmt.Sprintf("scripts/module_%03d/main_%02d.c", i%71, i%19))
		case 3:
			paths = append(paths, fmt.Sprintf("build_%03d/cache_%04d.bin", i%29, i))
		case 4:
			paths = append(paths, fmt.Sprintf("data/file_%03d_%d.bin", i%53, i%10))
		case 5:
			paths = append(paths, fmt.Sprintf("docs/section_%03d/chapter_%02d/readme.md", i%41, i%17))
		default:
			paths = append(paths, fmt.Sprintf("misc/file_%05d.txt", i))
		}
	}

	return paths
}

func benchmarkDirEntriesList(entryCount int) []DirEntry {
	entries := make([]DirEntry, 0, entryCount)
	for i := 0; i < entryCount; i++ {
		name := fmt.Sprintf("tex_%05d.paa", i)
		if i%11 == 0 {
			name = fmt.Sprintf("keep_%05d.paa", i)
		}

		entries = append(entries, DirEntry{
			Name:  name,
			IsDir: false,
		})
	}

	return entries
}

func prepareProviderBenchTree(b *testing.B, root string) {
	b.Helper()

	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "section_000"), 0o755); err != nil {
		b.Fatal(err)
	}

	rootRules := "*.tmp\nbuild_*/\nassets/group_007/**\n!assets/group_007/keep_*.paa\n"
	if err := os.WriteFile(filepath.Join(root, ".pboignore"), []byte(rootRules), 0o600); err != nil {
		b.Fatal(err)
	}

	scriptsRules := "!module_010/*.c\nmodule_010/private/**\n"
	if err := os.WriteFile(filepath.Join(root, "scripts", ".pboignore"), []byte(scriptsRules), 0o600); err != nil {
		b.Fatal(err)
	}

	docsRules := "!\n# keep markdown docs\n!**/*.md\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "section_000", ".pboignore"), []byte(docsRules), 0o600); err != nil {
		b.Fatal(err)
	}
}

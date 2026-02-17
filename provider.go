// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const defaultRulesFileName = ".pathrules"

// ProviderOptions configures recursive rules provider behavior.
type ProviderOptions struct {
	// RulesFileName is the rules file loaded in each directory in the path chain.
	// Empty value defaults to ".pathrules".
	RulesFileName string `json:"rules_file_name,omitempty" yaml:"rules_file_name,omitempty"`
	// BaseRules are in-memory rules evaluated before directory-loaded rules.
	BaseRules []Rule `json:"base_rules,omitempty" yaml:"base_rules,omitempty"`
	// MatcherOptions controls rule matching behavior for all compiled matchers.
	MatcherOptions MatcherOptions `json:"matcher_options" yaml:"matcher_options"`
	// EnableSymlinkEscapeCheck enables resolved-path validation to block
	// symlink/junction escapes outside provider root.
	// Default is false for lower cold-path overhead.
	EnableSymlinkEscapeCheck bool `json:"enable_symlink_escape_check,omitempty" yaml:"enable_symlink_escape_check,omitempty"`
}

// DirEntry is one directory entry input for Provider batch APIs.
type DirEntry struct {
	// Name is one entry name relative to target directory (without path separators).
	Name string `json:"name" yaml:"name"`
	// IsDir reports whether entry path is a directory.
	IsDir bool `json:"is_dir,omitempty" yaml:"is_dir,omitempty"`
}

// Provider loads rules files along path hierarchy and evaluates final decisions.
type Provider struct {
	// baseMatcher evaluates global in-memory rules before directory rules.
	baseMatcher *Matcher
	// cache stores directory-local compiled matcher by relative directory path.
	cache map[string]*cachedDirMatcher
	// root is absolute provider root directory path.
	root string
	// resolvedRoot is provider root with symlinks/junctions resolved when possible.
	resolvedRoot string
	// rulesFileName is per-directory rules file name.
	rulesFileName string

	// mu guards cache access.
	mu sync.Mutex
	// matcherOptions are shared compilation and decision options.
	matcherOptions MatcherOptions
	// defaultIncluded is fallback decision when no rule matched anywhere.
	defaultIncluded bool
	// enableSymlinkEscapeCheck enables resolved-path root boundary validation.
	enableSymlinkEscapeCheck bool
}

// cachedDirMatcher stores one directory rules matcher or a cached load error.
type cachedDirMatcher struct {
	// matcher is nil when directory has no rules file.
	matcher *Matcher
	// err stores parse/compile error for deterministic repeated calls.
	err error
	// loading reports whether matcher is currently being loaded by another goroutine.
	loading bool
	// wg coordinates concurrent waiters for one load attempt.
	wg sync.WaitGroup
}

// providerDirMatcher is one prepared directory-level matcher with prefix.
type providerDirMatcher struct {
	// matcher evaluates rules loaded from one directory.
	matcher *Matcher
	// prefix is relative directory prefix used for candidate trimming.
	prefix string
}

// NewProvider creates a recursive rules provider rooted at rootDir.
func NewProvider(rootDir string, opts ProviderOptions) (*Provider, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("abs root: %w", err)
	}

	resolvedRoot := absRoot
	if opts.EnableSymlinkEscapeCheck {
		resolvedRoot, err = resolvePathOrAbs(absRoot)
		if err != nil {
			return nil, fmt.Errorf("resolve root: %w", err)
		}
	}

	opts.MatcherOptions.applyDefaults()

	baseMatcher, err := NewMatcher(opts.BaseRules, opts.MatcherOptions)
	if err != nil {
		return nil, fmt.Errorf("compile base rules: %w", err)
	}

	rulesFileName, err := cleanRulesFileName(opts.RulesFileName)
	if err != nil {
		return nil, err
	}

	return &Provider{
		root:                     absRoot,
		resolvedRoot:             resolvedRoot,
		rulesFileName:            rulesFileName,
		matcherOptions:           opts.MatcherOptions,
		baseMatcher:              baseMatcher,
		defaultIncluded:          opts.MatcherOptions.DefaultAction == ActionInclude,
		enableSymlinkEscapeCheck: opts.EnableSymlinkEscapeCheck,
		cache:                    make(map[string]*cachedDirMatcher),
	}, nil
}

// Decide returns final include/exclude decision for a path relative to provider root.
//
// Decision order:
// 1. BaseRules matcher.
// 2. Rules files from root to deepest containing directory.
// Last matched rule wins.
func (p *Provider) Decide(relPath string, isDir bool) (MatchResult, error) {
	if p == nil {
		return MatchResult{}, ErrNilProvider
	}

	normalized, err := cleanRelPath(relPath)
	if err != nil {
		return MatchResult{}, err
	}

	res := MatchResult{
		Included:  p.defaultIncluded,
		Matched:   false,
		RuleIndex: -1,
	}

	if p.baseMatcher != nil {
		baseRes := p.baseMatcher.Decide(normalized, isDir)
		if baseRes.Matched {
			res = baseRes
		}
	}

	relDir := pathDir(normalized, isDir)
	if err := p.applyDirMatcherDecision("", normalized, isDir, &res); err != nil {
		return MatchResult{}, err
	}

	if relDir != "" {
		for i := 0; i < len(relDir); i++ {
			if relDir[i] != '/' {
				continue
			}

			if err := p.applyDirMatcherDecision(relDir[:i], normalized, isDir, &res); err != nil {
				return MatchResult{}, err
			}
		}

		if err := p.applyDirMatcherDecision(relDir, normalized, isDir, &res); err != nil {
			return MatchResult{}, err
		}
	}

	return res, nil
}

// DecideInDir returns decisions for multiple entries from one directory.
//
// The same directory matcher chain is loaded once and reused for every entry.
func (p *Provider) DecideInDir(relDir string, entries []DirEntry) ([]MatchResult, error) {
	if p == nil {
		return nil, ErrNilProvider
	}

	normalizedDir, err := cleanRelDir(relDir)
	if err != nil {
		return nil, err
	}

	dirMatchers, err := p.prepareProviderDirMatchers(normalizedDir)
	if err != nil {
		return nil, err
	}

	results := make([]MatchResult, len(entries))
	for i := range entries {
		entryName, err := cleanEntryName(entries[i].Name)
		if err != nil {
			return nil, fmt.Errorf("entry %d (%q): %w", i, entries[i].Name, err)
		}

		fullPath := entryName
		if normalizedDir != "" {
			fullPath = normalizedDir + "/" + entryName
		}

		res := MatchResult{
			Included:  p.defaultIncluded,
			Matched:   false,
			RuleIndex: -1,
		}

		if p.baseMatcher != nil {
			baseRes := p.baseMatcher.Decide(fullPath, entries[i].IsDir)
			if baseRes.Matched {
				res = baseRes
			}
		}

		p.applyPreparedDirMatchers(dirMatchers, fullPath, entries[i].IsDir, &res)

		results[i] = res
	}

	return results, nil
}

// Included reports whether path is included by provider decision.
func (p *Provider) Included(relPath string, isDir bool) (bool, error) {
	res, err := p.Decide(relPath, isDir)
	if err != nil {
		return false, err
	}

	return res.Included, nil
}

// Excluded reports whether path is excluded by provider decision.
func (p *Provider) Excluded(relPath string, isDir bool) (bool, error) {
	included, err := p.Included(relPath, isDir)
	if err != nil {
		return false, err
	}

	return !included, nil
}

// IncludedInDir reports include decisions for multiple entries from one directory.
func (p *Provider) IncludedInDir(relDir string, entries []DirEntry) ([]bool, error) {
	results, err := p.DecideInDir(relDir, entries)
	if err != nil {
		return nil, err
	}

	included := make([]bool, len(results))
	for i := range results {
		included[i] = results[i].Included
	}

	return included, nil
}

// ExcludedInDir reports exclude decisions for multiple entries from one directory.
func (p *Provider) ExcludedInDir(relDir string, entries []DirEntry) ([]bool, error) {
	included, err := p.IncludedInDir(relDir, entries)
	if err != nil {
		return nil, err
	}

	excluded := make([]bool, len(included))
	for i := range included {
		excluded[i] = !included[i]
	}

	return excluded, nil
}

// loadDirMatcher returns cached or newly loaded matcher for one relative directory.
func (p *Provider) loadDirMatcher(relDir string) (*Matcher, error) {
	p.mu.Lock()
	cached, ok := p.cache[relDir]
	if ok {
		loading := cached.loading
		p.mu.Unlock()
		if loading {
			cached.wg.Wait()
		}

		return unwrapCachedDirMatcher(cached)
	}

	cached = &cachedDirMatcher{
		loading: true,
	}
	cached.wg.Add(1)
	p.cache[relDir] = cached
	p.mu.Unlock()

	matcher, loadErr := p.loadAndCompileDirMatcher(relDir)

	p.mu.Lock()
	cached.matcher = matcher
	cached.err = loadErr
	cached.loading = false
	cached.wg.Done()
	p.mu.Unlock()

	return matcher, loadErr
}

// loadAndCompileDirMatcher loads and compiles one directory rules file.
func (p *Provider) loadAndCompileDirMatcher(relDir string) (*Matcher, error) {
	if !p.enableSymlinkEscapeCheck {
		fullDir := filepath.Join(p.root, filepath.FromSlash(relDir))
		rulesPath := filepath.Join(fullDir, p.rulesFileName)
		content, err := os.ReadFile(rulesPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}

			return nil, fmt.Errorf("read %s: %w", rulesPath, err)
		}

		rules, err := ParseRules(bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", rulesPath, err)
		}

		matcher, err := NewMatcher(rules, p.matcherOptions)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", rulesPath, err)
		}

		return matcher, nil
	}

	rulesPath, found, err := p.resolveAndValidateRulesPath(relDir)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil
	}

	content, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rulesPath, err)
	}

	rules, err := ParseRules(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rulesPath, err)
	}

	matcher, err := NewMatcher(rules, p.matcherOptions)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", rulesPath, err)
	}

	return matcher, nil
}

// resolveAndValidateRulesPath resolves one rules file path and ensures it stays under provider root.
func (p *Provider) resolveAndValidateRulesPath(relDir string) (string, bool, error) {
	fullDir := filepath.Join(p.root, filepath.FromSlash(relDir))
	rulesPath := filepath.Join(fullDir, p.rulesFileName)

	_, err := os.Lstat(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("stat %s: %w", rulesPath, err)
	}

	resolvedRulesPath, err := resolvePathOrAbs(rulesPath)
	if err != nil {
		return "", false, fmt.Errorf("resolve %s: %w", rulesPath, err)
	}

	if !isPathWithinRoot(p.resolvedRoot, resolvedRulesPath) {
		return "", false, fmt.Errorf("%w: %s", ErrRulesPathOutsideRoot, rulesPath)
	}

	return rulesPath, true, nil
}

// prepareProviderDirMatchers loads and prepares directory-level matchers for one directory.
func (p *Provider) prepareProviderDirMatchers(relDir string) ([]providerDirMatcher, error) {
	matchers := make([]providerDirMatcher, 0, strings.Count(relDir, "/")+2)

	if matcher, err := p.loadDirMatcher(""); err != nil {
		return nil, err
	} else if matcher != nil {
		matchers = append(matchers, providerDirMatcher{
			matcher: matcher,
			prefix:  "",
		})
	}

	if relDir == "" {
		return matchers, nil
	}

	for i := 0; i < len(relDir); i++ {
		if relDir[i] != '/' {
			continue
		}

		rel := relDir[:i]
		matcher, err := p.loadDirMatcher(rel)
		if err != nil {
			return nil, err
		}

		if matcher == nil {
			continue
		}

		matchers = append(matchers, providerDirMatcher{
			matcher: matcher,
			prefix:  rel,
		})
	}

	matcher, err := p.loadDirMatcher(relDir)
	if err != nil {
		return nil, err
	}

	if matcher != nil {
		matchers = append(matchers, providerDirMatcher{
			matcher: matcher,
			prefix:  relDir,
		})
	}

	return matchers, nil
}

// applyDirMatcherDecision evaluates one directory-level matcher and updates final result.
func (p *Provider) applyDirMatcherDecision(rel string, normalized string, isDir bool, res *MatchResult) error {
	matcher, err := p.loadDirMatcher(rel)
	if err != nil {
		return err
	}

	if matcher == nil {
		return nil
	}

	candidate := normalized
	if rel != "" {
		// Rules from "dir/.pathrules" apply to paths under that directory, not to the
		// directory path itself when it is being evaluated as a directory entry.
		if normalized == rel {
			return nil
		}

		prefix := rel + "/"
		if !strings.HasPrefix(candidate, prefix) {
			return nil
		}

		candidate = candidate[len(prefix):]
	}

	decision := matcher.Decide(candidate, isDir)
	if !decision.Matched {
		return nil
	}

	res.Included = decision.Included
	res.Matched = true
	res.RuleIndex = decision.RuleIndex
	return nil
}

// applyPreparedDirMatchers evaluates prepared directory matchers and updates result.
func (p *Provider) applyPreparedDirMatchers(
	matchers []providerDirMatcher,
	normalized string,
	isDir bool,
	res *MatchResult,
) {
	for i := range matchers {
		candidate := normalized
		if matchers[i].prefix != "" {
			// Rules from "dir/.pathrules" apply to paths under that directory, not to the
			// directory path itself when it is being evaluated as a directory entry.
			if normalized == matchers[i].prefix {
				continue
			}

			prefix := matchers[i].prefix + "/"
			if !strings.HasPrefix(candidate, prefix) {
				continue
			}

			candidate = candidate[len(prefix):]
		}

		decision := matchers[i].matcher.Decide(candidate, isDir)
		if !decision.Matched {
			continue
		}

		res.Included = decision.Included
		res.Matched = true
		res.RuleIndex = decision.RuleIndex
	}
}

// unwrapCachedDirMatcher unwraps cached directory matcher entry.
func unwrapCachedDirMatcher(entry *cachedDirMatcher) (*Matcher, error) {
	if entry == nil {
		return nil, nil
	}

	if entry.err != nil {
		return nil, entry.err
	}

	return entry.matcher, nil
}

// cleanRulesFileName validates and normalizes provider rules file name.
func cleanRulesFileName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = defaultRulesFileName
	}

	if filepath.IsAbs(name) {
		return "", ErrInvalidRulesFileName
	}

	name = filepath.ToSlash(name)
	if strings.Contains(name, "/") || name == "." || name == ".." {
		return "", ErrInvalidRulesFileName
	}

	return name, nil
}

// cleanRelDir normalizes and validates provider-relative directory path.
func cleanRelDir(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "." {
		return "", nil
	}

	return cleanRelPath(trimmed)
}

// cleanEntryName normalizes and validates one directory entry name.
func cleanEntryName(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidEntryName
	}

	if filepath.IsAbs(trimmed) {
		return "", ErrInvalidEntryName
	}

	path := filepath.ToSlash(trimmed)
	if strings.Contains(path, "/") {
		return "", ErrInvalidEntryName
	}

	path = normalizePath(path)
	if path == "" || strings.Contains(path, "/") || path == "." || path == ".." {
		return "", ErrInvalidEntryName
	}

	return path, nil
}

// resolvePathOrAbs resolves symlinks/junctions and falls back to absolute path for non-link paths.
func resolvePathOrAbs(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		return "", absErr
	}

	if os.IsNotExist(err) {
		return abs, nil
	}

	return "", err
}

// isPathWithinRoot reports whether target path is inside root path.
func isPathWithinRoot(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}

	if rel == "." {
		return true
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	return true
}

// cleanRelPath normalizes and validates one provider-relative path.
func cleanRelPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrPathOutsideRoot
	}

	if filepath.IsAbs(trimmed) {
		return "", ErrPathOutsideRoot
	}

	path := filepath.ToSlash(trimmed)
	if strings.HasPrefix(path, "/") {
		return "", ErrPathOutsideRoot
	}

	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "", ErrPathOutsideRoot
	}

	if path == "." || path == ".." || strings.HasPrefix(path, "../") {
		return "", ErrPathOutsideRoot
	}

	path = strings.ReplaceAll(path, "/./", "/")
	if after, ok := strings.CutPrefix(path, "./"); ok {
		path = after
	}

	if strings.Contains(path, "/../") || strings.HasSuffix(path, "/..") {
		return "", ErrPathOutsideRoot
	}

	path = normalizePath(path)
	if path == "" {
		return "", ErrPathOutsideRoot
	}

	return path, nil
}

// pathDir returns slash-separated directory part for a relative path.
func pathDir(relPath string, isDir bool) string {
	if isDir {
		return relPath
	}

	if i := strings.LastIndexByte(relPath, '/'); i >= 0 {
		return relPath[:i]
	}

	return ""
}

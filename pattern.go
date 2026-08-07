// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"fmt"
	"regexp"
	"strings"
)

// compileStrategy is the precompiled matching strategy for one pattern alternative:
// the cheapest matching approach that preserves expected gitignore-like semantics for that pattern text.
type compileStrategy struct {
	// componentRE matches basename/component patterns without slash in source.
	componentRE *regexp.Regexp
	// pathRE matches full path patterns.
	pathRE *regexp.Regexp
	// pathDirRE matches full path patterns targeting a directory subtree.
	pathDirRE *regexp.Regexp
	// componentExact matches basename/component patterns without glob meta.
	componentExact string
	// pathExact matches full path patterns without glob meta.
	pathExact string
	// componentGlob matches component patterns with "*" and "?" without regexp.
	componentGlob segmentPattern
	// pathSegments matches slash patterns without "**" and char-classes.
	pathSegments []segmentPattern
	// pathPrefixSegments matches slash patterns with trailing "/**".
	pathPrefixSegments []segmentPattern
	// hasSlash means pattern contains "/" after normalization.
	hasSlash bool
}

// compiledRule is matcher-internal compiled representation of one rule.
type compiledRule struct {
	// alternates holds one strategy per brace-expansion alternative when the
	// source pattern expanded into more than one; unused (nil) otherwise.
	alternates []compileStrategy
	// source is original source rule.
	source Rule
	// compileStrategy is the sole matching strategy, used when alternates is nil.
	compileStrategy
	// anchored means source pattern starts with "/".
	anchored bool
	// dirOnly means source pattern ends with "/".
	dirOnly bool
}

// segmentPattern is precompiled component/path segment matcher.
type segmentPattern struct {
	// text is raw segment pattern source.
	text string
	// wildcard reports whether text contains "*" or "?".
	wildcard bool
}

// compileRule compiles one source rule into the cheapest matching strategy
// (or, with brace expansion, strategies)
// that preserves expected gitignore-like semantics.
func compileRule(rule Rule, opts MatcherOptions) (compiledRule, error) {
	if !rule.Action.valid() {
		return compiledRule{}, fmt.Errorf("%w: unsupported action %d", ErrInvalidRule, rule.Action)
	}

	pattern := normalizePattern(rule.Pattern, opts.EnableEscaping)
	if opts.CaseInsensitive {
		pattern = asciiLower(pattern)
	}

	if pattern == "" {
		return compiledRule{}, fmt.Errorf("%w: empty", ErrInvalidPattern)
	}

	cr := compiledRule{
		source:   rule,
		anchored: strings.HasPrefix(pattern, "/"),
		dirOnly:  strings.HasSuffix(pattern, "/"),
	}

	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	pattern = strings.Trim(pattern, "/")
	if pattern == "" {
		return compiledRule{}, fmt.Errorf("%w: empty after normalization (%q)", ErrInvalidPattern, rule.Pattern)
	}

	patterns := []string{pattern}
	if opts.EnableBraceExpansion && strings.IndexByte(pattern, '{') >= 0 {
		expanded, err := expandBraces(pattern, opts.EnableEscaping)
		if err != nil {
			return compiledRule{}, fmt.Errorf("pattern %q: %w", rule.Pattern, err)
		}

		patterns = expanded
	}

	if len(patterns) == 1 {
		strat, err := compilePatternStrategy(patterns[0], rule.Pattern, cr.anchored, cr.dirOnly, opts.EnableEscaping)
		if err != nil {
			return compiledRule{}, err
		}

		cr.compileStrategy = strat
		return cr, nil
	}

	cr.alternates = make([]compileStrategy, 0, len(patterns))
	for _, p := range patterns {
		if p == "" {
			return compiledRule{}, fmt.Errorf("%w: brace expansion of %q produced an empty alternative",
				ErrInvalidPattern, rule.Pattern)
		}

		strat, err := compilePatternStrategy(p, rule.Pattern, cr.anchored, cr.dirOnly, opts.EnableEscaping)
		if err != nil {
			return compiledRule{}, err
		}

		cr.alternates = append(cr.alternates, strat)
	}

	return cr, nil
}

// compilePatternStrategy compiles one pattern alternative
// (already trimmed of leading/trailing "/") into the cheapest matching strategy
// that preserves expected gitignore-like semantics.
// anchored and dirOnly are rule-level (shared across all alternatives of the same rule);
// origPattern is the original, pre-expansion rule pattern, used only for error messages.
func compilePatternStrategy(pattern, origPattern string, anchored, dirOnly, escaping bool) (compileStrategy, error) {
	var s compileStrategy

	// Anchored patterns ("/name") must be matched against full path from root
	// even when they do not contain an explicit slash after normalization.
	s.hasSlash = strings.Contains(pattern, "/") || anchored

	// A pattern containing a backslash escape cannot use the byte-fast paths below
	// (none of them know that a metacharacter following "\" is literal),
	// so it always goes through the escape-aware regexp fallback.
	forceRegex := escaping && strings.IndexByte(pattern, '\\') >= 0

	if !forceRegex {
		hasMeta := patternHasGlobMeta(pattern)
		hasCharClass := patternHasCharClass(pattern)

		if !s.hasSlash {
			// Component-only rules can avoid regexp completely for exact and simple wildcard cases.
			if !hasMeta {
				s.componentExact = pattern
				return s, nil
			}

			if !hasCharClass {
				s.componentGlob = newSegmentPattern(pattern)
				return s, nil
			}
		} else {
			// Path rules get similar fast paths first: exact match, then segmented wildcard matching.
			if !hasMeta {
				s.pathExact = pattern
				return s, nil
			}

			if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
				// Trailing "/**" is common and can be matched as "prefix directory + any descendants".
				if prefix != "" && canUseSimplePathSegments(prefix) {
					s.pathPrefixSegments = compilePathSegments(prefix)
					return s, nil
				}
			}

			if canUseSimplePathSegments(pattern) {
				s.pathSegments = compilePathSegments(pattern)
				return s, nil
			}
		}
	}

	// Fallback for patterns with char classes, complex "**" combinations,
	// or (with EnableEscaping) a backslash escape.
	if !s.hasSlash {
		re, err := regexp.Compile("^" + globToRegexComponent(pattern, forceRegex) + "$")
		if err != nil {
			return compileStrategy{}, fmt.Errorf("%w: compile component %q: %v", ErrInvalidPattern, origPattern, err)
		}

		s.componentRE = re
		return s, nil
	}

	body := globToRegexPath(pattern, forceRegex)
	rePrefix := `(?:^|.*/)`
	if anchored {
		rePrefix = `^`
	}

	if dirOnly {
		re, err := regexp.Compile(rePrefix + body + `(?:/.*)?$`)
		if err != nil {
			return compileStrategy{}, fmt.Errorf("%w: compile dir pattern %q: %v", ErrInvalidPattern, origPattern, err)
		}

		s.pathDirRE = re
		return s, nil
	}

	re, err := regexp.Compile(rePrefix + body + `$`)
	if err != nil {
		return compileStrategy{}, fmt.Errorf("%w: compile path pattern %q: %v", ErrInvalidPattern, origPattern, err)
	}

	s.pathRE = re
	return s, nil
}

// matches reports whether compiled rule matches normalized candidate path.
func (r *compiledRule) matches(candidate string, isDir bool) bool {
	if candidate == "" {
		return false
	}

	if len(r.alternates) > 0 {
		// Brace-expanded rule: matched if any alternative matches.
		for i := range r.alternates {
			if r.alternates[i].matches(candidate, isDir, r.anchored, r.dirOnly) {
				return true
			}
		}

		return false
	}

	return r.compileStrategy.matches(candidate, isDir, r.anchored, r.dirOnly)
}

// matches reports whether one compiled pattern strategy matches candidate,
// given the rule-level anchored/dirOnly flags shared
// across all of a rule's brace-expansion alternatives.
func (s *compileStrategy) matches(candidate string, isDir, anchored, dirOnly bool) bool {
	if candidate == "" {
		return false
	}

	if s.hasSlash {
		// Path strategy priority mirrors compile-time selection: exact -> fast segmented -> regexp.
		if s.pathExact != "" {
			return matchExactPathRule(s.pathExact, candidate, isDir, anchored, dirOnly)
		}

		if len(s.pathPrefixSegments) > 0 {
			return matchPathPrefixDoubleStar(s.pathPrefixSegments, candidate, anchored)
		}

		if len(s.pathSegments) > 0 {
			return matchPathSegments(s.pathSegments, candidate, anchored, dirOnly)
		}

		if dirOnly {
			return s.pathDirRE != nil && s.pathDirRE.MatchString(candidate)
		}

		return s.pathRE != nil && s.pathRE.MatchString(candidate)
	}

	// Component strategy priority mirrors compile-time selection too.
	if s.componentExact != "" {
		if !dirOnly {
			return pathBase(candidate) == s.componentExact
		}

		return matchDirOnlyComponentExact(s.componentExact, candidate, isDir)
	}

	if s.componentGlob.text != "" {
		if !dirOnly {
			return matchSegmentPattern(s.componentGlob, pathBase(candidate))
		}

		return matchDirOnlyComponentPattern(s.componentGlob, candidate, isDir)
	}

	if s.componentRE == nil {
		return false
	}

	if !dirOnly {
		return s.componentRE.MatchString(pathBase(candidate))
	}

	return matchDirOnlyComponent(s.componentRE, candidate, isDir)
}

// patternHasGlobMeta reports whether pattern contains supported glob meta.
func patternHasGlobMeta(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?':
			return true
		case '[':
			if findCharClassEnd(pattern, i) >= 0 {
				return true
			}
		}
	}

	return false
}

// patternHasCharClass reports whether pattern contains at least one valid "[...]" class.
func patternHasCharClass(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '[' {
			continue
		}

		if findCharClassEnd(pattern, i) >= 0 {
			return true
		}
	}

	return false
}

// canUseSimplePathSegments reports whether slash pattern can use lightweight segment matching.
func canUseSimplePathSegments(pattern string) bool {
	if pattern == "" {
		return false
	}

	if strings.Contains(pattern, "**") {
		return false
	}

	return !patternHasCharClass(pattern)
}

// newSegmentPattern precompiles one segment pattern.
func newSegmentPattern(pattern string) segmentPattern {
	return segmentPattern{
		text:     pattern,
		wildcard: strings.ContainsAny(pattern, "*?"),
	}
}

// compilePathSegments precompiles slash-separated path pattern segments.
func compilePathSegments(pattern string) []segmentPattern {
	segments := make([]segmentPattern, 0, strings.Count(pattern, "/")+1)
	start := 0

	for i := 0; i <= len(pattern); i++ {
		if i != len(pattern) && pattern[i] != '/' {
			continue
		}

		segments = append(segments, newSegmentPattern(pattern[start:i]))
		start = i + 1
	}

	return segments
}

// matchSegmentPattern matches one precompiled segment pattern.
func matchSegmentPattern(pattern segmentPattern, segment string) bool {
	if !pattern.wildcard {
		return segment == pattern.text
	}

	return matchSimpleWildcard(pattern.text, segment)
}

// matchSimpleWildcard matches "*" and "?" wildcard pattern against one segment.
func matchSimpleWildcard(pattern string, input string) bool {
	pIdx := 0
	sIdx := 0
	starPattern := -1
	starInput := 0

	for sIdx < len(input) {
		if pIdx < len(pattern) && (pattern[pIdx] == '?' || pattern[pIdx] == input[sIdx]) {
			pIdx++
			sIdx++
			continue
		}

		if pIdx < len(pattern) && pattern[pIdx] == '*' {
			// Remember star position and continue greedily from current input index.
			starPattern = pIdx
			pIdx++
			starInput = sIdx
			continue
		}

		if starPattern >= 0 {
			// Mismatch after a previous star: backtrack pattern to token after '*'
			// and let '*' consume one more input byte.
			pIdx = starPattern + 1
			starInput++
			sIdx = starInput
			continue
		}

		return false
	}

	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}

	return pIdx == len(pattern)
}

// matchPathSegments matches slash patterns without "**" and char-classes.
func matchPathSegments(pattern []segmentPattern, candidate string, anchored bool, dirOnly bool) bool {
	if len(pattern) == 0 || candidate == "" {
		return false
	}

	if anchored {
		end, ok := matchPathSegmentsAt(pattern, candidate, 0)
		if !ok {
			return false
		}

		if dirOnly {
			return end == len(candidate) || (end < len(candidate) && candidate[end] == '/')
		}

		return end == len(candidate)
	}

	return matchPathSegmentsUnanchored(pattern, candidate, dirOnly)
}

// matchPathSegmentsUnanchored matches unanchored path segments from any segment boundary.
func matchPathSegmentsUnanchored(pattern []segmentPattern, candidate string, dirOnly bool) bool {
	for start := 0; ; {
		end, ok := matchPathSegmentsAt(pattern, candidate, start)
		if ok {
			if dirOnly {
				if end == len(candidate) || (end < len(candidate) && candidate[end] == '/') {
					return true
				}
			} else if end == len(candidate) {
				return true
			}
		}

		nextSlash := strings.IndexByte(candidate[start:], '/')
		if nextSlash < 0 {
			return false
		}

		// Shift to next segment boundary and retry, emulating "(^|.*/)" prefix.
		start += nextSlash + 1
	}
}

// matchPathSegmentsAt matches precompiled path segments starting at candidate boundary index.
func matchPathSegmentsAt(pattern []segmentPattern, candidate string, start int) (int, bool) {
	if start < 0 || start >= len(candidate) {
		return 0, false
	}

	index := start
	for seg := range pattern {
		end := index
		for end < len(candidate) && candidate[end] != '/' {
			end++
		}

		if end == index {
			return 0, false
		}

		if !matchSegmentPattern(pattern[seg], candidate[index:end]) {
			return 0, false
		}

		index = end
		if seg == len(pattern)-1 {
			// Return end position to let caller validate terminal constraints
			// (full match vs directory-subtree match).
			return index, true
		}

		if index >= len(candidate) || candidate[index] != '/' {
			return 0, false
		}

		index++
	}

	return index, true
}

// matchPathPrefixDoubleStar matches path pattern with trailing "/**".
func matchPathPrefixDoubleStar(prefix []segmentPattern, candidate string, anchored bool) bool {
	if len(prefix) == 0 || candidate == "" {
		return false
	}

	if anchored {
		end, ok := matchPathSegmentsAt(prefix, candidate, 0)
		// "/prefix/**" should match descendants only; exact directory alone does not match.
		return ok && end < len(candidate) && candidate[end] == '/'
	}

	for start := 0; ; {
		end, ok := matchPathSegmentsAt(prefix, candidate, start)
		if ok && end < len(candidate) && candidate[end] == '/' {
			return true
		}

		nextSlash := strings.IndexByte(candidate[start:], '/')
		if nextSlash < 0 {
			return false
		}

		start += nextSlash + 1
	}
}

// matchExactPathRule matches slash-containing literal pattern without regexp.
func matchExactPathRule(pattern string, candidate string, isDir bool, anchored bool, dirOnly bool) bool {
	if pattern == "" || candidate == "" {
		return false
	}

	if anchored {
		if !dirOnly {
			return candidate == pattern
		}

		return candidate == pattern || strings.HasPrefix(candidate, pattern+"/")
	}

	if !dirOnly {
		return candidate == pattern || strings.HasSuffix(candidate, "/"+pattern)
	}

	return containsDirPath(pattern, candidate, isDir)
}

// containsDirPath reports whether candidate contains pattern as directory path segment.
func containsDirPath(pattern string, candidate string, isDir bool) bool {
	for start := 0; start < len(candidate); {
		idx := strings.Index(candidate[start:], pattern)
		if idx < 0 {
			return false
		}

		idx += start
		beforeOK := idx == 0 || candidate[idx-1] == '/'
		after := idx + len(pattern)
		afterOK := after == len(candidate) || candidate[after] == '/'
		if beforeOK && afterOK {
			if after < len(candidate) {
				return true
			}

			if isDir {
				return true
			}
		}

		start = idx + 1
	}

	return false
}

// matchDirOnlyComponentExact matches dir-only component literal without regexp.
func matchDirOnlyComponentExact(component string, candidate string, isDir bool) bool {
	if component == "" || candidate == "" {
		return false
	}

	start := 0
	for i := 0; i <= len(candidate); i++ {
		if i != len(candidate) && candidate[i] != '/' {
			continue
		}

		if i > start {
			// For file paths, skip the last component (basename).
			if i == len(candidate) && !isDir {
				return false
			}

			if candidate[start:i] == component {
				return true
			}
		}

		start = i + 1
	}

	return false
}

// matchDirOnlyComponentPattern matches dir-only component wildcard pattern without regexp.
func matchDirOnlyComponentPattern(pattern segmentPattern, candidate string, isDir bool) bool {
	if pattern.text == "" || candidate == "" {
		return false
	}

	start := 0
	for i := 0; i <= len(candidate); i++ {
		if i != len(candidate) && candidate[i] != '/' {
			continue
		}

		if i > start {
			// For file paths, skip the last component (basename).
			if i == len(candidate) && !isDir {
				return false
			}

			if matchSegmentPattern(pattern, candidate[start:i]) {
				return true
			}
		}

		start = i + 1
	}

	return false
}

// globToRegexComponent converts a gitignore-like component pattern to regex body.
// When escaping is true, "\X" emits a literal X instead of letting X take on its usual glob meaning.
func globToRegexComponent(pat string, escaping bool) string {
	var b strings.Builder

	for i := 0; i < len(pat); i++ {
		if next, ok := appendEscapedLiteralRegex(pat, i, escaping, &b); ok {
			i = next
			continue
		}

		if next, ok := appendCharClassRegex(pat, i, &b); ok {
			i = next
			continue
		}

		c := pat[i]
		switch c {
		case '*':
			// Treat ** as * when matching a single path component.
			if i+1 < len(pat) && pat[i+1] == '*' {
				i++
			}
			b.WriteString(`[^/]*`)
		case '?':
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexEscapeByte(c))
		}
	}

	return b.String()
}

// globToRegexPath converts a gitignore-like path pattern to regex body.
// When escaping is true, "\X" emits a literal X instead of letting X take on its usual glob meaning.
func globToRegexPath(pat string, escaping bool) string {
	var b strings.Builder

	for i := 0; i < len(pat); i++ {
		if next, ok := appendEscapedLiteralRegex(pat, i, escaping, &b); ok {
			i = next
			continue
		}

		// Handle "**/" so it can match zero or more directories.
		if pat[i] == '*' && i+2 < len(pat) && pat[i+1] == '*' && pat[i+2] == '/' {
			b.WriteString(`(?:.*/)?`)
			i += 2
			continue
		}

		if next, ok := appendCharClassRegex(pat, i, &b); ok {
			i = next
			continue
		}

		c := pat[i]
		switch c {
		case '*':
			if i+1 < len(pat) && pat[i+1] == '*' {
				b.WriteString(`.*`)
				i++
				continue
			}
			b.WriteString(`[^/]*`)
		case '?':
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexEscapeByte(c))
		}
	}

	return b.String()
}

// appendEscapedLiteralRegex appends the regex-escaped literal for a "\X" escape sequence
// at pat[start] and reports the index of its last consumed byte.
// Returns ok=false (nothing appended) when escaping is disabled
// or pat[start] is not a backslash, leaving the byte for normal glob handling.
func appendEscapedLiteralRegex(pat string, start int, escaping bool, b *strings.Builder) (int, bool) {
	if !escaping || start >= len(pat) || pat[start] != '\\' {
		return start, false
	}

	if start+1 < len(pat) {
		b.WriteString(regexEscapeByte(pat[start+1]))
		return start + 1, true
	}

	// Trailing lone backslash: no character to escape, so it is itself literal.
	b.WriteString(regexEscapeByte('\\'))
	return start, true
}

// appendCharClassRegex appends a parsed glob char class (`[...]`) as regex class.
func appendCharClassRegex(pat string, start int, b *strings.Builder) (int, bool) {
	if start < 0 || start >= len(pat) || pat[start] != '[' {
		return start, false
	}

	end := findCharClassEnd(pat, start)
	if end < 0 {
		return start, false
	}

	b.WriteByte('[')

	idx := start + 1
	if idx < end && pat[idx] == '!' {
		// gitignore-style class negation "[!x]" maps to regex "[^x]".
		b.WriteByte('^')
		idx++
	} else if idx < end && pat[idx] == '^' {
		// Literal leading '^' must be escaped in regex char class.
		b.WriteString(`\^`)
		idx++
	}

	if idx < end && pat[idx] == ']' {
		// Leading ']' is treated as literal in both glob and regex classes.
		b.WriteByte(']')
		idx++
	}

	for ; idx < end; idx++ {
		if pat[idx] == '\\' {
			b.WriteString(`\\`)
			continue
		}

		b.WriteByte(pat[idx])
	}

	b.WriteByte(']')
	return end, true
}

// findCharClassEnd locates closing bracket for a glob char class.
func findCharClassEnd(pat string, start int) int {
	if start < 0 || start >= len(pat) || pat[start] != '[' {
		return -1
	}

	idx := start + 1
	if idx < len(pat) && (pat[idx] == '!' || pat[idx] == '^') {
		idx++
	}

	if idx < len(pat) && pat[idx] == ']' {
		idx++
	}

	for ; idx < len(pat); idx++ {
		if pat[idx] == ']' {
			return idx
		}
	}

	return -1
}

// regexEscapeByte escapes one byte for regexp source.
// "*" and "?" are included because appendEscapedLiteralRegex (an EnableEscaping "\X" escape)
// can call this for any byte, including glob wildcards forced literal;
// globToRegexComponent/globToRegexPath's own switch always intercepts
// an unescaped "*"/"?" before it would otherwise reach here.
func regexEscapeByte(c byte) string {
	switch c {
	case '.', '+', '*', '?', '(', ')', '|', '{', '}', '[', ']', '^', '$', '\\':
		return `\` + string(c)
	default:
		return string(c)
	}
}

// pathBase returns final path component using slash separator.
func pathBase(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}

	return path
}

// matchDirOnlyComponent matches component-based dir-only rule without allocating split slices.
func matchDirOnlyComponent(re *regexp.Regexp, candidate string, isDir bool) bool {
	if re == nil || candidate == "" {
		return false
	}

	start := 0
	for i := 0; i <= len(candidate); i++ {
		if i != len(candidate) && candidate[i] != '/' {
			continue
		}

		if i > start {
			// For file paths, skip the last component (basename).
			if i == len(candidate) && !isDir {
				return false
			}

			if re.MatchString(candidate[start:i]) {
				return true
			}
		}

		start = i + 1
	}

	return false
}

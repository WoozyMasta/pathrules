// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"path"
	"strings"
)

// normalizePath normalizes matching path to slash-separated relative clean form.
func normalizePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, `\`) {
		raw = strings.ReplaceAll(raw, `\`, `/`)
	}

	raw = strings.TrimPrefix(raw, "./")
	raw = strings.TrimPrefix(raw, "/")
	if raw == "" {
		return ""
	}

	// Fast path for already-normalized relative paths.
	if isSimpleNormalizedPath(raw) {
		return raw
	}

	raw = path.Clean("/" + raw)
	raw = strings.TrimPrefix(raw, "/")
	if raw == "." {
		return ""
	}

	return strings.TrimSuffix(raw, "/")
}

// normalizePattern normalizes source pattern for compilation.
// When escaping is false, backslashes are normalized to "/"
// (Windows-style path input, same as when EnableEscaping does not exist).
// When true, backslashes are left as-is for the compiler to resolve as escape sequences.
func normalizePattern(raw string, escaping bool) string {
	raw = strings.TrimSpace(raw)
	if !escaping {
		raw = strings.ReplaceAll(raw, `\`, `/`)
	}

	return raw
}

// asciiLower converts only ASCII A-Z to a-z and leaves all other bytes unchanged.
func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}

			return string(b)
		}
	}

	return s
}

// isSimpleNormalizedPath reports whether path is already normalized enough to skip path.Clean.
func isSimpleNormalizedPath(path string) bool {
	if path == "" ||
		path == "." ||
		path == ".." ||
		strings.HasPrefix(path, "/") ||
		strings.HasSuffix(path, "/") ||
		strings.HasPrefix(path, "./") ||
		strings.HasPrefix(path, "../") ||
		strings.Contains(path, "//") ||
		strings.Contains(path, "/./") ||
		strings.Contains(path, "/../") ||
		strings.HasSuffix(path, "/..") {
		return false
	}

	return true
}

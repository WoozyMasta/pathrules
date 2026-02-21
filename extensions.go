// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import "strings"

// ParseExtensions converts extension list to include rules.
//
// Accepted extension forms:
//   - "txt"
//   - ".txt"
//   - "*.txt"
//
// Empty values are skipped. Returned patterns are normalized to lower-case
// "*.ext" form and preserve input order.
func ParseExtensions(exts []string) []Rule {
	rules := make([]Rule, 0, len(exts))
	for _, ext := range exts {
		ext = strings.TrimSpace(ext)
		ext = strings.TrimPrefix(ext, "*.")
		ext = strings.TrimLeft(ext, ".")
		ext = asciiLower(ext)
		if ext == "" {
			continue
		}

		rules = append(rules, Rule{
			Action:  ActionInclude,
			Pattern: "*." + ext,
		})
	}

	return rules
}

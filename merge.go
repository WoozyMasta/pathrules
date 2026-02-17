// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

// MergeRules merges rule slices preserving input order.
func MergeRules(ruleSets ...[]Rule) []Rule {
	total := 0
	for _, set := range ruleSets {
		total += len(set)
	}

	out := make([]Rule, 0, total)
	for _, set := range ruleSets {
		out = append(out, set...)
	}

	return out
}

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import "testing"

func TestMergeRules(t *testing.T) {
	t.Parallel()

	a := []Rule{
		{Action: ActionExclude, Pattern: "*.tmp"},
	}
	b := []Rule{
		{Action: ActionInclude, Pattern: "keep.tmp"},
		{Action: ActionExclude, Pattern: "build/"},
	}

	merged := MergeRules(a, nil, b)
	if len(merged) != 3 {
		t.Fatalf("len(merged)=%d, want 3", len(merged))
	}

	if merged[0].Pattern != "*.tmp" || merged[1].Pattern != "keep.tmp" || merged[2].Pattern != "build/" {
		t.Fatalf("unexpected merged order: %+v", merged)
	}

	// Ensure result does not alias input backing arrays for appended tail.
	b[0].Pattern = "mutated"
	if merged[1].Pattern != "keep.tmp" {
		t.Fatalf("merged slice was unexpectedly aliased")
	}
}

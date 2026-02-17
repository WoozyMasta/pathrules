// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import "testing"

func TestParseRules(t *testing.T) {
	t.Parallel()

	rules, err := ParseRulesString(`
# comment
*.tmp
!keep.tmp
\#literal
\!bang
name\ 
`)
	if err != nil {
		t.Fatalf("ParseRulesString: %v", err)
	}

	if len(rules) != 5 {
		t.Fatalf("len(rules)=%d, want 5", len(rules))
	}

	if rules[0].Action != ActionExclude || rules[0].Pattern != "*.tmp" {
		t.Fatalf("rule[0]=%+v", rules[0])
	}

	if rules[1].Action != ActionInclude || rules[1].Pattern != "keep.tmp" {
		t.Fatalf("rule[1]=%+v", rules[1])
	}

	if rules[2].Action != ActionExclude || rules[2].Pattern != "#literal" {
		t.Fatalf("rule[2]=%+v", rules[2])
	}

	if rules[3].Action != ActionExclude || rules[3].Pattern != "!bang" {
		t.Fatalf("rule[3]=%+v", rules[3])
	}

	if rules[4].Action != ActionExclude || rules[4].Pattern != "name " {
		t.Fatalf("rule[4]=%+v", rules[4])
	}
}

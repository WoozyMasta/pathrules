// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRulesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".rules")
	err := os.WriteFile(path, []byte("*.tmp\n!keep.tmp\n"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rules, err := LoadRulesFile(path)
	if err != nil {
		t.Fatalf("LoadRulesFile: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("len(rules)=%d, want 2", len(rules))
	}

	if rules[0].Action != ActionExclude || rules[1].Action != ActionInclude {
		t.Fatalf("unexpected actions: %+v", rules)
	}
}

func TestLoadRulesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.rules")
	p2 := filepath.Join(dir, "b.rules")

	if err := os.WriteFile(p1, []byte("*.tmp\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", p1, err)
	}

	if err := os.WriteFile(p2, []byte("!keep.tmp\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", p2, err)
	}

	rules, err := LoadRulesFiles(p1, p2)
	if err != nil {
		t.Fatalf("LoadRulesFiles: %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("len(rules)=%d, want 2", len(rules))
	}

	if rules[0].Pattern != "*.tmp" || rules[1].Pattern != "keep.tmp" {
		t.Fatalf("unexpected merged rules: %+v", rules)
	}
}

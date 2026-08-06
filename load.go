// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"fmt"
	"os"
)

// LoadRulesFile reads and parses rules from a file according to opts.
func LoadRulesFile(path string, opts ParseOptions) ([]Rule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open rules file: %w", err)
	}
	defer func() { _ = f.Close() }()

	rules, err := ParseRules(f, opts)
	if err != nil {
		return nil, fmt.Errorf("parse rules file: %w", err)
	}

	return rules, nil
}

// LoadRulesFiles reads and merges rules from files in the given order, using opts for every file.
//
// Returned rules preserve file order and rule order inside each file.
func LoadRulesFiles(opts ParseOptions, paths ...string) ([]Rule, error) {
	out := make([]Rule, 0, len(paths)*8)
	for _, path := range paths {
		rules, err := LoadRulesFile(path, opts)
		if err != nil {
			return nil, err
		}

		out = append(out, rules...)
	}

	return out, nil
}

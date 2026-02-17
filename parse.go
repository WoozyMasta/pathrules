// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ParseRules parses gitignore-like rules from reader.
//
// Semantics:
// - blank lines and comments are ignored
// - "!" creates include rule
// - plain lines create exclude rule
// - "\#" and "\!" escape leading comment/negation tokens
func ParseRules(r io.Reader) ([]Rule, error) {
	s := bufio.NewScanner(r)
	rules := make([]Rule, 0, 16)

	for s.Scan() {
		line := strings.TrimRight(s.Text(), "\r")
		if line == "" {
			continue
		}

		line = trimTrailingSpaces(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, `\#`) {
			line = line[1:]
		}

		action := ActionExclude
		if strings.HasPrefix(line, "!") {
			action = ActionInclude
			line = line[1:]
		} else if strings.HasPrefix(line, `\!`) {
			line = line[1:]
		}

		if line == "" {
			continue
		}

		rules = append(rules, Rule{
			Action:  action,
			Pattern: line,
		})
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("scan rules: %w", err)
	}

	return rules, nil
}

// ParseRulesString parses rules from string input.
func ParseRulesString(src string) ([]Rule, error) {
	return ParseRules(strings.NewReader(src))
}

// trimTrailingSpaces removes trailing spaces unless escaped by "\".
func trimTrailingSpaces(s string) string {
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		if len(s) >= 2 && s[len(s)-2] == '\\' {
			s = s[:len(s)-2] + s[len(s)-1:]
			break
		}

		s = s[:len(s)-1]
	}

	return s
}

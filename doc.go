// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

/*
Package pathrules implements gitignore-like path matching with reusable include/exclude policies.

The package is intentionally generic and can be used for ignore workflows, allow-list workflows,
compression selection, conversion selection, and other path-based pipelines.

Basic flow:
  - parse rules from text (`ParseRules`)
  - optionally load rules from file (`LoadRulesFile`)
  - optionally build extension-based include rules (`ParseExtensions`)
  - compile matcher (`NewMatcher`)
  - ask for decision (`Decide` / `Included` / `Excluded`)

For hierarchical rule files, use `Provider`:
  - create provider with root directory and rules file name
  - evaluate paths relative to that root
  - provider caches compiled directory matchers
  - for one-directory batches use `DecideInDir` / `IncludedInDir`
  - optional symlink/junction escape hardening: `EnableSymlinkEscapeCheck` (disabled by default)
*/
package pathrules

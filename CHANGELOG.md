<!-- markdownlint-disable MD024 -->
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## Unreleased

### Changed

* `Matcher.Decide` scans compiled rules from the end
  and stops at the first match instead of always scanning all rules,
  since last-match-wins semantics guarantee the first hit found
  in reverse is already the answer: ~24% faster
  in the `MatcherDecide` benchmark, up to ~23% faster.
* Reduced allocations when compiling rules: `NewMatcher` and `NewProvider`
  (for non-empty `BaseRules`) allocate ~6-11% less,
  and `NewProvider` no longer allocates when `BaseRules` is empty.

### Added

* Fuzz tests and expanded benchmark and test coverage.

## [0.1.2][] - 2026-02-21

### Added

* `ParseExtensions` helper for converting extension lists to include rules.

[0.1.2]: https://github.com/WoozyMasta/pathrules/compare/v0.1.1...v0.1.2

## [0.1.1][] - 2026-02-18

### Fixed

* Anchored component patterns now match root-relative paths correctly:
  `/root.txt` matches `root.txt` and `/root.txt` does not match `dir/root.txt`

[0.1.1]: https://github.com/WoozyMasta/pathrules/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2026-02-18

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/pathrules/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html

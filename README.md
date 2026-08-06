# pathrules

Reusable gitignore-like path rule engine for include/exclude workflows.

## Installation

```shell
go get github.com/woozymasta/pathrules
```

## Features

* gitignore-like patterns: `*`, `?`, `**`, `**/`, `[char-class]`
* leading `/` anchored rules
* trailing `/` directory-only rules
* `!` negation support
* deterministic `last match wins`
* two policy modes:
  * ignore mode (`DefaultAction: ActionInclude`)
  * allow-list mode (`DefaultAction: ActionExclude`)

## Quick Start

```go
rules, _ := pathrules.ParseRulesString(`
*.tmp
!keep.tmp
`, pathrules.ParseOptions{})

m, _ := pathrules.NewMatcher(rules, pathrules.MatcherOptions{
    DefaultAction: pathrules.ActionInclude,
})

_ = m.Included("keep.tmp", false) // true
_ = m.Included("a.tmp", false)    // false
```

## Custom Parse Options

`ParseOptions` controls how plain and negated (`!`-prefixed) lines
are mapped to actions, and lets comment/negation prefixes be reconfigured.
The default mapping
(plain lines exclude, `!` includes) is the gitignore convention;
inverting it gives an allow-list where plain lines include and `!` excludes:

```go
rules, _ := pathrules.ParseRulesString(`
*.c
!*.tmp
`, pathrules.ParseOptions{
    PlainAction:   pathrules.ActionInclude,
    NegatedAction: pathrules.ActionExclude,
})
```

Other `ParseOptions` fields: `DisableNegation`
(treat `!` as a literal character instead of a prefix),
`CommentPrefix`/`NegationPrefix` (custom tokens instead of `#`/`!`),
`KeepTrailingSpaces` (skip trailing-space trimming).

## Recursive Provider

```go
p, _ := pathrules.NewProvider("/project", pathrules.ProviderOptions{
    RulesFileName: ".pboignore",
    BaseRules: []pathrules.Rule{
        {Action: pathrules.ActionInclude, Pattern: "*.c"},
    },
    MatcherOptions: pathrules.MatcherOptions{
        DefaultAction: pathrules.ActionExclude,
    },
})

ok, _ := p.Included("scripts/main.c", false)
_ = ok
```

`Provider` loads rules files from root to target directory,
caches compiled matchers, and applies deterministic last-match-wins.

> [!IMPORTANT]  
> for performance, reuse one `Provider` for the whole directory walk.
> Creating a new `Provider` per file forces cold path behavior on every check.

Provider hardening:

* rejects invalid `RulesFileName` values
  (path separators, absolute paths, `..`)
* optional symlink/junction escape check
  via `EnableSymlinkEscapeCheck` (disabled by default)

For one-directory batch checks, use `DecideInDir` / `IncludedInDir` and pass
entry names (`DirEntry`) instead of calling `Decide` per file.

## Extensions Helper

For workflows that configure only file extensions:

```go
exts := []string{"rvmat", ".paa", "*.ogg"}

rules := pathrules.ParseExtensions(exts)
// []Rule{
//   {Action: ActionInclude, Pattern: "*.rvmat"},
//   {Action: ActionInclude, Pattern: "*.paa"},
//   {Action: ActionInclude, Pattern: "*.ogg"},
// }
```

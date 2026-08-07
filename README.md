# pathrules

Reusable gitignore-like path rule engine for include/exclude workflows.

## Installation

```shell
go get github.com/woozymasta/pathrules
```

## Features

* gitignore-like patterns: `*`, `?`, `**`, `**/`, `[char-class]`
* optional brace alternation: `{a,b,c}` (`EnableBraceExpansion`, off by default)
* optional backslash escaping of metacharacters: `\*`, `\{`, ...
  (`EnableEscaping`, off by default, independent of `EnableBraceExpansion`)
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

## Brace Alternation

`{a,b,c}` alternation is opt-in via `MatcherOptions.EnableBraceExpansion`;
disabled by default so `{` and `}` stay literal for existing rule sets.
Each rule pattern is expanded into its alternatives at compile time
(a rule like `/CHANGELOG.{md,txt}` still counts and matches as one rule):

```go
m, _ := pathrules.NewMatcher([]pathrules.Rule{
    {Action: pathrules.ActionInclude, Pattern: "/README{,.md,.txt}"},
}, pathrules.MatcherOptions{
    DefaultAction:        pathrules.ActionExclude,
    EnableBraceExpansion: true,
})

_ = m.Included("README", false)    // true
_ = m.Included("README.md", false) // true
```

Empty alternatives (`{,.md}`) and multiple groups (cartesian product,
`{foo,bar}-{one,two}`) are supported; groups may contain `/`.
Expansion is capped at 256 alternatives per pattern.

The grammar is strict: once enabled, every `{` must open a complete,
non-nested group with at least two comma-separated alternatives
that are not all empty
(so `foo{bar}`, `foo{}`, and `foo{,}` are all compile errors, not literal text).
A literal `{` then requires `EnableEscaping`.

## Escaping

`\X` (a literal `X`) for pattern metacharacters - `\*`, `\?`, `\[`, `\]`,
`\{`, `\}`, `\,`, `\\` - is opt-in via `MatcherOptions.EnableEscaping`,
independent of `EnableBraceExpansion`.
Disabled by default, so a pattern's backslashes keep being normalized
to `/` (Windows-style path input), same as when this option does not exist:

```go
m, _ := pathrules.NewMatcher([]pathrules.Rule{
    {Action: pathrules.ActionInclude, Pattern: `file\*.txt`},
}, pathrules.MatcherOptions{
    DefaultAction:  pathrules.ActionExclude,
    EnableEscaping: true,
})

_ = m.Included("file*.txt", false) // true, literal "*"
_ = m.Included("fileA.txt", false) // false
```

Once enabled, use `/` for path separators instead of `\`.

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

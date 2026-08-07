// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestExpandBracesNoBraces(t *testing.T) {
	t.Parallel()

	for _, escaping := range []bool{false, true} {
		got, err := expandBraces("plain.txt", escaping)
		if err != nil {
			t.Fatalf("expandBraces(escaping=%v): unexpected error: %v", escaping, err)
		}

		if !reflect.DeepEqual(got, []string{"plain.txt"}) {
			t.Fatalf("expandBraces(escaping=%v) = %v, want unchanged single-element slice", escaping, got)
		}
	}
}

func TestExpandBracesSingleGroup(t *testing.T) {
	t.Parallel()

	got, err := expandBraces("*.{md,txt}", false)
	if err != nil {
		t.Fatalf("expandBraces: unexpected error: %v", err)
	}

	want := []string{"*.md", "*.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandBraces = %v, want %v", got, want)
	}
}

func TestExpandBracesEmptyAlternative(t *testing.T) {
	t.Parallel()

	got, err := expandBraces("README{,.md,.txt}", false)
	if err != nil {
		t.Fatalf("expandBraces: unexpected error: %v", err)
	}

	want := []string{"README", "README.md", "README.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandBraces = %v, want %v", got, want)
	}
}

func TestExpandBracesCartesianProduct(t *testing.T) {
	t.Parallel()

	got, err := expandBraces("{foo,bar}-{one,two}", false)
	if err != nil {
		t.Fatalf("expandBraces: unexpected error: %v", err)
	}

	// Leftmost group varies slowest, matching shell brace-expansion order.
	want := []string{"foo-one", "foo-two", "bar-one", "bar-two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandBraces = %v, want %v", got, want)
	}
}

func TestExpandBracesSlashInsideGroup(t *testing.T) {
	t.Parallel()

	got, err := expandBraces("a/{b,c}/d", false)
	if err != nil {
		t.Fatalf("expandBraces: unexpected error: %v", err)
	}

	want := []string{"a/b/d", "a/c/d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandBraces = %v, want %v", got, want)
	}
}

func TestExpandBracesNestedRejected(t *testing.T) {
	t.Parallel()

	_, err := expandBraces("foo.{md,{adoc,asciidoc}}", false)
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("expandBraces: got err=%v, want ErrInvalidPattern", err)
	}
}

func TestExpandBracesLimitExceeded(t *testing.T) {
	t.Parallel()

	group := "{a,b,c,d,e,f,g,h,i,j}"
	pattern := group + group + group // 10*10*10 = 1000 > 256

	_, err := expandBraces(pattern, false)
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("expandBraces: got err=%v, want ErrInvalidPattern", err)
	}

	if !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expandBraces: error %v should mention the limit", err)
	}
}

// The strict grammar removes the old "no comma means literal" fallback:
// any unescaped "{" must form a complete, >=2-alternative, not-all-empty group, or compilation fails.
// A literal "{" then requires escaping.
func TestExpandBracesNoCommaRejected(t *testing.T) {
	t.Parallel()

	for _, pattern := range []string{"foo{bar}.txt", "foo{}.txt"} {
		_, err := expandBraces(pattern, false)
		if !errors.Is(err, ErrInvalidPattern) {
			t.Fatalf("expandBraces(%q): got err=%v, want ErrInvalidPattern", pattern, err)
		}

		if !strings.Contains(err.Error(), "at least two alternatives") {
			t.Fatalf("expandBraces(%q): error %v should mention the two-alternative requirement", pattern, err)
		}
	}
}

func TestExpandBracesAllEmptyAlternativesRejected(t *testing.T) {
	t.Parallel()

	_, err := expandBraces("foo{,}bar", false)
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("expandBraces: got err=%v, want ErrInvalidPattern", err)
	}

	if !strings.Contains(err.Error(), "not all be empty") {
		t.Fatalf("expandBraces: error %v should mention empty alternatives", err)
	}
}

func TestExpandBracesUnterminatedRejected(t *testing.T) {
	t.Parallel()

	_, err := expandBraces("foo{bar", false)
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("expandBraces: got err=%v, want ErrInvalidPattern", err)
	}

	if !strings.Contains(err.Error(), "unterminated") {
		t.Fatalf("expandBraces: error %v should mention unterminated brace", err)
	}
}

// A "{"/"," /"}" inside a "[...]" character class is not brace syntax,
// even though it happens to contain the same bytes as a valid-looking group.
func TestExpandBracesCharClassNotMisparsed(t *testing.T) {
	t.Parallel()

	pattern := "path/[{,}]file"

	got, err := expandBraces(pattern, false)
	if err != nil {
		t.Fatalf("expandBraces(%q): unexpected error: %v", pattern, err)
	}

	if !reflect.DeepEqual(got, []string{pattern}) {
		t.Fatalf("expandBraces(%q) = %v, want unchanged single-element slice", pattern, got)
	}
}

func TestExpandBracesEscapedBraceStaysLiteral(t *testing.T) {
	t.Parallel()

	pattern := `foo\{bar\}.txt`

	got, err := expandBraces(pattern, true)
	if err != nil {
		t.Fatalf("expandBraces(%q): unexpected error: %v", pattern, err)
	}

	// No unescaped "{" found, so the raw (still-escaped) text passes through unchanged;
	// final "\X -> X" resolution happens later during pattern compilation.
	if !reflect.DeepEqual(got, []string{pattern}) {
		t.Fatalf("expandBraces(%q) = %v, want unchanged single-element slice", pattern, got)
	}
}

func TestExpandBracesEscapedCommaDoesNotSeparate(t *testing.T) {
	t.Parallel()

	// The only comma is escaped, so this group has one alternative, not two.
	_, err := expandBraces(`{a\,b}`, true)
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("expandBraces: got err=%v, want ErrInvalidPattern", err)
	}
}

func TestExpandBracesEscapedCommaWithRealGroup(t *testing.T) {
	t.Parallel()

	got, err := expandBraces(`{a\,b,c}`, true)
	if err != nil {
		t.Fatalf("expandBraces: unexpected error: %v", err)
	}

	// Two alternatives: the escaped-comma text (unresolved here) and "c".
	want := []string{`a\,b`, "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandBraces = %v, want %v", got, want)
	}
}

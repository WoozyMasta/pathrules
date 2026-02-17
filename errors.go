// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/pathrules

package pathrules

import "errors"

// Sentinel errors for pathrules operations.
var (
	// ErrInvalidRule indicates malformed or unsupported rule input.
	ErrInvalidRule = errors.New("invalid rule")
	// ErrInvalidPattern indicates malformed or unsupported rule pattern.
	ErrInvalidPattern = errors.New("invalid pattern")
	// ErrInvalidRulesFileName indicates invalid provider rules file name.
	ErrInvalidRulesFileName = errors.New("invalid rules file name")
	// ErrInvalidEntryName indicates invalid directory entry input for batch APIs.
	ErrInvalidEntryName = errors.New("invalid entry name")
	// ErrNilProvider indicates a nil Provider receiver.
	ErrNilProvider = errors.New("provider is nil")
	// ErrPathOutsideRoot indicates path traversal or non-relative input path.
	ErrPathOutsideRoot = errors.New("path is outside provider root")
	// ErrRulesPathOutsideRoot indicates resolved rules file path escaped provider root.
	ErrRulesPathOutsideRoot = errors.New("rules file path is outside provider root")
)

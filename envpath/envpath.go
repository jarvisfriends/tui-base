// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package envpath

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// varsToCollapse defines the environment variables we try to collapse in paths.
// We prioritize longer values (e.g. USERPROFILE before HOMEDRIVE) so
// the most specific match is replaced first.
var varsToCollapse = []string{
	"APPDATA",
	"LOCALAPPDATA",
	"USERPROFILE",
	"HOME",
	"TEMP",
	"TMP",
}

// Collapse takes an absolute path and replaces the prefix with an environment
// variable if a match is found. e.g., C:\Users\alice\AppData\Roaming\app -> $APPDATA/app.
func Collapse(path string) string {
	if path == "" {
		return ""
	}

	// Build a list of env vars and their values
	type envMatch struct {
		key   string
		value string
	}
	var matches []envMatch
	for _, key := range varsToCollapse {
		if val := os.Getenv(key); val != "" {
			// Normalize separators for comparison
			val = filepath.Clean(val)
			matches = append(matches, envMatch{key: key, value: val})
		}
	}

	// Sort by value length descending to match most specific prefix first
	sort.Slice(matches, func(i, j int) bool {
		return len(matches[i].value) > len(matches[j].value)
	})

	normPath := filepath.Clean(path)

	for _, match := range matches {
		if strings.HasPrefix(strings.ToLower(normPath), strings.ToLower(match.value)) {
			// Check if it's an exact match or followed by a separator
			if len(normPath) == len(match.value) {
				return "$" + match.key
			}
			if os.IsPathSeparator(normPath[len(match.value)]) {
				// Replace prefix and convert remaining backslashes to forward slashes
				// for a consistent cross-platform config representation
				remaining := normPath[len(match.value):]
				return filepath.ToSlash("$" + match.key + remaining)
			}
		}
	}

	return filepath.ToSlash(normPath)
}

// Expand takes a collapsed path (e.g. $APPDATA/app) and expands the environment
// variables to produce an absolute path.
func Expand(path string) string {
	if path == "" {
		return ""
	}
	expanded := os.ExpandEnv(path)
	return filepath.Clean(expanded)
}

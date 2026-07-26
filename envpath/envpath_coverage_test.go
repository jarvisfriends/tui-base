// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package envpath

import "testing"

// TestCollapseMatchesEnvPrefix pins the two prefix-match outcomes on
// platform-native separators: a path equal to the variable's value collapses
// to just "$VAR", and a path underneath it keeps the remainder with forward
// slashes.
func TestCollapseMatchesEnvPrefix(t *testing.T) {
	t.Setenv("HOME", "/collapse/home")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"exact match collapses to the variable", "/collapse/home", "$HOME"},
		{"child path keeps the remainder", "/collapse/home/sub/file.txt", "$HOME/sub/file.txt"},
		{"sibling prefix without separator is untouched", "/collapse/homestead", "/collapse/homestead"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Collapse(tc.input); got != tc.want {
				t.Errorf("Collapse(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

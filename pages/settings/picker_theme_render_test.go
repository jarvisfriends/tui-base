// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/jarvisfriends/tui-base/theme"
)

// TestThemeOptionRowIsAnsiLight guards the theme-picker row against re-bloating.
// huh re-parses each row's ANSI (wrap/stringWidth) on every keypress across ~300
// options, so a row dense with escape sequences reintroduces the scroll lag that
// dropping per-dot backgrounds and trimming the swatch fixed. Keep the escape
// count modest.
func TestThemeOptionRowIsAnsiLight(t *testing.T) {
	theme.EnsureRegistry()
	base := theme.Active().Styles.SwatchDot

	tt, ok := tint.GetTint("dracula")
	if !ok {
		t.Skip("dracula tint not registered")
	}
	row := themeOptionKey(tt, base)

	if !strings.Contains(row, "Dracula") {
		t.Errorf("row missing display name; got %q", row)
	}
	// One SGR-set + reset per colored cell. The lean row (name + 9 dots) stays
	// well under this bound; the old 18-dot, per-dot-background row roughly
	// doubled it. A regression that re-bloats the row will trip this.
	const maxEsc = 40
	if n := strings.Count(row, "\x1b["); n > maxEsc {
		t.Errorf("theme row has %d escape sequences, want <= %d (row too ANSI-dense)", n, maxEsc)
	}
	// Sanity: the row is a single line (multi-line rows break huh height math).
	if lipgloss.Height(row) != 1 {
		t.Errorf("theme row height = %d, want 1", lipgloss.Height(row))
	}
}

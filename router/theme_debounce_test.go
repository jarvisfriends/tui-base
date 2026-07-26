// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/theme"
)

// TestThemeMsgAppliesColorsImmediately verifies the color palette changes on the
// ThemeMsg itself (not on the debounced settle), so a live preview shows the new
// theme on the very next render.
func TestThemeMsgAppliesColorsImmediately(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	m.Update(settings.ThemeMsg{ID: testThemeDracula, Mode: testModeDark, ApplyPreferences: true})
	got := theme.ColorHex(m.colors.Bg)

	m.Update(settings.ThemeMsg{ID: "nord", Mode: testModeDark, ApplyPreferences: true})
	after := theme.ColorHex(m.colors.Bg)

	if got == after {
		t.Fatalf("expected background to change between themes, both were %s", after)
	}
}

// TestThemeSettleCoalesces verifies that only the newest theme change's settle
// runs the expensive relayout: stale themeSettleMsg values (from earlier
// selections in a fast scroll) are dropped, while the current generation runs.
func TestThemeSettleCoalesces(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Simulate three rapid selections; each bumps themePreviewGen.
	for _, id := range []string{testThemeDracula, "nord", "gruvbox_dark"} {
		m.Update(settings.ThemeMsg{ID: id, Mode: testModeDark, ApplyPreferences: true})
	}
	gen := m.themePreviewGen
	if gen == 0 {
		t.Fatal("themePreviewGen did not advance on theme changes")
	}

	// A settle from an earlier generation is stale and must be a no-op.
	if _, cmd := m.Update(themeSettleMsg{gen: gen - 1}); cmd != nil {
		t.Error("stale themeSettleMsg should not schedule any work")
	}

	// The current generation's settle runs (returns the relayout/sync batch).
	if _, cmd := m.Update(themeSettleMsg{gen: gen}); cmd == nil {
		t.Error("current themeSettleMsg should run the relayout + terminal sync")
	}
}

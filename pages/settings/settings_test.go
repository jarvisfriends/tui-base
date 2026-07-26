// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings_test

import (
	"os"
	"testing"

	"github.com/jarvisfriends/snap/rendercheck"
	settings "github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/theme"
	tint "github.com/lrstanley/bubbletint/v2"

	tea "charm.land/bubbletea/v2"
)

func TestSettingsResizeAndView(t *testing.T) {
	m := settings.NewWithOptions(settings.Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	v := m.View()
	if v.Content == "" {
		t.Fatal("settings View content should not be empty")
	}
	if !v.AltScreen {
		t.Fatal("expected AltScreen true on settings View")
	}
}

func TestMain(m *testing.M) {
	tint.NewDefaultRegistry()
	os.Exit(m.Run())
}

func TestSettingsLayoutOverflows(t *testing.T) {
	_ = theme.SetCurrentTint("dracula")
	m := settings.NewWithOptions(settings.Options{})
	m.SetColors(theme.Active())

	rendercheck.CheckNoLineOverflowAtSizes(t, m)
}

func TestSettingsNarrowWidths(t *testing.T) {
	_ = theme.SetCurrentTint("dracula")
	m := settings.NewWithOptions(settings.Options{})
	m.SetColors(theme.Active())

	// Check narrow width rendering down to min width (30 columns)
	rendercheck.CheckNoBorderOverflow(t, m, 30, 24)
}

package settings_test

import (
	"os"
	"testing"

	settings "github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/testutil"
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

	testutil.CheckNoLineOverflowAtSizes(t, m)
}

func TestSettingsNarrowWidths(t *testing.T) {
	_ = theme.SetCurrentTint("dracula")
	m := settings.NewWithOptions(settings.Options{})
	m.SetColors(theme.Active())

	// Check narrow width rendering down to min width (30 columns)
	testutil.CheckNoBorderOverflow(t, m, 30, 24)
}

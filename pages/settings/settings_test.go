package settings_test

import (
	"testing"

	settings "github.com/jarvisfriends/tui-base/pages/settings"

	tea "charm.land/bubbletea/v2"
)

func TestSettingsResizeAndView(t *testing.T) {
	m := settings.New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	v := m.View()
	if v.Content == "" {
		t.Fatal("settings View content should not be empty")
	}
	if !v.AltScreen {
		t.Fatal("expected AltScreen true on settings View")
	}
}

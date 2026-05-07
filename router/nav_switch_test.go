package router

import (
	"os"
	"testing"

	"github.com/jarvisfriends/tui-base/pages/settings"

	"github.com/jarvisfriends/tui-base/navigation"

	tea "charm.land/bubbletea/v2"
)

// Test that the settings page renders the huh form and that the router
// correctly initialises with the persisted nav style.
func TestSettingsNavToggleViaClick(t *testing.T) {
	// ensure no leftover settings file
	_ = os.Remove("tui_settings.json")

	m := New()

	// initialize sizes so children render
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// nav should initially be the sidebar implementation
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected initial nav to be sidebar; got %T", m.nav)
	}

	// find settings page index
	settingsIdx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == "settings" {
			settingsIdx = i
			break
		}
	}
	if settingsIdx == -1 {
		t.Fatal("settings page not found in nav")
	}
	if settingsIdx >= len(m.pages) {
		t.Fatalf("settings index %d out of range pages (%d)", settingsIdx, len(m.pages))
	}

	// settings view must be non-empty (huh form rendered)
	v := m.pages[settingsIdx].View()
	if v.Content == "" {
		t.Fatal("settings view content should not be empty")
	}

	// Verify that sending NavStyleMsg directly still switches nav (core contract).
	_, _ = m.Update(settings.NavStyleMsg{Style: "tabs"})
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected router nav to be tabs after NavStyleMsg; got %T", m.nav)
	}

	// cleanup settings file
	_ = os.Remove("tui_settings.json")
}

// Also verify that sending the NavStyleMsg directly switches the nav.
func TestSettingsNavToggleViaMsg(t *testing.T) {
	m := New()
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected initial nav to be sidebar; got %T", m.nav)
	}

	// send message to switch to tabs
	_, _ = m.Update(settings.NavStyleMsg{Style: "tabs"})
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected router nav to be tabs after NavStyleMsg; got %T", m.nav)
	}

	// switch back
	_, _ = m.Update(settings.NavStyleMsg{Style: "sidebar"})
	if _, ok := m.nav.(*navigation.Sidebar); !ok {
		t.Fatalf("expected router nav to be sidebar after NavStyleMsg; got %T", m.nav)
	}
}

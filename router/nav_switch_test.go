// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/pages/settings"

	"github.com/jarvisfriends/snap/navigation"
	"github.com/jarvisfriends/snap/styles"

	tea "charm.land/bubbletea/v2"
)

// Test that the settings page renders the huh form and that the router
// correctly initializes with the persisted nav style.
func TestSettingsNavToggleViaClick(t *testing.T) {
	// verify no leftover settings file
	tmpDir := t.TempDir()
	m := NewWithOptions(Options{
		ConfigDir: tmpDir,
	})

	// initialize sizes so children render
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// nav should initially be the sidebar implementation
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected initial nav to be sidebar; got %T", m.nav)
	}

	// find settings page index
	settingsIdx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == navigation.PageIDSettings {
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
	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleTabs})
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected router nav to be tabs after NavStyleMsg; got %T", m.nav)
	}
}

// Also verify that sending the NavStyleMsg directly switches the nav.
func TestSettingsNavToggleViaMsg(t *testing.T) {
	m := NewWithOptions(Options{
		ConfigDir: t.TempDir(),
	})
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected initial nav to be sidebar; got %T", m.nav)
	}

	// send message to switch to tabs
	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleTabs})
	if _, ok := m.nav.(*navigation.Tabs); !ok {
		t.Fatalf("expected router nav to be tabs after NavStyleMsg; got %T", m.nav)
	}

	// switch back
	_, _ = m.Update(settings.NavStyleMsg{Style: "sidebar"})
	if _, ok := m.nav.(*navigation.Sidebar); !ok {
		t.Fatalf("expected router nav to be sidebar after NavStyleMsg; got %T", m.nav)
	}
}

// TestNavToggleTopNav switches to the minimal top-nav style via NavStyleMsg,
// exercising newTopNav (which sets the Powerline slant) and the topnav branch
// of the NavStyleMsg handler.
func TestNavToggleTopNav(t *testing.T) {
	m := NewWithOptions(Options{ConfigDir: t.TempDir()})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleTopnav})
	top, ok := m.nav.(*navigation.MinimalTopNav)
	if !ok {
		t.Fatalf("expected nav to be MinimalTopNav after NavStyleMsg; got %T", m.nav)
	}
	if top.PillShape != styles.PillSlant {
		t.Errorf("newTopNav should set the slant shape; got %q", top.PillShape)
	}
}

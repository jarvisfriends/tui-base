package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/navigation"

	"github.com/jarvisfriends/tui-base/keys"

	tea "charm.land/bubbletea/v2"
)

func TestToggleHelpKey(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()
	m.width = 100
	m.height = 40
	m.status.SetKeys(m.keys)
	m.status.SetWidth(m.width)

	before := m.status.IsVisible()
	_, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+j"})
	if m.status.IsVisible() == before {
		t.Fatalf("expected status visibility to toggle")
	}
}

// TestToggleSidebarKey verifies the three-state Ctrl+B cycle:
//   visible+unfocused → visible+focused → hidden → visible+unfocused
func TestToggleSidebarKey(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()

	// State: visible, unfocused.
	m.navigationVisible = true
	m.sidebarFocused = false

	// First press: focus the sidebar (not hide it).
	_, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+b"})
	if !m.navigationVisible {
		t.Fatal("first ctrl+b should focus sidebar, not hide it")
	}
	if !m.sidebarFocused {
		t.Fatal("first ctrl+b should focus sidebar")
	}

	// Second press: hide the sidebar.
	_, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+b"})
	if m.navigationVisible {
		t.Fatal("second ctrl+b should hide sidebar")
	}
	if m.sidebarFocused {
		t.Fatal("second ctrl+b should clear sidebar focus")
	}

	// Third press: show the sidebar again (unfocused).
	_, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+b"})
	if !m.navigationVisible {
		t.Fatal("third ctrl+b should show sidebar")
	}
	if m.sidebarFocused {
		t.Fatal("third ctrl+b should leave sidebar unfocused")
	}
}

func TestOpenSettingsKey(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID == "settings" {
		t.Fatalf("expected test to start on a non-settings page")
	}

	_, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+,"})

	if got := m.nav.GetPages()[m.nav.GetActiveIndex()].ID; got != "settings" {
		t.Fatalf("active page = %q; want settings", got)
	}
}

func TestInvalidSelectedMsgReturnsResizeCmd(t *testing.T) {
	t.Parallel()
	m := New()
	prePageIndex := m.nav.GetActiveIndex()
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: -10})

	newPageIndex := m.nav.GetActiveIndex()
	if newPageIndex != prePageIndex {
		t.Fatalf("expected active page index to remain unchanged; got %d, want %d", newPageIndex, prePageIndex)
	}
}

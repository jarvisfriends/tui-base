package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/navigation"

	tea "charm.land/bubbletea/v2"
)

func TestSelectedMsgChangesActivePageAndLogs(t *testing.T) {
	m := New()
	// verify initial state
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != "home" {
		t.Fatalf("expected initial active page 'home'; got %s", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}

	// send a WindowSizeMsg to initialize children
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if m.inspector == nil {
		t.Fatal("expected non-nil inspector model")
	}
	dm := m.inspector
	initialLogs := len(dm.Logs)

	// find settings index in nav
	idx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == "settings" {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("could not find settings page in nav")
	}

	// send selection
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: idx})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != "settings" {
		t.Fatalf("expected active page 'settings' after selection; got %s", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}
	if m.nav.GetActiveIndex() != idx {
		t.Fatalf("expected nav.ActiveIndex=%d; got %d", idx, m.nav.GetActiveIndex())
	}

	if len(dm.Logs) <= initialLogs {
		t.Fatalf("expected debug logs to increase after SelectedMsg; before=%d after=%d", initialLogs, len(dm.Logs))
	}
}

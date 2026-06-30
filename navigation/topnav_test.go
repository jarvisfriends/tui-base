package navigation

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func selectedIndex(t *testing.T, cmd tea.Cmd) int {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	msg := cmd()
	sel, ok := msg.(SelectedMsg)
	if !ok {
		t.Fatalf("expected SelectedMsg, got %T", msg)
	}
	return sel.PageIndex
}

func TestMinimalTopNav_Defaults(t *testing.T) {
	m := NewMinimalTopNav()
	if m.ShowNumbers {
		t.Error("ShowNumbers should default to false")
	}
	if m.Dock() != DockTop {
		t.Error("MinimalTopNav should dock top")
	}
	if m.Width() != 0 {
		t.Errorf("top nav should reserve no side width, got %d", m.Width())
	}
}

func TestMinimalTopNav_NumberKeySelectsRegardlessOfPrefix(t *testing.T) {
	m := NewMinimalTopNav() // ShowNumbers=false
	_, cmd := m.Update(tea.KeyPressMsg{Text: "2"})
	if got := selectedIndex(t, cmd); got != 1 {
		t.Fatalf("key '2' should select index 1, got %d", got)
	}
	if m.GetActiveIndex() != 1 {
		t.Fatalf("active index should be 1, got %d", m.GetActiveIndex())
	}
	// Out-of-range digit is ignored (only 3 default pages).
	_, cmd = m.Update(tea.KeyPressMsg{Text: "9"})
	if cmd != nil {
		t.Error("out-of-range number key should be ignored")
	}
}

func TestMinimalTopNav_ArrowsWrap(t *testing.T) {
	m := NewMinimalTopNav() // 3 pages, active 0
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if got := selectedIndex(t, cmd); got != 2 {
		t.Fatalf("left from 0 should wrap to 2, got %d", got)
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if got := selectedIndex(t, cmd); got != 0 {
		t.Fatalf("right from 2 should wrap to 0, got %d", got)
	}
}

func TestMinimalTopNav_ShowNumbersTogglesPrefix(t *testing.T) {
	m := NewMinimalTopNav()
	if got := m.label(0, pageHome); got != pageHome {
		t.Errorf("hidden numbers: want %q, got %q", pageHome, got)
	}
	m.SetShowNumbers(true)
	if got := m.label(0, pageHome); got != "1:Home" {
		t.Errorf("shown numbers: want %q, got %q", "1:Home", got)
	}
	// The prefix appears in the rendered view too.
	if !strings.Contains(m.View().Content, "1:Home") {
		t.Error("rendered view should contain the numbered label when ShowNumbers is on")
	}
}

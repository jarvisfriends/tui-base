package navigation

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestKeyNavigation(t *testing.T) {
	t.Parallel()
	m := New()
	if m.ActiveIndex != 0 {
		t.Fatalf("initial ActiveIndex = %d; want 0", m.ActiveIndex)
	}

	// Test NextPage (was 'j', now 'down' or 'right' or 'tab')
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.ActiveIndex != 1 {
		t.Errorf("after 'down' ActiveIndex = %d; want 1", m.ActiveIndex)
	}

	// Test PreviousPage (was 'k', now 'up' or 'left' or 'shift+tab')
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.ActiveIndex != 0 {
		t.Errorf("after 'up' ActiveIndex = %d; want 0", m.ActiveIndex)
	}
}

func TestEnterEmitsSelectedMsg(t *testing.T) {
	t.Parallel()
	m := New()
	// select the last page
	m.ActiveIndex = len(m.Pages) - 1

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on enter")
	}
	msg := cmd()
	sel, ok := msg.(SelectedMsg)
	if !ok {
		t.Fatalf("expected SelectedMsg, got %T", msg)
	}
	if sel.PageIndex != m.ActiveIndex {
		t.Fatalf("SelectedMsg.PageIndex = %d; want %d", sel.PageIndex, m.ActiveIndex)
	}
}

func TestMouseSelectionUpdatesActiveIndexAndEmitsMsg(t *testing.T) {
	t.Parallel()
	m := New()
	// give the view a deterministic size so we can map lines
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	v := m.View()
	lines := strings.Split(v.Content, "\n")

	// extractSelected unwraps a SelectedMsg from either a direct cmd or a BatchMsg.
	extractSelected := func(cmd tea.Cmd) (SelectedMsg, bool) {
		if cmd == nil {
			return SelectedMsg{}, false
		}
		msg := cmd()
		if sel, ok := msg.(SelectedMsg); ok {
			return sel, true
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub == nil {
					continue
				}
				if sel, ok := sub().(SelectedMsg); ok {
					return sel, true
				}
			}
		}
		return SelectedMsg{}, false
	}

	for i, page := range m.Pages {
		for y, line := range lines {
			if !strings.Contains(line, page.Title) {
				continue
			}
			cmd := v.OnMouse(tea.MouseReleaseMsg{X: 0, Y: y, Button: tea.MouseLeft})
			if cmd == nil {
				t.Fatalf("OnMouse returned nil cmd for page %s at y=%d", page.Title, y)
			}
			sel, ok := extractSelected(cmd)
			if !ok {
				t.Fatalf("expected SelectedMsg from OnMouse, got %T", cmd())
			}
			if sel.PageIndex != i {
				t.Fatalf("SelectedMsg.PageIndex = %d; want %d", sel.PageIndex, i)
			}
			if m.ActiveIndex != i {
				t.Fatalf("model ActiveIndex = %d; want %d", m.ActiveIndex, i)
			}
			return
		}
	}

	// Debug: print the view lines
	for li, line := range lines {
		t.Logf("View content lines:\n%d: %s", li, line)
	}
	t.Fatalf("did not find any page title in rendered view")
}

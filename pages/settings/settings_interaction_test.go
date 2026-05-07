package settings

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOverviewEnterStartsEditOnKeyPress(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 16})
	if len(m.items) == 0 {
		t.Fatal("expected built-in settings items")
	}

	m.cursor = 1
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.editing {
		t.Fatal("expected editing mode to start on Enter key press")
	}
	if m.editForm == nil {
		t.Fatal("expected edit form to be created on Enter key press")
	}
	if cmd == nil {
		t.Fatal("expected Enter key press to return edit init cmd")
	}
}

func TestOverviewMouseWheelMovesCursorAndScrollWindow(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	if len(m.items) < 4 {
		t.Fatalf("expected enough settings rows for scrolling; got %d", len(m.items))
	}

	_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	if m.cursor != 1 {
		t.Fatalf("cursor after one wheel-down = %d; want 1", m.cursor)
	}

	for range 64 {
		_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	}
	if m.cursor != len(m.items)-1 {
		t.Fatalf("cursor at bottom = %d; want %d", m.cursor, len(m.items)-1)
	}
	if m.scrollTop <= 0 {
		t.Fatalf("expected scrollTop to advance once cursor reaches lower rows; got %d", m.scrollTop)
	}

	before := m.cursor
	_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if m.cursor != before-1 {
		t.Fatalf("cursor after wheel-up = %d; want %d", m.cursor, before-1)
	}
}

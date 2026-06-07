package settings

import (
	"strings"
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
	if !m.editOverlay.IsOpen() {
		t.Fatal("expected editing mode to start on Enter key press")
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

func TestSettingsOverviewHasCategories(t *testing.T) {
	t.Parallel()

	m := New()
	if len(m.categories) == 0 {
		t.Fatal("expected built-in settings categories")
	}

	entries := m.flattenedOverviewEntries()
	if len(entries) == 0 {
		t.Fatal("expected flattened overview entries")
	}
	if !entries[0].isHeader {
		t.Fatal("expected first overview entry to be a category header")
	}
}

func TestPreferredColumnWidthIgnoresLogPathLength(t *testing.T) {
	t.Parallel()

	m := New()
	base := m.preferredColumnWidth()
	m.LogPath = strings.Repeat("x", 1024)
	after := m.preferredColumnWidth()

	if after != base {
		t.Fatalf("preferred column width changed with long log path: got %d want %d", after, base)
	}
}

func TestWideViewportUsesMultipleColumns(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 220, Height: 20})
	layout := m.overviewLayout()
	if layout.columns < 2 {
		t.Fatalf("expected at least 2 columns on wide viewport, got %d", layout.columns)
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 7})

	m.cursor = len(m.items) - 1
	m.ensureCursorVisible()

	layout := m.overviewLayout()
	if layout.cursorEntry < m.scrollTop {
		t.Fatalf("cursor entry %d before scroll top %d", layout.cursorEntry, m.scrollTop)
	}
	if layout.cursorEntry >= m.scrollTop+layout.visibleCount {
		t.Fatalf("cursor entry %d after visible window [%d,%d)", layout.cursorEntry, m.scrollTop, m.scrollTop+layout.visibleCount)
	}
}

func TestShortAndFullHelp(t *testing.T) {
	t.Parallel()

	m := New()
	if len(m.ShortHelp()) == 0 {
		t.Fatal("expected short help bindings")
	}
	if len(m.FullHelp()) == 0 {
		t.Fatal("expected full help rows")
	}

	m.cursor = 1
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.ShortHelp()) == 0 {
		t.Fatal("expected editing short help bindings")
	}
	if len(m.FullHelp()) == 0 {
		t.Fatal("expected editing full help rows")
	}
}

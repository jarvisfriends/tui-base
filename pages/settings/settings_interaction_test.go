package settings

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOverviewEnterStartsEditOnKeyPress(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
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

	m := NewWithOptions(Options{})
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
		t.Fatalf(
			"expected scrollTop to advance once cursor reaches lower rows; got %d",
			m.scrollTop,
		)
	}

	before := m.cursor
	_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if m.cursor != before-1 {
		t.Fatalf("cursor after wheel-up = %d; want %d", m.cursor, before-1)
	}
}

func TestSettingsOverviewHasCategories(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
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

	m := NewWithOptions(Options{})
	base := m.preferredColumnWidth()
	m.LogPath = strings.Repeat("x", 1024)
	after := m.preferredColumnWidth()

	if after != base {
		t.Fatalf("preferred column width changed with long log path: got %d want %d", after, base)
	}
}

func TestWideViewportUsesMultipleColumns(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 220, Height: 20})
	layout := m.overviewLayout()
	if layout.columns < 2 {
		t.Fatalf("expected at least 2 columns on wide viewport, got %d", layout.columns)
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 7})

	m.cursor = len(m.items) - 1
	m.ensureCursorVisible()

	layout := m.overviewLayout()
	if layout.cursorEntry < m.scrollTop {
		t.Fatalf("cursor entry %d before scroll top %d", layout.cursorEntry, m.scrollTop)
	}
	if layout.cursorEntry >= m.scrollTop+layout.visibleCount {
		t.Fatalf(
			"cursor entry %d after visible window [%d,%d)",
			layout.cursorEntry,
			m.scrollTop,
			m.scrollTop+layout.visibleCount,
		)
	}
}

func TestShortAndFullHelp(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
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

func TestKeyRecorderValidation(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	var quitItem *settingItem
	var nextPageItem *settingItem
	for i, item := range m.items {
		if item.title == "Quit Application" {
			quitItem = &m.items[i]
		}
		if item.title == "Next Page" {
			nextPageItem = &m.items[i]
		}
	}

	if quitItem == nil || nextPageItem == nil {
		t.Fatal("expected 'Quit Application' and 'Next Page' settings items")
		return
	}

	model := quitItem.buildModel()
	kr, ok := model.(*KeyRecorder)
	if !ok {
		t.Fatalf("expected *KeyRecorder, got %T", model)
	}
	if kr.Error != "" {
		t.Fatalf("expected no initial error, got %q", kr.Error)
	}

	kr.cursor = len(kr.keys)
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !kr.recording {
		t.Fatal("expected recording mode to be active")
	}
	_, _ = kr.Update(tea.KeyPressMsg{Text: "q"})
	if kr.recording {
		t.Fatal("expected recording mode to end")
	}

	if kr.Error == "" {
		t.Fatal("expected error for duplicate key within shortcut, got none")
	}
	if !strings.Contains(kr.Error, "duplicate key") {
		t.Fatalf("expected error message to contain 'duplicate key', got %q", kr.Error)
	}

	_, _ = kr.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if kr.Done {
		t.Fatal("expected Done to be false when there is a validation error")
	}

	modelNP := nextPageItem.buildModel()
	krNP, ok := modelNP.(*KeyRecorder)
	if !ok {
		t.Fatalf("expected *KeyRecorder, got %T", modelNP)
	}

	krNP.cursor = len(krNP.keys)
	_, _ = krNP.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, _ = krNP.Update(tea.KeyPressMsg{Text: "q"})

	if krNP.Error == "" {
		t.Fatal("expected error for duplicate key across different shortcuts, got none")
	}
	if !strings.Contains(krNP.Error, "already assigned to") {
		t.Fatalf("expected error message to contain 'already assigned to', got %q", krNP.Error)
	}

	_, _ = krNP.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if krNP.Done {
		t.Fatal("expected Done to be false when there is a conflict")
	}

	krNP.cursor = 1
	_, _ = krNP.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if len(krNP.keys) != 1 {
		t.Fatalf("expected 1 key remaining, got %d", len(krNP.keys))
	}
	if krNP.Error != "" {
		t.Fatalf("expected error to be cleared after deleting the conflict, got %q", krNP.Error)
	}

	krNP.cursor = 1
	_, _ = krNP.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, _ = krNP.Update(tea.KeyPressMsg{Text: "k"})
	if krNP.Error != "" {
		t.Fatalf("expected no error for non-conflicting key, got %q", krNP.Error)
	}

	_, _ = krNP.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if !krNP.Done {
		t.Fatal("expected Done to be true when saving non-conflicting key")
	}
}

package settings

import (
	"strings"
	"testing"

	"github.com/jarvisfriends/snap/pickers"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/config"
)

// findItemIndex returns the index of the settings item with the given title.
func findItemIndex(t *testing.T, m *SettingsModel, title string) int {
	t.Helper()
	for i := range m.items {
		if m.items[i].title == title {
			return i
		}
	}
	t.Fatalf("settings item %q not found", title)
	return -1
}

func TestLogPathEditBlockedForTempDestination(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.LogOutput = logOutputTemp
	m.cursor = findItemIndex(t, m, itemTitleLogPath)

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.editOverlay.IsOpen() || m.modelOverlay.IsOpen() {
		t.Fatal("Log Path must not open an editor while the destination is Temporary")
	}
}

func TestLogPathDirDestinationOpensDirPicker(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.LogOutput = logOutputDir
	m.cursor = findItemIndex(t, m, itemTitleLogPath)

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.editOverlay.IsOpen() {
		t.Fatal("dir destination must use the directory picker, not the huh form")
	}
	if !m.modelOverlay.IsOpen() {
		t.Fatal("expected the directory-picker overlay to open")
	}
	if _, ok := m.modelOverlay.Model().(*pickers.DirPicker); !ok {
		t.Fatalf("overlay model = %T; want *pickers.DirPicker", m.modelOverlay.Model())
	}
}

func TestLogPathFileDestinationOpensFilePickerForm(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.LogOutput = logOutputFile
	m.cursor = findItemIndex(t, m, itemTitleLogPath)

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.editOverlay.IsOpen() {
		t.Fatal("expected the huh file-picker form for the file destination")
	}
	if m.modelOverlay.IsOpen() {
		t.Fatal("file destination must not open the directory picker")
	}
}

// assertFrameFits fails if any rendered line exceeds width or the content
// exceeds height (trailing blank lines ignored).
func assertFrameFits(t *testing.T, content string, width, height int) {
	t.Helper()
	lines := strings.Split(content, "\n")
	n := len(lines)
	for n > 0 && strings.TrimSpace(lines[n-1]) == "" {
		n--
	}
	if n > height {
		t.Errorf("frame is %d lines tall; exceeds height %d", n, height)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("line %d overflows width %d by %d cell(s): %q", i, width, got-width, line)
		}
	}
}

// TestLogPathDirOverlayFitsNarrowTallTerminal reproduces the 90x76 regression:
// the directory-picker overlay rendered lines wider than the page (its help
// line alone was ~96 cells), wrapping them and corrupting the layout.
func TestLogPathDirOverlayFitsNarrowTallTerminal(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 76})
	m.LogOutput = logOutputDir
	m.cursor = findItemIndex(t, m, itemTitleLogPath)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.modelOverlay.IsOpen() {
		t.Fatal("expected the directory-picker overlay to open")
	}
	// Load the directory listing (the overlay's Init command).
	if cmd != nil {
		if msg := cmd(); msg != nil {
			_, _ = m.Update(msg)
		}
	}

	assertFrameFits(t, m.View().Content, 90, 76)
}

func TestDirOnlyConsumerFieldUsesDirPicker(t *testing.T) {
	t.Parallel()

	val := ""
	m := NewWithOptions(Options{ExtraSections: []config.Section[string]{{
		Title: "App",
		Fields: []config.FieldDef[string]{{
			Kind:       config.FieldFilePicker,
			Title:      "Data Dir",
			Value:      &val,
			DirAllowed: true,
		}},
	}}})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.cursor = findItemIndex(t, m, "Data Dir")

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.modelOverlay.IsOpen() {
		t.Fatal("expected dir-only consumer field to open the directory picker")
	}
	if _, ok := m.modelOverlay.Model().(*pickers.DirPicker); !ok {
		t.Fatalf("overlay model = %T; want *pickers.DirPicker", m.modelOverlay.Model())
	}
}

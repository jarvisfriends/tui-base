package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/config"
)

// makePickerTree builds a temp directory containing two subdirectories and
// two files, returning its path.
func makePickerTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	for _, f := range []string{"notes.txt", "config.json"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}
	return root
}

// newTestDirPicker returns a DirPicker browsing root with its initial
// directory listing loaded (executing the Init command like tea.Program would).
func newTestDirPicker(t *testing.T, root string) *DirPicker {
	t.Helper()
	dp := NewDirPicker(root)
	dp.Width, dp.Height = 80, 24
	if cmd := dp.Init(); cmd != nil {
		_, _ = dp.Update(cmd())
	}
	return dp
}

func TestDirPickerHidesFiles(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	dp := newTestDirPicker(t, root)

	if len(dp.entries) != 2 {
		t.Fatalf("entries = %v; want only the 2 subdirectories", dp.entries)
	}
	view := dp.View().Content
	for _, dir := range []string{"alpha", "beta"} {
		if !strings.Contains(view, dir) {
			t.Errorf("view is missing directory %q", dir)
		}
	}
	for _, file := range []string{"notes.txt", "config.json"} {
		if strings.Contains(view, file) {
			t.Errorf("view shows file %q; files must be hidden", file)
		}
	}
}

func TestDirPickerEnterOpensFolder(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	dp := newTestDirPicker(t, root)

	_, cmd := dp.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on a folder to return a read command")
	}
	_, _ = dp.Update(cmd())
	if dp.dir != filepath.Join(root, "alpha") {
		t.Fatalf("dir = %q; want %q", dp.dir, filepath.Join(root, "alpha"))
	}
	if dp.Done || dp.Aborted {
		t.Fatal("enter must browse, not complete the picker")
	}
}

func TestDirPickerBackGoesToParent(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	dp := newTestDirPicker(t, filepath.Join(root, "alpha"))

	_, cmd := dp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if cmd == nil {
		t.Fatal("expected left to return a read command for the parent")
	}
	_, _ = dp.Update(cmd())
	if dp.dir != root {
		t.Fatalf("dir = %q; want parent %q", dp.dir, root)
	}
}

func TestDirPickerSpaceSelectsHighlighted(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	dp := newTestDirPicker(t, root)

	_, _ = dp.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // highlight "beta"
	_, _ = dp.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if !dp.Done {
		t.Fatal("expected space to complete the picker")
	}
	if want := filepath.Join(root, "beta"); dp.Value() != want {
		t.Fatalf("Value() = %q; want %q", dp.Value(), want)
	}
}

func TestDirPickerCtrlSSelectsCurrentDir(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	dp := newTestDirPicker(t, root)

	_, _ = dp.Update(tea.KeyPressMsg{Text: keyCtrlS})
	if !dp.Done {
		t.Fatal("expected ctrl+s to complete the picker")
	}
	if dp.Value() != root {
		t.Fatalf("Value() = %q; want the browsed dir %q", dp.Value(), root)
	}
}

func TestDirPickerEscAborts(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	dp := newTestDirPicker(t, root)

	_, _ = dp.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !dp.Aborted {
		t.Fatal("expected esc to abort the picker")
	}
	if dp.Done || dp.Value() != "" {
		t.Fatal("aborted picker must not report a value")
	}
}

func TestDirPickerStartsAtParentForFilePath(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	dp := NewDirPicker(filepath.Join(root, "notes.txt"))
	if dp.dir != root {
		t.Fatalf("dir = %q; want file's parent %q", dp.dir, root)
	}
}

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
	if _, ok := m.modelOverlay.Model().(*DirPicker); !ok {
		t.Fatalf("overlay model = %T; want *DirPicker", m.modelOverlay.Model())
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

// TestDirPickerViewFitsWidth drives the picker directly with entries and a
// long browse path; every line must fit the declared width.
func TestDirPickerViewFitsWidth(t *testing.T) {
	t.Parallel()

	root := makePickerTree(t)
	deep := filepath.Join(
		root,
		"alpha",
		strings.Repeat("very-long-directory-name-", 4),
	)
	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}

	dp := NewDirPicker(deep)
	dp.Width, dp.Height = 90, 76
	if cmd := dp.Init(); cmd != nil {
		_, _ = dp.Update(cmd())
	}

	assertFrameFits(t, dp.View().Content, 90, 76)
}

// TestMultiFileEditorViewFitsWidth guards the same class of bug in the
// multi-file editor: long stored paths must be truncated, not wrapped.
func TestMultiFileEditorViewFitsWidth(t *testing.T) {
	t.Parallel()

	long := "C:\\" + strings.Repeat("deeply\\nested\\folders\\", 8) + "file.log"
	e := NewMultiFileEditor(long + "; " + long)
	_, _ = e.Update(tea.WindowSizeMsg{Width: 90, Height: 76})

	assertFrameFits(t, e.View().Content, 90, 76)
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
	if _, ok := m.modelOverlay.Model().(*DirPicker); !ok {
		t.Fatalf("overlay model = %T; want *DirPicker", m.modelOverlay.Model())
	}
}

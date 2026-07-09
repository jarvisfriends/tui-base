package settings

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// pumpCmds executes a command tree, feeding produced messages back into the
// editor — the job tea.Program does in the real app.
func pumpCmds(t *testing.T, e *MultiFileEditor, cmd tea.Cmd, depth int) {
	t.Helper()
	if cmd == nil || depth > 16 {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			pumpCmds(t, e, c, depth+1)
		}
		return
	}
	_, next := e.Update(msg)
	pumpCmds(t, e, next, depth+1)
}

// TestMultiFilePickerShowsAllRowsOnOpen guards against the picker opening
// with a one-row browse window: immediately after Enter opens the picker,
// the rendered view must already list multiple entries of the directory,
// not a single row the user has to scroll or re-enter the directory to
// expand. (bubbles' filepicker starts with a collapsed window that only a
// dispatched readDir repairs — so the form's Init command must be returned
// to the runtime, not discarded.)
func TestMultiFilePickerShowsAllRowsOnOpen(t *testing.T) {
	t.Parallel()

	e := NewMultiFileEditor("")
	_, _ = e.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// Enter on "[ Add Path ]" opens the picker; pump the returned commands so
	// the picker's readDir actually runs, as it would under tea.Program.
	_, cmd := e.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pumpCmds(t, e, cmd, 0)

	if !e.picking || e.pickerForm == nil {
		t.Fatal("expected Enter to start picking")
	}

	// This package's directory has many .go files; a correctly sized browse
	// window must show several of them at once.
	view := e.View().Content
	shown := 0
	for _, name := range []string{"multi_file.go", "settings.go", "dir_picker_test.go", "picker_theme_test.go"} {
		if strings.Contains(view, name) {
			shown++
		}
	}
	if shown < 3 {
		t.Fatalf(
			"picker opened showing %d/4 known entries - browse window is collapsed: %s",
			shown,
			view,
		)
	}
}

// TestDirPickerNavigatesAboveStartAndListsDrives drives the DirPicker from
// its start directory up past the filesystem root: Back must keep working
// above the starting directory, and at a drive root it must switch to the
// drive list so any drive can be reached.
func TestDirPickerNavigatesAboveStartAndListsDrives(t *testing.T) {
	t.Parallel()

	dp := NewDirPicker("")
	dp.Width, dp.Height = 100, 30
	msg := dp.Init()()
	_, _ = dp.Update(msg)

	back := tea.KeyPressMsg{Code: tea.KeyLeft}
	for range 40 { // more than enough to reach the drive list from anywhere
		_, cmd := dp.Update(back)
		if cmd != nil {
			if m := cmd(); m != nil {
				_, _ = dp.Update(m)
			}
		}
		if dp.dir == "" {
			break
		}
	}
	if dp.dir != "" {
		t.Fatalf("Back never reached the drive list; still browsing %q", dp.dir)
	}
	if len(dp.entries) == 0 {
		t.Fatal("drive list is empty")
	}

	// Selecting a drive entry must yield that drive root.
	_, _ = dp.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if !dp.Done || dp.Value() == "" {
		t.Fatalf("selecting a drive did not complete; value=%q done=%v", dp.Value(), dp.Done)
	}
}

// TestMultiFileEditorDirsOnlyUsesDirPicker verifies the DirsOnly flag routes
// row editing to the DirPicker instead of the huh file picker.
func TestMultiFileEditorDirsOnlyUsesDirPicker(t *testing.T) {
	t.Parallel()

	e := NewMultiFileEditor("")
	e.DirsOnly = true
	_, _ = e.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_, cmd := e.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !e.picking || e.dirPicker == nil {
		t.Fatal("expected DirsOnly editing to open the DirPicker")
	}
	if e.pickerForm != nil {
		t.Fatal("DirsOnly editing must not build the huh file-picker form")
	}
	pumpCmds(t, e, cmd, 0)

	// Ctrl+S selects the directory being browsed and closes the picker.
	_, _ = e.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if e.picking {
		t.Fatal("Ctrl+S did not close the DirPicker")
	}
	if len(e.paths) != 1 || e.paths[0] == "" {
		t.Fatalf("expected the browsed directory to be added; paths=%v", e.paths)
	}
}

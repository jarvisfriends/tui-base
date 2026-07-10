package settings

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/overlay"
)

// TestLogPathPickerFillsAvailableHeight guards against the file picker
// regressing to its default one-row browse list: the Log Path edit form must
// expand to most of the page height.
func TestLogPathPickerFillsAvailableHeight(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	// The picker form only exists for the fixed-file destination; temp has no
	// editor and dir uses the DirPicker overlay.
	m.LogOutput = logOutputFile

	f := m.items[findItemIndex(t, m, itemTitleLogPath)].buildForm()
	if f == nil {
		t.Fatal("Log Path item returned nil form")
	}
	f.Init()

	got := lipgloss.Height(f.View())
	want := overlay.FormHeight(m.Height())
	if got < want {
		t.Fatalf(
			"log-path picker form height = %d; want at least %d (page height %d)",
			got, want, m.Height(),
		)
	}
}

package home

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jarvisfriends/inspector"

	tea "charm.land/bubbletea/v2"
)

// driveC is the Windows-style drive path reused across the disk tests.
const driveC = "C:\\"

// TestFileManagerResolvesExplorer guards the Windows fix: a bare "explorer" is
// not reliably on PATH in a Git Bash / MSYS launch, so the command must resolve
// to an absolute explorer.exe under %SystemRoot%. Other platforms use the
// conventional openers.
func TestFileManagerResolvesExplorer(t *testing.T) {
	got := fileManager()
	switch runtime.GOOS {
	case osWindows:
		if !strings.EqualFold(filepath.Base(got), "explorer.exe") {
			t.Fatalf("fileManager() = %q, want an explorer.exe path", got)
		}
		if !filepath.IsAbs(got) && got != "explorer" {
			t.Fatalf("fileManager() = %q, want an absolute path (or the bare fallback)", got)
		}
	case osDarwin:
		if got != "open" {
			t.Fatalf("fileManager() = %q, want open", got)
		}
	default:
		if got != "xdg-open" {
			t.Fatalf("fileManager() = %q, want xdg-open", got)
		}
	}
}

const (
	giB = uint64(1) << 30
	tiB = uint64(1) << 40
)

// sampleDisks returns three drives whose Total sizes deliberately sort
// differently by magnitude than by display text: lexical order would put
// "9.0 GiB" first (leading '9'), numeric order puts "1.5 TiB" first.
func sampleDisks() []inspector.DiskUsage {
	return []inspector.DiskUsage{
		{Path: "A:\\", Total: 9 * giB, Used: 3 * giB, Free: 6 * giB},
		{Path: "B:\\", Total: 500 * giB, Used: 400 * giB, Free: 100 * giB},
		{Path: driveC, Total: 1536 * giB, Used: 1024 * giB, Free: 512 * giB}, // 1.5 TiB
	}
}

// TestDiskRowsSortByMagnitude verifies the default Total-descending sort orders
// by the real byte count, not the humanized text — the "1.4 TiB before 500 GiB"
// requirement. It also pins that the displayed cell text is the humanized form.
func TestDiskRowsSortByMagnitude(t *testing.T) {
	tbl := newDisksTable()
	tbl.SetRows(diskRows(sampleDisks()))

	top, ok := tbl.SelectedRow()
	if !ok {
		t.Fatal("expected a highlighted row after SetRows")
	}
	if top.Key != driveC {
		t.Fatalf("default Total-desc sort should lead with the 1.5 TiB drive; got %q", top.Key)
	}
	if got := top.Cells[diskColTotal].Text; got != "1.5 TiB" {
		t.Fatalf("Total cell text = %q, want %q", got, "1.5 TiB")
	}
}

// TestDiskRowsUnavailable confirms an errored drive collapses to one keyed
// "unavailable" row rather than being dropped.
func TestDiskRowsUnavailable(t *testing.T) {
	rows := diskRows([]inspector.DiskUsage{{Path: "Z:\\", Error: "not ready"}})
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Key != "Z:\\" || rows[0].Cells[diskColUsed].Text != "unavailable" {
		t.Fatalf("unexpected unavailable row: %+v", rows[0])
	}
}

// TestDiskMenuOpenAction: opening the disk menu records the target drive, and
// the Open action yields a (non-nil) command. The command is NOT run — doing so
// would launch a real file-explorer window.
func TestDiskMenuOpenAction(t *testing.T) {
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})

	m.openDiskMenu(5, 5, driveC)
	if !m.contextMenu.IsOpen() {
		t.Fatal("openDiskMenu should open the context menu")
	}
	if m.menuDiskPath != driveC {
		t.Fatalf("menuDiskPath = %q, want %q", m.menuDiskPath, driveC)
	}
	if cmd := m.applyContextChoice(diskActionOpen); cmd == nil {
		t.Fatal("Open-in-file-explorer action should return a command")
	}
}

// TestDisksHandlesKey: the table claims its own nav/sort/open keys but leaves
// Esc for the router (when no filter is active).
func TestDisksHandlesKey(t *testing.T) {
	m := New()
	for _, k := range []string{"up", "down", "s", "/", "enter"} {
		if !m.disksHandlesKey(tea.KeyPressMsg{Code: keyCode(k)}) {
			t.Errorf("disks table should handle %q", k)
		}
	}
	if m.disksHandlesKey(tea.KeyPressMsg{Code: tea.KeyEsc}) {
		t.Error("Esc should fall through to the router when not filtering")
	}
}

// keyCode maps the single-rune test keys to their tea key codes.
func keyCode(k string) rune {
	switch k {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "enter":
		return tea.KeyEnter
	default:
		r, _ := utf8.DecodeRuneInString(k)
		return r
	}
}

// TestDisksHeaderClickSorts drives a real click through View().OnMouse onto the
// Drive column header and checks it re-sorts the table — end-to-end proof that
// the recorded table geometry (disksLeft/disksTop) maps pointer coordinates
// into the table correctly. A large terminal keeps the content unscrolled so
// screen and content coordinates coincide.
func TestDisksHeaderClickSorts(t *testing.T) {
	m := New()
	m.disks = sampleDisks()
	m.disksTbl.SetRows(diskRows(m.disks))
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	_ = m.View() // populates disksLeft/disksTop

	if m.vp.YOffset() != 0 {
		t.Skipf("content scrolled (YOffset=%d); screen≠content coords", m.vp.YOffset())
	}
	// Default sort is Total desc → the 1.5 TiB drive leads.
	if top, _ := m.disksTbl.SelectedRow(); top.Key != driveC {
		t.Fatalf("precondition: default top row = %q, want %q", top.Key, driveC)
	}

	// Click the Drive column header (first column, one cell past its left edge).
	click(m, m.disksLeft+1, m.disksTop)

	if top, _ := m.disksTbl.SelectedRow(); top.Key != "A:\\" {
		t.Fatalf("after Drive-header click, top row = %q, want A:\\ (ascending)", top.Key)
	}
}

// TestDisksRenderInContent: the Disks section and its zone appear in a rendered
// home page so pointer routing has something to hit.
func TestDisksRenderInContent(t *testing.T) {
	m := New()
	m.disks = sampleDisks()
	m.disksTbl.SetRows(diskRows(m.disks))
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	out := m.View().Content

	if !strings.Contains(out, "Disks") {
		t.Fatal("Disks heading should render on the home page")
	}
	if _, ok := m.zones.Bounds(zoneDisks); !ok {
		t.Fatal("disks zone should be registered for hit-testing")
	}
}

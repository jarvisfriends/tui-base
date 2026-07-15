package home

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jarvisfriends/inspector"
	"github.com/jarvisfriends/snap/menu"
	"github.com/jarvisfriends/snap/styles"
	"github.com/jarvisfriends/snap/table"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dustin/go-humanize"
)

// Disk table column indices. The byte columns and Use% carry a numeric sort
// value (table.Num) so the column sorts by magnitude — e.g. 1.4 TiB before
// 500 GiB — instead of by the displayed text.
const (
	diskColDrive = iota
	diskColUsed
	diskColTotal
	diskColFree
	diskColUsePct
)

// Disk context-menu action IDs (dispatched in applyContextChoice).
const (
	diskActionOpen    = "disk-open"
	diskActionRefresh = "disk-refresh"
)

// Disks-table key hints for the status bar / help menu. These are display-only
// bindings — key.Matches still runs against the table's own KeyMap in
// disksHandlesKey — so they mirror the interactions the Home page forwards to
// the table, worded for this page (enter opens the per-drive actions menu, not
// a detail view). homeKeyNav folds ↑/↓ into one compact short-help slot the way
// the settings page does.
var (
	homeKeyNav     = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move"))
	homeKeyUp      = key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "move up"))
	homeKeyDown    = key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "move down"))
	homeKeySort    = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort"))
	homeKeyActions = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "row actions"))
	homeKeyFilter  = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"))
	homeKeyClear   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter"))
)

// ShortHelp implements help.KeyMap so the status bar shows the Disks table's
// keys — including s to sort — instead of the generic router shortcuts. While
// the filter input is focused only the clear-filter hint is relevant.
func (m *HomePageModel) ShortHelp() []key.Binding {
	if m.disksTbl != nil && m.disksTbl.Filtering() {
		return []key.Binding{homeKeyClear}
	}
	return []key.Binding{homeKeyNav, homeKeySort, homeKeyActions, homeKeyFilter}
}

// FullHelp implements help.KeyMap (the ctrl+h expanded view).
func (m *HomePageModel) FullHelp() [][]key.Binding {
	if m.disksTbl != nil && m.disksTbl.Filtering() {
		return [][]key.Binding{{homeKeyClear}}
	}
	return [][]key.Binding{
		{homeKeyUp, homeKeyDown},
		{homeKeySort, homeKeyFilter},
		{homeKeyActions},
	}
}

var _ help.KeyMap = (*HomePageModel)(nil)

// newDisksTable builds the Disks table: five columns, defaulting to Total
// descending so the biggest drive leads. HideFooterHint is set because the
// block's own heading spells out the controls.
func newDisksTable() *table.TableModel {
	t := table.New([]table.Column{
		{Title: "Drive", Filter: true},
		{Title: "Used"},
		{Title: "Total"},
		{Title: "Free"},
		{Title: "Use%"},
	}, table.WithSort(diskColTotal, false))
	t.HideFooterHint = true
	return t
}

// refreshDisks re-enumerates the machine's drives (via the Inspector's
// cross-platform collector) and rebuilds the table rows, keeping the active
// sort. Each byte column stores the real byte count as its sort value so the
// storage numbers order correctly regardless of their TiB/GiB/MiB unit.
//
// This is called from the update loop (OnEnter and the Refresh action), never
// from the constructor: all model mutation stays on Bubble Tea's single
// goroutine, so the disk enumeration (a syscall) and the row rebuild never run
// concurrently with another page instance's.
func (m *HomePageModel) refreshDisks() {
	m.disks = inspector.Disks()
	m.disksTbl.SetRows(diskRows(m.disks))
	m.syncDisksSize()
}

// diskRows converts collected disk usage into table rows. Byte columns and
// Use% carry the raw number as their sort value (table.Num) so they order by
// magnitude — 1.4 TiB above 500 GiB — while displaying the humanized text.
// Unreadable drives become a single "unavailable" row keyed by their path.
func diskRows(disks []inspector.DiskUsage) []table.Row {
	rows := make([]table.Row, 0, len(disks))
	for _, d := range disks {
		if d.Error != "" {
			rows = append(rows, table.Row{Key: d.Path, Cells: []table.Cell{
				table.Text(d.Path),
				table.Text("unavailable"),
				table.Text(""),
				table.Text(""),
				table.Text(""),
			}})
			continue
		}
		pct := 0.0
		if d.Total > 0 {
			pct = float64(d.Used) / float64(d.Total) * 100
		}
		rows = append(rows, table.Row{Key: d.Path, Cells: []table.Cell{
			table.Text(d.Path),
			table.Num(humanize.IBytes(d.Used), float64(d.Used)),
			table.Num(humanize.IBytes(d.Total), float64(d.Total)),
			table.Num(humanize.IBytes(d.Free), float64(d.Free)),
			table.Num(fmt.Sprintf("%.0f%%", pct), pct),
		}})
	}
	return rows
}

// syncDisksSize gives the table its width and a height that shows every drive
// (capped so a long list still leaves the page scrollable; the table paginates
// beyond the cap). It is a no-op until the page has a real width.
func (m *HomePageModel) syncDisksSize() {
	if m.Width() <= 0 {
		return
	}
	w := max(min(m.Width()-4, 72), 24)
	n := max(len(m.disks), 1)
	h := min(n, 15) + 2 // +chrome (header + footer)
	m.disksTbl.SetSize(w, h)
}

// disksBlock renders the Disks section — a heading with the sort/action hint
// above the table — and returns the block plus the heading's height so
// content() can place the table's hit-test geometry (see disksTop).
func (m *HomePageModel) disksBlock(c *styles.AppStyle) (block string, headingH int) {
	heading := c.Styles.Title.Render("Disks") +
		c.Styles.Subtitle.Render("   click a header to sort · enter or right-click a row for actions")
	headingH = lipgloss.Height(heading)
	// Render at origin 0; the table's output doesn't depend on originY, and its
	// click hit-testing is done in table-local coordinates (the handlers
	// subtract disksTop), so no second render is needed to place its geometry.
	tbl := m.disksTbl.View(c, 0)
	return lipgloss.JoinVertical(lipgloss.Left, heading, tbl), headingH
}

// disksHandlesKey reports whether the Disks table should consume a key: while
// its filter input is focused every key belongs to it, otherwise only its
// navigation/sort/filter/open bindings (Cancel is intentionally excluded so Esc
// still reaches the router for focus/dismiss when no filter is active).
func (m *HomePageModel) disksHandlesKey(msg tea.KeyPressMsg) bool {
	if m.disksTbl == nil {
		return false
	}
	if m.disksTbl.Filtering() {
		return true
	}
	km := m.disksTbl.KeyMap
	for _, b := range []key.Binding{
		km.Up, km.Down, km.PageUp, km.PageDown, km.Top, km.Bottom, km.Sort, km.Filter, km.Open,
	} {
		if key.Matches(msg, b) {
			return true
		}
	}
	return false
}

// openDiskMenu opens the shared context menu with the actions for one drive,
// anchored at screen (x, y). menuDiskPath records which drive the actions apply
// to (item IDs are handled in applyContextChoice).
func (m *HomePageModel) openDiskMenu(x, y int, path string) {
	if path == "" {
		return
	}
	m.menuDiskPath = path
	info := "Drive " + path
	for _, d := range m.disks {
		if d.Path == path && d.Error == "" {
			info = fmt.Sprintf("%s — %s free of %s", path, humanize.IBytes(d.Free), humanize.IBytes(d.Total))
			break
		}
	}
	items := []menu.Item{
		{Label: info, Disabled: true},
		{ID: diskActionOpen, Label: "Open in file explorer"},
		{ID: diskActionRefresh, Label: "Refresh disks"},
	}
	m.contextMenu.Open(x, y, items, zoneDisks)
}

// openPathCmd opens path in the OS default file manager (Explorer on Windows,
// Finder via `open` on macOS, `xdg-open` elsewhere) — the file-manager twin of
// openBrowserCmd. It reuses browserOpenedMsg so failures land in the log.
func openPathCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if err := exec.Command(fileManager(), path).Start(); err != nil {
			return browserOpenedMsg{URL: path, Err: err}
		}
		return browserOpenedMsg{URL: path}
	}
}

// fileManager returns the command that opens a path in the OS file manager. On
// Windows it resolves explorer.exe by absolute path: explorer lives in
// %SystemRoot% (not System32), which is not reliably on PATH in the environment
// a Git Bash / MSYS launch inherits, so a bare "explorer" lookup fails.
func fileManager() string {
	switch runtime.GOOS {
	case osWindows:
		if dir := os.Getenv("SystemRoot"); dir != "" {
			return filepath.Join(dir, "explorer.exe")
		}
		if dir := os.Getenv("windir"); dir != "" {
			return filepath.Join(dir, "explorer.exe")
		}
		return "explorer"
	case osDarwin:
		return "open"
	default:
		return "xdg-open"
	}
}

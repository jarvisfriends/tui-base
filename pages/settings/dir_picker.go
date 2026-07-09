package settings

import (
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/tui-base/envpath"
	"github.com/jarvisfriends/tui-base/theme"
)

// overlayContentWidth returns how many columns a model hosted in a
// ModelOverlayHost may use: the page width minus the overlay box chrome
// (2 border + 4 padding) and a 2-column margin. Content wider than this wraps
// inside the box and corrupts the layout. Zero/unset widths fall back to a
// conservative 74 (an 80-column terminal minus the chrome).
func overlayContentWidth(pageWidth int) int {
	if pageWidth <= 0 {
		return 74
	}
	return max(20, pageWidth-8)
}

// fitLine truncates a styled line to w display cells with an ellipsis; lines
// already within w are returned unchanged.
func fitLine(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}

// DirPickerKeyMap defines the key bindings for the directory picker.
type DirPickerKeyMap struct {
	Cancel        key.Binding
	Up            key.Binding
	Down          key.Binding
	Open          key.Binding
	Back          key.Binding
	Select        key.Binding
	SelectCurrent key.Binding
}

// DefaultDirPickerKeyMap returns the standard directory-picker bindings,
// mirroring the file-picker split: Enter/→ browses, Space selects.
func DefaultDirPickerKeyMap() DirPickerKeyMap {
	return DirPickerKeyMap{
		Cancel: key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "cancel")),
		Up:     key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		Down:   key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		Open: key.NewBinding(
			key.WithKeys("enter", "right"),
			key.WithHelp("enter/→", "open folder"),
		),
		Back: key.NewBinding(
			key.WithKeys("left", "backspace"),
			key.WithHelp("←", "up folder"),
		),
		Select: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select folder")),
		SelectCurrent: key.NewBinding(
			key.WithKeys(keyCtrlS),
			key.WithHelp("ctrl+s", "select this folder"),
		),
	}
}

// dirEntriesMsg carries the subdirectory listing produced by readDirCmd.
type dirEntriesMsg struct {
	dir     string
	entries []string
	err     error
}

// DirPicker is a directory-only browser overlay: unlike the huh/bubbles file
// picker it lists no files at all, which is the right presentation when the
// user can only choose a directory. Selection follows the file-picker split
// (Enter/→ browses, Space selects the highlighted folder) plus Ctrl+S to
// select the directory currently being browsed.
type DirPicker struct {
	dir       string   // directory currently being browsed (absolute)
	entries   []string // subdirectory names of dir
	cursor    int
	scrollTop int
	err       error
	selected  string

	KeyMap  DirPickerKeyMap
	Done    bool
	Aborted bool
	Width   int
	Height  int
}

// NewDirPicker returns a picker browsing from initial: the value itself when
// it is an existing directory, its parent when it is a file path, and the
// working directory when it is empty or invalid.
func NewDirPicker(initial string) *DirPicker {
	dir := initial
	if dir != "" {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			dir = filepath.Dir(dir)
		}
	}
	if st, err := os.Stat(dir); dir == "" || err != nil || !st.IsDir() {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			dir = wd
		} else {
			dir = "."
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return &DirPicker{
		dir:    dir,
		KeyMap: DefaultDirPickerKeyMap(),
	}
}

// Value returns the selected directory path (empty until Done).
func (m *DirPicker) Value() string {
	return m.selected
}

func (m *DirPicker) Init() tea.Cmd {
	return readDirCmd(m.dir)
}

// listDrives returns the mounted drive roots (e.g. ["C:\", "E:\"]) by
// probing every letter; on non-Windows systems no letter resolves so the
// list is empty and the drive view is never entered ("/" is its own parent,
// but Back at "/" simply finds nothing to list).
func listDrives() []string {
	var drives []string
	for l := 'A'; l <= 'Z'; l++ {
		root := string(l) + `:\`
		if st, err := os.Stat(root); err == nil && st.IsDir() {
			drives = append(drives, root)
		}
	}
	return drives
}

// readDirCmd lists the subdirectories of dir, hiding all files.
func readDirCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return dirEntriesMsg{dir: dir, err: err}
		}
		var names []string
		for _, e := range ents {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
		return dirEntriesMsg{dir: dir, entries: names}
	}
}

func (m *DirPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		m.ensureCursorVisible()

	case dirEntriesMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.dir = msg.dir
		m.entries = msg.entries
		m.cursor = 0
		m.scrollTop = 0
		m.err = nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Cancel):
			m.Aborted = true
		case key.Matches(msg, m.KeyMap.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			m.ensureCursorVisible()
		case key.Matches(msg, m.KeyMap.Down):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
			m.ensureCursorVisible()
		case key.Matches(msg, m.KeyMap.Open):
			if m.cursor < len(m.entries) {
				return m, readDirCmd(filepath.Join(m.dir, m.entries[m.cursor]))
			}
		case key.Matches(msg, m.KeyMap.Back):
			if m.dir == "" {
				break // already at the drive list
			}
			if parent := filepath.Dir(m.dir); parent != m.dir {
				return m, readDirCmd(parent)
			}
			// At a filesystem root: offer the available drives so the user
			// can navigate anywhere, not just below the starting directory.
			if drives := listDrives(); len(drives) > 0 {
				m.dir = ""
				m.entries = drives
				m.cursor, m.scrollTop = 0, 0
				m.err = nil
			}
		case key.Matches(msg, m.KeyMap.Select):
			if m.cursor < len(m.entries) {
				m.selected = filepath.Join(m.dir, m.entries[m.cursor])
				m.Done = true
			}
		case key.Matches(msg, m.KeyMap.SelectCurrent):
			if m.dir != "" {
				m.selected = m.dir
				m.Done = true
			}
		}
	}
	return m, nil
}

// listHeight returns how many directory rows fit between the header and help
// chrome for the current overlay height.
func (m *DirPicker) listHeight() int {
	return max(3, m.Height-8)
}

func (m *DirPicker) ensureCursorVisible() {
	h := m.listHeight()
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
	if m.cursor >= m.scrollTop+h {
		m.scrollTop = m.cursor - h + 1
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
}

func (m *DirPicker) View() tea.View {
	c := theme.Active()
	maxW := overlayContentWidth(m.Width)
	title := c.Styles.Title.Bold(true).Padding(0, 1).Render("Select Directory")
	pathStyle := c.Styles.Subtitle
	selStyle := c.Styles.SelectedItem.Bold(true)
	normStyle := c.Styles.TextOnBg
	dimStyle := c.Styles.Dim

	location := "📁 " + envpath.Collapse(m.dir)
	if m.dir == "" {
		location = "💾 Drives"
	}
	current := fitLine(pathStyle.Render(location), maxW)

	var rows []string
	switch {
	case m.err != nil:
		rows = append(rows, fitLine(dimStyle.Render("(unreadable: "+m.err.Error()+")"), maxW))
	case len(m.entries) == 0:
		rows = append(rows, dimStyle.Render("(no subdirectories)"))
	default:
		h := m.listHeight()
		end := min(m.scrollTop+h, len(m.entries))
		for i := m.scrollTop; i < end; i++ {
			prefix, style := "  ", normStyle
			if m.cursor == i {
				prefix, style = "▶ ", selStyle
			}
			rows = append(rows, fitLine(style.Render(prefix+m.entries[i]), maxW))
		}
	}

	help := dimStyle.MarginTop(1).Render(fitLine(
		"↑/↓: Navigate • Enter/→: Open • ←: Up • Space: Select • Ctrl+S: Select This Folder • Esc: Cancel",
		maxW,
	))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, current, body, help))
}

package settings

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/overlay"
	"github.com/jarvisfriends/tui-base/theme"
)

type MultiFileKeyMap struct {
	Cancel key.Binding
	Up     key.Binding
	Down   key.Binding
	Submit key.Binding
	Delete key.Binding
	Save   key.Binding
}

func DefaultMultiFileKeyMap() MultiFileKeyMap {
	return MultiFileKeyMap{
		Cancel: key.NewBinding(key.WithKeys("esc", "ctrl+c", "q")),
		Up:     key.NewBinding(key.WithKeys("up")),
		Down:   key.NewBinding(key.WithKeys("down")),
		Submit: key.NewBinding(key.WithKeys("enter")),
		Delete: key.NewBinding(key.WithKeys("delete", "d", "backspace")),
		Save:   key.NewBinding(key.WithKeys(keyCtrlS)),
	}
}

type MultiFileEditor struct {
	paths       []string
	cursor      int
	picking     bool
	pickerForm  *huh.Form
	dirPicker   *DirPicker
	pickerIndex int

	KeyMap MultiFileKeyMap
	// DirsOnly makes each row's picker a directory-only DirPicker (with
	// drive navigation) instead of the mixed file/folder browser.
	DirsOnly bool
	Done     bool
	Aborted  bool
	Width    int
	Height   int
}

func NewMultiFileEditor(value string) *MultiFileEditor {
	var paths []string
	if strings.TrimSpace(value) != "" {
		parts := strings.SplitSeq(value, ";")
		for p := range parts {
			paths = append(paths, strings.TrimSpace(p))
		}
	}
	return &MultiFileEditor{
		paths:  paths,
		KeyMap: DefaultMultiFileKeyMap(),
	}
}

func (m *MultiFileEditor) Value() string {
	return strings.Join(m.paths, "; ")
}

func (m *MultiFileEditor) Init() tea.Cmd {
	return nil
}

// updatePicker forwards msg to the active picker form (keeping any replacement
// model it returns) and applies the completed/aborted result.
func (m *MultiFileEditor) updatePicker(msg tea.Msg) tea.Cmd {
	model, cmd := m.pickerForm.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		m.pickerForm = f
	}
	switch m.pickerForm.State {
	case huh.StateCompleted:
		m.applyPickedPath(m.pickerForm.GetString("path"))
		m.picking = false
		m.pickerForm = nil
	case huh.StateAborted:
		m.picking = false
		m.pickerForm = nil
	case huh.StateNormal:
		// form still in progress — no action
	}
	return cmd
}

// updateDirPicker forwards msg to the active directory picker and applies
// the completed/aborted result.
func (m *MultiFileEditor) updateDirPicker(msg tea.Msg) tea.Cmd {
	model, cmd := m.dirPicker.Update(msg)
	if dp, ok := model.(*DirPicker); ok {
		m.dirPicker = dp
	}
	switch {
	case m.dirPicker.Done:
		m.applyPickedPath(m.dirPicker.Value())
		m.picking = false
		m.dirPicker = nil
	case m.dirPicker.Aborted:
		m.picking = false
		m.dirPicker = nil
	}
	return cmd
}

// applyPickedPath stores a non-empty picker result at the pending index,
// appending when the pick targeted the "add new" slot.
func (m *MultiFileEditor) applyPickedPath(val string) {
	if val == "" {
		return
	}
	if m.pickerIndex == len(m.paths) {
		m.paths = append(m.paths, val)
	} else {
		m.paths[m.pickerIndex] = val
	}
}

func (m *MultiFileEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.Width, m.Height = ws.Width, ws.Height
	}

	if m.picking && m.dirPicker != nil {
		return m, m.updateDirPicker(msg)
	}
	if m.picking && m.pickerForm != nil {
		return m, m.updatePicker(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Cancel):
			m.Aborted = true
			return m, nil
		case key.Matches(msg, m.KeyMap.Up):
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.paths) // "Add Row" is at the bottom
			}
		case key.Matches(msg, m.KeyMap.Down):
			m.cursor++
			if m.cursor > len(m.paths) {
				m.cursor = 0
			}
		case key.Matches(msg, m.KeyMap.Submit):
			return m, m.startPicking(m.cursor)
		case key.Matches(msg, m.KeyMap.Delete):
			if m.cursor < len(m.paths) {
				m.paths = append(m.paths[:m.cursor], m.paths[m.cursor+1:]...)
				if m.cursor > len(m.paths) {
					m.cursor = len(m.paths)
				}
			}
		case key.Matches(msg, m.KeyMap.Save):
			m.Done = true
			return m, nil
		}
	}

	return m, nil
}

// startPicking opens the per-row picker form and returns its Init command.
// The command must be dispatched to the runtime: it carries the picker's
// readDir, and bubbles' filepicker opens with a collapsed one-row browse
// window until that readDir's message arrives and expands it.
func (m *MultiFileEditor) startPicking(index int) tea.Cmd {
	m.pickerIndex = index
	initialValue := ""
	if index < len(m.paths) {
		initialValue = m.paths[index]
	}

	if m.DirsOnly {
		dp := NewDirPicker(initialValue)
		dp.Width, dp.Height = m.Width, m.Height
		m.dirPicker = dp
		m.picking = true
		return dp.Init()
	}

	fp := huh.NewFilePicker().
		Key("path").
		Title("Select Path").
		DirAllowed(true).
		FileAllowed(true).
		// Open directly in browse mode: the embedded picker otherwise
		// defaults to a one-row list.
		Picking(true).
		Value(&initialValue)
	if m.Height > 0 {
		// Fill most of the available area with the file listing.
		fp.Height(overlay.FormHeight(m.Height))
	}

	m.pickerForm = huh.NewForm(huh.NewGroup(fp)).
		WithTheme(theme.HuhThemeFunc()).
		WithKeyMap(filePickerKeyMap()).
		WithWidth(overlayContentWidth(m.Width))
	if m.Height > 0 {
		// The form height must be set explicitly: huh freezes each group's
		// height from the fields' first render, which for a picker is the
		// collapsed pre-readDir browse list, and the zoomed picker gets that
		// frozen height re-imposed on every render.
		m.pickerForm = m.pickerForm.WithHeight(overlay.FormHeight(m.Height))
	}
	m.picking = true
	initCmd := m.pickerForm.Init()
	if m.Width > 0 && m.Height > 0 {
		// Deliver the current size the way tea.Program would on open:
		// bubbles' filepicker starts with a collapsed one-row browse window
		// that only its WindowSizeMsg handler unconditionally expands —
		// setting heights through huh's builder API alone leaves the window
		// collapsed until the user resizes the terminal or changes
		// directory. Sized here, before initCmd's readDir message arrives,
		// the first directory listing renders at full height.
		model, _ := m.pickerForm.Update(tea.WindowSizeMsg{
			Width:  overlayContentWidth(m.Width),
			Height: overlay.FormHeight(m.Height),
		})
		if f, ok := model.(*huh.Form); ok {
			m.pickerForm = f
		}
	}
	return initCmd
}

func (m *MultiFileEditor) View() tea.View {
	if m.picking && m.dirPicker != nil {
		return m.dirPicker.View()
	}
	if m.picking && m.pickerForm != nil {
		return tea.NewView(m.pickerForm.View())
	}

	c := theme.Active()
	maxW := overlayContentWidth(m.Width)
	title := c.Styles.Title.Bold(true).Padding(0, 1).Render("Multi-File Picker")

	var rows []string
	selStyle := c.Styles.SelectedItem.Bold(true)
	normStyle := c.Styles.TextOnBg
	delStyle := c.Styles.Dim

	for i, p := range m.paths {
		prefix := "  "
		style := normStyle
		if m.cursor == i {
			prefix = "▶ "
			style = selStyle
		}
		rows = append(rows, fitLine(style.Render(prefix+p), maxW))
	}

	addPrefix := "  "
	addStyle := normStyle
	if m.cursor == len(m.paths) {
		addPrefix = "▶ "
		addStyle = selStyle
	}
	rows = append(rows, addStyle.Render(addPrefix+"[ Add Path ]"))

	help := delStyle.MarginTop(1).Render(fitLine(
		"↑/↓: Navigate • Enter: Edit/Add • Del: Remove • Ctrl+S: Save • Esc: Cancel",
		maxW,
	))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, body, help))
}

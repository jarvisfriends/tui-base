package settings

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"

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
	pickerIndex int

	KeyMap  MultiFileKeyMap
	Done    bool
	Aborted bool
	Width   int
	Height  int
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

func (m *MultiFileEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.picking && m.pickerForm != nil {
		_, cmd := m.pickerForm.Update(msg)
		switch m.pickerForm.State {
		case huh.StateCompleted:
			val := m.pickerForm.GetString("path")
			if val != "" {
				if m.pickerIndex == len(m.paths) {
					m.paths = append(m.paths, val)
				} else {
					m.paths[m.pickerIndex] = val
				}
			}
			m.picking = false
			m.pickerForm = nil
		case huh.StateAborted:
			m.picking = false
			m.pickerForm = nil
		case huh.StateNormal:
			// form still in progress — no action
		}
		return m, cmd
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
			m.startPicking(m.cursor)
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

func (m *MultiFileEditor) startPicking(index int) {
	m.pickerIndex = index
	initialValue := ""
	if index < len(m.paths) {
		initialValue = m.paths[index]
	}

	fp := huh.NewFilePicker().
		Key("path").
		Title("Select Path").
		DirAllowed(true).
		FileAllowed(true).
		Value(&initialValue)

	m.pickerForm = huh.NewForm(huh.NewGroup(fp)).WithTheme(theme.HuhThemeFunc())
	m.pickerForm.Init()
	m.picking = true
}

func (m *MultiFileEditor) View() tea.View {
	if m.picking && m.pickerForm != nil {
		return tea.NewView(m.pickerForm.View())
	}

	title := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("Multi-File Picker")

	var rows []string
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	normStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	for i, p := range m.paths {
		prefix := "  "
		style := normStyle
		if m.cursor == i {
			prefix = "▶ "
			style = selStyle
		}
		rows = append(rows, style.Render(prefix+p))
	}

	addPrefix := "  "
	addStyle := normStyle
	if m.cursor == len(m.paths) {
		addPrefix = "▶ "
		addStyle = selStyle
	}
	rows = append(rows, addStyle.Render(addPrefix+"[ Add Path ]"))

	help := delStyle.MarginTop(1).Render("↑/↓: Navigate • Enter: Edit/Add • Del: Remove • Ctrl+S: Save • Esc: Cancel")

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, title, body, help))
}

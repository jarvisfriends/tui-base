// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/theme"
)

type KeyRecorder struct {
	keys      []string
	cursor    int
	recording bool

	Validate func([]string) error
	Error    string

	Done    bool
	Aborted bool
	Width   int
	Height  int
}

func NewKeyRecorder(value string) *KeyRecorder {
	var keys []string
	if strings.TrimSpace(value) != "" {
		parts := strings.SplitSeq(value, ",")
		for p := range parts {
			if strings.TrimSpace(p) != "" {
				keys = append(keys, strings.TrimSpace(p))
			}
		}
	}
	return &KeyRecorder{
		keys: keys,
	}
}

func (m *KeyRecorder) validate() {
	if m.Validate != nil {
		if err := m.Validate(m.keys); err != nil {
			m.Error = err.Error()
		} else {
			m.Error = ""
		}
	}
}

func (m *KeyRecorder) Value() string {
	return strings.Join(m.keys, ",")
}

func (m *KeyRecorder) Init() tea.Cmd {
	return nil
}

func (m *KeyRecorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.recording {
			// Stop recording and save the key
			s := msg.String()

			// Let user cancel recording with Esc
			if msg.Code == tea.KeyEscape {
				m.recording = false
				return m, nil
			}

			if m.cursor == len(m.keys) {
				m.keys = append(m.keys, s)
			} else {
				m.keys[m.cursor] = s
			}
			m.recording = false
			m.validate()
			return m, nil
		}

		switch msg.Code {
		case tea.KeyEscape:
			m.Aborted = true
			return m, nil
		case tea.KeyUp:
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.keys)
			}
		case tea.KeyDown:
			m.cursor++
			if m.cursor > len(m.keys) {
				m.cursor = 0
			}
		case tea.KeyEnter:
			m.recording = true
		case tea.KeyDelete, tea.KeyBackspace:
			if m.cursor < len(m.keys) {
				m.keys = append(m.keys[:m.cursor], m.keys[m.cursor+1:]...)
				if m.cursor > len(m.keys) {
					m.cursor = len(m.keys)
				}
				m.validate()
			}
		default:
			// Ctrl+S saves; compared by Code+Mod, not fragile string forms.
			if msg.Code == 's' && msg.Mod&tea.ModCtrl != 0 {
				m.validate()
				if m.Error == "" {
					m.Done = true
				}
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *KeyRecorder) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("Keybinding Recorder")

	c := theme.Active()
	rows := make([]string, 0, len(m.keys)+1)
	selStyle := c.Styles.SelectedItem.Bold(true)
	normStyle := c.Styles.TextOnBg
	delStyle := c.Styles.Dim
	errStyle := c.Styles.Error.Bold(true).MarginTop(1)

	for i, k := range m.keys {
		prefix := "  "
		style := normStyle
		if m.cursor == i {
			prefix = "▶ "
			style = selStyle
		}

		val := k
		if m.recording && m.cursor == i {
			val = "[ Press any key... (Esc to cancel) ]"
		}

		rows = append(rows, style.Render(prefix+val))
	}

	addPrefix := "  "
	addStyle := normStyle
	if m.cursor == len(m.keys) {
		addPrefix = "▶ "
		addStyle = selStyle
	}

	addVal := "[ Add Keybinding ]"
	if m.recording && m.cursor == len(m.keys) {
		addVal = "[ Press any key... (Esc to cancel) ]"
	}
	rows = append(rows, addStyle.Render(addPrefix+addVal))

	var errView string
	if m.Error != "" {
		errView = errStyle.Render("Error: " + m.Error)
	}

	help := delStyle.MarginTop(1).
		Render("↑/↓: Navigate • Enter: Record Key • Del: Remove • Ctrl+S: Save • Esc: Cancel")

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)

	items := []string{title, body}
	if errView != "" {
		items = append(items, errView)
	}
	items = append(items, help)

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, items...))
}

package timepicker

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Field int

const (
	FieldHours Field = iota
	FieldMinutes
	FieldSeconds
)

type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Submit key.Binding
	Quit   key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:     key.NewBinding(key.WithKeys("up")),
		Down:   key.NewBinding(key.WithKeys("down")),
		Left:   key.NewBinding(key.WithKeys("left", "shift+tab")),
		Right:  key.NewBinding(key.WithKeys("right", "tab")),
		Submit: key.NewBinding(key.WithKeys("enter")),
		Quit:   key.NewBinding(key.WithKeys("ctrl+c", "esc", "q")),
	}
}

type TimePickerModel struct {
	Duration time.Duration
	KeyMap   KeyMap
	Focused  Field
	Done     bool
	Aborted  bool

	ActiveStyle   lipgloss.Style
	InactiveStyle lipgloss.Style
	HelpStyle     lipgloss.Style
}

func New(d time.Duration) *TimePickerModel {
	return &TimePickerModel{
		Duration: d,
		KeyMap:   DefaultKeyMap(),
		Focused:  FieldHours,
		ActiveStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()),
		InactiveStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()),
		HelpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
	}
}

func (m *TimePickerModel) Init() tea.Cmd {
	return nil
}

func (m *TimePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Quit):
			m.Aborted = true
			return m, nil
		case key.Matches(msg, m.KeyMap.Submit):
			m.Done = true
			return m, nil
		case key.Matches(msg, m.KeyMap.Left):
			m.Focused = (m.Focused - 1)
			if m.Focused < FieldHours {
				m.Focused = FieldSeconds
			}
		case key.Matches(msg, m.KeyMap.Right):
			m.Focused = (m.Focused + 1)
			if m.Focused > FieldSeconds {
				m.Focused = FieldHours
			}
		case key.Matches(msg, m.KeyMap.Up):
			m.increment(1)
		case key.Matches(msg, m.KeyMap.Down):
			m.increment(-1)
		}
	case tea.MouseWheelMsg:
		if msg.Mouse().Button == tea.MouseWheelUp {
			m.increment(1)
		} else {
			m.increment(-1)
		}
	}
	return m, nil
}

func (m *TimePickerModel) increment(dir int) {
	hours := int64(m.Duration.Hours())
	minutes := int64(m.Duration.Minutes()) % 60
	seconds := int64(m.Duration.Seconds()) % 60

	switch m.Focused {
	case FieldHours:
		hours += int64(dir)
		if hours < 0 {
			hours = 0
		}
	case FieldMinutes:
		minutes += int64(dir)
		if minutes > 59 {
			minutes = 0
			hours++
		} else if minutes < 0 {
			minutes = 59
			if hours > 0 {
				hours--
			}
		}
	case FieldSeconds:
		seconds += int64(dir)
		if seconds > 59 {
			seconds = 0
			minutes++
			if minutes > 59 {
				minutes = 0
				hours++
			}
		} else if seconds < 0 {
			seconds = 59
			if minutes > 0 {
				minutes--
			} else if hours > 0 {
				minutes = 59
				hours--
			}
		}
	}
	m.Duration = time.Duration(
		hours,
	)*time.Hour + time.Duration(
		minutes,
	)*time.Minute + time.Duration(
		seconds,
	)*time.Second
}

func (m *TimePickerModel) View() tea.View {
	hours := int64(m.Duration.Hours())
	minutes := int64(m.Duration.Minutes()) % 60
	seconds := int64(m.Duration.Seconds()) % 60

	hStr := fmt.Sprintf("%02dh", hours)
	mStr := fmt.Sprintf("%02dm", minutes)
	sStr := fmt.Sprintf("%02ds", seconds)

	styleFor := func(f Field) lipgloss.Style {
		if m.Focused == f {
			return m.ActiveStyle
		}
		return m.InactiveStyle
	}

	title := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("Duration")

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		styleFor(FieldHours).Render(hStr),
		styleFor(FieldMinutes).Render(mStr),
		styleFor(FieldSeconds).Render(sStr),
	)

	help := m.HelpStyle.
		MarginTop(1).
		Render("↑/↓: Adjust • ←/→: Select • Enter: Save")

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Center, title, body, help))
}

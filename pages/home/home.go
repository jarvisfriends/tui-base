package home

import (
	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Model struct {
	width  int
	height int
	colors *theme.AppStyle
}

// SetColors stores a shared AppColors pointer so the router can update the
// theme in one place and this model sees the change immediately.
func (m *Model) SetColors(c *theme.AppStyle) { m.colors = c }

// resolveColors returns the current palette from the shared pointer, falling
// back to theme.Active() when no pointer has been set (e.g. in tests).
func (m *Model) resolveColors() *theme.AppStyle {
	if m.colors != nil {
		return m.colors
	}
	return theme.Active()
}

func New() *Model {
	return &Model{}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *Model) View() tea.View {
	c := m.resolveColors()
	bg := c.Styles.TextOnBg.GetBackground()

	// Clamp box width to terminal: border uses 2 + padding uses 4 = 6 cols overhead.
	// Ensure inner content gets at least 1 column even at very narrow widths.
	availW := max(m.width, 10)
	style := c.Styles.Success.
		Bold(true).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		MaxWidth(availW)

	content := "Welcome to the V2 Terminal Hub\n\nUse Tab to switch pages.\nCtrl+B to toggle sidebar.\nCtrl+H to toggle full help."
	box := style.Render(content)

	placed := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	v := tea.NewView(placed)
	v.BackgroundColor = bg
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	return v
}

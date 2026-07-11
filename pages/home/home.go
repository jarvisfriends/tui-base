package home

import (
	"github.com/jarvisfriends/snap/page"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const welcomeText = "Welcome to the V2 Terminal Hub\n\nUse Tab to switch pages.\nCtrl+B to toggle sidebar.\nCtrl+H to toggle full help."

type HomePageModel struct {
	page.Base

	// vp scrolls the welcome content. On a normal terminal the content fits and
	// the viewport is a no-op; on a very small terminal it scrolls (mouse wheel
	// and up/down/PgUp/PgDn) instead of clipping.
	vp viewport.Model
	// lastContent guards SetContent so we only reset the viewport (and its scroll
	// position) when the rendered content actually changes — not every frame.
	lastContent string
}

func New() *HomePageModel {
	return &HomePageModel{vp: viewport.New()}
}

func (m *HomePageModel) Init() tea.Cmd { return nil }

func (m *HomePageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(ws.Width, ws.Height)
		m.vp.SetWidth(ws.Width)
		m.vp.SetHeight(ws.Height)
		m.syncContent()
		return m, nil
	}
	// Forward everything else (keys, mouse wheel) to the viewport so it scrolls.
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// content renders the centered welcome box, padded to at least the viewport
// height so it stays vertically centered when it fits and scrolls when it does not.
func (m *HomePageModel) content() string {
	c := m.Colors()
	availW := max(m.Width(), 10)
	box := c.Styles.Success.
		Bold(true).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		MaxWidth(availW).
		Render(welcomeText)

	h := max(m.Height(), lipgloss.Height(box))
	fill := lipgloss.NewStyle().Background(c.Styles.TextOnBg.GetBackground())
	return lipgloss.Place(m.Width(), h, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceStyle(fill))
}

// syncContent updates the viewport content only when it has changed, preserving
// the scroll position across unrelated re-renders (e.g. live theme ticks).
func (m *HomePageModel) syncContent() {
	s := m.content()
	if s != m.lastContent {
		m.vp.SetContent(s)
		m.lastContent = s
	}
}

func (m *HomePageModel) View() tea.View {
	c := m.Colors()
	m.syncContent()
	v := tea.NewView(m.vp.View())
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	return v
}

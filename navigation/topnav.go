package navigation

import (
	"fmt"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"github.com/jarvisfriends/tui-base/page"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// MinimalTopNav is a compact top-docked navigator styled like the inspector's
// tab line: a single horizontal row of labels, the active one highlighted, with
// no borders. A leading per-tab number ("1:", "2:", …) is optional and defaults
// to hidden. The number keys 1–9 select a tab directly only when the user has
// enabled "Number Key Select" in Settings (off by default); the router maps the
// digits for top-docked navs independently of whether the prefix is shown.
type MinimalTopNav struct {
	Pages       []Page
	ActiveIndex int
	ShowNumbers bool
	KeyMap      NavKeyMap

	width  int
	height int
	starts []int // per-tab click ranges, rebuilt in View()
	ends   []int
	page.Base
}

// NewMinimalTopNav returns a minimal top nav with the default page set and the
// number prefixes hidden.
func NewMinimalTopNav() *MinimalTopNav {
	return &MinimalTopNav{
		Pages: []Page{
			{ID: "home", Title: "Home"},
			{ID: "debug", Title: "Inspector"},
			{ID: "settings", Title: "Settings"},
		},
		ActiveIndex: 0,
		ShowNumbers: false,
		KeyMap:      DefaultNavKeyMap(),
	}
}

func (m *MinimalTopNav) Init() tea.Cmd { return nil }

func (m *MinimalTopNav) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		if len(m.Pages) == 0 {
			return m, nil
		}
		switch {
		case key.Matches(msg, m.KeyMap.PreviousPage):
			m.ActiveIndex = (m.ActiveIndex - 1 + len(m.Pages)) % len(m.Pages)
			return m, m.selectCmd()
		case key.Matches(msg, m.KeyMap.NextPage):
			m.ActiveIndex = (m.ActiveIndex + 1) % len(m.Pages)
			return m, m.selectCmd()
		case key.Matches(msg, m.KeyMap.Select):
			return m, m.selectCmd()
		default:
			// Number keys select directly (always active, even when prefixes hidden).
			if i, ok := digitIndex(msg.Text); ok && i < len(m.Pages) {
				m.ActiveIndex = i
				return m, m.selectCmd()
			}
		}
	}
	return m, nil
}

func (m *MinimalTopNav) selectCmd() tea.Cmd {
	idx := m.ActiveIndex
	return func() tea.Msg { return SelectedMsg{PageIndex: idx} }
}

func (m *MinimalTopNav) label(i int, title string) string {
	if m.ShowNumbers {
		return fmt.Sprintf("%d:%s", i+1, title)
	}
	return title
}

func (m *MinimalTopNav) View() tea.View {
	c := m.Colors()

	activeStyle := lipgloss.NewStyle().
		Background(c.Accent).
		Foreground(c.Bg).
		Bold(true).
		Padding(0, 1)
	inactiveStyle := c.Styles.Item.Padding(0, 1)

	parts := make([]string, 0, len(m.Pages))
	m.starts = make([]int, len(m.Pages))
	m.ends = make([]int, len(m.Pages))
	x := 0
	for i, p := range m.Pages {
		style := inactiveStyle
		if i == m.ActiveIndex {
			style = activeStyle
		}
		rendered := style.Render(m.label(i, p.Title))
		parts = append(parts, rendered)
		w := lipgloss.Width(rendered)
		m.starts[i] = x
		if w > 0 {
			m.ends[i] = x + w - 1
		} else {
			m.ends[i] = x
		}
		x += w
	}

	row := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	v := tea.NewView(row)
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		switch ev := mm.(type) {
		case tea.MouseClickMsg, tea.MouseReleaseMsg:
			me := ev.Mouse()
			if me.Button != tea.MouseLeft {
				return nil
			}
			if me.Y < 0 || me.Y >= lipgloss.Height(v.Content) {
				return nil
			}
			for i := range m.Pages {
				if me.X >= m.starts[i] && me.X <= m.ends[i] {
					m.ActiveIndex = i
					return m.selectCmd()
				}
			}
		}
		return nil
	}
	return v
}

// SetShowNumbers toggles the leading per-tab number prefix.
func (m *MinimalTopNav) SetShowNumbers(show bool) { m.ShowNumbers = show }

// Width reports the horizontal layout space consumed; a top nav stacks above
// content and reserves no side width.
func (m *MinimalTopNav) Width() int  { return 0 }
func (m *MinimalTopNav) Height() int { return lipgloss.Height(m.View().Content) }

func (m *MinimalTopNav) Dock() Side           { return DockTop }
func (m *MinimalTopNav) GetPages() []Page     { return m.Pages }
func (m *MinimalTopNav) SetPages(p []Page)    { m.Pages = p }
func (m *MinimalTopNav) SetActiveIndex(i int) { m.ActiveIndex = i }
func (m *MinimalTopNav) GetActiveIndex() int  { return m.ActiveIndex }

// digitIndex maps a single key string "1".."9" to a zero-based index.
func digitIndex(s string) (int, bool) {
	if utf8.RuneCountInString(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		return int(s[0] - '1'), true
	}
	return 0, false
}

var (
	_ Navigator     = (*MinimalTopNav)(nil)
	_ NumberLabeled = (*MinimalTopNav)(nil)
)

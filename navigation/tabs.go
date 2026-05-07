package navigation

import (
	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Tabs struct {
	Pages       []Page
	ActiveIndex int
	HoverIndex  int
	width       int
	height      int
	colors      *theme.AppStyle
}

// SetColors stores a shared AppColors pointer so the router can update the
// theme in one place and all components see the change immediately.
func (m *Tabs) SetColors(c *theme.AppStyle) { m.colors = c }

// resolveColors returns the current palette from the shared pointer, falling
// back to theme.Active() when no pointer has been set (e.g. in tests).
func (m *Tabs) resolveColors() *theme.AppStyle {
	if m.colors != nil {
		return m.colors
	}
	return theme.Active()
}

func NewTabs() *Tabs {
	return &Tabs{
		Pages: []Page{
			{ID: "home", Title: "Home"},
			{ID: "debug", Title: "Inspector"},
			{ID: "settings", Title: "Settings"},
		},
		ActiveIndex: 0,
		HoverIndex:  -1,
	}
}

func (m *Tabs) Init() tea.Cmd { return nil }

func (m *Tabs) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case TabHoverMsg:
		if m.HoverIndex != msg.Index {
			m.HoverIndex = msg.Index
		}
	case tea.KeyMsg:
		switch keyMsg := msg.(type) {
		case tea.KeyPressMsg:
			switch keyMsg.String() {
			case "left", "shift+tab":
				if len(m.Pages) > 0 {
					m.ActiveIndex = (m.ActiveIndex - 1 + len(m.Pages)) % len(m.Pages)
					return m, func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
				}
			case "right", "tab":
				if len(m.Pages) > 0 {
					m.ActiveIndex = (m.ActiveIndex + 1) % len(m.Pages)
					return m, func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
				}
			case "enter":
				if m.ActiveIndex >= 0 && m.ActiveIndex < len(m.Pages) {
					return m, func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
				}
			}
		}
	}
	return m, nil
}

// TabHoverMsg reports that the mouse is over a tab index (or -1 for none).
type TabHoverMsg struct{ Index int }

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

func (m *Tabs) View() tea.View {
	c := m.resolveColors()

	inactiveTabBorder := tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder := tabBorderWithBottom("┘", " ", "└")
	inactiveTabStyle := c.Styles.TabInactive.Border(inactiveTabBorder, true).Padding(0, 1)
	activeTabStyle := inactiveTabStyle.Border(activeTabBorder, true)
	hoverTabStyle := c.Styles.TabHover.Border(inactiveTabBorder, true).Padding(0, 1)

	var rendered []string
	var tabWidths []int
	for i, t := range m.Pages {
		var style lipgloss.Style
		switch i {
		case m.ActiveIndex:
			style = activeTabStyle
		case m.HoverIndex:
			style = hoverTabStyle
		default:
			style = inactiveTabStyle
		}
		s := style.Render(t.Title)
		rendered = append(rendered, s)
		tabWidths = append(tabWidths, lipgloss.Width(s))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	tabWidth := lipgloss.Width(row)
	rightStyle := inactiveTabStyle.Width(max(0, m.width-tabWidth)).Border(inactiveTabBorder, false, false, true, false)
	styled := lipgloss.JoinHorizontal(lipgloss.Top, row, rightStyle.Render("\n"))

	// ensure the tab row fits the width
	// styled := docStyle.Width(m.width).Render(row + windowStyle.Width(m.width-tabWidth).Render(""))

	v := tea.NewView(styled)
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	// compute X ranges for each tab so we can map a click X coordinate to a tab
	starts := make([]int, len(tabWidths))
	ends := make([]int, len(tabWidths))
	cur := 0
	for i, w := range tabWidths {
		starts[i] = cur
		if w > 0 {
			ends[i] = cur + w - 1
		} else {
			ends[i] = cur
		}
		cur += w
	}

	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		switch ev := mm.(type) {
		case tea.MouseClickMsg, tea.MouseReleaseMsg:
			me := ev.Mouse()
			if me.Button != tea.MouseLeft {
				return nil
			}
			x := me.X
			y := me.Y
			// ensure click is within the tab view vertical bounds
			if y < 0 || y >= lipgloss.Height(v.Content) {
				return nil
			}
			for i := range m.Pages {
				if x >= starts[i] && x <= ends[i] {
					m.ActiveIndex = i
					return func() tea.Msg { return SelectedMsg{PageIndex: i} }
				}
			}
			return nil
		case tea.MouseMotionMsg:
			me := ev.Mouse()
			x := me.X
			y := me.Y
			if y < 0 || y >= lipgloss.Height(v.Content) {
				// outside vertical bounds, clear hover if needed
				if m.HoverIndex != -1 {
					return func() tea.Msg { return TabHoverMsg{Index: -1} }
				}
				return nil
			}
			// determine which tab (if any) the mouse is over
			for i := range m.Pages {
				if x >= starts[i] && x <= ends[i] {
					if m.HoverIndex != i {
						return func() tea.Msg { return TabHoverMsg{Index: i} }
					}
					return nil
				}
			}
			if m.HoverIndex != -1 {
				return func() tea.Msg { return TabHoverMsg{Index: -1} }
			}
			return nil
		default:
			return nil
		}
	}

	return v
}

// Width reports the horizontal layout space consumed by this navigator.
// Tabs are stacked above content, so they do not consume side width.
func (m *Tabs) Width() int           { return 0 }
func (m *Tabs) Height() int          { return lipgloss.Height(m.View().Content) }
func (m *Tabs) GetPages() []Page     { return m.Pages }
func (m *Tabs) SetPages(p []Page)    { m.Pages = p }
func (m *Tabs) SetActiveIndex(i int) { m.ActiveIndex = i }
func (m *Tabs) GetActiveIndex() int  { return m.ActiveIndex }

var _ Navigator = (*Tabs)(nil)

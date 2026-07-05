package navigation

import (
	"github.com/jarvisfriends/tui-base/page"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Tabs struct {
	Pages       []Page
	ActiveIndex int
	HoverIndex  int
	KeyMap      NavKeyMap
	width       int
	height      int
	page.Base
}

func NewTabs() *Tabs {
	return &Tabs{
		Pages: []Page{
			{ID: pageIDHome, Title: pageHome},
			{ID: pageIDInspector, Title: pageInspector},
			{ID: pageIDSettings, Title: pageSettings},
		},
		ActiveIndex: 0,
		HoverIndex:  -1,
		KeyMap:      DefaultNavKeyMap(),
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
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.PreviousPage):
			if len(m.Pages) > 0 {
				m.ActiveIndex = (m.ActiveIndex - 1 + len(m.Pages)) % len(m.Pages)
				return m, func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
			}
		case key.Matches(msg, m.KeyMap.NextPage):
			if len(m.Pages) > 0 {
				m.ActiveIndex = (m.ActiveIndex + 1) % len(m.Pages)
				return m, func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
			}
		case key.Matches(msg, m.KeyMap.Select):
			if m.ActiveIndex >= 0 && m.ActiveIndex < len(m.Pages) {
				return m, func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
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

// computeTabWindow returns the inclusive range [first, last] of tab indices to
// render so the active tab stays visible within avail columns, plus whether the
// left/right overflow arrows should be shown. leftW/rightW are the rendered
// widths of those arrows; they are reserved only on the side that is actually
// clipped. When all tabs fit, it returns the full range with no arrows so the
// caller falls back to a plain, unscrolled row.
//
// The window is the left-most range that still reaches the active tab, so
// paging right reveals the active tab at the right edge and paging left reveals
// it at the left edge — predictable, terminal-pager behavior with no wrapping.
func computeTabWindow(
	widths []int,
	avail, active, leftW, rightW int,
) (first, last int, showLeft, showRight bool) {
	n := len(widths)
	if n == 0 {
		return 0, -1, false, false
	}
	total := 0
	for _, w := range widths {
		total += w
	}
	if avail <= 0 || total <= avail {
		return 0, n - 1, false, false
	}
	active = max(0, min(active, n-1))
	for first = range active + 1 {
		showLeft = first > 0
		budget := avail
		if showLeft {
			budget -= leftW
		}
		used := 0
		last = first - 1
		for i := first; i < n; i++ {
			b := budget
			if i < n-1 {
				// A following tab may still be clipped; reserve the right arrow.
				b -= rightW
			}
			// Always show at least the first windowed tab, even if it alone
			// overflows, so the loop always makes progress.
			if i == first || used+widths[i] <= b {
				used += widths[i]
				last = i
			} else {
				break
			}
		}
		showRight = last < n-1
		if last >= active {
			return first, last, showLeft, showRight
		}
	}
	// Degenerate fallback: a single tab wider than avail — show just the active.
	return active, active, active > 0, active < n-1
}

func (m *Tabs) View() tea.View {
	c := m.Colors()

	inactiveTabBorder := tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder := tabBorderWithBottom("┘", " ", "└")
	inactiveTabStyle := c.Styles.TabInactive.Border(inactiveTabBorder, true).Padding(0, 1)
	activeTabStyle := inactiveTabStyle.Border(activeTabBorder, true)
	hoverTabStyle := c.Styles.TabHover.Border(inactiveTabBorder, true).Padding(0, 1)

	rendered := make([]string, 0, len(m.Pages))
	tabWidths := make([]int, 0, len(m.Pages))
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

	// Overflow arrows are styled like inactive tabs so they align with the row's
	// border height. They are reserved only on the side that is actually clipped.
	arrowStyle := inactiveTabStyle
	leftArrow := arrowStyle.Render("‹")
	rightArrow := arrowStyle.Render("›")
	leftArrowW := lipgloss.Width(leftArrow)
	rightArrowW := lipgloss.Width(rightArrow)

	first, last, showLeft, showRight := computeTabWindow(
		tabWidths,
		m.width,
		m.ActiveIndex,
		leftArrowW,
		rightArrowW,
	)

	// Assemble the visible row (optional left arrow, windowed tabs, optional
	// right arrow) and record on-screen X ranges so clicks map to the right tab.
	// Off-window tabs get an impossible range so they never match a click.
	starts := make([]int, len(tabWidths))
	ends := make([]int, len(tabWidths))
	for i := range starts {
		starts[i], ends[i] = -1, -2
	}

	var segments []string
	cur := 0
	leftArrowStart, leftArrowEnd := -1, -2
	if showLeft {
		segments = append(segments, leftArrow)
		leftArrowStart, leftArrowEnd = cur, cur+leftArrowW-1
		cur += leftArrowW
	}
	for i := first; i <= last; i++ {
		segments = append(segments, rendered[i])
		w := tabWidths[i]
		starts[i] = cur
		if w > 0 {
			ends[i] = cur + w - 1
		} else {
			ends[i] = cur
		}
		cur += w
	}
	rightArrowStart, rightArrowEnd := -1, -2
	if showRight {
		segments = append(segments, rightArrow)
		rightArrowStart, rightArrowEnd = cur, cur+rightArrowW-1
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, segments...)
	rowWidth := lipgloss.Width(row)
	styled := row
	if rowWidth < m.width {
		rightStyle := c.Styles.TabInactive.Width(m.width-rowWidth).
			Border(inactiveTabBorder, false, false, true, false)
		styled = lipgloss.JoinHorizontal(lipgloss.Top, row, rightStyle.Render("\n"))
	}

	v := tea.NewView(styled)
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		// Horizontal wheel scrolls through the tabs; the render window follows
		// the active tab, so this also pages hidden tabs into view.
		if d := horizontalWheelDelta(mm); d != 0 && len(m.Pages) > 0 {
			me := mm.Mouse()
			if me.Y < 0 || me.Y >= lipgloss.Height(v.Content) {
				return nil
			}
			m.ActiveIndex = (m.ActiveIndex + d + len(m.Pages)) % len(m.Pages)
			return func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
		}
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
			// Clicking an overflow arrow pages to the next hidden tab on that
			// side, which both scrolls the window and switches pages.
			if showLeft && x >= leftArrowStart && x <= leftArrowEnd && first > 0 {
				m.ActiveIndex = first - 1
				return func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
			}
			if showRight && x >= rightArrowStart && x <= rightArrowEnd && last < len(m.Pages)-1 {
				m.ActiveIndex = last + 1
				return func() tea.Msg { return SelectedMsg{PageIndex: m.ActiveIndex} }
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
func (m *Tabs) Width() int  { return 0 }
func (m *Tabs) Height() int { return lipgloss.Height(m.View().Content) }

// Dock reports that the tab bar occupies the top edge.
func (m *Tabs) Dock() Side           { return DockTop }
func (m *Tabs) GetPages() []Page     { return m.Pages }
func (m *Tabs) SetPages(p []Page)    { m.Pages = p }
func (m *Tabs) SetActiveIndex(i int) { m.ActiveIndex = i }
func (m *Tabs) GetActiveIndex() int  { return m.ActiveIndex }

var _ Navigator = (*Tabs)(nil)

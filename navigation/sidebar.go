package navigation

import (
	"fmt"
	"image/color"
	"io"
	"strings"

	"github.com/jarvisfriends/tui-base/page"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ─── list item ───────────────────────────────────────────────────────────────

// pageItem adapts a navigation.Page to the bubbles/list.Item interface.
type pageItem struct {
	id    string
	title string
}

func (p pageItem) FilterValue() string { return p.title }

// ─── delegate ─────────────────────────────────────────────────────────────────

// navDelegate is a custom list.ItemDelegate. It owns the active-index state so
// the cursor can span across the pinned Settings item without relying on the
// list's own internal selection index.
type navDelegate struct {
	activeIdx      int // index within main list of the active page; -1 = none in main list
	sidebarFocused bool

	focusedStyle lipgloss.Style // ▶ item — sidebar has keyboard focus
	activeStyle  lipgloss.Style // ● item — active page when sidebar unfocused
	normalStyle  lipgloss.Style
	itemWidth    int // text-render width (terminal columns)
}

// navItemHeight and navItemSpacing define the list delegate's row geometry: one
// rendered row per item plus a blank spacing row between items. handleMouse uses
// the resulting stride to map a click row back to an item index.
const (
	navItemHeight  = 1
	navItemSpacing = 1
	navItemStride  = navItemHeight + navItemSpacing // rows occupied per list item
)

func (d navDelegate) Height() int                             { return navItemHeight }
func (d navDelegate) Spacing() int                            { return navItemSpacing }
func (d navDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d navDelegate) Render(w io.Writer, _ list.Model, index int, item list.Item) {
	pi, ok := item.(pageItem)
	if !ok {
		return
	}
	prefix := "  "
	style := d.normalStyle
	if index == d.activeIdx {
		if d.sidebarFocused {
			prefix = "▶ "
			style = d.focusedStyle
		} else {
			prefix = "● "
			style = d.activeStyle
		}
	}
	_, _ = fmt.Fprintf(w, "%s%s", prefix, style.Width(max(d.itemWidth, 1)).Render(pi.title))
}

// ─── constants ────────────────────────────────────────────────────────────────

const (
	sidebarCollapsedWidth = 3 // columns when collapsed (shows expand button only)
)

// ─── Sidebar ──────────────────────────────────────────────────────────────────

// Sidebar is a panel-style Navigator backed by a bubbles/list for the main
// navigation items, with the Settings page pinned to the very bottom.
type Sidebar struct {
	// mainList contains all pages except the pinned Settings page.
	mainList list.Model
	// Pages is the full page list (Settings last when present).
	Pages []Page
	// settingsIdx is the index of the Settings page in Pages, or -1 if absent.
	settingsIdx int

	// ActiveIndex is the globally active page index (mirrors the router).
	ActiveIndex int

	// focused is true when the sidebar holds keyboard focus.
	focused bool

	// collapsed switches the sidebar to a narrow (3-column) strip.
	collapsed bool

	// expandedWidth is computed from page title lengths; replaces the old magic constant.
	expandedWidth int

	keyMap NavKeyMap

	width  int
	height int
	page.Base
}

// New creates a Sidebar with the standard Home / Inspector / Settings pages.
func New() *Sidebar {
	pages := []Page{
		{ID: "home", Title: "Home"},
		{ID: "debug", Title: "Inspector"},
		{ID: "settings", Title: "Settings"},
	}
	sb := &Sidebar{
		Pages:       pages,
		settingsIdx: 2,
		ActiveIndex: 0,
		keyMap:      DefaultNavKeyMap(),
	}
	sb.expandedWidth = sb.computeExpandedWidth()
	sb.width = sb.expandedWidth
	sb.rebuildList()
	return sb
}

// computeExpandedWidth derives the sidebar width from the longest page title.
// prefix (2 cols) + title + right clearance (2 cols), minimum 12.
func (m *Sidebar) computeExpandedWidth() int {
	maxTitle := 0
	for _, p := range m.Pages {
		if w := lipgloss.Width(p.Title); w > maxTitle {
			maxTitle = w
		}
	}
	return max(maxTitle+4, 12)
}

// rebuildList recreates the bubbles/list model from Pages, excluding Settings.
func (m *Sidebar) rebuildList() {
	var items []list.Item
	for i, p := range m.Pages {
		if i == m.settingsIdx {
			continue
		}
		items = append(items, pageItem{id: p.ID, title: p.Title})
	}
	// Height is corrected by the first WindowSizeMsg; use 1 as a safe default.
	l := list.New(items, navDelegate{}, max(m.width-2, 1), max(len(items), 1))
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	m.mainList = l
	m.syncListCursor()
}

// syncListCursor tells the bubbles/list which item to highlight.
func (m *Sidebar) syncListCursor() {
	if m.settingsIdx < 0 || m.ActiveIndex < m.settingsIdx {
		idx := max(m.ActiveIndex, 0)
		if idx < len(m.mainList.Items()) {
			m.mainList.Select(idx)
		}
	}
}

// mainListActiveIdx returns the active index within the main list, or -1 when
// the active page is the pinned Settings item (rendered outside the list).
func (m *Sidebar) mainListActiveIdx() int {
	if m.settingsIdx >= 0 && m.ActiveIndex == m.settingsIdx {
		return -1
	}
	return m.ActiveIndex
}

// numMainItems returns how many pages are shown in the main list (all – Settings).
func (m *Sidebar) numMainItems() int {
	n := len(m.Pages)
	if m.settingsIdx >= 0 {
		n--
	}
	return n
}

// ─── tea.Model ────────────────────────────────────────────────────────────────

func (m *Sidebar) Init() tea.Cmd { return nil }

func (m *Sidebar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.collapsed {
			m.width = sidebarCollapsedWidth
		} else {
			// Use the full expanded width unless the terminal is too narrow.
			// Always leave at least 10 columns for the page content area.
			m.width = min(m.expandedWidth, max(msg.Width-10, sidebarCollapsedWidth))
		}
		m.height = msg.Height
		innerW := max(m.width-2, 1)
		// Reserve header(1) + separator(1) + settings(1) rows; list gets the rest.
		listH := max(m.height-3, 1)
		m.mainList.SetWidth(innerW)
		m.mainList.SetHeight(listH)
		return m, nil

	case NavFocusMsg:
		m.focused = msg.Focused
		return m, nil

	case CollapseToggleMsg:
		m.collapsed = !m.collapsed
		if m.collapsed {
			m.width = sidebarCollapsedWidth
		} else {
			m.width = m.expandedWidth
		}
		return m, nil

	case tea.KeyMsg:
		// The router controls whether key events reach the sidebar (sidebarFocused
		// gate). We process whatever arrives so direct unit tests stay simple.
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch {
			case key.Matches(keyMsg, m.keyMap.Up):
				m.ActiveIndex = (m.ActiveIndex - 1 + len(m.Pages)) % len(m.Pages)
				m.syncListCursor()
				return m, m.emitSelected()
			case key.Matches(keyMsg, m.keyMap.Down):
				m.ActiveIndex = (m.ActiveIndex + 1) % len(m.Pages)
				m.syncListCursor()
				return m, m.emitSelected()
			case key.Matches(keyMsg, m.keyMap.Select):
				return m, m.emitSelected()
			case key.Matches(keyMsg, m.keyMap.Dismiss):
				m.focused = false
				return m, func() tea.Msg { return NavFocusMsg{Focused: false} }
			}
		}
	}
	return m, nil
}

func (m *Sidebar) emitSelected() tea.Cmd {
	idx := m.ActiveIndex
	return func() tea.Msg { return SelectedMsg{PageIndex: idx} }
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *Sidebar) View() tea.View {
	c := m.Colors()
	if m.collapsed {
		return m.collapsedView(c)
	}
	return m.expandedView(c)
}

func (m *Sidebar) collapsedView(c *theme.AppStyle) tea.View {
	strip := c.Styles.NavTitle.
		Width(sidebarCollapsedWidth).
		Height(m.height).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		Render("≡")
	v := tea.NewView(strip)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		if _, ok := mm.(tea.MouseReleaseMsg); ok {
			return func() tea.Msg { return CollapseToggleMsg{} }
		}
		return nil
	}
	return v
}

func (m *Sidebar) expandedView(c *theme.AppStyle) tea.View {
	innerW := max(m.width-2, 1)

	// Push current theme + focus state into the delegate before rendering.
	m.mainList.SetDelegate(m.buildDelegate(c, innerW))

	// ── Header / collapse button ─────────────────────────────────────────
	var headerStyle lipgloss.Style
	if m.focused {
		headerStyle = c.Styles.NavTitle.
			Width(innerW).
			Padding(0, 1).
			Bold(true).
			Background(c.Accent).
			Foreground(c.Bg)
	} else {
		headerStyle = c.Styles.NavTitle.
			Width(innerW).
			Padding(0, 1).
			Align(lipgloss.Left)
	}
	header := headerStyle.Render("≡  NAV")

	// ── Main list ────────────────────────────────────────────────────────
	listStr := m.mainList.View()

	// ── Separator + pinned Settings ──────────────────────────────────────
	sep := c.Styles.NavInactive.
		Width(innerW).
		Foreground(c.Border).
		Render(strings.Repeat("─", innerW))
	settingsStr := m.renderSettingsItem(c, innerW)

	// ── Vertical padding pushes Settings to the absolute bottom ──────────
	headerH := lipgloss.Height(header)
	sepH := lipgloss.Height(sep)
	settingsH := lipgloss.Height(settingsStr)
	listAreaH := max(m.height-headerH-sepH-settingsH, 1)
	paddedList := lipgloss.NewStyle().Height(listAreaH).Render(listStr)

	inner := lipgloss.JoinVertical(lipgloss.Left,
		header,
		paddedList,
		sep,
		settingsStr,
	)

	// Border colour signals focus state to the user.
	var borderFg color.Color
	if m.focused {
		borderFg = c.Accent
	} else {
		borderFg = c.Border
	}

	background := c.Styles.NavContainer.
		Width(m.width).
		Height(m.height).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(borderFg)

	rendered := background.Render(inner)
	v := tea.NewView(rendered)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()

	// Capture height at render time for the mouse closure.
	height := m.height
	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		return m.handleMouse(mm, height)
	}
	return v
}

// buildDelegate creates a navDelegate with the current theme and focus state.
func (m *Sidebar) buildDelegate(c *theme.AppStyle, innerW int) navDelegate {
	return navDelegate{
		activeIdx:      m.mainListActiveIdx(),
		sidebarFocused: m.focused,
		focusedStyle:   c.Styles.NavActive.Padding(0, 0).Bold(true),
		activeStyle:    c.Styles.NavActive.Padding(0, 0),
		normalStyle:    c.Styles.NavInactive.Padding(0, 0),
		// 2 chars consumed by the prefix ("▶ " / "● " / "  ")
		itemWidth: max(innerW-2, 1),
	}
}

// renderSettingsItem renders the pinned Settings entry with the current state.
func (m *Sidebar) renderSettingsItem(c *theme.AppStyle, innerW int) string {
	if m.settingsIdx < 0 || m.settingsIdx >= len(m.Pages) {
		return ""
	}
	title := m.Pages[m.settingsIdx].Title
	prefix := "  "
	var style lipgloss.Style

	if m.ActiveIndex == m.settingsIdx {
		if m.focused {
			prefix = "▶ "
			style = c.Styles.NavActive.Padding(0, 0).Bold(true)
		} else {
			prefix = "● "
			style = c.Styles.NavActive.Padding(0, 0)
		}
	} else {
		style = c.Styles.NavInactive.Padding(0, 0)
	}
	return fmt.Sprintf("%s%s", prefix, style.Width(max(innerW-2, 1)).Render(title))
}

// handleMouse routes a mouse event to the correct sidebar zone.
// height is the captured sidebar height at View() time.
func (m *Sidebar) handleMouse(mm tea.MouseMsg, height int) tea.Cmd {
	rel, ok := mm.(tea.MouseReleaseMsg)
	if !ok {
		return nil
	}
	me := rel.Mouse()
	if me.Button != tea.MouseLeft {
		return nil
	}

	// Row 0 is the header — clicking it collapses the sidebar.
	if me.Y == 0 {
		return func() tea.Msg { return CollapseToggleMsg{} }
	}

	// The Settings row is at height-2 (1 header + listAreaH + 1 sep + 1 settings).
	// Accept clicks from settingsRow onward as targeting Settings.
	settingsRow := height - 2
	if m.settingsIdx >= 0 && me.Y >= settingsRow {
		m.ActiveIndex = m.settingsIdx
		return tea.Batch(
			func() tea.Msg { return NavFocusMsg{Focused: true} },
			func() tea.Msg { return SelectedMsg{PageIndex: m.settingsIdx} },
		)
	}

	// List items render below the header, one item every navItemStride rows (one
	// row for the item plus a blank spacing row between items). Map the click row
	// back to an item index; clicks on a spacing row fall through to focus.
	listY := me.Y - 1
	numMain := m.numMainItems()
	if listY >= 0 && listY%navItemStride == 0 {
		idx := listY / navItemStride
		if idx < numMain {
			// Adjust for any pages that appear at/after settingsIdx in the full slice.
			if m.settingsIdx >= 0 && idx >= m.settingsIdx {
				idx++
			}
			if idx >= 0 && idx < len(m.Pages) {
				m.ActiveIndex = idx
				m.syncListCursor()
				capturedIdx := idx
				return tea.Batch(
					func() tea.Msg { return NavFocusMsg{Focused: true} },
					func() tea.Msg { return SelectedMsg{PageIndex: capturedIdx} },
				)
			}
		}
	}

	// Click in the padding area: focus the sidebar without switching pages.
	m.focused = true
	return func() tea.Msg { return NavFocusMsg{Focused: true} }
}

// ─── Navigator interface ──────────────────────────────────────────────────────

func (m *Sidebar) Width() int  { return m.width }
func (m *Sidebar) Height() int { return m.height }

// Dock reports that the sidebar occupies the left edge.
func (m *Sidebar) Dock() Side { return DockLeft }

func (m *Sidebar) GetPages() []Page { return m.Pages }

// SetPages replaces the page list and identifies the Settings pin by ID.
func (m *Sidebar) SetPages(p []Page) {
	m.Pages = p
	m.settingsIdx = -1
	for i, pg := range p {
		if pg.ID == "settings" {
			m.settingsIdx = i
			break
		}
	}
	m.expandedWidth = m.computeExpandedWidth()
	if !m.collapsed {
		m.width = m.expandedWidth
	}
	m.rebuildList()
}

func (m *Sidebar) SetActiveIndex(i int) {
	m.ActiveIndex = i
	m.syncListCursor()
}

func (m *Sidebar) GetActiveIndex() int { return m.ActiveIndex }

// SetFocused lets the router update the sidebar's visual focus state without
// going through the message loop (e.g. when Tab cycles pages).
func (m *Sidebar) SetFocused(f bool) { m.focused = f }

var _ Navigator = (*Sidebar)(nil)

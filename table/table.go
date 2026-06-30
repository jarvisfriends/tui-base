// Package table is a themed, interactive data table widget for tui-base apps.
//
// It fills the gaps the charm tables leave open: charm.land/bubbles/table has no
// sorting, filtering, or mouse, and charm.land/lipgloss/table is a pure renderer
// with no state. This widget keeps the state (cursor, paging, 3-state column
// sort, `/` filter, double-click tracking) and delegates the actual drawing to
// lipgloss/table, so borders, column balancing, and truncation stay
// library-maintained. Colors come from a theme.AppStyle passed to View, so the
// table recolors live on theme changes.
//
// Mouse interaction is cooperative: tui-base routes mouse events to a page's
// View().OnMouse with page-relative coordinates, and the page forwards clicks to
// HandleClick / wheels to HandleWheel. Clicking a header sorts that column;
// double-clicking a row (or pressing the Open key) emits an OpenDetailMsg the
// host page can act on (e.g. open a detail overlay).
package table

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ltable "charm.land/lipgloss/v2/table"

	"github.com/jarvisfriends/tui-base/theme"
)

// doubleClickWindow is how close two clicks on the same row must land to count
// as a double-click (which opens the row's details).
const doubleClickWindow = 450 * time.Millisecond

// Column describes one column header.
type Column struct {
	Title  string
	Filter bool // included in the `/` text filter
}

// Cell is one rendered cell. Numeric cells carry a real value so a column sorts
// by magnitude rather than lexically (e.g. "9" before "10", "2h 8m" by ms).
type Cell struct {
	Text  string
	Num   float64
	IsNum bool
}

// Text returns a string cell. Num returns a numeric cell whose display text and
// sort value can differ (e.g. a duration shown as "2h 8m" but sorted by ms).
func Text(s string) Cell                 { return Cell{Text: s} }
func Num(display string, v float64) Cell { return Cell{Text: display, Num: v, IsNum: true} }

// Row is one data row plus a stable Key the host uses to identify it (e.g. when
// opening details).
type Row struct {
	Key   string
	Cells []Cell
}

// OpenDetailMsg is emitted when the user opens a row (Enter or double-click).
// The host page handles it (e.g. by opening a detail overlay for Key).
type OpenDetailMsg struct{ Key string }

// KeyMap is the table's key bindings. Use DefaultKeyMap for sensible defaults.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Sort     key.Binding // cycle the sort column/direction
	Filter   key.Binding // enter `/` filter mode
	Open     key.Binding // open the selected row's details
	Cancel   key.Binding // leave/clear filter mode
}

// DefaultKeyMap returns the standard bindings (vim + arrows, `/` filter, `s`
// sort, Enter open).
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:       key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		Down:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		Top:      key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		Bottom:   key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		Sort:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Open:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
		Cancel:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),
	}
}

// ShortHelp implements help.KeyMap.
func (km KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Up, km.Down, km.Sort, km.Filter, km.Open}
}

// FullHelp implements help.KeyMap.
func (km KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Up, km.Down, km.PageUp, km.PageDown, km.Top, km.Bottom},
		{km.Sort, km.Filter, km.Open, km.Cancel},
	}
}

var _ help.KeyMap = (*KeyMap)(nil)

// Model is the table widget. Construct it with New.
type TableModel struct {
	KeyMap KeyMap

	cols []Column
	rows []Row // sorted in place; filtering selects a subset via `filtered`

	filtered []int
	cursor   int
	offset   int
	pageSize int

	sortCol    int
	sortAsc    bool
	sortActive bool

	filtering bool
	filter    string

	width int

	// Geometry recorded by View (page-content coordinates) for HandleClick.
	colBoundaries []int
	headerY       int
	dataStartY    int
	visibleCount  int

	lastClickRow  int
	lastClickTime time.Time
}

// Option configures a Model at construction.
type Option func(*TableModel)

// WithKeyMap overrides the default key bindings.
func WithKeyMap(km KeyMap) Option { return func(m *TableModel) { m.KeyMap = km } }

// WithPageSize sets a fixed page size (otherwise it's derived from the height
// passed to SetSize).
func WithPageSize(n int) Option { return func(m *TableModel) { m.pageSize = n } }

// WithSort sets the initial sort column and direction.
func WithSort(col int, asc bool) Option {
	return func(m *TableModel) {
		if col >= 0 {
			m.sortCol, m.sortAsc, m.sortActive = col, asc, true
		}
	}
}

// New builds a table for the given columns.
func New(cols []Column, opts ...Option) *TableModel {
	m := &TableModel{
		KeyMap:       DefaultKeyMap(),
		cols:         cols,
		pageSize:     20,
		sortCol:      -1,
		lastClickRow: -1,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// ─── data ────────────────────────────────────────────────────────────────────

// SetRows replaces the data, re-applying the active sort + filter.
func (m *TableModel) SetRows(rows []Row) {
	m.rows = rows
	m.doSort()
	m.rebuildFilter()
}

// SetSize informs the table of its available width/height. The page size is
// derived from height unless WithPageSize fixed it.
func (m *TableModel) SetSize(w, h int) {
	m.width = w
	m.pageSize = max(h-4, 3) // reserve header, rule, footer
	m.clampCursor()
}

// SelectedRow returns the highlighted row, if any.
func (m *TableModel) SelectedRow() (Row, bool) {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return m.rows[m.filtered[m.cursor]], true
	}
	return Row{}, false
}

// Filtering reports whether the `/` filter input is active (the host should
// claim keyboard focus while it is, e.g. via navigation.KeyCapturer).
func (m *TableModel) Filtering() bool { return m.filtering }

// ─── input ───────────────────────────────────────────────────────────────────

// Update handles a message (currently key presses) and may return a Cmd — e.g.
// an OpenDetailMsg-producing Cmd when the user opens a row.
func (m *TableModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return nil
}

func (m *TableModel) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.filtering {
		switch {
		case key.Matches(msg, m.KeyMap.Cancel):
			m.filtering, m.filter = false, ""
			m.rebuildFilter()
		case msg.Code == tea.KeyEnter:
			m.filtering = false
		case msg.Code == tea.KeyBackspace:
			if m.filter != "" {
				runes := []rune(m.filter)
				m.filter = string(runes[:len(runes)-1])
				m.rebuildFilter()
			}
		case msg.Code == tea.KeySpace:
			m.filter += " "
			m.rebuildFilter()
		default:
			if msg.Text != "" {
				m.filter += msg.Text
				m.rebuildFilter()
			}
		}
		return nil
	}

	switch {
	case key.Matches(msg, m.KeyMap.Up):
		m.moveCursor(-1)
	case key.Matches(msg, m.KeyMap.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.KeyMap.PageUp):
		m.moveCursor(-m.pageSize)
	case key.Matches(msg, m.KeyMap.PageDown):
		m.moveCursor(m.pageSize)
	case key.Matches(msg, m.KeyMap.Top):
		m.cursor = 0
		m.clampCursor()
	case key.Matches(msg, m.KeyMap.Bottom):
		m.cursor = len(m.filtered) - 1
		m.clampCursor()
	case key.Matches(msg, m.KeyMap.Filter):
		m.filtering = true
	case key.Matches(msg, m.KeyMap.Sort):
		m.cycleSort()
	case key.Matches(msg, m.KeyMap.Open):
		return m.openSelected()
	}
	return nil
}

// HandleClick processes a left click at page-relative (x, y): a header click
// sorts that column; a data-row click selects it, and a quick second click on
// the same row opens its details.
func (m *TableModel) HandleClick(x, y int) tea.Cmd {
	if y == m.headerY {
		if col := m.columnAtX(x); col >= 0 {
			m.sortByCol(col)
		}
		return nil
	}
	idx := y - m.dataStartY
	if idx < 0 || idx >= m.visibleCount {
		return nil
	}
	row := m.offset + idx
	if row < 0 || row >= len(m.filtered) {
		return nil
	}
	now := time.Now()
	double := row == m.lastClickRow && now.Sub(m.lastClickTime) < doubleClickWindow
	m.cursor = row
	m.clampCursor()
	m.lastClickRow, m.lastClickTime = row, now
	if double {
		return m.openSelected()
	}
	return nil
}

// HandleWheel scrolls the selection by a few rows.
func (m *TableModel) HandleWheel(up bool) {
	if up {
		m.moveCursor(-3)
	} else {
		m.moveCursor(3)
	}
}

func (m *TableModel) openSelected() tea.Cmd {
	r, ok := m.SelectedRow()
	if !ok {
		return nil
	}
	rowKey := r.Key
	return func() tea.Msg { return OpenDetailMsg{Key: rowKey} }
}

// ─── sorting / filtering / nav ────────────────────────────────────────────────

func (m *TableModel) doSort() {
	if !m.sortActive || m.sortCol < 0 || m.sortCol >= len(m.cols) {
		return
	}
	col, asc := m.sortCol, m.sortAsc
	sort.SliceStable(m.rows, func(i, j int) bool {
		return lessCell(m.rows[i], m.rows[j], col, asc)
	})
}

func lessCell(a, b Row, col int, asc bool) bool {
	var ca, cb Cell
	if col < len(a.Cells) {
		ca = a.Cells[col]
	}
	if col < len(b.Cells) {
		cb = b.Cells[col]
	}
	if ca.IsNum && cb.IsNum {
		if asc {
			return ca.Num < cb.Num
		}
		return ca.Num > cb.Num
	}
	la, lb := strings.ToLower(ca.Text), strings.ToLower(cb.Text)
	if asc {
		return la < lb
	}
	return la > lb
}

// sortByCol cycles a column: asc → desc → unsorted.
func (m *TableModel) sortByCol(col int) {
	if col < 0 || col >= len(m.cols) {
		return
	}
	switch {
	case !m.sortActive || m.sortCol != col:
		m.sortCol, m.sortAsc, m.sortActive = col, true, true
	case m.sortAsc:
		m.sortAsc = false
	default:
		m.sortActive = false
	}
	m.doSort()
	m.rebuildFilter()
}

// cycleSort walks the current column asc → desc, then advances to the next.
func (m *TableModel) cycleSort() {
	if len(m.cols) == 0 {
		return
	}
	switch {
	case !m.sortActive:
		m.sortByCol(max(m.sortCol, 0))
	case m.sortAsc:
		m.sortByCol(m.sortCol)
	default:
		next := (m.sortCol + 1) % len(m.cols)
		m.sortActive = false
		m.sortByCol(next)
	}
}

func (m *TableModel) rebuildFilter() {
	m.filtered = m.filtered[:0]
	q := strings.ToLower(strings.TrimSpace(m.filter))
	for i, r := range m.rows {
		if q == "" || rowMatches(r, m.cols, q) {
			m.filtered = append(m.filtered, i)
		}
	}
	m.clampCursor()
}

func rowMatches(r Row, cols []Column, q string) bool {
	for i, c := range cols {
		if !c.Filter || i >= len(r.Cells) {
			continue
		}
		if strings.Contains(strings.ToLower(r.Cells[i].Text), q) {
			return true
		}
	}
	return false
}

func (m *TableModel) clampCursor() {
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.pageSize {
		m.offset = m.cursor - m.pageSize + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *TableModel) moveCursor(d int) { m.cursor += d; m.clampCursor() }

// columnAtX maps a page-relative X to a column index using the ┬ separators
// parsed from the rendered top border.
func (m *TableModel) columnAtX(x int) int {
	for i, b := range m.colBoundaries {
		if x < b {
			return i
		}
	}
	if len(m.cols) > 0 {
		return len(m.cols) - 1
	}
	return -1
}

// ─── rendering (delegated to lipgloss/table) ──────────────────────────────────

// View renders the table for the given palette, at vertical origin originY
// within the page content (the number of lines drawn above it). It records
// geometry for HandleClick and appends a status footer below the table.
func (m *TableModel) View(c *theme.AppStyle, originY int) string {
	st := c.Styles

	if len(m.cols) == 0 {
		return ""
	}

	headers := make([]string, len(m.cols))
	for i, col := range m.cols {
		h := col.Title
		if m.sortActive && i == m.sortCol {
			if m.sortAsc {
				h += " ▲"
			} else {
				h += " ▼"
			}
		}
		headers[i] = h
	}

	end := min(m.offset+m.pageSize, len(m.filtered))
	visible := make([][]string, 0, max(end-m.offset, 0))
	for fi := m.offset; fi < end; fi++ {
		r := m.rows[m.filtered[fi]]
		cells := make([]string, len(m.cols))
		for i := range m.cols {
			if i < len(r.Cells) {
				cells[i] = r.Cells[i].Text
			}
		}
		visible = append(visible, cells)
	}
	m.visibleCount = len(visible)
	selectedIdx := m.cursor - m.offset

	headerStyle := st.Title.Padding(0, 1)
	sortedHeaderStyle := headerStyle.Background(c.SelectionBg)
	selectedStyle := st.SelectedItem.Padding(0, 1)
	cellStyle := st.TextOnBg.Padding(0, 1)
	zebraStyle := st.Subtitle.Padding(0, 1)

	tbl := ltable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(c.Border)).
		Wrap(false).
		Width(m.width).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == ltable.HeaderRow {
				if m.sortActive && col == m.sortCol {
					return sortedHeaderStyle
				}
				return headerStyle
			}
			if row == selectedIdx {
				return selectedStyle
			}
			if row%2 == 1 {
				return zebraStyle
			}
			return cellStyle
		}).
		Headers(headers...).
		Rows(visible...)

	out := tbl.String()

	// Record geometry: top border (originY), header (originY+1), header
	// separator (originY+2), data rows from originY+3.
	m.headerY = originY + 1
	m.dataStartY = originY + 3
	m.colBoundaries = m.colBoundaries[:0]
	if lines := strings.Split(out, "\n"); len(lines) > 0 {
		for x, ch := range []rune(lines[0]) {
			if ch == '┬' {
				m.colBoundaries = append(m.colBoundaries, x)
			}
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, out, m.footer(c, lipgloss.Width(out)))
}

func (m *TableModel) footer(c *theme.AppStyle, w int) string {
	st := c.Styles
	var left string
	if total := len(m.filtered); total > 0 {
		left = " " + strconv.Itoa(m.cursor+1) + "/" + strconv.Itoa(total) + " "
	} else {
		left = " 0/0 "
	}
	if m.sortActive {
		dir := "▲"
		if !m.sortAsc {
			dir = "▼"
		}
		left += "· sorted by " + m.cols[m.sortCol].Title + " " + dir + " "
	}

	var right string
	switch {
	case m.filtering:
		right = "/" + m.filter + "▏"
	case m.filter != "":
		right = "filter: " + m.filter + " (esc clears) "
	default:
		right = "s/click header: sort · enter/double-click: details · / filter "
	}

	gap := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	return st.Subtitle.Render(left) + strings.Repeat(" ", gap) + st.Subtitle.Render(right)
}

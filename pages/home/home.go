package home

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jarvisfriends/snap/charts"
	"github.com/jarvisfriends/snap/page"
	"github.com/jarvisfriends/snap/scrollbar"
	"github.com/jarvisfriends/snap/styles"
	"github.com/jarvisfriends/snap/uifx"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const welcomeText = "Welcome to the V2 Terminal Hub\n\nUse Tab to switch pages.\nCtrl+B to toggle sidebar.\nCtrl+H to toggle full help."

// Interactive zone IDs — registered from the same layers the View renders.
const (
	zonePills  = "pills"
	zoneCharts = "charts"
)

// chartID routes the demo data messages to this page's chart models.
const chartID = "home"

// tickInterval paces the demo metrics stream feeding the charts.
const tickInterval = 800 * time.Millisecond

// tickMsg advances the live demo charts.
type tickMsg time.Time

// HomePageModel is the reference app's landing page and doubles as a small
// snap showcase: live ID-routed charts, a pill strip whose shape cycles on
// click, and a snap scrollbar with click/drag-to-jump on small terminals.
// Every pointer event is handled in View().OnMouse (uifx.MouseHandlers +
// Zones); Update never sees a tea.MouseMsg.
type HomePageModel struct {
	page.Base

	// vp scrolls the content. On a normal terminal the content fits and the
	// viewport is a no-op; on a very small terminal it scrolls (wheel, keys,
	// or the snap scrollbar) instead of clipping.
	vp viewport.Model
	// lastContent guards SetContent so we only reset the viewport (and its
	// scroll position) when the rendered content actually changes.
	lastContent string

	spark *charts.SparklineModel
	load  *charts.HBarModel
	// level is the random-walk value behind the demo stream.
	level float64

	// shape indexes styles.PillShapes(); clicking the pill strip cycles it.
	shape int
	// paused stops the demo stream; clicking the charts block toggles it.
	paused bool
	// ticking gates the demo stream to the time this page is actually the
	// active page (I-1 lifecycle hooks: OnEnter starts, OnLeave stops).
	ticking bool

	// zones hit-tests the interactive blocks in content coordinates; rebuilt
	// by every content() pass from the same layers it renders.
	zones *uifx.Zones
	// dragging tracks a scrollbar drag between Click and Release.
	dragging bool
}

func New() *HomePageModel {
	m := &HomePageModel{
		vp:    viewport.New(),
		spark: charts.NewSparkline(chartID),
		load:  charts.NewHBar(chartID),
		level: 42,
	}
	m.spark.SetSize(26, 1)
	m.load.SetSize(20, 1)
	return m
}

func (m *HomePageModel) Init() tea.Cmd { return nil }

// OnEnter starts the demo metrics stream when the page becomes active and
// OnLeave stops it — the reference implementation of the I-1 lifecycle hooks.
func (m *HomePageModel) OnEnter() tea.Cmd {
	m.ticking = true
	return m.tickCmd()
}

func (m *HomePageModel) OnLeave() tea.Cmd {
	m.ticking = false
	return nil
}

func (m *HomePageModel) tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *HomePageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.vp.SetWidth(msg.Width)
		m.vp.SetHeight(msg.Height)
		m.syncContent()
		return m, nil

	case tickMsg:
		if !m.ticking {
			// The page went inactive; drop this tick and stay quiet until
			// OnEnter re-arms the stream.
			return m, nil
		}
		if !m.paused {
			m.step()
		}
		return m, m.tickCmd()

	case tea.MouseMsg:
		// Pointer input arrives exclusively through View().OnMouse.
		return m, nil
	}

	// Forward keys (PgUp/PgDn/arrows) to the viewport so it scrolls.
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// step advances the random walk and feeds both charts through their ID-routed
// data messages — the same wiring an app streaming real metrics would use.
func (m *HomePageModel) step() {
	m.level += rand.Float64()*16 - 8 //nolint:gosec // demo jitter, not crypto
	m.level = max(5, min(95, m.level))
	_, _ = m.spark.Update(charts.SparklinePointMsg{ID: chartID, Value: m.level})
	_, _ = m.load.Update(charts.HBarDataMsg{ID: chartID, Pct: m.level})
}

// pillStrip renders the segmented status pill in the currently selected shape.
func (m *HomePageModel) pillStrip() string {
	c := m.Colors()
	shapes := styles.PillShapes()
	shape := shapes[m.shape%len(shapes)]
	pill := styles.SegmentedPill([]styles.PillSegment{
		{Text: " tui-base ", Bg: c.Accent},
		{Text: " snap ", Bg: c.SelectionBg, Fg: c.SelectionFg},
		{Text: " live ", Bg: c.Success},
	}, styles.PillStyles{Shape: shape})
	hint := c.Styles.Subtitle.Render(
		fmt.Sprintf("  click to cycle shape (%s)", shape.DisplayName()))
	return pill + hint
}

// chartBlock renders the live sparkline + load bar with a pause hint.
func (m *HomePageModel) chartBlock() string {
	c := m.Colors()
	label := c.Styles.Subtitle
	state := "click to pause"
	if m.paused {
		state = "click to resume"
	}
	return label.Render("activity ") + m.spark.View().Content + "\n" +
		label.Render("load     ") + m.load.View().Content +
		label.Render(fmt.Sprintf(" %3.0f%%  %s", m.load.Pct(), state))
}

// content builds the centered hub card from positioned layers and registers
// the interactive blocks as hit zones from those same layers, so the zones
// can never drift from what is on screen.
func (m *HomePageModel) content() string {
	c := m.Colors()
	availW := max(m.Width(), 10)

	box := c.Styles.Success.
		Bold(true).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		MaxWidth(availW).
		Render(welcomeText)
	pills := m.pillStrip()
	chartsBlk := m.chartBlock()

	blocks := []struct {
		id  string
		str string
	}{
		{"", box},
		{zonePills, pills},
		{zoneCharts, chartsBlk},
	}

	// Stack the blocks with one blank line between, horizontally centered;
	// vertically centered as a group when the page is tall enough.
	totalH := 0
	for _, b := range blocks {
		totalH += lipgloss.Height(b.str) + 1
	}
	totalH--
	top := max((m.Height()-totalH)/2, 0)

	layers := make([]*lipgloss.Layer, 0, len(blocks)+1)
	zoneLayers := make([]*lipgloss.Layer, 0, len(blocks))
	// A transparent backdrop pins the compositor canvas to the full page
	// size so centered blocks keep their offsets.
	h := max(m.Height(), totalH)
	backdrop := lipgloss.NewStyle().Width(availW).Height(h).Render("")
	layers = append(layers, lipgloss.NewLayer(backdrop))
	y := top
	for _, b := range blocks {
		x := max((availW-lipgloss.Width(b.str))/2, 0)
		layers = append(layers, lipgloss.NewLayer(b.str).X(x).Y(y).Z(1))
		if b.id != "" {
			zoneLayers = append(zoneLayers, lipgloss.NewLayer(b.str).ID(b.id).X(x).Y(y))
		}
		y += lipgloss.Height(b.str) + 1
	}
	m.zones = uifx.NewZones(zoneLayers...)

	content := lipgloss.NewCompositor(layers...).Render()
	fill := lipgloss.NewStyle().Background(c.Styles.TextOnBg.GetBackground())
	return fill.Render(content)
}

// syncContent updates the viewport content only when it has changed,
// preserving the scroll position across unrelated re-renders.
func (m *HomePageModel) syncContent() {
	s := m.content()
	if s != m.lastContent {
		m.vp.SetContent(s)
		m.lastContent = s
	}
}

// scrollbarNeeded reports whether the content overflows the page.
func (m *HomePageModel) scrollbarNeeded() bool {
	return m.vp.TotalLineCount() > m.vp.VisibleLineCount()
}

// jumpTo maps a pointer row on the scrollbar to a viewport offset — click
// the track or drag the thumb anywhere on the bar.
func (m *HomePageModel) jumpTo(y int) {
	total := m.vp.TotalLineCount()
	visible := m.vp.VisibleLineCount()
	m.vp.SetYOffset(offsetAt(y, visible, total, visible))
}

// offsetAt is the inverse of the scrollbar's thumb placement: the offset that
// puts the thumb's center on row y. TEMPORARY inline copy of snap's pending
// scrollbar.OffsetAt — swap to it at the next snap tag that ships it.
func offsetAt(y, barHeight, total, visible int) int {
	if total <= visible || barHeight <= 0 || visible <= 0 {
		return 0
	}
	thumbSize := max(1, barHeight*visible/total)
	track := barHeight - thumbSize
	if track <= 0 {
		return 0
	}
	maxScroll := total - visible
	pos := min(max(y-thumbSize/2, 0), track)
	// Round to nearest so mid-track clicks don't all bias toward the top.
	return min(max((pos*maxScroll+track/2)/track, 0), maxScroll)
}

func (m *HomePageModel) onClick(mo tea.Mouse) tea.Cmd {
	if m.scrollbarNeeded() && mo.X >= m.Width()-1 {
		m.dragging = true
		m.jumpTo(mo.Y)
		return nil
	}
	switch m.zones.Hit(mo.X, mo.Y+m.vp.YOffset()) {
	case zonePills:
		m.shape = (m.shape + 1) % len(styles.PillShapes())
	case zoneCharts:
		m.paused = !m.paused
	}
	return nil
}

func (m *HomePageModel) onMotion(mo tea.Mouse) tea.Cmd {
	if m.dragging {
		m.jumpTo(mo.Y)
	}
	return nil
}

func (m *HomePageModel) onRelease(tea.Mouse) tea.Cmd {
	m.dragging = false
	return nil
}

func (m *HomePageModel) onWheel(mo tea.Mouse) tea.Cmd {
	switch mo.Button {
	case tea.MouseWheelUp:
		m.vp.ScrollUp(1)
	case tea.MouseWheelDown:
		m.vp.ScrollDown(1)
	}
	return nil
}

func (m *HomePageModel) View() tea.View {
	c := m.Colors()
	m.syncContent()
	body := m.vp.View()

	// A snap scrollbar overlays the right edge when the content overflows;
	// clicking or dragging it jumps the view (see onClick/onMotion).
	if m.scrollbarNeeded() {
		bar := scrollbar.Vertical(
			m.vp.TotalLineCount(), m.vp.VisibleLineCount(),
			m.vp.YOffset(), m.vp.VisibleLineCount(), scrollbar.DefaultStyles())
		body = lipgloss.NewCompositor(
			lipgloss.NewLayer(body),
			lipgloss.NewLayer(bar).X(max(m.Width()-1, 0)).Y(0).Z(1),
		).Render()
	}

	v := tea.NewView(body)
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	v.OnMouse = uifx.MouseHandlers{
		Click:   m.onClick,
		Release: m.onRelease,
		Motion:  m.onMotion,
		Wheel:   m.onWheel,
	}.OnMouse
	return v
}

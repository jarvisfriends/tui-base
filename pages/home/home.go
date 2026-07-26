// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package home

import (
	"fmt"
	"image/color"
	"math/rand/v2"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/jarvisfriends/inspector"
	"github.com/jarvisfriends/snap/charts"
	"github.com/jarvisfriends/snap/menu"
	"github.com/jarvisfriends/snap/page"
	"github.com/jarvisfriends/snap/scrollbar"
	"github.com/jarvisfriends/snap/styles"
	"github.com/jarvisfriends/snap/table"
	"github.com/jarvisfriends/snap/uifx"

	"github.com/jarvisfriends/tui-base/logging"
	"github.com/jarvisfriends/tui-base/pages/settings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

// wideBreakpoint is the terminal width at which the chart block switches from
// a stacked to a side-by-side layout and charts grow — the home page's
// responsive-layout showcase (see wide, breakpoint, resizeCharts).
const wideBreakpoint = 84

// compactBreakpoint is the width below which secondary labels (theme swatch
// names, hints) drop to keep every line on one row.
const compactBreakpoint = 60

const welcomeText = "Welcome to the V2 Terminal Hub\n\nUse Tab to switch pages.\nCtrl+B to toggle sidebar.\nCtrl+H to toggle full help.\nHome showcase: t=theme, o=shape, p=bar, r=spark, f=fx."

// Interactive zone IDs — registered from the same layers the View renders.
const (
	zoneBox           = "box"
	zonePills         = "pills"
	zoneCharts        = "charts"
	zoneProgressStyle = "progress-style"
	zoneTheme         = "theme"
	zoneThemeURL      = "theme-url"
	zoneEffects       = "effects"
	zoneResponsive    = "responsive"
	zoneDisks         = "disks"
)

// actionOpenThemeSource is the context-menu item ID for opening the active
// theme's attribution URL.
const actionOpenThemeSource = "open-theme-source"

// runtime.GOOS values used by the browser/file-manager launchers.
const (
	osWindows = "windows"
	osDarwin  = "darwin"
)

// sparklineStyleCount is the number of charts.SparklineStyle values
// (UserBlocks, BrailleUp, BrailleDown, StdBlocks) the charts menu cycles.
const sparklineStyleCount = 4

// progressStyle selects how the load bar renders — a little gallery of what
// you get almost for free from charm.land/bubbles/v2/progress plus the
// page's own hand-rolled gradient, so a dev comparing options can see them
// side by side instead of reading four separate demos.
type progressStyle int

const (
	// progressClassic is the page's original hand-rolled gradientBar — a
	// smooth accent→success sweep with no external dependency.
	progressClassic progressStyle = iota
	// progressBlend uses bubbles progress.WithColors + WithScaled for the
	// same kind of sweep in two option calls instead of a hand-rolled loop.
	progressBlend
	// progressSolid is a solid accent fill with a different glyph pair
	// (▰/▱), from bubbles progress.WithColors + WithFillCharacters.
	progressSolid
	// progressZones stripes the bar into fixed error/warning/success bands
	// via bubbles progress.WithColorFunc keyed on cell position — a
	// decorative gauge look, not a value-driven color.
	progressZones
	// progressAnimated is the only stateful style: a persistent
	// bubbles progress.Model whose fill springs smoothly toward each new
	// value (SetPercent + Update(progress.FrameMsg)), and whose single fill
	// color shifts from accent toward success as the value rises (via
	// WithColorFunc keyed on the overall percent). Gated to LevelHigh since
	// it needs a redraw ticker while animating.
	progressAnimated
)

// String names a progressStyle for the progress-style strip and its
// right-click menu.
func (s progressStyle) String() string {
	switch s {
	case progressBlend:
		return "Blend"
	case progressSolid:
		return "Solid"
	case progressZones:
		return "Zones"
	case progressAnimated:
		return "Animated"
	case progressClassic:
		return "Classic"
	default:
		return "Unknown"
	}
}

// chartID routes the demo data messages to this page's chart models.
const chartID = "home"

// tickInterval paces the demo metrics stream feeding the charts.
const tickInterval = 800 * time.Millisecond

// tickMsg advances the live demo charts.
type tickMsg time.Time

// contentBlock is one positioned, optionally hit-testable region of the
// home page — see content().
type contentBlock struct {
	id  string
	str string
}

// HomePageModel is the reference app's landing page and doubles as a small
// snap showcase: live ID-routed charts with a colored braille sparkline
// (right-click cycles the glyph style) and a load bar cyclable between four
// looks — a hand-rolled gradient plus three bubbles/progress styles,
// culminating in a spring-animated bar at LevelHigh; a pill strip whose
// shape cycles on click; a live theme swatch strip that cycles the color
// theme on click and links to its source; a responsive layout that reflows
// at wideBreakpoint; an Effects (uifx.Level) control that scales the whole
// page's fanciness — hover highlighting and the animated progress style
// only exist at LevelHigh, which also opts the page into AllMotion; a
// right-click context menu (snap/menu) on every block; and a snap scrollbar
// with click/drag-to-jump on small terminals. Every pointer event is
// handled in View().OnMouse (routed through onMouseRoot to the context menu
// first, then uifx.MouseHandlers + Zones); Update never sees a tea.MouseMsg.
type HomePageModel struct {
	page.Base

	// Effects scales how fancy the page's own rendering gets — separate from
	// (but inspired by) uifx.Level's original mouse-feedback-cost meaning:
	// LevelMinimal forces the cheapest static bar and no hover; LevelMedium
	// (default) unlocks the three static bubbles/progress styles; LevelHigh
	// adds the spring-animated bar and hover highlighting, and requests
	// AllMotion (see View, onMotion, availableProgressStyles).
	Effects uifx.Level

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
	// sparkStyle selects the sparkline's glyph set; the charts context menu
	// cycles it (see contextItems, applyContextChoice).
	sparkStyle charts.SparklineStyle
	// progStyle selects the load bar's look; the progress-style strip and
	// context menu cycle it within availableProgressStyles(Effects).
	progStyle progressStyle
	// animProgress is the persistent bubbles progress.Model backing
	// progressAnimated — the only style with animation state to keep across
	// renders (see progressView, Update's progress.FrameMsg case).
	animProgress progress.Model

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
	// hoverZone is the zone under the pointer, tracked only when
	// Effects.Hover() is true (LevelHigh); see onMotion, hoverWrap.
	hoverZone string

	// contextMenu is the right-click popup shared by every block; Tag() holds
	// the zone ID it was opened on so applyContextChoice knows what to do.
	contextMenu menu.Menu

	// disksTbl is the sortable/selectable Disks table; disks is the data behind
	// it (re-collected on OnEnter and the Refresh action). menuDiskPath records
	// which drive a currently-open disk context menu acts on. disksLeft/disksTop
	// are the table's content-space origin, recorded by content() each render so
	// pointer events can be translated into the table's own coordinates.
	disksTbl     *table.TableModel
	disks        []inspector.DiskUsage
	menuDiskPath string
	disksLeft    int
	disksTop     int
}

func New() *HomePageModel {
	m := &HomePageModel{
		vp:         viewport.New(),
		spark:      charts.NewSparkline(chartID),
		load:       charts.NewHBar(chartID),
		level:      42,
		sparkStyle: charts.SparklineBrailleUp,
		disksTbl:   newDisksTable(),
	}
	m.spark.SetSize(26, 2)
	m.load.SetSize(20, 2)
	m.animProgress = m.newAnimProgress()
	// Disk enumeration and the initial row build happen in OnEnter (the router
	// fires it at startup), not here — the constructor stays free of I/O and
	// off the shared bubble-table row counter, so parallel construction is safe.
	return m
}

// newAnimProgress builds the animated style's persistent bubbles model. Its
// color comes from a closure over m rather than a snapshot of m.Colors(), so
// it stays correct across theme changes without ever needing to be rebuilt.
func (m *HomePageModel) newAnimProgress() progress.Model {
	p := progress.New(
		progress.WithColorFunc(func(total, _ float64) color.Color {
			c := m.Colors()
			return blendAt(c.Accent, c.Success, total)
		}),
		progress.WithoutPercentage(),
	)
	p.SetWidth(20)
	return p
}

// blendAt samples the smooth color ramp between from and to at fraction t
// (0..1) — the single-sample twin of gradientBar's per-cell ramp, and the
// building block progressAnimated uses to recolor by overall value instead
// of cell position.
func blendAt(from, to color.Color, t float64) color.Color {
	const steps = 100
	t = min(max(t, 0), 1)
	return charts.Gradient(from, to, steps)[int(t*(steps-1))]
}

func (m *HomePageModel) Init() tea.Cmd { return nil }

// OnEnter starts the demo metrics stream when the page becomes active and
// OnLeave stops it — the reference implementation of the I-1 lifecycle hooks.
func (m *HomePageModel) OnEnter() tea.Cmd {
	m.ticking = true
	// Re-collect disk usage so the table reflects the current state each time
	// the page is shown.
	m.refreshDisks()
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
		m.resizeCharts()
		m.syncDisksSize()
		m.syncContent()
		return m, nil

	case table.OpenDetailMsg:
		// Enter or double-click on a disk row opens its actions menu, anchored
		// just below the table's top-left (translated from content space to the
		// screen through the viewport offset).
		x := m.disksLeft + 2
		y := max(m.disksTop-m.vp.YOffset()+1, 0)
		m.openDiskMenu(x, y, msg.Key)
		return m, nil

	case tickMsg:
		if !m.ticking {
			// The page went inactive; drop this tick and stay quiet until
			// OnEnter re-arms the stream.
			return m, nil
		}
		var stepCmd tea.Cmd
		if !m.paused {
			stepCmd = m.step()
		}
		return m, tea.Batch(stepCmd, m.tickCmd())

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.animProgress, cmd = m.animProgress.Update(msg)
		return m, cmd

	case tea.MouseMsg:
		// Pointer input arrives exclusively through View().OnMouse.
		return m, nil

	case browserOpenedMsg:
		if msg.Err != nil {
			logging.Errorf("home: failed to open %s: %v", msg.URL, msg.Err)
		}
		return m, nil

	case tea.KeyPressMsg:
		// While the context menu is open it owns the keyboard.
		if m.contextMenu.IsOpen() {
			if chosen, _ := m.contextMenu.HandleKey(msg); chosen != nil {
				return m, m.applyContextChoice(chosen.ID)
			}
			return m, nil
		}
		switch msg.String() {
		case "t":
			return m, m.cycleThemeCmd()
		case "o":
			m.shape = (m.shape + 1) % len(styles.PillShapes())
			return m, nil
		case "p":
			return m, m.cycleProgressStyle()
		case "r":
			m.sparkStyle = charts.SparklineStyle((int(m.sparkStyle) + 1) % sparklineStyleCount)
			return m, nil
		case "f":
			m.cycleEffects()
			return m, nil
		}
		// The Disks table claims its navigation/sort/filter/open keys; anything
		// else falls through to the viewport so it can still scroll.
		if m.disksHandlesKey(msg) {
			return m, m.disksTbl.Update(msg)
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	// Forward remaining messages to the viewport.
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// step advances the random walk and feeds both charts through their ID-routed
// data messages — the same wiring an app streaming real metrics would use.
// When the animated progress style is active it also nudges that model's
// target percent, kicking off (or continuing) its spring animation.
func (m *HomePageModel) step() tea.Cmd {
	m.level += rand.Float64()*16 - 8 //nolint:gosec // demo jitter, not crypto
	m.level = max(5, min(95, m.level))
	_, _ = m.spark.Update(charts.SparklinePointMsg{ID: chartID, Value: m.level})
	_, _ = m.load.Update(charts.HBarDataMsg{ID: chartID, Pct: m.level})
	if m.progStyle == progressAnimated {
		return m.animProgress.SetPercent(m.level / 100)
	}
	return nil
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
		fmt.Sprintf("  click to cycle shape (%s)", shape.DisplayName()),
	)
	return pill + hint
}

// chartBlock renders the live sparkline + load bar with a pause hint. The
// sparkline uses colored braille glyphs (direction-coded: rising green,
// falling red) and the load bar sweeps through a theme-derived gradient
// (Accent → Success), so both visibly repaint when the user switches themes
// in Settings. On wide terminals the two rows collapse into one (see wide).
func (m *HomePageModel) chartBlock() string {
	c := m.Colors()
	label := c.Styles.Subtitle
	state := "click to pause"
	if m.paused {
		state = "click to resume"
	}

	m.spark.Opts = charts.SparklineOpts{Style: m.sparkStyle, Colors: c}
	activity := label.Render("activity ") + m.spark.View().Content
	load := label.Render("load     ") + m.progressView(m.load.MaxWidth) +
		label.Render(fmt.Sprintf(" %3.0f%%  %s", m.load.Pct(), state))

	if m.wide() {
		return activity + label.Render("    ") + load
	}
	return activity + "\n" + load
}

// gradientBar renders a proportional bar whose filled cells sweep through a
// smooth color ramp between from and to (see charts.Gradient) instead of a
// single flat color — the load meter's answer to "why not just a plain
// progress bar".
func gradientBar(pct float64, width int, from, to color.Color) string {
	if width <= 0 {
		return ""
	}
	pct = min(max(pct, 0), 100)
	filled := min(width, int(pct/100.0*float64(width)+0.5))
	if filled == 0 {
		return strings.Repeat("░", width)
	}
	ramp := charts.Gradient(from, to, filled)
	var sb strings.Builder
	for _, col := range ramp {
		sb.WriteString(lipgloss.NewStyle().Foreground(col).Render("█"))
	}
	sb.WriteString(strings.Repeat("░", width-filled))
	return sb.String()
}

// progressView renders the load bar in whatever style m.progStyle currently
// selects. The three static styles are built fresh every call — cheap,
// since they're plain option calls plus one ViewAs render, and it means
// they're always in step with the live theme colors without needing a
// rebuild hook. Only progressAnimated carries state across calls (see
// m.animProgress); everything else is a pure function of the current pct.
func (m *HomePageModel) progressView(width int) string {
	c := m.Colors()
	pct := m.load.Pct() / 100

	switch m.progStyle {
	case progressBlend:
		p := progress.New(
			progress.WithColors(c.Accent, c.Success),
			progress.WithScaled(true),
			progress.WithoutPercentage(),
		)
		p.SetWidth(width)
		return p.ViewAs(pct)

	case progressSolid:
		p := progress.New(
			progress.WithColors(c.Accent),
			progress.WithFillCharacters('▰', '▱'),
			progress.WithoutPercentage(),
		)
		p.SetWidth(width)
		return p.ViewAs(pct)

	case progressZones:
		p := progress.New(
			progress.WithColorFunc(func(_, current float64) color.Color {
				switch {
				case current < 0.3:
					return c.Error
				case current < 0.7:
					return c.Warning
				default:
					return c.Success
				}
			}),
			progress.WithoutPercentage(),
		)
		p.SetWidth(width)
		return p.ViewAs(pct)

	case progressAnimated:
		m.animProgress.SetWidth(width)
		return m.animProgress.View()

	case progressClassic:
		return gradientBar(m.load.Pct(), width, c.Accent, c.Success)
	}
	return ""
}

// availableProgressStyles lists the load-bar styles unlocked at the given
// Effects tier — the page's reinterpretation of uifx.Level as a fanciness
// dial (see HomePageModel.Effects doc): LevelMinimal is static-only-and-cheap,
// LevelHigh adds the one style with real animation state and a redraw
// ticker.
func availableProgressStyles(level uifx.Level) []progressStyle {
	switch level {
	case uifx.LevelMinimal:
		return []progressStyle{progressClassic}
	case uifx.LevelHigh:
		return []progressStyle{progressClassic, progressBlend, progressSolid, progressZones, progressAnimated}
	case uifx.LevelMedium:
		return []progressStyle{progressClassic, progressBlend, progressSolid, progressZones}
	}
	return nil
}

// clampProgressStyle snaps m.progStyle back to progressClassic if the
// current Effects tier no longer offers it (e.g. dropping from LevelHigh to
// LevelMedium while progressAnimated was selected).
func (m *HomePageModel) clampProgressStyle() {
	if slices.Contains(availableProgressStyles(m.Effects), m.progStyle) {
		return
	}
	m.progStyle = progressClassic
}

// cycleProgressStyle advances to the next style within the current Effects
// tier's set, wrapping around, and kicks the animated model's target percent
// when switching into it so it doesn't render a stale value.
func (m *HomePageModel) cycleProgressStyle() tea.Cmd {
	avail := availableProgressStyles(m.Effects)
	idx := 0
	for i, s := range avail {
		if s == m.progStyle {
			idx = i
			break
		}
	}
	m.progStyle = avail[(idx+1)%len(avail)]
	if m.progStyle == progressAnimated {
		return m.animProgress.SetPercent(m.load.Pct() / 100)
	}
	return nil
}

// progressStyleStrip shows the load bar's current style and doubles as a
// click-to-cycle control (see cycleProgressStyle) — a quick way to compare
// the four looks side by side without editing code.
func (m *HomePageModel) progressStyleStrip() string {
	c := m.Colors()
	return c.Styles.Subtitle.Render("bar style  ") +
		c.Styles.Title.Render(fmt.Sprintf("%s  (click to cycle)", m.progStyle))
}

// cycleEffects advances Effects through Minimal → Medium → High → Minimal —
// the fanciness dial for the whole page (hover highlighting, AllMotion, and
// the animated progress style all gate on it) — and clamps the progress
// style if it's no longer available at the new tier.
func (m *HomePageModel) cycleEffects() {
	switch m.Effects {
	case uifx.LevelMinimal:
		m.Effects = uifx.LevelMedium
	case uifx.LevelMedium:
		m.Effects = uifx.LevelHigh
	case uifx.LevelHigh:
		m.Effects = uifx.LevelMinimal
	}
	m.clampProgressStyle()
}

// effectsStrip shows the current fanciness tier and doubles as a
// click-to-cycle control (see cycleEffects) — the page's own dogfooding of
// uifx.Level, discoverable without a trip to Settings.
func (m *HomePageModel) effectsStrip() string {
	c := m.Colors()
	return c.Styles.Subtitle.Render("fx  ") +
		c.Styles.Title.Render(fmt.Sprintf("%s  (click to cycle)", m.Effects))
}

// themeStrip shows live swatches of the palette driving every color on this
// page plus the active theme's name, and doubles as a click-to-cycle control
// (see cycleThemeCmd) — no trip to Settings required. Switch it here and the
// whole page repaints instantly — proof it's theme-driven, not hardcoded.
// Swatch names drop below compactBreakpoint so the strip never wraps.
func (m *HomePageModel) themeStrip() string {
	c := m.Colors()
	compact := m.Width() < compactBreakpoint

	dot := func(col color.Color) string {
		return lipgloss.NewStyle().Foreground(col).Render("●")
	}
	swatches := []struct {
		name string
		col  color.Color
	}{
		{"accent", c.Accent},
		{"success", c.Success},
		{"warning", c.Warning},
		{"error", c.Error},
	}

	sep := "  "
	parts := make([]string, 0, len(swatches))
	for _, s := range swatches {
		if compact {
			sep = " "
			parts = append(parts, dot(s.col))
			continue
		}
		parts = append(parts, dot(s.col)+" "+c.Styles.Subtitle.Render(s.name))
	}
	swatchLine := c.Styles.Subtitle.Render("theme  ") + strings.Join(parts, sep)

	name := "custom"
	if c.OrigTint != nil {
		name = c.OrigTint.DisplayName
	}
	label := name + "  (click to cycle)"
	if compact {
		label = name
	}
	return swatchLine + "  " + c.Styles.Title.Render(label)
}

// themeSource returns the current theme's attribution — the name and URL of
// the terminal color scheme it was ported from — when bubbletint recorded
// one. Not every tint has credit sources, so ok is false when there's
// nothing to show or link to.
func (m *HomePageModel) themeSource() (name, url string, ok bool) {
	c := m.Colors()
	if c.OrigTint == nil || len(c.OrigTint.CreditSources) == 0 {
		return "", "", false
	}
	src := c.OrigTint.CreditSources[0]
	if src.Link == "" {
		return "", "", false
	}
	return src.Name, src.Link, true
}

// themeURLLine renders the current theme's attribution URL as a real OSC-8
// terminal hyperlink (lipgloss Style.Hyperlink) — clicking it (see onClick,
// zoneThemeURL) also opens the OS default browser for terminals that don't
// render OSC-8 as clickable. Empty when the active theme has no attribution.
func (m *HomePageModel) themeURLLine() string {
	name, url, ok := m.themeSource()
	if !ok {
		return ""
	}
	c := m.Colors()
	label := url
	if name != "" {
		label = name + " — " + url
	}
	return c.Styles.Subtitle.Render("↗ ") + c.Styles.Subtitle.Hyperlink(url).Render(label)
}

// cycleThemeCmd advances to the next tint in bubbletint's registry (wrapping
// around) and applies it directly — the same settings.ThemeMsg the Settings
// page's theme picker emits, but with ApplyPreferences left false so it just
// swaps the palette without touching mode/style/accessibility prefs.
func (m *HomePageModel) cycleThemeCmd() tea.Cmd {
	tints := tint.Tints()
	if len(tints) == 0 {
		return nil
	}
	currentID := ""
	if c := m.Colors(); c.OrigTint != nil {
		currentID = c.OrigTint.ID
	}
	next := tints[0].ID
	for i, t := range tints {
		if t.ID == currentID {
			next = tints[(i+1)%len(tints)].ID
			break
		}
	}
	return func() tea.Msg { return settings.ThemeMsg{ID: next} }
}

// openThemeSourceCmd opens the active theme's attribution URL in the OS
// default browser; nil when the theme has none.
func (m *HomePageModel) openThemeSourceCmd() tea.Cmd {
	_, url, ok := m.themeSource()
	if !ok {
		return nil
	}
	return openBrowserCmd(url)
}

// browserOpenedMsg reports the result of an openBrowserCmd attempt.
type browserOpenedMsg struct {
	URL string
	Err error
}

// openBrowserCmd launches the OS default browser on url (the same technique
// as the Inspector's pprof web-endpoint links).
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case osWindows:
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		case osDarwin:
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err != nil {
			return browserOpenedMsg{URL: url, Err: err}
		}
		return browserOpenedMsg{URL: url}
	}
}

// responsiveStrip reports the live terminal size and the layout breakpoint it
// falls in. Resize the terminal and watch this line, the chart block, and the
// theme strip all reflow together.
func (m *HomePageModel) responsiveStrip() string {
	c := m.Colors()
	return c.Styles.Subtitle.Render(
		fmt.Sprintf("viewport %dx%d  ·  %s layout  (try resizing the terminal)", m.Width(), m.Height(), m.breakpoint()),
	)
}

// hoverWrap borders a block so hovering it (LevelHigh only, see onMotion)
// highlights it in the selection/highlight color (c.SelectionBg) — the theme's
// secondary highlight — rather than the primary accent, so a hovered block
// reads as distinct from the accent-colored welcome border and tab lines. The
// border is always reserved once Effects.Hover() is true — only its color
// toggles — so a block lighting up under the pointer never shifts its neighbors.
func (m *HomePageModel) hoverWrap(zone, s string) string {
	c := m.Colors()
	borderColor := c.Styles.TextOnBg.GetBackground()
	if m.hoverZone == zone {
		borderColor = c.SelectionBg
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(s)
}

// wide reports whether the page should use the side-by-side chart layout.
func (m *HomePageModel) wide() bool { return m.Width() >= wideBreakpoint }

// breakpoint names the current responsive layout tier.
func (m *HomePageModel) breakpoint() string {
	switch {
	case m.Width() < compactBreakpoint:
		return "compact"
	case m.Width() < wideBreakpoint:
		return "cozy"
	default:
		return "wide"
	}
}

// resizeCharts scales the sparkline and load bar with the terminal so the
// activity charts visibly grow on wide terminals — part of the responsive
// layout showcase (see wide, breakpoint).
func (m *HomePageModel) resizeCharts() {
	sparkW, barW := 26, 20
	if m.wide() {
		sparkW, barW = 40, 28
	}
	m.spark.SetSize(sparkW, 2)
	m.load.SetSize(barW, 2)
}

// content builds the centered hub card from positioned layers and registers
// the interactive blocks as hit zones from those same layers, so the zones
// can never drift from what is on screen.
func (m *HomePageModel) content() string {
	c := m.Colors()
	availW := max(m.Width(), 10)

	// The welcome card reads in the theme's normal foreground, framed by a
	// border in the accent color so it matches the tab/nav line color (see
	// styles.TabInactive) rather than the green it used to borrow from Success.
	box := c.Styles.TextOnBg.
		Bold(true).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.Accent).
		MaxWidth(availW).
		Render(welcomeText)
	pills := m.pillStrip()
	chartsBlk := m.chartBlock()
	progressStyleBlk := m.progressStyleStrip()
	theme := m.themeStrip()
	effects := m.effectsStrip()
	responsive := m.responsiveStrip()

	blocks := []contentBlock{
		{zoneBox, box},
		{zonePills, pills},
		{zoneCharts, chartsBlk},
		{zoneProgressStyle, progressStyleBlk},
		{zoneTheme, theme},
	}
	if url := m.themeURLLine(); url != "" {
		blocks = append(blocks, contentBlock{zoneThemeURL, url})
	}
	blocks = append(blocks, contentBlock{zoneEffects, effects}, contentBlock{zoneResponsive, responsive})

	disksBlk, disksHeadingH := m.disksBlock(c)
	blocks = append(blocks, contentBlock{zoneDisks, disksBlk})

	// Hover highlighting only exists at LevelHigh (see Effects doc): every
	// zone gets a border reserved up front so a block lighting up under the
	// pointer never reflows its neighbors (see hoverWrap). The Disks table is
	// skipped — it has its own selection highlight, and an extra border would
	// offset the coordinates its click hit-testing depends on.
	if m.Effects.Hover() {
		for i := range blocks {
			if blocks[i].id == zoneDisks {
				continue
			}
			blocks[i].str = m.hoverWrap(blocks[i].id, blocks[i].str)
		}
	}

	// Stack the blocks with one blank line between, horizontally centered and
	// anchored to the top of the page (not vertically centered) so the layout
	// stays put as the Disks table grows or the terminal resizes.
	totalH := 0
	for _, b := range blocks {
		totalH += lipgloss.Height(b.str) + 1
	}
	totalH--

	layers := make([]*lipgloss.Layer, 0, len(blocks)+1)
	zoneLayers := make([]*lipgloss.Layer, 0, len(blocks))
	// A transparent backdrop pins the compositor canvas to the full page
	// size so centered blocks keep their offsets.
	h := max(m.Height(), totalH)
	backdrop := lipgloss.NewStyle().Width(availW).Height(h).Render("")
	layers = append(layers, lipgloss.NewLayer(backdrop))
	y := 0
	for _, b := range blocks {
		x := max((availW-lipgloss.Width(b.str))/2, 0)
		layers = append(layers, lipgloss.NewLayer(b.str).X(x).Y(y).Z(1))
		if b.id != "" {
			zoneLayers = append(zoneLayers, lipgloss.NewLayer(b.str).ID(b.id).X(x).Y(y))
		}
		if b.id == zoneDisks {
			// Record where the table lands so pointer events can be mapped into
			// the table's own coordinates. The table is rendered once (at origin
			// 0, in disksBlock) — its output doesn't depend on originY — so the
			// click handlers translate content coordinates by subtracting this
			// origin instead of forcing a second render just to fix geometry.
			m.disksLeft = x
			m.disksTop = y + disksHeadingH
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
	if mo.Button == tea.MouseRight {
		zone := m.zones.Hit(mo.X, mo.Y+m.vp.YOffset())
		if zone == "" {
			return nil
		}
		// A right-click on the Disks table selects the row under the pointer and
		// opens that drive's actions menu instead of the generic block menu.
		if zone == zoneDisks {
			if r, ok := m.disksTbl.SelectRowAt(mo.Y + m.vp.YOffset() - m.disksTop); ok {
				m.openDiskMenu(mo.X, mo.Y, r.Key)
			}
			return nil
		}
		// Open at the raw screen coordinates: Composite draws the menu over
		// the already-scrolled body, so it belongs where the click visually
		// landed, not in content (pre-scroll) space.
		m.contextMenu.Open(mo.X, mo.Y, m.contextItems(zone), zone)
		return nil
	}
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
	case zoneProgressStyle:
		return m.cycleProgressStyle()
	case zoneTheme:
		return m.cycleThemeCmd()
	case zoneThemeURL:
		return m.openThemeSourceCmd()
	case zoneEffects:
		m.cycleEffects()
	case zoneDisks:
		// Forward to the table in its own (origin-0) coordinates: X relative to
		// the table's left edge, Y relative to the table's top. A header click
		// sorts; a row click selects; a quick second click opens.
		return m.disksTbl.HandleClick(mo.X-m.disksLeft, mo.Y+m.vp.YOffset()-m.disksTop)
	}
	return nil
}

// onMouseRoot is the page's actual View().OnMouse: while the context menu is
// open it gets first look at every mouse event (per the snap menu contract);
// otherwise events fall through to the normal zone-based handlers below.
func (m *HomePageModel) onMouseRoot(msg tea.MouseMsg) tea.Cmd {
	if m.contextMenu.IsOpen() {
		chosen, handled := m.contextMenu.HandleMouse(msg, m.Width(), m.Height())
		if chosen != nil {
			return m.applyContextChoice(chosen.ID)
		}
		if handled {
			return nil
		}
		// The click landed outside the menu: HandleMouse already closed it,
		// so fall through and let the page handle the same click (e.g. it
		// may open a different component's menu, or a normal left click).
	}
	return uifx.MouseHandlers{
		Click:   m.onClick,
		Release: m.onRelease,
		Motion:  m.onMotion,
		Wheel:   m.onWheel,
	}.OnMouse(msg)
}

// contextItems builds the right-click menu for one zone — an informational
// line about that block plus whatever action(s) it already supports, so
// every component on the page has something to show off via right-click.
func (m *HomePageModel) contextItems(zone string) []menu.Item {
	switch zone {
	case zoneBox:
		return []menu.Item{
			{Label: "Welcome panel — a snap bordered card", Disabled: true},
			{Label: "Border: RoundedBorder, tinted with theme Accent (matches tabs)", Disabled: true},
		}
	case zonePills:
		shapes := styles.PillShapes()
		shape := shapes[m.shape%len(shapes)]
		return []menu.Item{
			{Label: "Pill shape: " + shape.DisplayName(), Disabled: true},
			{ID: "cycle-shape", Label: "Cycle shape"},
		}
	case zoneCharts:
		pauseLabel := "Pause"
		if m.paused {
			pauseLabel = "Resume"
		}
		return []menu.Item{
			{Label: "Sparkline style: " + charts.SparklineStyleName(m.sparkStyle), Disabled: true},
			{ID: "toggle-pause", Label: pauseLabel},
			{ID: "cycle-spark-style", Label: "Cycle sparkline style"},
		}
	case zoneProgressStyle:
		items := make([]menu.Item, 0, 3)
		items = append(
			items,
			menu.Item{Label: fmt.Sprintf("Bar style: %s", m.progStyle), Disabled: true},
			menu.Item{ID: "cycle-progress-style", Label: "Cycle bar style"},
		)
		names := make([]string, 0, 5)
		for _, s := range availableProgressStyles(m.Effects) {
			names = append(names, s.String())
		}
		items = append(items, menu.Item{Label: "Available at fx " + m.Effects.String() + ": " + strings.Join(names, ", "), Disabled: true})
		return items
	case zoneTheme:
		name := "custom"
		if c := m.Colors(); c.OrigTint != nil {
			name = c.OrigTint.DisplayName
		}
		items := []menu.Item{
			{Label: "Theme: " + name, Disabled: true},
			{ID: "cycle-theme", Label: "Cycle theme"},
		}
		if _, _, ok := m.themeSource(); ok {
			items = append(items, menu.Item{ID: actionOpenThemeSource, Label: "Open theme source"})
		}
		return items
	case zoneThemeURL:
		_, url, _ := m.themeSource()
		return []menu.Item{
			{Label: url, Disabled: true},
			{ID: actionOpenThemeSource, Label: "Open in browser"},
		}
	case zoneEffects:
		return []menu.Item{
			{Label: "minimal: cheapest static bar, no hover", Disabled: true},
			{Label: "medium (default): + 3 static bubbles/progress styles", Disabled: true},
			{Label: "high: + animated bar, hover, AllMotion", Disabled: true},
			{ID: "cycle-effects", Label: "Cycle fx level"},
		}
	case zoneResponsive:
		return []menu.Item{
			{Label: "compact <60 · cozy 60-83 · wide ≥84", Disabled: true},
			{Label: fmt.Sprintf("current: %s (%dx%d)", m.breakpoint(), m.Width(), m.Height()), Disabled: true},
		}
	default:
		return nil
	}
}

// applyContextChoice runs the action behind a chosen context-menu item ID —
// the same effects the page's left clicks already have, reached from the
// right-click menu instead. Item IDs are unique across every zone's menu, so
// no zone tag is needed to disambiguate.
func (m *HomePageModel) applyContextChoice(id string) tea.Cmd {
	switch id {
	case "cycle-shape":
		m.shape = (m.shape + 1) % len(styles.PillShapes())
	case "toggle-pause":
		m.paused = !m.paused
	case "cycle-spark-style":
		m.sparkStyle = charts.SparklineStyle((int(m.sparkStyle) + 1) % sparklineStyleCount)
	case "cycle-progress-style":
		return m.cycleProgressStyle()
	case "cycle-theme":
		return m.cycleThemeCmd()
	case actionOpenThemeSource:
		return m.openThemeSourceCmd()
	case "cycle-effects":
		m.cycleEffects()
	case diskActionOpen:
		return openPathCmd(m.menuDiskPath)
	case diskActionRefresh:
		m.refreshDisks()
	}
	return nil
}

// CapturesKeys reports that the page needs exclusive keyboard focus while the
// Disks table's filter input is active, so the router forwards every key (the
// filter text, not global shortcuts) straight to the page. See
// navigation.KeyCapturer.
func (m *HomePageModel) CapturesKeys() bool {
	return m.disksTbl != nil && m.disksTbl.Filtering()
}

func (m *HomePageModel) onMotion(mo tea.Mouse) tea.Cmd {
	if m.dragging {
		m.jumpTo(mo.Y)
	}
	// Hover highlighting (see hoverWrap) only tracks at LevelHigh — it's the
	// one feature that actually needs the AllMotion firehose (see View).
	if m.Effects.Hover() {
		m.hoverZone = m.zones.Hit(mo.X, mo.Y+m.vp.YOffset())
	}
	return nil
}

func (m *HomePageModel) onRelease(tea.Mouse) tea.Cmd {
	m.dragging = false
	return nil
}

func (m *HomePageModel) onWheel(mo tea.Mouse) tea.Cmd {
	// Over the Disks table the wheel moves the row selection (and pages long
	// lists); anywhere else it scrolls the page viewport.
	if m.zones.Hit(mo.X, mo.Y+m.vp.YOffset()) == zoneDisks {
		m.disksTbl.HandleWheel(mo.Button == tea.MouseWheelUp)
		return nil
	}
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
			m.vp.YOffset(), m.vp.VisibleLineCount(), styles.ScrollbarStyles(c),
		)
		body = lipgloss.NewCompositor(
			lipgloss.NewLayer(body),
			lipgloss.NewLayer(bar).X(max(m.Width()-1, 0)).Y(0).Z(1),
		).Render()
	}

	// The right-click context menu draws on top of everything, including the
	// scrollbar — it's positioned in absolute screen coordinates (see onClick).
	body = m.contextMenu.Composite(body, m.Width(), m.Height())

	v := tea.NewView(body)
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	// Only LevelHigh asks for AllMotion (the router forwards this up — see
	// router.RouterModel.View); everything else stays on the cheaper
	// CellMotion the rest of the app uses.
	v.MouseMode = m.Effects.MouseMode()
	v.OnMouse = m.onMouseRoot
	return v
}

package status

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type UserNotificationOverlay struct {
	width int

	// visibility / animation
	visible       bool
	animating     bool
	animFrame     int
	animFrames    int
	animDirection int // 1 showing, -1 hiding, 0 idle

	// notification manager (shared pointer, nil = no notifications)
	notifMgr *notifications.Manager

	// history panel state
	showHistory   bool
	historyCursor int

	tickInterval time.Duration
}

// Public messages for controlling the status bar.
type ToggleVisibilityMsg struct{}
type TickMsg struct{}

// UserNotificationOverlay owns status-bar visibility, animation, full-help state, and the link
// to the shared notification manager. It is not a full tea.UserNotificationOverlay; it exposes
// helpers that integrate with the parent BarModel event loop.
func NewUserNotificationOverlay() *UserNotificationOverlay {
	return &UserNotificationOverlay{
		visible:      true,
		animFrames:   8,
		tickInterval: 40 * time.Millisecond,
	}
}

// SetWidth stores the terminal width for rendering.
func (m *UserNotificationOverlay) SetWidth(w int) { m.width = w }

// SetNotifManager wires the shared notification manager.
func (m *UserNotificationOverlay) SetNotifManager(nm *notifications.Manager) { m.notifMgr = nm }

// ShouldShow reports whether the bar should be rendered.
func (m *UserNotificationOverlay) ShouldShow() bool { return m.visible || m.animating }

// Visible returns the immediate visible flag.
func (m *UserNotificationOverlay) Visible() bool { return m.visible }

// ShowHistory reports whether the notification history panel is open.
func (m *UserNotificationOverlay) ShowHistory() bool { return m.showHistory }

// ForceToggleVisibility toggles visibility without animation.
func (m *UserNotificationOverlay) ForceToggleVisibility() {
	m.visible = !m.visible
	m.animating = false
	m.animFrame = 0
	m.animDirection = 0
}

// ToggleVisibility starts an animated show/hide.
func (m *UserNotificationOverlay) ToggleVisibility() tea.Cmd {
	if m.animating {
		return nil
	}
	if m.visible {
		m.animDirection = -1
	} else {
		m.animDirection = 1
	}
	m.visible = !m.visible
	m.animating = true
	m.animFrame = 0
	return tea.Tick(m.tickInterval, func(t time.Time) tea.Msg { return TickMsg{} })
}

// ToggleHistory opens or closes the notification history panel.
func (m *UserNotificationOverlay) ToggleHistory() tea.Cmd {
	m.showHistory = !m.showHistory
	if m.showHistory {
		m.historyCursor = 0
	}
	return nil
}

// CloseHistory closes the history panel.
func (m *UserNotificationOverlay) CloseHistory() { m.showHistory = false }

// HistoryCursorUp moves the history cursor up.
func (m *UserNotificationOverlay) HistoryCursorUp() {
	if m.historyCursor > 0 {
		m.historyCursor--
	}
}

// HistoryCursorDown moves the history cursor down.
func (m *UserNotificationOverlay) HistoryCursorDown(maxItems int) {
	if m.historyCursor < maxItems-1 {
		m.historyCursor++
	}
}

// HistoryCursor returns the current cursor index.
func (m *UserNotificationOverlay) HistoryCursor() int { return m.historyCursor }

// Update processes incoming control messages and returns any follow-up command.
// It is called by BarModel.Update rather than directly by the bubbletea runtime.
func (m *UserNotificationOverlay) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case ToggleVisibilityMsg:
		return m.ToggleVisibility()
	case TickMsg:
		if !m.animating {
			return nil
		}
		m.animFrame++
		if m.animFrame < m.animFrames {
			return tea.Tick(m.tickInterval, func(t time.Time) tea.Msg { return TickMsg{} })
		}
		m.animating = false
		m.animFrame = 0
		m.animDirection = 0
		return nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return nil
	}
	return nil
}

// Render builds and returns the status line and interactive click regions.
// Overlays (history panel, info modal) are rendered separately and composited
// by the router so they float above all other content.
func (m *UserNotificationOverlay) Render(width int, left, right string) (string, []ClickRegion) {
	// fade progress for animation
	f := 1.0
	if m.animating && m.animFrames > 0 {
		prog := float64(m.animFrame) / float64(m.animFrames-1)
		if m.animDirection == -1 {
			prog = 1 - prog
		}
		prog = max(0, min(1, prog))
		f = prog
	}

	// horizontal slide (left indent) and ANSI color animation
	indentN := max(int((1.0-f)*8.0), 0)
	left = strings.Repeat(" ", indentN) + left
	colorMin, colorMax := 242, 250
	color := colorMin + int(f*float64(colorMax-colorMin))

	// Determine notification icon (🔔 enabled, 🔕 disabled).
	notifEnabled := true
	if m.notifMgr != nil {
		notifEnabled = m.notifMgr.Enabled()
	}
	statusLine, regions := RenderStyled(width, left, right, color, notifEnabled)

	return statusLine, regions
}

// RenderHistoryOverlay builds the notification history panel sized to fit within
// maxW columns and maxH rows, returning an empty string when not visible.
// The caller (router) positions it via canvas compositing.
func (m *UserNotificationOverlay) RenderHistoryOverlay(maxW, maxH int) string {
	return m.renderHistoryOverlay(maxW, maxH)
}

// renderHistoryOverlay builds the notification history panel sized to fit within
// maxW columns and maxH rows, returning "" when not visible. It is called by
// RenderHistoryOverlay which is called by the router for canvas compositing.
func (m *UserNotificationOverlay) renderHistoryOverlay(maxW, maxH int) string {
	if !m.showHistory || m.notifMgr == nil {
		return ""
	}
	c := theme.Active()
	active := m.notifMgr.Active()
	activeCount := len(active)
	if activeCount == 0 {
		m.historyCursor = 0
	} else if m.historyCursor >= activeCount {
		m.historyCursor = activeCount - 1
	}

	// Cap the panel width: at most 80 cols, always fits on screen.
	panelW := min(80, maxW-2)
	if panelW < 20 {
		panelW = maxW
	}
	// Inner content width = panelW - border L/R (2).
	innerW := panelW - 2

	// header — must fill innerW so JoinVertical adds no unstyled padding.
	headerStyle := c.Styles.FilterDim.Bold(true).Foreground(c.Styles.Title.GetForeground())
	header := headerStyle.Width(innerW).Render(fmt.Sprintf(" Notifications (%d active) ", activeCount))

	// rows (cap to maxH - 5 to leave room for header, footer, borders)
	maxRows := max(maxH-5, 1)
	start := 0
	if activeCount > maxRows {
		start = max(m.historyCursor-maxRows+1, 0)
		maxStart := activeCount - maxRows
		if start > maxStart {
			start = maxStart
		}
	}
	end := min(start+maxRows, activeCount)
	var rows []string
	for i := start; i < end; i++ {
		n := active[i]
		ageStr := formatAge(time.Since(n.CreatedAt))
		sevStyle := c.Styles.FilterDim.Foreground(lipgloss.Color(m.colorForSeverity(n.Severity))).Bold(true)
		badge := sevStyle.Render("[" + n.Severity.Badge() + "]")

		// Layout: [badge][space+content][gap][space+ageStr] = innerW.
		// Subtract 2 for the two leading spaces in contentPart and agePart.
		contentMaxW := innerW - lipgloss.Width(badge) - lipgloss.Width(ageStr) - 2
		content := n.Content
		if contentMaxW > 1 && lipgloss.Width(content) > contentMaxW {
			runes := []rune(content)
			content = string(runes[:contentMaxW-1]) + "…"
		}

		rowBg := c.Styles.TextOnBg.GetBackground()
		rowFg := c.Styles.StatusBase.GetForeground()
		if i == m.historyCursor {
			rowBg = c.Styles.TextOnBg.GetBackground()
			rowFg = c.Styles.SelectedItem.GetForeground()
		}

		rowStyle := c.Styles.Row.Background(rowBg)
		contentStyle := c.Styles.Row.Background(rowBg).Foreground(rowFg)
		ageStyle := c.Styles.Row.Background(rowBg).Foreground(c.Styles.Subtitle.GetForeground())

		contentPart := contentStyle.Render(" " + content)
		agePart := ageStyle.Render(" " + ageStr)
		gapW := max(innerW-lipgloss.Width(badge)-lipgloss.Width(contentPart)-lipgloss.Width(agePart), 0)
		gap := rowStyle.Render(strings.Repeat(" ", gapW))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, badge, contentPart, gap, agePart))
	}

	if len(rows) == 0 {
		emptyStyle := c.Styles.FilterDim.Width(innerW)
		rows = append(rows, emptyStyle.Render("  No notifications"))
	}

	// footer
	footerStyle := c.Styles.FilterDim
	footer := footerStyle.Width(innerW).Render(" ↑↓ navigate  Enter dismiss  d dismiss all  Esc close")

	inner := lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(rows, "\n"), footer)
	borderStyle := c.Styles.OverlayBorder.
		Border(lipgloss.RoundedBorder()).
		BorderBackground(c.Styles.FilterDim.GetBackground()).
		Background(c.Styles.FilterDim.GetBackground())
	// Width(panelW) sets the TOTAL width including border.
	return borderStyle.Width(panelW).Render(inner)
}

// reapplyBg replaces every ANSI reset sequence (\x1b[m or \x1b[0m) with the
// reset immediately followed by the given background escape code. This patches
// the gaps left by the bubbles help widget, which resets styles between elements
// and then emits a plain (no-background) space before the next styled segment.
func reapplyBg(s string, bg color.Color) string {
	bgCode := firstEscapeFromStyle(lipgloss.NewStyle().Background(bg).Render("X"))
	if bgCode == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+bgCode)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+bgCode)
	return s
}

// firstEscapeFromStyle extracts the first ANSI escape sequence from a lipgloss-
// rendered string (e.g. "\x1b[48;5;236m" from "…\x1b[48;5;236mX\x1b[m").
func firstEscapeFromStyle(s string) string {
	i := strings.Index(s, "\x1b[")
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], "m")
	if j < 0 {
		return ""
	}
	return s[i : i+j+1]
}

// colorForSeverity returns a hex color string for the given severity badge.
func (m *UserNotificationOverlay) colorForSeverity(s notifications.Severity) string {
	switch s {
	case notifications.SeverityWarning:
		return "#F9C513"
	case notifications.SeverityError:
		return "#FF5757"
	default:
		return "#4FC3F7"
	}
}

// formatAge formats a duration as a short human-readable string.
func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

// ClickRegion describes an interactive horizontal region on the status bar
// by column start/end (inclusive) and a name identifier.
type ClickRegion struct {
	Start int
	End   int
	Name  string
}

const SettingsRegionName = "settings"
const NotificationsRegionName = "notifications"
const InfoRegionName = "info"

// RenderStyled composes a full-width status bar and returns its interactive
// click regions. Every segment (left text, gap, right text, icon pills) is
// individually styled with Background(StatusBg) so the bar has a consistent,
// unbroken background across its full width — no outer wrapper required.
//
// colorIndex overrides the foreground with that ANSI index (0-255) to drive
// fade-in/out animations; pass -1 to use the theme's StatusFg color.
//
// Icons are rendered as padded pills using Padding(0, 1) so there is one cell
// of breathing room on each side, and the entire pill is the click target.
//
// Layout: [left text] [gap] [right text] [⚙ pill] [🔔 pill]
func RenderStyled(width int, left, right string, colorIndex int, notifEnabled bool) (string, []ClickRegion) {
	c := theme.Active()
	fg := c.Styles.StatusBase.GetForeground()
	if colorIndex >= 0 {
		fg = lipgloss.Color(strconv.Itoa(colorIndex))
	}

	settingsIcon := "⚙️"
	notificationIcon := "🔔"
	if !notifEnabled {
		notificationIcon = "🔕"
	}
	infoIcon := "ℹ️"

	// Base style covers all non-icon segments.
	baseStyle := c.Styles.StatusBase.Foreground(fg)

	// Icon pills: Padding(0, 1) adds one cell on each side of the glyph,
	// giving each icon a generous click target and visual breathing room.
	iconStyle := c.Styles.StatusBase.Foreground(fg).Padding(0, 1)
	settingsPill := iconStyle.Render(settingsIcon)
	notifPill := iconStyle.Render(notificationIcon)
	infoPill := iconStyle.Render(infoIcon)
	spw := lipgloss.Width(settingsPill)
	npw := lipgloss.Width(notifPill)
	ipw := lipgloss.Width(infoPill)

	rightRendered := baseStyle.Render(right)
	rw := lipgloss.Width(rightRendered)

	// Split left into lines; full-help mode produces multiple lines.
	// Icons always appear on the last row so they don't float to the top.
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	lastLeftLine := leftLines[len(leftLines)-1]
	lastLineRendered := baseStyle.Render(lastLeftLine)
	llw := lipgloss.Width(lastLineRendered)

	// Gap computed from the last row's content width so icons stay flush right.
	gap := max(width-llw-rw-spw-npw-ipw, 1)

	// Build the bottom row that carries the icons.
	lastRow := lipgloss.JoinHorizontal(lipgloss.Left,
		lastLineRendered,
		baseStyle.Render(strings.Repeat(" ", gap)),
		rightRendered,
		settingsPill,
		notifPill,
		infoPill,
	)

	var rendered string
	if len(leftLines) <= 1 {
		rendered = lastRow
	} else {
		// Prefix rows: pad each to full width with StatusBg so no background holes.
		rows := make([]string, 0, len(leftLines))
		for _, line := range leftLines[:len(leftLines)-1] {
			lineRendered := baseStyle.Render(line)
			lw := lipgloss.Width(lineRendered)
			if pad := max(width-lw, 0); pad > 0 {
				rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
					lineRendered,
					baseStyle.Render(strings.Repeat(" ", pad)),
				))
			} else {
				rows = append(rows, lineRendered)
			}
		}
		rows = append(rows, lastRow)
		rendered = strings.Join(rows, "\n")
	}

	// Click regions are on the last row. Column offsets are relative to that row.
	settingsStart := llw + gap + rw
	settingsEnd := settingsStart + spw - 1
	notifStart := settingsEnd + 1
	notifEnd := notifStart + npw - 1
	infoStart := notifEnd + 1
	infoEnd := infoStart + ipw - 1

	regions := []ClickRegion{
		{Start: settingsStart, End: settingsEnd, Name: SettingsRegionName},
		{Start: notifStart, End: notifEnd, Name: NotificationsRegionName},
		{Start: infoStart, End: infoEnd, Name: InfoRegionName},
	}
	return rendered, regions
}

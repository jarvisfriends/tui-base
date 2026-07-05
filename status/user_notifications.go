package status

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type UserNotificationOverlay struct {
	width int

	visible       bool
	animating     bool
	animFrame     int
	animFrames    int
	animDirection int

	notifMgr *notifications.Manager

	showHistory   bool
	historyCursor int

	tickInterval time.Duration
}

type (
	ToggleVisibilityMsg struct{}
	TickMsg             struct{}
)

func NewUserNotificationOverlay() *UserNotificationOverlay {
	return &UserNotificationOverlay{
		visible:      true,
		animFrames:   8,
		tickInterval: 40 * time.Millisecond,
	}
}

func (m *UserNotificationOverlay) SetWidth(w int)                            { m.width = w }
func (m *UserNotificationOverlay) SetNotifManager(nm *notifications.Manager) { m.notifMgr = nm }

func (m *UserNotificationOverlay) ShouldShow() bool { return m.visible || m.animating }
func (m *UserNotificationOverlay) Visible() bool    { return m.visible }

func (m *UserNotificationOverlay) ShowHistory() bool { return m.showHistory }

func (m *UserNotificationOverlay) ForceToggleVisibility() {
	m.visible = !m.visible
	m.animating = false
	m.animFrame = 0
	m.animDirection = 0
}

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

func (m *UserNotificationOverlay) ToggleHistory() tea.Cmd {
	m.showHistory = !m.showHistory
	if m.showHistory {
		m.historyCursor = 0
	}
	return nil
}

func (m *UserNotificationOverlay) CloseHistory() { m.showHistory = false }

func (m *UserNotificationOverlay) HistoryCursorUp() {
	if m.historyCursor > 0 {
		m.historyCursor--
	}
}

func (m *UserNotificationOverlay) HistoryCursorDown(maxItems int) {
	if m.historyCursor < maxItems-1 {
		m.historyCursor++
	}
}

func (m *UserNotificationOverlay) HistoryCursor() int { return m.historyCursor }

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

func (m *UserNotificationOverlay) Render(width int, left, right string) (string, []ClickRegion) {
	f := 1.0
	if m.animating && m.animFrames > 0 {
		prog := float64(m.animFrame) / float64(m.animFrames-1)
		if m.animDirection == -1 {
			prog = 1 - prog
		}
		prog = max(0, min(1, prog))
		f = prog
	}

	indentN := max(int((1.0-f)*8.0), 0)
	left = strings.Repeat(" ", indentN) + left
	colorMin, colorMax := 242, 250
	color := colorMin + int(f*float64(colorMax-colorMin))

	notifEnabled := true
	pendingCount := 0
	if m.notifMgr != nil {
		notifEnabled = m.notifMgr.Enabled()
		pendingCount = m.notifMgr.PendingCount()
	}
	statusLine, regions := RenderStyled(width, left, right, color, notifEnabled, pendingCount)

	return statusLine, regions
}

func (m *UserNotificationOverlay) RenderHistoryOverlay(maxW, maxH int) string {
	if !m.showHistory || m.notifMgr == nil {
		return ""
	}
	c := theme.Active()
	active := m.notifMgr.Active()
	activeCount := len(active)
	pendingCount := m.notifMgr.PendingCount()
	if activeCount == 0 {
		m.historyCursor = 0
	} else if m.historyCursor >= activeCount {
		m.historyCursor = activeCount - 1
	}

	// The whole panel sits on the main app background (it reads as part of the
	// page, not the status bar). Backgrounds do not cascade through nested
	// lipgloss renders, so every segment style must carry it — onBg states it
	// once and every style below derives from it.
	onBg := func(s lipgloss.Style) lipgloss.Style { return s.Background(c.Bg) }

	titleText := fmt.Sprintf("🔔 Notifications (%d active", activeCount)
	if pendingCount > 0 {
		titleText += fmt.Sprintf(", %d pending", pendingCount)
	}
	titleText += ")"
	footerText := "↑/↓ navigate • Enter open/dismiss • d dismiss all • Esc close"

	maxRows := max(maxH-c.Styles.OverlayBorder.GetVerticalFrameSize()-historyOverlayChromeRows, 1)
	start := 0
	if activeCount > maxRows {
		start = max(m.historyCursor-maxRows+1, 0)
		maxStart := activeCount - maxRows
		if start > maxStart {
			start = maxStart
		}
	}
	end := min(start+maxRows, activeCount)

	// First pass: plain row parts, so the panel can size itself to fit
	// everything it needs before any truncation kicks in.
	type rowParts struct {
		badge, content, age string
		severity            notifications.Severity
	}
	parts := make([]rowParts, 0, end-start)
	needW := max(lipgloss.Width(titleText), lipgloss.Width(footerText))
	for i := start; i < end; i++ {
		n := active[i]
		content := n.Content
		if n.Pending {
			content += " [pending]"
		}
		p := rowParts{
			badge:    "[" + n.Severity.Badge() + "]",
			content:  content,
			age:      formatAge(time.Since(n.CreatedAt)),
			severity: n.Severity,
		}
		parts = append(parts, p)
		rowNeed := lipgloss.Width(p.badge) + lipgloss.Width(p.content) + lipgloss.Width(p.age) +
			historyRowLeadingSpaces + historyRowMinGap
		needW = max(needW, rowNeed)
	}

	// Fit-to-content width, capped by the available screen space.
	frameW := c.Styles.OverlayBorder.GetHorizontalFrameSize()
	innerW := min(needW, max(maxW-historyOverlayMinMarginX*2-frameW, 20))
	panelW := innerW + frameW

	// Info-modal chrome: centered title, full-width rules, centered footer.
	title := onBg(c.Styles.Title.Bold(true)).Width(innerW).Align(lipgloss.Center).Render(titleText)
	sep := onBg(c.Styles.Title).Render(strings.Repeat("─", innerW))
	footer := onBg(c.Styles.Subtitle).Width(innerW).Align(lipgloss.Center).Render(footerText)

	var rows []string
	for idx, p := range parts {
		// The cursor row is one continuous selection-background bar, matching
		// the sidebar, tables, and settings lists.
		rowBase := onBg(c.Styles.Row)
		contentFg := c.Styles.StatusBase.GetForeground()
		ageFg := c.Styles.Subtitle.GetForeground()
		if start+idx == m.historyCursor {
			rowBase = lipgloss.NewStyle().Background(c.SelectionBg)
			contentFg = c.SelectionFg
			ageFg = c.SelectionFg
		}
		badgeStyle := rowBase.
			Foreground(lipgloss.Color(m.colorForSeverity(p.severity))).
			Bold(true)

		content := p.content
		contentMaxW := innerW - lipgloss.Width(p.badge) - lipgloss.Width(p.age) -
			historyRowLeadingSpaces
		if contentMaxW > 1 && lipgloss.Width(content) > contentMaxW {
			runes := []rune(content)
			content = string(runes[:contentMaxW-historyEllipsisReserve]) + "..."
		}

		leadIn := badgeStyle.Render(p.badge) + rowBase.Foreground(contentFg).Render(" "+content)
		rows = append(rows, leadIn+lipgloss.PlaceHorizontal(
			innerW-lipgloss.Width(leadIn),
			lipgloss.Right,
			rowBase.Foreground(ageFg).Render(" "+p.age),
			lipgloss.WithWhitespaceStyle(rowBase),
		))
	}

	if len(rows) == 0 {
		emptyStyle := onBg(c.Styles.FilterDim).Width(innerW)
		rows = append(rows, emptyStyle.Render("  No notifications"))
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, title, sep, strings.Join(rows, "\n"), sep, footer)
	// c.Styles.OverlayBorder already carries the border config that innerW
	// and maxRows above measured via GetHorizontalFrameSize()/
	// GetVerticalFrameSize() — reused here so the two stay in sync. The accent
	// border matches the info modal's frame.
	borderStyle := onBg(c.Styles.OverlayBorder).
		BorderBackground(c.Bg).
		BorderForeground(c.Accent)
	return borderStyle.Width(panelW).Render(inner)
}

func (m *UserNotificationOverlay) colorForSeverity(s notifications.Severity) string {
	switch s {
	case notifications.SeverityWarning:
		return "#F9C513"
	case notifications.SeverityError:
		return "#FF5757"
	case notifications.SeverityInfo:
		return "#4FC3F7"
	}
	return "#4FC3F7"
}

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

const (
	// historyOverlayMinMarginX keeps breathing room between the history
	// overlay panel and the screen edge on each side.
	historyOverlayMinMarginX = 1

	// historyOverlayChromeRows accounts for the chrome lines drawn inside the
	// border: centered title, two separator rules, and the centered footer
	// (the info-modal layout). The border's own top/bottom rows are accounted
	// for separately via c.Styles.OverlayBorder.GetVerticalFrameSize().
	historyOverlayChromeRows = 4

	// historyRowMinGap is the minimum styled gap kept between a row's content
	// and its right-aligned age when the panel sizes itself to fit.
	historyRowMinGap = 2

	// historyRowLeadingSpaces is the one leading space rendered before both
	// the content and age segments of each notification row.
	historyRowLeadingSpaces = 2

	// historyEllipsisReserve leaves room to swap the row's last rune for the
	// truncation ellipsis when content overflows contentMaxW.
	historyEllipsisReserve = 1
)

type ClickRegion struct {
	Start int
	End   int
	Name  string
}

const (
	SettingsRegionName      = "settings"
	NotificationsRegionName = "notifications"
	InfoRegionName          = "info"
)

// RenderStyled composes a full-width status bar and returns its interactive click regions.
// Every segment is individually styled with Background(StatusBg) so the bar has a consistent
// background across its full width.
func RenderStyled(
	width int,
	left, right string,
	colorIndex int,
	notifEnabled bool,
	pendingCount int,
) (string, []ClickRegion) {
	c := theme.Active()
	fg := c.Styles.StatusBase.GetForeground()
	if colorIndex >= 0 {
		fg = lipgloss.Color(strconv.Itoa(colorIndex))
	}

	settingsIcon := "⚙️"
	notificationIcon := "🔔"
	if pendingCount > 0 {
		notificationIcon = fmt.Sprintf("🔔 %d", pendingCount)
	}
	if !notifEnabled {
		notificationIcon = "🔕"
	}
	infoIcon := "ℹ️"

	baseStyle := c.Styles.StatusBase.Foreground(fg)
	iconStyle := c.Styles.StatusBase.Foreground(fg).Padding(0, 1)
	settingsPill := iconStyle.Render(settingsIcon)
	notifPill := iconStyle.Render(notificationIcon)
	infoPill := iconStyle.Render(infoIcon)
	spw := lipgloss.Width(settingsPill)
	npw := lipgloss.Width(notifPill)
	ipw := lipgloss.Width(infoPill)

	rightRendered := baseStyle.Render(right)
	rw := lipgloss.Width(rightRendered)

	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	lastLeftLine := leftLines[len(leftLines)-1]
	lastLineRendered := baseStyle.Render(lastLeftLine)
	llw := lipgloss.Width(lastLineRendered)

	// Truncate the variable-length left text so the right segment and icons
	// always fit; otherwise the row exceeds the terminal width and the last
	// icon wraps onto its own line, corrupting the frame.
	if avail := width - rw - spw - npw - ipw - 1; llw > avail && avail > 0 {
		lastLineRendered = ansi.Truncate(lastLineRendered, avail, "…")
		llw = lipgloss.Width(lastLineRendered)
	}

	// Left-aligned help text, right-aligned summary + icon cluster. The gap is
	// produced by PlaceHorizontal with baseStyle whitespace so the status
	// background runs unbroken across the full row — no manual gap math.
	rightCluster := rightRendered + settingsPill + notifPill + infoPill
	lastRow := lastLineRendered + lipgloss.PlaceHorizontal(
		width-llw,
		lipgloss.Right,
		rightCluster,
		lipgloss.WithWhitespaceStyle(baseStyle),
	)

	var rendered string
	if len(leftLines) <= 1 {
		rendered = lastRow
	} else {
		rows := make([]string, 0, len(leftLines))
		for _, line := range leftLines[:len(leftLines)-1] {
			// Width() pads the row to the full bar width and the padding
			// inherits the style's background.
			line = ansi.Truncate(line, width, "…")
			rows = append(rows, baseStyle.Width(width).Render(line))
		}
		rows = append(rows, lastRow)
		rendered = strings.Join(rows, "\n")
	}

	gap := max(width-llw-rw-spw-npw-ipw, 0)
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

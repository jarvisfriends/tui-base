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
func (m *UserNotificationOverlay) ShouldShow() bool                          { return m.visible || m.animating }
func (m *UserNotificationOverlay) Visible() bool                             { return m.visible }
func (m *UserNotificationOverlay) ShowHistory() bool                         { return m.showHistory }

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

	panelW := min(84, maxW-2)
	if panelW < 20 {
		panelW = maxW
	}
	innerW := panelW - 2

	headerStyle := c.Styles.FilterDim.Bold(true).Foreground(c.Styles.Title.GetForeground()).Background(c.StatusBg)
	headerText := fmt.Sprintf(" Notifications (%d active", activeCount)
	if pendingCount > 0 {
		headerText += fmt.Sprintf(", %d pending", pendingCount)
	}
	headerText += " ) "
	header := headerStyle.Width(innerW).Render(headerText)

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
		sevStyle = sevStyle.Background(c.StatusBg)
		badge := sevStyle.Render("[" + n.Severity.Badge() + "]")

		pendingLabel := ""
		if n.Pending {
			pendingLabel = " [pending]"
		}
		content := n.Content + pendingLabel
		contentMaxW := innerW - lipgloss.Width(badge) - lipgloss.Width(ageStr) - 2
		if contentMaxW > 1 && lipgloss.Width(content) > contentMaxW {
			runes := []rune(content)
			content = string(runes[:contentMaxW-1]) + "..."
		}

		rowFg := c.Styles.StatusBase.GetForeground()
		if i == m.historyCursor {
			rowFg = c.Styles.SelectedItem.GetForeground()
		}

		rowStyle := c.Styles.Row.Background(c.StatusBg)
		contentStyle := c.Styles.Row.Background(c.StatusBg).Foreground(rowFg)
		ageStyle := c.Styles.Row.Background(c.StatusBg).Foreground(c.Styles.Subtitle.GetForeground())

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

	footerStyle := c.Styles.FilterDim.Background(c.StatusBg)
	footer := footerStyle.Width(innerW).Render(" up/down navigate  Enter open/dismiss  d dismiss all  Esc close")

	inner := lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(rows, "\n"), footer)
	borderStyle := c.Styles.OverlayBorder.
		Border(lipgloss.RoundedBorder()).
		BorderBackground(c.StatusBg).
		Background(c.StatusBg)
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
func RenderStyled(width int, left, right string, colorIndex int, notifEnabled bool, pendingCount int) (string, []ClickRegion) {
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

	gap := max(width-llw-rw-spw-npw-ipw, 1)

	lastRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
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
		rows := make([]string, 0, len(leftLines))
		for _, line := range leftLines[:len(leftLines)-1] {
			lineRendered := baseStyle.Render(line)
			lw := lipgloss.Width(lineRendered)
			if pad := max(width-lw, 0); pad > 0 {
				rows = append(rows, lipgloss.JoinHorizontal(
					lipgloss.Left,
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

package inspector

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

const (
	inputColMod    = "Mod"
	inputColButton = "Button"
)

func (m *InspectorModel) colorStat(
	c *theme.AppStyle,
	val, warn, crit float64,
	rendered string,
) string {
	switch {
	case val >= crit:
		return c.Styles.Error.Bold(true).Render(rendered)
	case val >= warn:
		return c.Styles.Warning.Render(rendered)
	default:
		return c.Styles.Success.Render(rendered)
	}
}

func (m *InspectorModel) buildRuntimeRows(c *theme.AppStyle) []table.Row {
	valStyle := c.Styles.TextOnBg
	st := m.stats
	pr := m.prevStats
	elapsed := st.CapturedAt.Sub(pr.CapturedAt).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	gcPerSec := float64(0)
	allocsPerSec := float64(0)
	if st.NumGC >= pr.NumGC {
		gcPerSec = float64(st.NumGC-pr.NumGC) / elapsed
	}
	if st.Mallocs >= pr.Mallocs {
		allocsPerSec = float64(st.Mallocs-pr.Mallocs) / elapsed
	}
	p := m.printer

	return []table.Row{
		{
			"Uptime", st.Uptime.Round(time.Second).String(),
			"Go", valStyle.Render(st.GoVersion),
			"OS/Arch", valStyle.Render(p.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)),
			"PID", valStyle.Render(p.Sprintf("%d", os.Getpid())),
		},
		{
			"Goroutines",
			m.colorStat(c, float64(st.Goroutines), 100, 500, strconv.Itoa(st.Goroutines)),
			"GOMAXPROCS",
			valStyle.Render(p.Sprintf("%d/%d", st.GOMAXPROCS, st.NumCPU)),
			"CGO Calls",
			valStyle.Render(p.Sprintf("%d", st.NumCgoCalls)),
			"Term Size",
			valStyle.Render(p.Sprintf("%dx%d", m.Width(), m.Height())),
		},
		{
			"Heap Alloc",
			m.colorStat(
				c,
				float64(st.HeapAllocBytes)/1024/1024,
				100,
				500,
				humanize.IBytes(st.HeapAllocBytes),
			),
			"Heap InUse",
			valStyle.Render(humanize.IBytes(st.HeapInUseBytes)),
			"Heap Sys",
			valStyle.Render(humanize.IBytes(st.HeapSysBytes)),
			"Stack InUse",
			valStyle.Render(humanize.IBytes(st.StackInUseBytes)),
		},
		{
			"GC Cycles",
			valStyle.Render(p.Sprintf("%d", st.NumGC)),
			"Last Pause",
			m.colorStat(
				c,
				float64(st.LastPause.Milliseconds()),
				1,
				10,
				st.LastPause.Round(time.Microsecond).String(),
			),
			"Total Paused",
			valStyle.Render(st.PauseTotal.Round(time.Millisecond).String()),
			"Bin Size",
			valStyle.Render(humanize.IBytes(uint64(max(0, st.Launch.BinarySize)))),
		},
		{
			"GC CPU %",
			m.colorStat(c, st.GcCPUFraction*100, 5, 25, p.Sprintf("%.2f%%", st.GcCPUFraction*100)),
			"GC/sec",
			m.colorStat(c, gcPerSec, 10, 50, p.Sprintf("%.1f", gcPerSec)),
			"Allocs/sec",
			m.colorStat(c, allocsPerSec, 10000, 100000, p.Sprintf("%.0f", allocsPerSec)),
			"Heap Objects",
			valStyle.Render(p.Sprintf("%d", st.HeapObjects)),
		},
	}
}

func (m *InspectorModel) buildInputRows(c *theme.AppStyle) []table.Row {
	valStyle := c.Styles.TextOnBg
	return []table.Row{
		{
			"Mouse Click",
			valStyle.Render(fmt.Sprintf("%d,%d", m.LastMouseClick.X, m.LastMouseClick.Y)),
			inputColMod,
			valStyle.Render(KeyModToString(m.LastMouseClick.Mod)),
			inputColButton,
			valStyle.Render(m.LastMouseClick.Button.String()),
		},
		{
			"Mouse Release",
			valStyle.Render(fmt.Sprintf("%d,%d", m.LastMouseRelease.X, m.LastMouseRelease.Y)),
			inputColMod,
			valStyle.Render(KeyModToString(m.LastMouseRelease.Mod)),
			inputColButton,
			valStyle.Render(m.LastMouseRelease.Button.String()),
		},
		{
			"Mouse Motion",
			valStyle.Render(fmt.Sprintf("%d,%d", m.LastMouseMotion.X, m.LastMouseMotion.Y)),
			inputColMod,
			valStyle.Render(KeyModToString(m.LastMouseMotion.Mod)),
			inputColButton,
			valStyle.Render(m.LastMouseMotion.Button.String()),
		},
		{
			"Mouse Wheel",
			valStyle.Render(fmt.Sprintf("%d,%d", m.LastMouseWheel.X, m.LastMouseWheel.Y)),
			inputColMod,
			valStyle.Render(KeyModToString(m.LastMouseWheel.Mod)),
			inputColButton,
			valStyle.Render(m.LastMouseWheel.Button.String()),
		},
		{
			"Key Press String",
			valStyle.Render(m.LastKeyPress.String()),
			"Text/Repeated",
			valStyle.Render(fmt.Sprintf("%s/%t", m.LastKeyPress.Text, m.LastKeyPress.IsRepeat)),
			inputColMod,
			valStyle.Render(KeyModToString(m.LastKeyPress.Mod)),
		},
		{
			"Key Rel String",
			valStyle.Render(m.LastKeyRel.String()),
			"Text/Repeated",
			valStyle.Render(fmt.Sprintf("%s/%t", m.LastKeyRel.Text, m.LastKeyRel.IsRepeat)),
			inputColMod,
			valStyle.Render(KeyModToString(m.LastKeyRel.Mod)),
		},
	}
}

func (m *InspectorModel) updateRuntimeColumnWidths(rows []table.Row) {
	for i := range m.runtimeColumns {
		m.runtimeColumns[i].Width = max(
			lipgloss.Width(m.runtimeColumns[i].Title),
			m.runtimeColMaxW[i],
		)
	}
	for _, row := range rows {
		for j := range row {
			w := lipgloss.Width(row[j])
			if w > m.runtimeColMaxW[j] {
				m.runtimeColMaxW[j] = w
			}
			m.runtimeColumns[j].Width = max(m.runtimeColumns[j].Width, w)
		}
	}
}

func (m *InspectorModel) updateInputColumnWidths(rows []table.Row) {
	for i := range m.inputDbgColumns {
		m.inputDbgColumns[i].Width = max(
			lipgloss.Width(m.inputDbgColumns[i].Title),
			m.inputDbgColMaxW[i],
		)
	}
	for _, row := range rows {
		for j := range row {
			w := lipgloss.Width(row[j])
			if w > m.inputDbgColMaxW[j] {
				m.inputDbgColMaxW[j] = w
			}
			m.inputDbgColumns[j].Width = max(m.inputDbgColumns[j].Width, w)
		}
	}
}

// baseTableStyles delegates to the shared theme mapping (TC-1) so the
// inspector's tables and any consumer table built via theme.TableStyles stay
// visually identical.
func (m *InspectorModel) baseTableStyles(c *theme.AppStyle) table.Styles {
	return theme.TableStyles(c)
}

func (m *InspectorModel) renderRuntimeSection(
	c *theme.AppStyle,
	s table.Styles,
	rows []table.Row,
	availW int,
) string {
	naturalW := 1
	for _, col := range m.runtimeColumns {
		naturalW += col.Width + s.Cell.GetHorizontalFrameSize()
	}
	if naturalW <= availW {
		m.setTableActive(debugTabRuntime, true)
		m.runtimeTbl.SetStyles(s)
		m.runtimeTbl.SetColumns(m.runtimeColumns)
		m.runtimeTbl.SetRows(rows)
		m.runtimeTbl.SetHeight(m.tableHeight(len(rows)))
		m.runtimeTbl.SetWidth(availW)
		return m.runtimeTbl.View()
	}
	m.setTableActive(debugTabRuntime, false)
	return renderRuntimeFlat(rows, c, availW)
}

// tableHeight caps a data table at the visible section height (min 3 rows so
// the header and at least one row always show); the table scrolls internally
// with the row cursor when its rows exceed the cap.
func (m *InspectorModel) tableHeight(rowCount int) int {
	h := rowCount + 2 // + header + header border
	if m.sectionHeight > 0 {
		h = min(h, max(m.sectionHeight, 3))
	}
	return h
}

func (m *InspectorModel) renderInputSection(
	c *theme.AppStyle,
	s table.Styles,
	rows []table.Row,
	availW int,
) string {
	naturalW := 1
	for _, col := range m.inputDbgColumns {
		naturalW += col.Width + s.Cell.GetHorizontalFrameSize()
	}
	if naturalW <= availW {
		m.setTableActive(debugTabInput, true)
		m.inputDbgTbl.SetStyles(s)
		m.inputDbgTbl.SetColumns(m.inputDbgColumns)
		m.inputDbgTbl.SetRows(rows)
		m.inputDbgTbl.SetHeight(m.tableHeight(len(rows)))
		m.inputDbgTbl.SetWidth(availW)
		return m.inputDbgTbl.View()
	}
	m.setTableActive(debugTabInput, false)
	return renderRuntimeFlat(rows, c, availW)
}

func (m *InspectorModel) renderDisksSection(c *theme.AppStyle, s table.Styles) string {
	st := m.stats
	var diskRows []table.Row
	for i := range m.diskHeader {
		m.diskHeader[i].Width = lipgloss.Width(m.diskHeader[i].Title)
	}
	for _, d := range st.Disks {
		if d.Error != "" {
			m.diskHeader[5].Width = max(m.diskHeader[5].Width, lipgloss.Width(d.Error))
			diskRows = append(diskRows, table.Row{d.Path, "unavailable", "", "", "", d.Error})
			continue
		}
		pct := 0.0
		if d.Total > 0 {
			pct = float64(d.Used) / float64(d.Total) * 100
		}

		var freeStr string
		switch {
		case d.Free < 100*1024*1024:
			freeStr = c.Styles.Error.Bold(true).Render(humanize.IBytes(d.Free))
		case d.Free < 1*1024*1024*1024:
			freeStr = c.Styles.Warning.Render(humanize.IBytes(d.Free))
		default:
			freeStr = c.Styles.Success.Render(humanize.IBytes(d.Free))
		}

		var pctStr string
		switch {
		case pct >= 90:
			pctStr = c.Styles.Error.Bold(true).Render(fmt.Sprintf("%0.0f%%", pct))
		case pct >= 75:
			pctStr = c.Styles.Warning.Render(fmt.Sprintf("%0.0f%%", pct))
		default:
			pctStr = c.Styles.Success.Render(fmt.Sprintf("%0.0f%%", pct))
		}

		usedStr := humanize.IBytes(d.Used)
		totalStr := humanize.IBytes(d.Total)
		m.diskHeader[0].Width = max(m.diskHeader[0].Width, lipgloss.Width(d.Path))
		m.diskHeader[1].Width = max(m.diskHeader[1].Width, lipgloss.Width(usedStr))
		m.diskHeader[2].Width = max(m.diskHeader[2].Width, lipgloss.Width(totalStr))
		m.diskHeader[3].Width = max(m.diskHeader[3].Width, lipgloss.Width(freeStr))
		m.diskHeader[4].Width = max(m.diskHeader[4].Width, lipgloss.Width(pctStr))

		diskRows = append(diskRows, table.Row{d.Path, usedStr, totalStr, freeStr, pctStr, ""})
	}

	diskStyles := s
	diskStyles.Cell = diskStyles.Cell.Align(lipgloss.Right, lipgloss.Right)
	m.setTableActive(debugTabDisks, true)
	m.diskTbl.SetStyles(diskStyles)
	m.diskTbl.SetColumns(m.diskHeader)
	m.diskTbl.SetRows(diskRows)
	m.diskTbl.SetHeight(m.tableHeight(len(diskRows)))
	m.diskTbl.SetWidth(max(m.Width()-4, 20))
	return m.diskTbl.View()
}

// logLevelRank maps a log entry's Type to an ordered severity rank. Entries from
// the runtime logger carry a level name (DEBUG/INFO/WARN/ERROR); intercepted
// tea-message entries carry a Go type name and rank -1 (not a level).
func logLevelRank(t string) int {
	switch strings.ToUpper(t) {
	case "DEBUG":
		return 0
	case "INFO":
		return 1
	case "WARN", "WARNING":
		return 2
	case "ERROR":
		return 3
	}
	return -1
}

// logLevelRankWarn is the threshold used by the WARN+ filter (I-6).
const logLevelRankWarn = 2

func (m *InspectorModel) renderLogContent(c *theme.AppStyle) string {
	var b strings.Builder
	for _, log := range m.Logs {
		if m.logWarnPlus && logLevelRank(log.Type) < logLevelRankWarn {
			continue
		}
		timestamp := log.Timestamp.Format("15:04:05")
		countStr := ""
		if log.Count > 1 {
			countStr = c.Styles.Warning.Render(fmt.Sprintf(" [%d events]", log.Count))
		}
		typeStr := c.Styles.Title.Render(log.Type)
		line := fmt.Sprintf(
			"%s %s%s\n  %s",
			c.Styles.Subtitle.Render(timestamp),
			typeStr,
			countStr,
			c.Styles.TextOnBg.Render(log.Content),
		)
		b.WriteString(line)
		b.WriteString("\n\n")
	}
	if b.Len() == 0 {
		if m.logWarnPlus {
			return "No WARN+ messages. Press 'f' to show all."
		}
		return "No messages intercepted yet..."
	}
	return b.String()
}

func (m *InspectorModel) buildTabsLine(c *theme.AppStyle) string {
	// Only visible tabs are rendered (gate-hidden ones don't consume a number),
	// and the recorded mouse ranges match the rendered layout, so click
	// hit-testing stays correct as tabs show and hide at runtime.
	vis := m.visibleTabs()
	tabParts := make([]string, 0, len(vis))
	tabRanges := make([]tabMouseRange, 0, len(vis))
	tabX := 0
	activeTabStyle := lipgloss.NewStyle().
		Background(c.Accent).
		Foreground(c.Bg).
		Bold(true).
		Padding(0, 1)
	inactiveTabStyle := c.Styles.Item.Padding(0, 1)
	for i, tab := range vis {
		label := fmt.Sprintf("%d:%s", i+1, m.tabTitle(tab))
		var rendered string
		if tab == m.activeTab {
			rendered = activeTabStyle.Render(label)
		} else {
			rendered = inactiveTabStyle.Render(label)
		}
		tabParts = append(tabParts, rendered)
		w := lipgloss.Width(rendered)
		tabRanges = append(tabRanges, tabMouseRange{
			Tab:    tab,
			StartX: tabX + debugBorderPaddingX,
			EndX:   tabX + w - 1 + debugBorderPaddingX,
		})
		tabX += w
	}
	m.tabRanges = tabRanges
	return lipgloss.JoinHorizontal(lipgloss.Left, tabParts...)
}

func (m *InspectorModel) sectionForActiveTab(
	c *theme.AppStyle,
	availW int,
	s table.Styles,
	runtimeSection string,
	inputRows []table.Row,
	logContent string,
) (title, content string) {
	switch m.activeTab {
	case debugTabRuntime:
		return "Runtime Profiling", runtimeSection
	case debugTabInput:
		return "Input Debugging", m.renderInputSection(c, s, inputRows, availW)
	case debugTabDisks:
		return "Disks", m.renderDisksSection(c, s)
	case debugTabTerminal:
		return "Terminal & Theme", m.buildTermSection(c, availW)
	case debugTabAccessibility:
		if m.acPanel != nil {
			return debugTabTitleAccessibility, m.acPanel.View().Content
		}
		return debugTabTitleAccessibility, "(accessibility panel not available)"
	case debugTabLog:
		title := "Message Log"
		if m.logWarnPlus {
			title += " [WARN+ only]"
		}
		return title, logContent
	case debugTabSettings:
		return "Debug Settings", m.renderSettingsSection(c)
	default:
		// Custom provider tabs (E-5) live past the built-in range.
		if p := m.providerForTab(m.activeTab); p != nil {
			return p.TabName(), m.renderProviderSection(p, c)
		}
		return "Runtime Profiling", runtimeSection
	}
}

package debug

import (
	"fmt"
	"image/color"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/theme"
	tint "github.com/lrstanley/bubbletint/v2"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// TermDiagMsg carries terminal environment diagnostics to the inspector.
// The router forwards this after receiving tea.BackgroundColorMsg.
type TermDiagMsg struct {
	// DetectedBg is the terminal background color reported via OSC 11.
	DetectedBg color.Color
	// BgIsDark is the result of tea.BackgroundColorMsg.IsDark().
	BgIsDark bool
	// ColorProfile is the detected terminal color capability.
	Profile colorprofile.Profile
}

type MsgLog struct {
	Timestamp time.Time
	Type      string
	Content   string
	Count     int
}

type Model struct {
	Logs           []MsgLog
	width          int
	height         int
	logViewport    viewport.Model
	scrollToBottom bool
	colors         *theme.AppStyle
	// Accessibility panel — toggled with 'a'
	acPanel *AccessibilityPanel
	// mouse highlight support
	ShowHighlight   bool
	LastMouseX      int
	LastMouseY      int
	LastMouseChild  string
	LastMouseButton string
	LastMouseMod    int
	LastMouseOffX   int
	LastMouseOffY   int
	LastKeyPress    string
	LastKeyMod      []string
	LastKeyRelease  string
	LastKeyRelMod   []string
	// runtime stats refreshed by background tick (≤1 s cadence)
	stats     runtimeStatsSnapshot
	prevStats runtimeStatsSnapshot // previous snapshot for computing per-second deltas
	startTime time.Time
	printer   *message.Printer

	// Terminal diagnostics — populated by TermDiagMsg forwarded from the router.
	termDiag       *TermDiagMsg
	termDiagSet    bool
	initialProfile colorprofile.Profile // detected at New() time

	// Cached render artifacts to reduce per-frame allocations.
	view           tea.View
	dirty          bool
	runtimeTbl     table.Model
	diskTbl        table.Model
	runtimeColumns []table.Column // pre-allocated column slice for the runtime stats table (to avoid reallocating on every View)
	runtimeColMaxW []int          // high-watermark: max rendered width ever seen per column
	diskHeader     []table.Column // pre-allocated column slice for the disk stats table
}

// SetColors stores a shared AppColors pointer so the router can update the
// theme in one place and this model sees the change immediately.
func (m *Model) SetColors(c *theme.AppStyle) {
	m.colors = c
	m.dirty = true
	if m.acPanel != nil {
		m.acPanel.SetColors(c)
	}
}

// resolveColors returns the current palette from the shared pointer, falling
// back to theme.Active() when no pointer has been set (e.g. in tests).
func (m *Model) resolveColors() *theme.AppStyle {
	if m.colors != nil {
		return m.colors
	}
	return theme.Active()
}

func New() *Model {
	m := &Model{
		Logs:           make([]MsgLog, 0),
		startTime:      time.Now(),
		acPanel:        NewAccessibilityPanel(),
		printer:        message.NewPrinter(language.English),
		dirty:          true,
		initialProfile: colorprofile.Detect(os.Stdout, os.Environ()),
	}
	// populate stats immediately so View() has data before the first tick fires
	m.stats = collectSnapshot(m.startTime)
	m.prevStats = m.stats
	m.runtimeColumns = make([]table.Column, 8)
	m.runtimeColMaxW = make([]int, 8)
	for i := range m.runtimeColumns {
		if i%2 == 0 {
			m.runtimeColumns[i] = table.Column{Title: "Metric", Width: -1}
		} else {
			m.runtimeColumns[i] = table.Column{Title: "Value", Width: -1}
		}
	}

	m.diskHeader = []table.Column{
		{Title: "Drive", Width: 5},
		{Title: "Used", Width: 5},
		{Title: "Total", Width: 5},
		{Title: "Free", Width: 5},
		{Title: "Use%", Width: 5},
		{Title: "Error", Width: 0},
	}

	m.runtimeTbl = table.New(
		table.WithColumns(m.runtimeColumns),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithHeight(2),
		table.WithWidth(80),
	)
	m.diskTbl = table.New(
		table.WithColumns(m.diskHeader),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithHeight(2),
		table.WithWidth(80),
	)

	m.view = tea.NewView("")
	return m
}

func (m *Model) Init() tea.Cmd {
	return m.scheduleStatsTick()
}

// statsTickMsg carries a freshly collected snapshot back to Update.
type statsTickMsg struct{ snapshot runtimeStatsSnapshot }

// scheduleStatsTick returns a Cmd that fires once after 1 s and delivers a
// new snapshot. Update() reschedules it so stats refresh continuously.
func (m *Model) scheduleStatsTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return statsTickMsg{snapshot: collectSnapshot(m.startTime)}
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.dirty = true
		if m.acPanel != nil {
			m.acPanel.SetSize(msg.Width, msg.Height)
		}
		m.logViewport.SetWidth(msg.Width)
		m.logViewport.SetHeight(msg.Height)
		return m, nil // size changes are silent — do not log
	case tea.KeyMsg:
		// When the accessibility panel is open it handles its own navigation;
		// only 'a' is intercepted here to toggle it.
		if m.acPanel != nil && m.acPanel.IsVisible() {
			if press, ok := msg.(tea.KeyPressMsg); ok && press.String() == "a" {
				m.acPanel.Toggle()
				return m, nil
			}
			_, cmd := m.acPanel.Update(msg)
			return m, cmd
		}
		switch km := msg.(type) {
		case tea.KeyPressMsg:
			switch km.String() {
			case "a", "A":
				if m.acPanel != nil {
					m.acPanel.Toggle()
				}
				m.dirty = true
				return m, nil
			case "h", "H":
				m.ShowHighlight = !m.ShowHighlight
				m.dirty = true
				return m, nil
			case "i":
				return m, func() tea.Msg {
					return notifications.AddMsg{Content: "Test: Info notification from Inspector", Severity: notifications.SeverityInfo, TTL: notifications.SeverityInfo.DefaultTTL()}
				}
			case "w":
				return m, func() tea.Msg {
					return notifications.AddMsg{Content: "Test: Warning notification from Inspector", Severity: notifications.SeverityWarning, TTL: notifications.SeverityWarning.DefaultTTL()}
				}
			case "e":
				return m, func() tea.Msg {
					return notifications.AddMsg{Content: "Test: Error notification from Inspector", Severity: notifications.SeverityError, TTL: notifications.SeverityError.DefaultTTL()}
				}
			}
		}
	case TermDiagMsg:
		m.termDiag = &msg
		m.termDiagSet = true
		m.dirty = true

	case statsTickMsg:
		m.prevStats = m.stats
		m.stats = msg.snapshot
		m.dirty = true
		return m, m.scheduleStatsTick()
	case MouseHighlightMsg:
		// update last mouse highlight info (sent from router)
		m.LastMouseX = msg.GlobalX
		m.LastMouseY = msg.GlobalY
		m.LastMouseChild = msg.Child
		m.LastMouseOffX = msg.OffX
		m.LastMouseOffY = msg.OffY
		// Mouse motion can be extremely high frequency. Only invalidate the view
		// when highlight UI is enabled; otherwise keep cached rendering.
		if m.ShowHighlight {
			m.dirty = true
		}
	case tea.MouseMsg: //, tea.MousePressMsg, tea.MouseWheelMsg:
		m.LastMouseX = msg.Mouse().X
		m.LastMouseY = msg.Mouse().Y
		m.LastMouseButton = msg.Mouse().String()
		m.LastMouseMod = int(msg.Mouse().Mod)
		m.LastMouseOffX = 0
		m.LastMouseOffY = 0
		// Avoid logging cell-motion spam; only log user-significant mouse actions.
		switch msg.(type) {
		case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseWheelMsg:
			if wheel, ok := msg.(tea.MouseWheelMsg); ok {
				if wheel.Mouse().Button == tea.MouseWheelUp {
					m.logViewport.ScrollUp(3)
				} else {
					m.logViewport.ScrollDown(3)
				}
				m.dirty = true
				return m, nil
			}
			m.dirty = true
		case tea.MouseMotionMsg:
			if m.ShowHighlight {
				m.dirty = true
			}
		}
	}
	// Record every message the inspector sees (deduped/stacked) so the log pane
	// reflects live traffic. Silent, high-frequency messages return early above.
	m.LogMessageForDebugging(msg)
	return m, nil
}

// MouseHighlightMsg is emitted by the router when a mouse event is routed
// into a child view. It contains the global coordinates and which child was
// under the pointer so the inspector can render a visual indicator.
type MouseHighlightMsg struct {
	GlobalX int
	GlobalY int
	Child   string
	OffX    int
	OffY    int
}

func (m *Model) LogMessageForDebugging(msg tea.Msg) {
	msgType := fmt.Sprintf("%T", msg)
	msgContent := fmt.Sprintf("%+v", msg)
	switch mt := msg.(type) {
	case statsTickMsg:
		return // skip logging internal stats ticks to reduce noise
	case tea.WindowSizeMsg:
		msgContent = fmt.Sprintf("Width: %d, Height: %d", mt.Width, mt.Height)
	case tea.MouseReleaseMsg:
		curMouse := mt.Mouse()
		msgContent = fmt.Sprintf("Global: %d,%d  Button: %s  Mod: %d",
			curMouse.X, curMouse.Y, curMouse.Button, curMouse.Mod)
	case tea.MouseMsg:
		curMouse := mt.Mouse()
		msgContent = fmt.Sprintf("Global: %d,%d  Button: %s  Mod: %d",
			curMouse.X, curMouse.Y, curMouse.Button, curMouse.Mod)
	case tea.KeyMsg:
		switch km := msg.(type) {
		case tea.KeyPressMsg:
			m.LastKeyPress = km.Key().String()
			if sp := strings.Split(km.String(), "+"); len(sp) > 1 {
				m.LastKeyMod = strings.Split(km.String(), "+")[:len(sp)-1]
				m.LastKeyPress = strings.Split(km.String(), "+")[len(sp)-1]
			} else {
				m.LastKeyMod = []string{}
			}
			return // skip logging every key press to reduce noise; tracked separately in the view
		case tea.KeyReleaseMsg:
			m.LastKeyRelease = km.Key().String()
			if sp := strings.Split(km.String(), "+"); len(sp) > 1 {
				m.LastKeyRelMod = strings.Split(km.String(), "+")[:len(sp)-1]
				m.LastKeyRelease = strings.Split(km.String(), "+")[len(sp)-1]
			} else {
				m.LastKeyRelMod = []string{}
			}
			msgContent = fmt.Sprintf("Key Release: %s", km.Keystroke())
		default:
			msgContent = fmt.Sprintf("%T Key: %s", km, mt.String())
		}

	}

	// Check if the last log is the same to stack them
	if len(m.Logs) > 0 {
		last := &m.Logs[len(m.Logs)-1]
		if last.Type == msgType && last.Content == msgContent {
			last.Count++
			last.Timestamp = time.Now()
			m.dirty = true
			return
		}
	}

	m.Logs = append(m.Logs, MsgLog{
		Timestamp: time.Now(),
		Type:      msgType,
		Content:   msgContent,
		Count:     1,
	})
	m.dirty = true

	// Keep only last 50 logs
	if len(m.Logs) > 50 {
		m.Logs = m.Logs[1:]
	}
	m.scrollToBottom = true
}

// AddLog adds an external log entry (from the runtime logger) to the
// inspector. This is used for file-backed runtime logging so the same data
// can be shown in the UI with a brief timestamp while the log file contains
// the full timestamp and details.
func (m *Model) AddLog(level string, ts time.Time, content string) {
	// Check if the last log is the same to stack them
	if len(m.Logs) > 0 {
		last := &m.Logs[len(m.Logs)-1]
		if last.Type == level && last.Content == content {
			last.Count++
			last.Timestamp = ts
			m.dirty = true
			return
		}
	}

	m.Logs = append(m.Logs, MsgLog{
		Timestamp: ts,
		Type:      level,
		Content:   content,
		Count:     1,
	})
	m.dirty = true

	// Keep only last 50 logs for the inspector
	if len(m.Logs) > 50 {
		m.Logs = m.Logs[1:]
	}
	m.scrollToBottom = true
}

// diskStat holds space information for a single mounted drive or volume.
type diskStat struct {
	Path  string
	Total uint64
	Free  uint64
	Used  uint64
	Error string
}

// launchInfo captures static process metadata collected once at startup.
type launchInfo struct {
	Executable string
	Args       []string
	WorkDir    string
	Username   string
	Hostname   string
	BinarySize int64
	Error      string
}

func collectLaunchInfo() launchInfo {
	var info launchInfo
	info.Args = os.Args

	exe, err := os.Executable()
	if err != nil {
		info.Error = fmt.Sprintf("executable: %v", err)
	} else {
		info.Executable = filepath.Clean(exe)
		if fi, statErr := os.Stat(exe); statErr == nil {
			info.BinarySize = fi.Size()
		}
	}

	if wd, err := os.Getwd(); err == nil {
		info.WorkDir = wd
	}

	if u, err := user.Current(); err == nil {
		info.Username = u.Username
	}

	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	return info
}

type runtimeStatsSnapshot struct {
	CapturedAt time.Time

	// Runtime/process
	Uptime        time.Duration
	GoVersion     string
	NumCPU        int
	GOMAXPROCS    int
	Goroutines    int
	NumCgoCalls   int64
	AppCPUPercent float64

	// Memory/GC
	HeapAllocBytes  uint64
	HeapSysBytes    uint64
	HeapInUseBytes  uint64
	StackInUseBytes uint64
	HeapObjects     uint64
	Mallocs         uint64 // total allocation count (monotonic; delta gives allocs/sec)
	NumGC           uint32
	PauseTotal      time.Duration
	LastPause       time.Duration
	GcCPUFraction   float64

	// Disks — one entry per mounted volume/drive, populated by listDriveStats.
	Disks []diskStat

	// Launch details (collected once at startup)
	Launch launchInfo
}

// launchOnce is collected a single time when the package is first used.
var launchOnce = collectLaunchInfo()

const diskStatsRefreshInterval = 10 * time.Second

var (
	diskStatsMu    sync.Mutex
	diskStatsAt    time.Time
	diskStatsCache []diskStat
)

func cachedDriveStats(now time.Time) []diskStat {
	diskStatsMu.Lock()
	defer diskStatsMu.Unlock()

	if len(diskStatsCache) == 0 || now.Sub(diskStatsAt) >= diskStatsRefreshInterval {
		diskStatsCache = listDriveStats()
		diskStatsAt = now
	}
	return diskStatsCache
}

// collectSnapshot reads all runtime metrics and returns a fresh snapshot.
// It is called on the background tick goroutine, not in Update().
func collectSnapshot(startTime time.Time) runtimeStatsSnapshot {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	now := time.Now()

	snap := runtimeStatsSnapshot{
		CapturedAt:      now,
		Uptime:          time.Since(startTime),
		GoVersion:       runtime.Version(),
		NumCPU:          runtime.NumCPU(),
		GOMAXPROCS:      runtime.GOMAXPROCS(0),
		Goroutines:      runtime.NumGoroutine(),
		NumCgoCalls:     runtime.NumCgoCall(),
		HeapAllocBytes:  ms.HeapAlloc,
		HeapSysBytes:    ms.HeapSys,
		HeapInUseBytes:  ms.HeapInuse,
		StackInUseBytes: ms.StackInuse,
		HeapObjects:     ms.HeapObjects,
		Mallocs:         ms.Mallocs,
		NumGC:           ms.NumGC,
		PauseTotal:      time.Duration(ms.PauseTotalNs),
		GcCPUFraction:   ms.GCCPUFraction,
		Disks:           cachedDriveStats(now),
		Launch:          launchOnce,
	}
	if ms.NumGC > 0 {
		snap.LastPause = time.Duration(ms.PauseNs[(ms.NumGC+255)%256])
	}
	return snap
}

// formatBytes formats a byte count as a human-readable IEC string (KiB, MiB, …).
func formatBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(1024), 0
	for n := b / 1024; n >= 1024; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (m *Model) View() tea.View {
	if !m.dirty {
		return m.view
	}

	c := m.resolveColors()
	titleStyle := c.Styles.Title.Padding(0, 1)
	subtitleStyle := c.Styles.Subtitle.Padding(0, 3)
	valStyle := c.Styles.TextOnBg
	title := titleStyle.Render("MESSAGE INSPECTOR (DEBUG)")

	// colorStat colours a rendered value based on warn/crit thresholds.
	colorStat := func(val, warn, crit float64, rendered string) string {
		switch {
		case val >= crit:
			return c.Styles.Error.Bold(true).Render(rendered)
		case val >= warn:
			return c.Styles.Warning.Render(rendered)
		default:
			return c.Styles.Success.Render(rendered)
		}
	}

	st := m.stats // local copy — View is called on the UI goroutine
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

	rows := []table.Row{
		// Line 1: Uptime / Go version / OS / arch
		{"Uptime", st.Uptime.Round(time.Second).String(),
			"Go", valStyle.Render(st.GoVersion),
			"OS/Arch", valStyle.Render(p.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)),
			"PID", valStyle.Render(p.Sprintf("%d", os.Getpid())),
		},
		// Line 2: Goroutines / GOMAXPROCS / CGO
		{"Goroutines", colorStat(float64(st.Goroutines), 100, 500, fmt.Sprintf("%d", st.Goroutines)),
			"GOMAXPROCS", valStyle.Render(p.Sprintf("%d/%d", st.GOMAXPROCS, st.NumCPU)),
			"CGO Calls", valStyle.Render(p.Sprintf("%d", st.NumCgoCalls)),
			"Term Size", valStyle.Render(p.Sprintf("%dx%d", m.width, m.height)),
		},
		// Line 3: Heap alloc / in-use / sys / stack
		{"Heap Alloc", colorStat(float64(st.HeapAllocBytes)/1024/1024, 100, 500, formatBytes(st.HeapAllocBytes)),
			"Heap InUse", valStyle.Render(formatBytes(st.HeapInUseBytes)),
			"Heap Sys", valStyle.Render(formatBytes(st.HeapSysBytes)),
			"Stack InUse", valStyle.Render(formatBytes(st.StackInUseBytes)),
		},
		// Line 4: GC cycles / last pause / total paused
		{"GC Cycles", valStyle.Render(p.Sprintf("%d", st.NumGC)),
			"Last Pause", colorStat(float64(st.LastPause.Milliseconds()), 1, 10, st.LastPause.Round(time.Microsecond).String()),
			"Total Paused", valStyle.Render(st.PauseTotal.Round(time.Millisecond).String()),
			"Bin Size", valStyle.Render(formatBytes(uint64(st.Launch.BinarySize))),
		},
		{"GC CPU %", colorStat(st.GcCPUFraction*100, 5, 25, p.Sprintf("%.2f%%", st.GcCPUFraction*100)),
			"GC/sec", colorStat(gcPerSec, 10, 50, p.Sprintf("%.1f", gcPerSec)),
			"Allocs/sec", colorStat(allocsPerSec, 10000, 100000, p.Sprintf("%.0f", allocsPerSec)),
			"Heap Objects", valStyle.Render(p.Sprintf("%d", st.HeapObjects)),
		},
		{"Mouse Down", valStyle.Render(fmt.Sprintf("%d,%d", m.LastMouseX, m.LastMouseY)),
			"Mod", valStyle.Render(fmt.Sprintf("%d", m.LastMouseMod)),
			"Button", valStyle.Render(m.LastMouseButton),
			"OffX/Y", valStyle.Render(fmt.Sprintf("%d,%d", m.LastMouseOffX, m.LastMouseOffY)),
		},
		{"Key Mod", valStyle.Render(strings.Join(m.LastKeyMod, "+")),
			"Press", valStyle.Render(m.LastKeyPress),
			"Rel Mod", valStyle.Render(strings.Join(m.LastKeyRelMod, "+")),
			"Rel", valStyle.Render(m.LastKeyRelease),
		},
	}
	for i := range m.runtimeColumns {
		m.runtimeColumns[i].Width = max(lipgloss.Width(m.runtimeColumns[i].Title), m.runtimeColMaxW[i])
	}
	for _, val := range rows {
		for j := range val {
			w := lipgloss.Width(val[j])
			if w > m.runtimeColMaxW[j] {
				m.runtimeColMaxW[j] = w
			}
			m.runtimeColumns[j].Width = max(m.runtimeColumns[j].Width, w)
		}
	}
	s := table.DefaultStyles()
	s.Cell = s.Cell.
		Background(c.Styles.Item.GetBackground()).
		Foreground(c.Styles.Item.GetForeground())
	// Keep unfocused table rows on the page background instead of bubbling the
	// table package's default selected-row background, which can differ
	// dramatically over ANSI256/SSH terminals.
	s.Selected = s.Selected.
		Background(c.Styles.TextOnBg.GetBackground()).
		Foreground(c.Styles.TextOnBg.GetForeground())
	s.Header = s.Header.
		Background(c.Styles.SelectedItem.GetBackground()).
		Foreground(c.Styles.SelectedItem.GetForeground()).
		Bold(true)

	// Natural table width: left border + (width + 2 padding + 1 right border) per column.
	naturalW := 1
	for _, col := range m.runtimeColumns {
		naturalW += col.Width + 2
	}
	availW := max(m.width-4, 20)

	var runtimeSection string
	if naturalW <= availW {
		m.runtimeTbl.SetStyles(s)
		m.runtimeTbl.SetColumns(m.runtimeColumns)
		m.runtimeTbl.SetRows(rows)
		m.runtimeTbl.SetHeight(len(rows) + 2)
		m.runtimeTbl.SetWidth(availW)
		runtimeSection = m.runtimeTbl.View()
	} else {
		runtimeSection = renderRuntimeFlat(rows, c, availW)
	}

	// Disk table — fixed-width columns
	// hdrStyle := c.Styles.Subtitle.Bold(true)
	// row := c.Styles.TextOnBg.Render(diskHeader)
	var disksRow []table.Row
	m.diskHeader[0].Width = lipgloss.Width(m.diskHeader[0].Title)
	m.diskHeader[1].Width = lipgloss.Width(m.diskHeader[1].Title)
	m.diskHeader[2].Width = lipgloss.Width(m.diskHeader[2].Title)
	m.diskHeader[3].Width = lipgloss.Width(m.diskHeader[3].Title)
	m.diskHeader[4].Width = lipgloss.Width(m.diskHeader[4].Title)
	m.diskHeader[5].Width = lipgloss.Width(m.diskHeader[5].Title)
	for _, d := range st.Disks {
		if d.Error != "" {
			m.diskHeader[5].Width = max(m.diskHeader[5].Width, lipgloss.Width(d.Error))
			disksRow = append(disksRow, table.Row{d.Path, "unavailable", "", "", "", d.Error})
			continue
		}
		pct := 0.0
		if d.Total > 0 {
			pct = float64(d.Used) / float64(d.Total) * 100
		}
		var freeStr string
		switch {
		case d.Free < 100*1024*1024:
			freeStr = c.Styles.Error.Bold(true).Render(formatBytes(d.Free))
		case d.Free < 1*1024*1024*1024:
			freeStr = c.Styles.Warning.Render(formatBytes(d.Free))
		default:
			freeStr = c.Styles.Success.Render(formatBytes(d.Free))
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
		m.diskHeader[0].Width = max(m.diskHeader[0].Width, lipgloss.Width(d.Path))
		m.diskHeader[1].Width = max(m.diskHeader[1].Width, lipgloss.Width(formatBytes(d.Used)))
		m.diskHeader[2].Width = max(m.diskHeader[2].Width, lipgloss.Width(formatBytes(d.Total)))
		m.diskHeader[3].Width = max(m.diskHeader[3].Width, lipgloss.Width(freeStr))
		m.diskHeader[4].Width = max(m.diskHeader[4].Width, lipgloss.Width(pctStr))

		disksRow = append(disksRow, table.Row{d.Path, formatBytes(d.Used), formatBytes(d.Total), freeStr, pctStr, ""})
	}
	s.Cell.Align(lipgloss.Right, lipgloss.Right)
	m.diskTbl.SetStyles(s)
	m.diskTbl.SetColumns(m.diskHeader)
	m.diskTbl.SetRows(disksRow)
	m.diskTbl.SetHeight(len(disksRow) + 2)
	m.diskTbl.SetWidth(max(m.width-4, 20))

	var contentBuilder strings.Builder
	for _, log := range m.Logs {
		timestamp := log.Timestamp.Format("15:04:05")

		countStr := ""
		if log.Count > 1 {
			countStr = c.Styles.Warning.Render(fmt.Sprintf(" [%d events]", log.Count))
		}

		typeStr := c.Styles.Title.Render(log.Type)

		line := fmt.Sprintf("%s %s%s\n  %s",
			c.Styles.Subtitle.Render(timestamp),
			typeStr,
			countStr,
			c.Styles.TextOnBg.Render(log.Content),
		)
		contentBuilder.WriteString(line)
		contentBuilder.WriteString("\n\n")
	}
	content := contentBuilder.String()

	if content == "" {
		content = "No messages intercepted yet..."
	}

	// Notification test buttons
	btnStyle := c.Styles.SelectedItem.Padding(0, 1)
	infoBtn := btnStyle.Foreground(c.Styles.Success.GetForeground()).Render("[i] Send Info")
	warnBtn := btnStyle.Foreground(c.Styles.Warning.GetForeground()).Render("[w] Send Warning")
	errBtn := btnStyle.Foreground(c.Styles.Error.GetForeground()).Render("[e] Send Error")
	hintStyle := c.Styles.Subtitle
	acHint := hintStyle.Render("  [a] Accessibility browser")
	buttons := hintStyle.Render("Notification test: ") + infoBtn + "  " + warnBtn + "  " + errBtn + acHint

	// Mouse highlight block (optional)
	highlight := ""
	if m.ShowHighlight {
		highlight = c.Styles.SelectedItem.
			Background(c.Styles.SelectedItem.GetBackground()).
			Foreground(c.Styles.SelectedItem.GetForeground()).
			Padding(0, 1).
			Render(fmt.Sprintf("Mouse: %d,%d  over: %s  (off: %d,%d)  [H to toggle]", m.LastMouseX, m.LastMouseY, m.LastMouseChild, m.LastMouseOffX, m.LastMouseOffY))
	}

	// When the accessibility panel is open, render it in place of the log area.
	if m.acPanel != nil && m.acPanel.IsVisible() {
		panelH := max(m.height-4, 6)
		m.acPanel.SetSize(m.width, panelH)
		acStr := c.Styles.TextOnBg.
			Width(m.width).
			Height(panelH).
			MaxHeight(panelH).
			Render(m.acPanel.View().Content)
		m.view.SetContent(lipgloss.JoinVertical(lipgloss.Left, title, buttons, highlight, "", acStr))
		m.view.BackgroundColor = c.Styles.TextOnBg.GetBackground()
		m.view.ForegroundColor = c.Styles.TextOnBg.GetForeground()
		m.dirty = false
		return m.view
	}

	termSection := m.buildTermSection(c, availW)

	// Keep runtime diagnostics fixed at the top and reserve a dedicated
	// viewport for the message log so incoming log volume cannot push the
	// diagnostics area off-screen.
	staticContent := lipgloss.JoinVertical(lipgloss.Left,
		subtitleStyle.Render("Runtime Profiling"),
		runtimeSection,
		subtitleStyle.Render("Disks"),
		m.diskTbl.View(),
		subtitleStyle.Render("Terminal & Theme"),
		termSection,
		buttons,
		highlight,
	)
	logTitle := subtitleStyle.Render("Message Log")
	logH := max(m.height-lipgloss.Height(title)-lipgloss.Height(staticContent)-lipgloss.Height(logTitle), 1)
	m.logViewport.SetWidth(m.width)
	m.logViewport.SetHeight(logH)
	m.logViewport.SetContent(content)
	if m.scrollToBottom {
		m.logViewport.GotoBottom()
		m.scrollToBottom = false
	}

	m.view.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	m.view.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	m.view.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		if wheel, ok := mm.(tea.MouseWheelMsg); ok {
			if wheel.Mouse().Button == tea.MouseWheelUp {
				m.logViewport.ScrollUp(3)
			} else {
				m.logViewport.ScrollDown(3)
			}
			m.dirty = true
			return nil
		}
		return nil
	}
	m.view.SetContent(lipgloss.JoinVertical(lipgloss.Left, title, staticContent, logTitle, m.logViewport.View()))
	m.dirty = false
	return m.view
}

// buildTermSection renders the terminal environment and active theme diagnostic rows.
func (m *Model) buildTermSection(c *theme.AppStyle, width int) string {
	kv := func(k, v string) string {
		return c.Styles.Item.Render(k+": ") + c.Styles.TextOnBg.Render(v)
	}
	warn := func(k, v string) string {
		return c.Styles.Dim.Render(k+": ") + c.Styles.Warning.Render(v)
	}

	// Terminal env vars
	termEnv := getEnvOr("TERM", "(not set)")
	colorterm := getEnvOr("COLORTERM", "(not set)")
	termProg := getEnvOr("TERM_PROGRAM", "(not set)")
	sshClient := getEnvOr("SSH_CLIENT", "")
	sshTTY := getEnvOr("SSH_TTY", "")

	isSSH := sshClient != "" || sshTTY != ""
	sshStr := "no"
	if isSSH {
		sshStr = "YES — SSH_CLIENT=" + sshClient
	}

	// Color profile
	prof := m.initialProfile
	profStr := prof.String()
	if m.termDiagSet && m.termDiag != nil {
		prof = m.termDiag.Profile
		profStr = prof.String()
	}
	// Surface an active profile override. The env var name mirrors
	// router.ColorProfileEnvVar; read directly here to avoid an import cycle
	// (router imports this debug package).
	profileOverride := strings.TrimSpace(os.Getenv("TUI_BASE_COLOR_PROFILE"))
	if profileOverride != "" {
		profStr += " (forced: TUI_BASE_COLOR_PROFILE=" + profileOverride + ")"
	}

	// Detected background color from BackgroundColorMsg
	bgDetStr := "(waiting for OSC 11 response)"
	isDarkStr := "-"
	bgSwatch := ""
	if m.termDiagSet && m.termDiag != nil {
		bgDetStr = colorHex(m.termDiag.DetectedBg)
		isDarkStr = fmt.Sprintf("%v", m.termDiag.BgIsDark)
		bgSwatch = lipgloss.NewStyle().
			Background(m.termDiag.DetectedBg).
			Foreground(m.termDiag.DetectedBg).
			Render("■■")
		bgDetStr = bgSwatch + " " + bgDetStr
	}

	// Active tint key colors
	activeTint := func() *tint.Tint {
		defer func() { recover() }() //nolint:errcheck
		return tint.Current()
	}()
	tintID := "(none)"
	bgHex, fgHex, accentHex, selBgHex := "?", "?", "?", "?"
	if activeTint != nil {
		tintID = activeTint.ID
		if activeTint.Bg != nil {
			col := lipgloss.Color(activeTint.Bg.Hex())
			bgHex = activeTint.Bg.Hex()
			bgSw := lipgloss.NewStyle().Background(col).Foreground(col).Render("■")
			bgHex = bgSw + " " + bgHex
		}
		if activeTint.Fg != nil {
			fgHex = activeTint.Fg.Hex()
		}
		if activeTint.Purple != nil {
			accentHex = activeTint.Purple.Hex()
		}
		if activeTint.SelectionBg != nil {
			col := lipgloss.Color(activeTint.SelectionBg.Hex())
			selBgHex = activeTint.SelectionBg.Hex()
			sw := lipgloss.NewStyle().Background(col).Foreground(col).Render("■")
			selBgHex = sw + " " + selBgHex
		}
	}

	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			kv("TERM", termEnv), "   ",
			kv("COLORTERM", colorterm), "   ",
			kv("TERM_PROGRAM", termProg),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			kv("Color Profile", profStr), "   ",
			kv("IsDark", isDarkStr),
		),
	}
	if isSSH {
		rows = append(rows, warn("SSH", sshStr))
	} else {
		rows = append(rows, kv("SSH", sshStr))
	}
	rows = append(rows,
		kv("OSC11 Bg", bgDetStr),
		lipgloss.JoinHorizontal(lipgloss.Top,
			kv("Tint", tintID), "   ",
			kv("Tint Bg", bgHex), "   ",
			kv("Tint Fg", fgHex),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			kv("Tint Accent", accentHex), "   ",
			kv("Tint SelBg", selBgHex),
		),
	)

	// Remedy hint: colors look washed-out / wrong over SSH when the profile
	// downsamples 24-bit theme colors. Surface the fix when we detect the
	// classic signature (ANSI256 + SSH + no override + COLORTERM unset).
	if prof == colorprofile.ANSI256 && isSSH && profileOverride == "" && colorterm == "(not set)" {
		rows = append(rows,
			warn("Color hint", "24-bit colors quantized to ANSI256 (COLORTERM not forwarded over SSH)"),
			warn("Fix", "set COLORTERM=truecolor on the remote, or run with TUI_BASE_COLOR_PROFILE=truecolor"),
		)
	}

	// Config + launch info
	st := m.stats
	rows = append(rows,
		kv("Executable", st.Launch.Executable),
		kv("Args", strings.Join(st.Launch.Args, " ")),
		kv("WorkDir", st.Launch.WorkDir),
		kv("User@Host", st.Launch.Username+"@"+st.Launch.Hostname),
	)

	joined := strings.Join(rows, "\n")
	return c.Styles.TextOnBg.Width(width).Render(joined)
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// colorHex returns a #rrggbb hex string for any color.Color.
func colorHex(col color.Color) string {
	if col == nil {
		return "(nil)"
	}
	type hexer interface{ Hex() string }
	if h, ok := col.(hexer); ok {
		return "#" + h.Hex()
	}
	r, g, b, _ := col.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// renderRuntimeFlat renders all metric+value pairs from the runtime profiling
// table as a compact 2-column key→value list. Used when the terminal is too
// narrow to fit all columns side-by-side without clipping.
func renderRuntimeFlat(rows []table.Row, c *theme.AppStyle, width int) string {
	// Flatten every (metric, value) pair from all rows.
	type pair struct{ k, v string }
	var pairs []pair
	for _, row := range rows {
		for i := 0; i+1 < len(row); i += 2 {
			if row[i] == "" {
				continue
			}
			pairs = append(pairs, pair{k: row[i], v: row[i+1]})
		}
	}

	// Key column width: widest metric name, capped at 1/3 of available width.
	maxK := 0
	for _, p := range pairs {
		if w := lipgloss.Width(p.k); w > maxK {
			maxK = w
		}
	}
	keyW := min(maxK, width/3)
	valW := max(width-keyW-2, 4) // 2 for the " " separator + left margin

	keyStyle := c.Styles.Item.Width(keyW)
	sep := c.Styles.RealHeader.Render(strings.Repeat("─", min(width, 60)))

	// Group every 4 pairs with a thin separator line (mirrors original row grouping).
	var sb strings.Builder
	for i, p := range pairs {
		if i > 0 && i%4 == 0 {
			sb.WriteString(sep)
			sb.WriteByte('\n')
		}
		val := lipgloss.NewStyle().MaxWidth(valW).Render(p.v)
		sb.WriteString(keyStyle.Render(p.k))
		sb.WriteByte(' ')
		sb.WriteString(val)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

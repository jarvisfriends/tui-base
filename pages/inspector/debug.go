package inspector

import (
	"context"
	"fmt"
	"image/color"
	"net"
	"net/http"
	netpprof "net/http/pprof"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	runtimepprof "runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/page"
	"github.com/jarvisfriends/tui-base/theme"
	tint "github.com/lrstanley/bubbletint/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const defaultLatestValueRenderInterval = 500 * time.Millisecond
const defaultStatsRefreshInterval = time.Second

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

// DebugKeyMap holds key bindings for the debug inspector page. All bindings
// are exported so consumers can rebind them without forking the package.
type DebugKeyMap struct {
	Accessibility key.Binding // toggle accessibility panel
	Highlight     key.Binding // toggle mouse-highlight overlay
	NotifyInfo    key.Binding // fire a test info notification
	NotifyWarning key.Binding // fire a test warning notification
	NotifyError   key.Binding // fire a test error notification
}

// DefaultDebugKeys returns the default key bindings for the debug inspector.
func DefaultDebugKeys() DebugKeyMap {
	return DebugKeyMap{
		Accessibility: key.NewBinding(key.WithKeys("a", "A"), key.WithHelp("a", "accessibility panel")),
		Highlight:     key.NewBinding(key.WithKeys("h", "H"), key.WithHelp("h", "highlight toggle")),
		NotifyInfo:    key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "test info notification")),
		NotifyWarning: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "test warning notification")),
		NotifyError:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "test error notification")),
	}
}

type debugTab int

const (
	debugTabRuntime debugTab = iota
	debugTabInput
	debugTabDisks
	debugTabTerminal
	debugTabLog
	debugTabSettings
)

var debugTabTitles = []string{"Runtime", "Input", "Disks", "Terminal", "Log", "Settings"}

// pprof address constants — used in New() and handleSettingsKey to avoid
// magic string literals scattered across the function.
const (
	pprofDefaultAddr   = "127.0.0.1:6060"
	pprofAltAddr       = "127.0.0.1:7070"
	pprofDefaultToolUI = "127.0.0.1:18080"
	pprofAltToolUI     = "127.0.0.1:18081"
	// pprofDefaultCaptureSecs is the default CPU profile capture duration.
	pprofDefaultCaptureSecs = 10
)

// debugBorderPaddingX is the number of extra characters the border style adds
// between the border char and the inner content on each horizontal side.
// This must match the Padding(0, N) argument passed to borderStyle in View().
const debugBorderPaddingX = 1

// settingsRowIndex identifies rows in the debug Settings tab by their position
// in the slice returned by settingsRows(). Using a typed constant set here
// eliminates magic numbers from the handleSettingsKey switch statement and
// makes it obvious when a row is inserted or removed that all cases must update.
type settingsRowIndex int

const (
	settingsRowLatestRefresh   settingsRowIndex = iota // 0
	settingsRowStatsRefresh                            // 1
	settingsRowStatusSummary                           // 2
	settingsRowShowTerm                                // 3
	settingsRowShowHeap                                // 4
	settingsRowShowGC                                  // 5
	settingsRowShowGoroutines                          // 6
	settingsRowPprofEnabled                            // 7
	settingsRowPprofAddr                               // 8
	settingsRowPprofToolAddr                           // 9
	settingsRowPprofViewMode                           // 10
	settingsRowCPUSecs                                 // 11
	settingsRowOutputDir                               // 12 — read-only display
	settingsRowWriteHeap                               // 13
	settingsRowCaptureCPU                              // 14
	settingsRowBuiltinHeader                           // 15 — SectionOnly
	settingsRowPprofIndex                              // 16
	settingsRowHeapDebug1                              // 17
	settingsRowHeapDebug2                              // 18
	settingsRowGoroutineDebug1                         // 19
	settingsRowGoroutineDebug2                         // 20
	settingsRowAllocsDebug1                            // 21
	settingsRowBlockDebug1                             // 22
	settingsRowMutexDebug1                             // 23
	settingsRowCPUStream                               // 24
	settingsRowTraceStream                             // 25
	settingsRowGotoolHeader                            // 26 — SectionOnly
	settingsRowGotoolLatest                            // 27
	settingsRowGotoolLiveHeap                          // 28
	settingsRowGotoolLiveCPU                           // 29
	settingsRowServerState                             // 30 — read-only display
)

type summaryFlags struct {
	Enabled    bool
	ShowTerm   bool
	ShowHeap   bool
	ShowGC     bool
	ShowGorout bool
}

type pprofConfig struct {
	Enabled         bool
	Addr            string
	ToolUIAddr      string
	ViewMode        string
	OutputDir       string
	CPUCaptureSecs  int
	ServerURL       string
	server          *http.Server
	LastProfilePath string
}

type pprofServerStartedMsg struct {
	Server *http.Server
	URL    string
	Err    error
}

type pprofServerStoppedMsg struct{ Err error }

type pprofActionMsg struct {
	Kind string
	Path string
	Text string
	Err  error
}

type tabMouseRange struct {
	Tab    debugTab
	StartX int
	EndX   int
}

type debugSettingRow struct {
	Field       string
	Value       string
	Help        string
	SectionOnly bool
	ActionOnly  bool
}

type InspectorModel struct {
	page.Base

	Logs            []MsgLog
	logViewport     viewport.Model
	sectionViewport viewport.Model
	inputViewport   viewport.Model
	scrollToBottom  bool
	// Accessibility panel — toggled with 'a'
	acPanel *AccessibilityPanel
	// highlight when stable values change, Change background color
	ShowHighlight    bool
	LastMouseClick   tea.Mouse
	LastMouseRelease tea.Mouse
	LastMouseMotion  tea.Mouse
	LastMouseWheel   tea.Mouse
	LastKeyPress     tea.Key
	LastKeyRel       tea.Key
	// runtime stats refreshed by background tick (≤1 s cadence)
	stats     runtimeStatsSnapshot
	prevStats runtimeStatsSnapshot // previous snapshot for computing per-second deltas
	startTime time.Time
	printer   *message.Printer

	// Terminal diagnostics — populated by TermDiagMsg forwarded from the router.
	termDiag       *TermDiagMsg
	termDiagSet    bool
	initialProfile colorprofile.Profile // detected at New() time
	visible        bool

	// Cached render artifacts to reduce per-frame allocations.
	view            tea.View
	dirty           bool
	runtimeTbl      table.Model
	inputDbgTbl     table.Model
	inputDbgColumns []table.Column // pre-allocated column slice for the input debug table
	inputDbgColMaxW []int          // high-watermark: max rendered width ever seen per column
	diskTbl         table.Model
	runtimeColumns  []table.Column // pre-allocated column slice for the runtime stats table (to avoid reallocating on every View)
	runtimeColMaxW  []int          // high-watermark: max rendered width ever seen per column
	diskHeader      []table.Column // pre-allocated column slice for the disk stats table

	// colorProfileEnvVar is the app-specific env var name for color-profile
	// overrides, set by the router via SetColorProfileEnvVar.
	colorProfileEnvVar string

	// Coalesces high-frequency "latest value" updates (mouse/key telemetry)
	// so we don't force a full inspector rebuild for every single event.
	latestValueDirty      bool
	latestValueFlushTimer bool
	latestValueInterval   time.Duration
	statsRefreshInterval  time.Duration

	activeTab       debugTab
	settingsCursor  int
	statusSummary   summaryFlags
	pprof           pprofConfig
	settingsMessage string

	// Per-tab scrolling and mouse-hit metadata.
	tabScrollY     map[debugTab]int
	tabRanges      []tabMouseRange
	sectionOriginY int
	sectionOriginX int
	tabsOriginY    int
	tabsHeight     int
	sectionHeight  int

	// keys holds rebindable key bindings for the inspector.
	keys DebugKeyMap

	// logMu guards pendingLogs, which is the cross-goroutine inbox for log
	// entries. AddLog (called from the logging subscriber goroutine) appends
	// here; Update drains it into m.Logs on the tea goroutine. m.Logs itself
	// is tea-goroutine-only and needs no lock.
	logMu       sync.Mutex
	pendingLogs []MsgLog
}

type latestValueFlushMsg struct{}

func (m *InspectorModel) IsVisible() bool { return m.visible }
func (m *InspectorModel) ToggleVisible()  { m.visible = !m.visible }

// ShortHelp implements [help.KeyMap]. Returns a compact list of bindings for
// the current tab shown in the status bar one-liner.
func (m *InspectorModel) ShortHelp() []key.Binding {
	tabSwitch := key.NewBinding(
		key.WithKeys("left", "right", "1", "2", "3", "4", "5", "6"),
		key.WithHelp("←/→ 1-6", "switch tab"),
	)
	scroll := key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "scroll"),
	)
	switch m.activeTab {
	case debugTabSettings:
		enterRun := key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "change/run"),
		)
		return []key.Binding{tabSwitch, scroll, enterRun}
	case debugTabLog:
		return []key.Binding{
			tabSwitch, scroll,
			m.keys.NotifyInfo, m.keys.NotifyWarning, m.keys.NotifyError,
			m.keys.Accessibility,
		}
	default:
		return []key.Binding{tabSwitch, scroll, m.keys.Highlight}
	}
}

// FullHelp implements [help.KeyMap]. Returns the expanded binding table shown
// when the user presses '?' in the status bar.
func (m *InspectorModel) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// SetColorProfileEnvVar tells the inspector which env-var name the embedding
// app uses for color-profile overrides (e.g. "MY_APP_COLOR_PROFILE").
// The router calls this immediately after construction so the inspector can
// surface the correct env-var name in its terminal diagnostics section.
func (m *InspectorModel) SetColorProfileEnvVar(name string) {
	if name == "" {
		name = "TUI_BASE_COLOR_PROFILE"
	}
	m.colorProfileEnvVar = name
}

// SetColors stores a shared AppColors pointer so the router can update the
// theme in one place and this model sees the change immediately.
func (m *InspectorModel) SetColors(c *theme.AppStyle) {
	m.Base.SetColors(c)
	m.dirty = true
	if m.acPanel != nil {
		m.acPanel.SetColors(c)
	}
}

func (m *InspectorModel) saveActiveTabScroll() {
	if m.tabScrollY == nil {
		m.tabScrollY = make(map[debugTab]int)
	}
	if m.activeTab == debugTabLog {
		m.tabScrollY[m.activeTab] = max(0, m.logViewport.YOffset())
		return
	}
	m.tabScrollY[m.activeTab] = max(0, m.sectionViewport.YOffset())
}

func (m *InspectorModel) restoreActiveTabScroll() {
	y := max(0, m.tabScrollY[m.activeTab])
	if m.activeTab == debugTabLog {
		m.logViewport.SetYOffset(y)
		return
	}
	m.sectionViewport.SetYOffset(y)
}

func (m *InspectorModel) switchTab(tab debugTab) {
	if tab < 0 || int(tab) >= len(debugTabTitles) || tab == m.activeTab {
		return
	}
	m.saveActiveTabScroll()
	m.activeTab = tab
	m.restoreActiveTabScroll()
	m.dirty = true
}

func (m *InspectorModel) scrollActiveSection(lines int) {
	if lines == 0 {
		return
	}
	if m.activeTab == debugTabSettings {
		items := m.settingsRows()
		if len(items) == 0 {
			return
		}
		m.settingsCursor = max(0, min(len(items)-1, m.settingsCursor+lines))
		m.ensureSettingsCursorVisible(len(items))
		m.saveActiveTabScroll()
		m.dirty = true
		return
	}
	if m.activeTab == debugTabLog {
		if lines < 0 {
			m.logViewport.ScrollUp(-lines)
		} else {
			m.logViewport.ScrollDown(lines)
		}
		m.saveActiveTabScroll()
		m.dirty = true
		return
	}
	if lines < 0 {
		m.sectionViewport.ScrollUp(-lines)
	} else {
		m.sectionViewport.ScrollDown(lines)
	}
	m.saveActiveTabScroll()
	m.dirty = true
}

func (m *InspectorModel) ensureSettingsCursorVisible(itemCount int) {
	if itemCount <= 0 || m.sectionViewport.Height() <= 0 {
		return
	}
	row := max(0, min(itemCount-1, m.settingsCursor))
	top := m.sectionViewport.YOffset()
	bottom := top + m.sectionViewport.Height() - 1
	if row < top {
		m.sectionViewport.SetYOffset(row)
		return
	}
	if row > bottom {
		m.sectionViewport.SetYOffset(max(0, row-m.sectionViewport.Height()+1))
	}
}

func (m *InspectorModel) activateSettingsRowByClick(localY int) tea.Cmd {
	if localY < m.sectionOriginY {
		return nil
	}
	items := m.settingsRows()
	if len(items) == 0 {
		return nil
	}
	line := m.sectionViewport.YOffset() + (localY - m.sectionOriginY)
	if line < 0 || line >= len(items) {
		return nil
	}
	m.settingsCursor = line
	m.ensureSettingsCursorVisible(len(items))
	m.dirty = true
	return m.handleSettingsKey(tea.KeyPressMsg{Code: tea.KeyEnter})
}

func (m *InspectorModel) selectTabByX(localX int) bool {
	for _, r := range m.tabRanges {
		if localX >= r.StartX && localX <= r.EndX {
			m.switchTab(r.Tab)
			return true
		}
	}
	return false
}

func New() *InspectorModel {
	m := &InspectorModel{
		Logs:                 make([]MsgLog, 0),
		startTime:            time.Now(),
		acPanel:              NewAccessibilityPanel(),
		printer:              message.NewPrinter(language.English),
		dirty:                true,
		initialProfile:       colorprofile.Detect(os.Stdout, os.Environ()),
		latestValueInterval:  defaultLatestValueRenderInterval,
		statsRefreshInterval: defaultStatsRefreshInterval,
		activeTab:            debugTabRuntime,
		tabScrollY:           make(map[debugTab]int),
		statusSummary: summaryFlags{
			Enabled:    false,
			ShowTerm:   true,
			ShowHeap:   true,
			ShowGC:     true,
			ShowGorout: true,
		},
		pprof: pprofConfig{
			Enabled:        false,
			Addr:           pprofDefaultAddr,
			ToolUIAddr:     pprofDefaultToolUI,
			ViewMode:       "builtin",
			OutputDir:      filepath.Join(os.TempDir(), "tui-base", "pprof"),
			CPUCaptureSecs: pprofDefaultCaptureSecs,
		},
		keys: DefaultDebugKeys(),
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

	m.inputDbgColumns = make([]table.Column, 6)
	for i := range m.inputDbgColumns {
		if i%2 == 0 {
			m.inputDbgColumns[i] = table.Column{Title: "Metric", Width: -1}
		} else {
			m.inputDbgColumns[i] = table.Column{Title: "Value", Width: -1}
		}
	}
	m.inputDbgColMaxW = make([]int, 6)

	m.inputDbgTbl = table.New(
		table.WithColumns(m.inputDbgColumns),
		table.WithRows(nil),
		table.WithFocused(false),
		table.WithHeight(2),
		table.WithWidth(60),
	)

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
	m.sectionViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	m.inputViewport = viewport.New(viewport.WithWidth(60), viewport.WithHeight(2))

	m.view = tea.NewView("")
	return m
}

func (m *InspectorModel) Init() tea.Cmd {
	return m.scheduleStatsTick()
}

func (m *InspectorModel) scheduleLatestValueFlush() tea.Cmd {
	if m.latestValueFlushTimer {
		return nil
	}
	m.latestValueFlushTimer = true
	return tea.Tick(m.latestValueInterval, func(time.Time) tea.Msg { return latestValueFlushMsg{} })
}

// statsTickMsg carries a freshly collected snapshot back to Update.
type statsTickMsg struct{ snapshot runtimeStatsSnapshot }

// scheduleStatsTick returns a Cmd that fires once after 1 s and delivers a
// new snapshot. Update() reschedules it so stats refresh continuously.
func (m *InspectorModel) scheduleStatsTick() tea.Cmd {
	return tea.Tick(m.statsRefreshInterval, func(t time.Time) tea.Msg {
		return statsTickMsg{snapshot: collectSnapshot(m.startTime)}
	})
}

func (m *InspectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Drain any log entries queued by AddLog (subscriber goroutine) into m.Logs.
	// m.Logs is tea-goroutine-only; pendingLogs is the cross-goroutine inbox.
	m.logMu.Lock()
	pending := m.pendingLogs
	m.pendingLogs = nil
	m.logMu.Unlock()
	for _, entry := range pending {
		m.appendExternalLog(entry)
	}

	// Record every message the inspector sees (deduped/stacked) so the log pane
	// reflects live traffic. Silent, high-frequency messages return early.
	preCmd := m.LogMessageForDebugging(msg)
	switch msg := msg.(type) {
	case latestValueFlushMsg:
		m.latestValueFlushTimer = false
		if m.latestValueDirty {
			m.latestValueDirty = false
			m.dirty = true
		}
		return m, nil
	case pprofServerStartedMsg:
		if msg.Err != nil {
			m.settingsMessage = "pprof server start failed: " + msg.Err.Error()
		} else {
			m.pprof.server = msg.Server
			m.pprof.ServerURL = msg.URL
			m.settingsMessage = "pprof server running at " + msg.URL
		}
		m.dirty = true
		return m, preCmd
	case pprofServerStoppedMsg:
		if msg.Err != nil {
			m.settingsMessage = "pprof server stop failed: " + msg.Err.Error()
		} else {
			m.pprof.server = nil
			m.pprof.ServerURL = ""
			m.settingsMessage = "pprof server stopped"
		}
		m.dirty = true
		return m, preCmd
	case pprofActionMsg:
		if msg.Err != nil {
			m.settingsMessage = msg.Kind + " failed: " + msg.Err.Error()
		} else if msg.Text != "" {
			m.settingsMessage = msg.Text
		} else {
			m.settingsMessage = msg.Kind + " complete"
		}
		if msg.Path != "" {
			m.pprof.LastProfilePath = msg.Path
		}
		m.dirty = true
		return m, preCmd
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.dirty = true
		if m.acPanel != nil {
			m.acPanel.SetSize(msg.Width, msg.Height)
		}
		m.logViewport.SetWidth(msg.Width)
		m.logViewport.SetHeight(msg.Height)
		return m, preCmd // size changes are silent — do not log
	case tea.KeyMsg:
		// When the accessibility panel is open it handles its own navigation;
		// only 'a' is intercepted here to toggle it.
		if m.acPanel != nil && m.acPanel.IsVisible() {
			if press, ok := msg.(tea.KeyPressMsg); ok && key.Matches(press, m.keys.Accessibility) {
				m.acPanel.Toggle()
				return m, preCmd
			}
			_, cmd := m.acPanel.Update(msg)
			return m, tea.Batch(preCmd, cmd)
		}
		switch km := msg.(type) {
		case tea.KeyPressMsg:
			if m.activeTab == debugTabSettings {
				// Settings tab: Up/Down navigate rows, Enter toggles/runs.
				// Left/Right fall through so they always switch tabs.
				switch km.Code {
				case tea.KeyUp, tea.KeyDown, tea.KeyEnter:
					return m, tea.Batch(preCmd, m.handleSettingsKey(km))
				}
			} else {
				switch km.Code {
				case tea.KeyUp:
					m.scrollActiveSection(-1)
					return m, preCmd
				case tea.KeyDown:
					m.scrollActiveSection(1)
					return m, preCmd
				case tea.KeyPgUp:
					m.scrollActiveSection(-max(1, m.sectionHeight/2))
					return m, preCmd
				case tea.KeyPgDown:
					m.scrollActiveSection(max(1, m.sectionHeight/2))
					return m, preCmd
				}
			}
			if km.Code == tea.KeyLeft {
				m.switchTab(debugTab((int(m.activeTab) - 1 + len(debugTabTitles)) % len(debugTabTitles)))
				return m, preCmd
			}
			if km.Code == tea.KeyRight {
				m.switchTab(debugTab((int(m.activeTab) + 1) % len(debugTabTitles)))
				return m, preCmd
			}
			if km.Text >= "1" && km.Text <= "6" {
				m.switchTab(debugTab(int(km.Text[0] - '1')))
				return m, preCmd
			}
			switch {
			case key.Matches(km, m.keys.Accessibility):
				if m.acPanel != nil {
					m.acPanel.Toggle()
				}
				m.dirty = true
				return m, preCmd
			case key.Matches(km, m.keys.Highlight):
				m.ShowHighlight = !m.ShowHighlight
				m.dirty = true
				return m, preCmd
			case key.Matches(km, m.keys.NotifyInfo):
				return m, tea.Batch(preCmd, func() tea.Msg {
					return notifications.AddMsg{Content: "Test: Info notification from Inspector", Severity: notifications.SeverityInfo, TTL: notifications.SeverityInfo.DefaultTTL()}
				})
			case key.Matches(km, m.keys.NotifyWarning):
				return m, tea.Batch(preCmd, func() tea.Msg {
					return notifications.AddMsg{Content: "Test: Warning notification from Inspector", Severity: notifications.SeverityWarning, TTL: notifications.SeverityWarning.DefaultTTL()}
				})
			case key.Matches(km, m.keys.NotifyError):
				return m, tea.Batch(preCmd, func() tea.Msg {
					return notifications.AddMsg{Content: "Test: Error notification from Inspector", Severity: notifications.SeverityError, TTL: notifications.SeverityError.DefaultTTL()}
				})
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
		return m, tea.Batch(preCmd, m.scheduleStatsTick())
	case tea.MouseWheelMsg:
		if msg.Mouse().Button == tea.MouseWheelUp {
			m.scrollActiveSection(-3)
		} else {
			m.scrollActiveSection(3)
		}
		return m, preCmd
	}
	return m, preCmd
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

func KeyModToString(mod tea.KeyMod) string {
	return strings.ReplaceAll(strings.TrimSpace(tea.Key{Mod: mod}.Keystroke()), "+", " ")
}

func (m *InspectorModel) LogMessageForDebugging(msg tea.Msg) tea.Cmd {
	msgType := fmt.Sprintf("%T", msg)
	msgContent := fmt.Sprintf("%+v", msg)
	switch mt := msg.(type) {
	case statsTickMsg:
		return nil // skip logging internal stats ticks to reduce noise
	case tea.WindowSizeMsg:
		_ = mt
		return nil // window resize is high-frequency and already reflected in the layout; skip logging
	case tea.EnvMsg:
		msgContent = ""
		for _, kv := range mt {
			// Ex. [ACLOCAL_PATH=C:\Program Files\Git\mingw64\share\aclocal;C:\Program Files\Git\usr\share\aclocal ALLUSERSPROFILE=C:\ProgramData APPDATA=C:\Users
			if len(msgContent) > 0 {
				msgContent += "\n  "
			}
			if pair := strings.SplitN(kv, "=", 2); len(pair) == 2 {
				msgContent += fmt.Sprintf("Key: %s  Value: %s", pair[0], pair[1])
			} else {
				msgContent += fmt.Sprintf("Env: %s", kv)
			}
		}
	case tea.MouseMsg:
		curMouse := mt.Mouse()
		switch mt.(type) {
		case tea.MouseClickMsg:
			m.LastMouseClick = curMouse
			m.latestValueDirty = true
			return m.scheduleLatestValueFlush()
		case tea.MouseReleaseMsg:
			m.LastMouseRelease = curMouse
			m.latestValueDirty = true
			return m.scheduleLatestValueFlush()
		case tea.MouseWheelMsg:
			m.LastMouseWheel = curMouse
			m.latestValueDirty = true
			return m.scheduleLatestValueFlush()
		case tea.MouseMotionMsg:
			// Mouse motion can be extremely high frequency. Only invalidate the view
			// when highlight UI is enabled; otherwise keep cached rendering.
			if m.ShowHighlight {
				m.latestValueDirty = true
				return m.scheduleLatestValueFlush()
			}
			// Avoid logging cell-motion spam; only log user-significant mouse actions.
			m.LastMouseMotion = curMouse
			return nil
		default:
			msgContent = fmt.Sprintf("Global: %d,%d  Button: %s  Mod: %d(%s)",
				curMouse.X, curMouse.Y, curMouse.Button, curMouse.Mod, KeyModToString(curMouse.Mod))
		}
	case tea.KeyMsg:
		switch km := msg.(type) {
		case tea.KeyPressMsg:
			m.LastKeyPress = km.Key()
			m.latestValueDirty = true
			return m.scheduleLatestValueFlush() // skip logging every key press to reduce noise; tracked separately in the view
		case tea.KeyReleaseMsg:
			m.LastKeyRel = km.Key()
			m.latestValueDirty = true
			return m.scheduleLatestValueFlush()
		default:
			msgContent = fmt.Sprintf("%T Key: %s", km, mt.String())
		}
	}

	// Check if the last log is the same to stack them.
	if len(m.Logs) > 0 {
		last := &m.Logs[len(m.Logs)-1]
		if last.Type == msgType && last.Content == msgContent {
			last.Count++
			last.Timestamp = time.Now()
			m.dirty = true
			return nil
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
	return nil
}

// appendExternalLog merges one MsgLog (received from the pending queue) into
// m.Logs with deduplication. Must only be called from the tea goroutine.
func (m *InspectorModel) appendExternalLog(entry MsgLog) {
	if len(m.Logs) > 0 {
		last := &m.Logs[len(m.Logs)-1]
		if last.Type == entry.Type && last.Content == entry.Content {
			last.Count++
			last.Timestamp = entry.Timestamp
			m.dirty = true
			return
		}
	}
	m.Logs = append(m.Logs, entry)
	m.dirty = true
	if len(m.Logs) > 50 {
		m.Logs = m.Logs[1:]
	}
	m.scrollToBottom = true
}

// AddLog adds an external log entry (from the runtime logger) to the
// inspector. It is safe to call from any goroutine. Entries are buffered in
// pendingLogs and drained into m.Logs on the next Update() call so that
// m.Logs is only ever accessed by the tea goroutine.
func (m *InspectorModel) AddLog(level string, ts time.Time, content string) {
	m.logMu.Lock()
	m.pendingLogs = append(m.pendingLogs, MsgLog{
		Timestamp: ts,
		Type:      level,
		Content:   content,
		Count:     1,
	})
	m.logMu.Unlock()
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

func (m *InspectorModel) View() tea.View {
	if !m.dirty {
		return m.view
	}

	c := m.Colors()
	availW := max(m.Width()-2, 20)
	runtimeRows := m.buildRuntimeRows(c)
	inputRows := m.buildInputRows(c)
	m.updateRuntimeColumnWidths(runtimeRows)
	m.updateInputColumnWidths(inputRows)
	tblStyles := m.baseTableStyles(c)
	runtimeSection := m.renderRuntimeSection(c, tblStyles, runtimeRows, availW)
	logContent := m.renderLogContent(c)
	tabsLine := m.buildTabsLine(c)
	sectionTitle, sectionContent := m.sectionForActiveTab(c, availW, tblStyles, runtimeSection, inputRows, logContent)

	// When the accessibility panel is open, render it in place of the section content.
	if m.acPanel != nil && m.acPanel.IsVisible() {
		panelH := max(m.Height()-4, 6)
		m.acPanel.SetSize(m.Width(), panelH)
		acStr := c.Styles.TextOnBg.
			Width(m.Width()).
			Height(panelH).
			MaxHeight(panelH).
			Render(m.acPanel.View().Content)
		titleLine := c.Styles.Title.Padding(0, 1).Render("MESSAGE INSPECTOR (Inspector)")
		m.view.SetContent(lipgloss.JoinVertical(lipgloss.Left, titleLine, tabsLine, acStr))
		m.view.BackgroundColor = c.Styles.TextOnBg.GetBackground()
		m.view.ForegroundColor = c.Styles.TextOnBg.GetForeground()
		m.dirty = false
		return m.view
	}

	titleText := sectionTitle + " (Inspector)"
	titleLine := lipgloss.PlaceHorizontal(availW, lipgloss.Center, c.Styles.Title.Bold(true).Render(titleText))
	sep := c.Styles.Title.Render(strings.Repeat("─", availW))

	topH := lipgloss.Height(titleLine) + lipgloss.Height(tabsLine) + lipgloss.Height(sep)
	m.sectionOriginX = 0
	// tabsOriginY: the inner content layout is titleLine → sep → tabsLine, so
	// tabs start after both title AND separator lines.
	m.tabsOriginY = lipgloss.Height(titleLine) + lipgloss.Height(sep)
	m.tabsHeight = lipgloss.Height(tabsLine)
	m.sectionOriginY = topH
	m.sectionHeight = max(1, m.Height()-topH)
	m.logViewport.SetWidth(max(m.Width(), 1))
	m.logViewport.SetHeight(m.sectionHeight)
	m.logViewport.SetContent(logContent)
	if m.scrollToBottom {
		m.logViewport.GotoBottom()
		m.scrollToBottom = false
	}

	if m.activeTab == debugTabLog {
		m.restoreActiveTabScroll()
		sectionContent = m.logViewport.View()
	} else {
		m.sectionViewport.SetWidth(max(m.Width(), 1))
		m.sectionViewport.SetHeight(m.sectionHeight)
		m.sectionViewport.SetContent(sectionContent)
		m.restoreActiveTabScroll()
		if m.activeTab == debugTabSettings {
			m.ensureSettingsCursorVisible(len(m.settingsRows()))
		}
		sectionContent = m.sectionViewport.View()
	}

	m.view.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	m.view.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	m.view.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		if wheel, ok := mm.(tea.MouseWheelMsg); ok {
			if wheel.Mouse().Button == tea.MouseWheelUp {
				m.scrollActiveSection(-3)
			} else {
				m.scrollActiveSection(3)
			}
			return nil
		}
		if rel, ok := mm.(tea.MouseReleaseMsg); ok && rel.Mouse().Button == tea.MouseLeft {
			my := rel.Mouse().Y
			if my >= m.tabsOriginY && my < m.tabsOriginY+m.tabsHeight {
				m.selectTabByX(rel.Mouse().X)
				return nil
			}
			if m.activeTab == debugTabSettings {
				return m.activateSettingsRowByClick(my)
			}
		}
		return nil
	}
	borderStyle := c.Styles.OverlayBorder.
		Border(lipgloss.RoundedBorder()).
		Background(c.Styles.TextOnBg.GetBackground()).
		Foreground(c.Styles.TextOnBg.GetForeground()).
		Padding(0, 1)
	inner := lipgloss.JoinVertical(
		lipgloss.Left,
		titleLine,
		sep,
		tabsLine,
		sectionContent,
	)

	m.view.SetContent(borderStyle.Render(inner))
	m.saveActiveTabScroll()
	m.dirty = false
	return m.view
}

// buildTermSection renders the terminal environment and active theme diagnostic rows.
func (m *InspectorModel) buildTermSection(c *theme.AppStyle, width int) string {
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
	envName := m.colorProfileEnvVar
	if envName == "" {
		envName = "TUI_BASE_COLOR_PROFILE"
	}
	profileOverride := strings.TrimSpace(os.Getenv(envName))
	if profileOverride != "" {
		profStr += " (forced: " + envName + "=" + profileOverride + ")"
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
			warn("Fix", "set COLORTERM=truecolor on the remote, or run with "+envName+"=truecolor"),
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

func (m *InspectorModel) renderSettingsSection(c *theme.AppStyle) string {
	items := m.settingsRows()
	availW := max(m.Width()-2, 28)
	fieldW := 0
	for _, row := range items {
		if row.SectionOnly {
			continue
		}
		fieldW = max(fieldW, lipgloss.Width(row.Field))
	}
	fieldW = min(fieldW, max(availW/2, 10))
	valueW := max(availW-fieldW-5, 6)

	normalField := c.Styles.Item.Width(fieldW)
	normalValue := c.Styles.TextOnBg.Width(valueW)
	selectedField := c.Styles.Title.Width(fieldW)
	selectedValue := c.Styles.TextOnBg.Width(valueW)
	selectedRow := c.Styles.Row.Background(c.Styles.TabHover.GetBackground()).Width(availW)

	var out []string
	for i, row := range items {
		if row.SectionOnly {
			out = append(out, c.Styles.Subtitle.Render(row.Field))
			continue
		}
		field := truncate(row.Field, fieldW)
		value := truncate(row.Value, valueW)
		if i == m.settingsCursor {
			prefix := "▶ "
			if row.ActionOnly {
				prefix = "↵ "
			}
			line := prefix + selectedField.Render(field) + "   " + selectedValue.Render(value)
			out = append(out, selectedRow.Render(line))
			if row.Help != "" {
				out = append(out, c.Styles.Dim.Render("   "+row.Help))
			}
			continue
		}
		out = append(out, "  "+normalField.Render(field)+"   "+normalValue.Render(value))
	}
	if m.settingsMessage != "" {
		out = append(out, "", c.Styles.Subtitle.Render(m.settingsMessage))
	}

	return strings.Join(out, "\n")
}

func (m *InspectorModel) settingsRows() []debugSettingRow {
	pprofState := "off"
	if m.pprof.Enabled {
		pprofState = "on"
	}
	serverState := "stopped"
	if m.pprof.ServerURL != "" {
		serverState = m.pprof.ServerURL
	}
	secs := strconv.Itoa(max(1, m.pprof.CPUCaptureSecs))
	return []debugSettingRow{
		// 0-6: general display settings
		{Field: "Latest-value refresh", Value: fmt.Sprintf("%dms", m.latestValueInterval/time.Millisecond), Help: "Enter increases cadence by 100 ms (mouse/key telemetry redraw interval)"},
		{Field: "Runtime tick refresh", Value: fmt.Sprintf("%dms", m.statsRefreshInterval/time.Millisecond), Help: "Enter increases cadence by 100 ms (runtime snapshot update interval)"},
		{Field: "Status summary on close", Value: fmt.Sprintf("%t", m.statusSummary.Enabled), Help: "Enter toggles compact runtime summary in status bar when inspector is closed"},
		{Field: "Include terminal size", Value: fmt.Sprintf("%t", m.statusSummary.ShowTerm), Help: "Enter toggles terminal dimensions in the status summary"},
		{Field: "Include heap size", Value: fmt.Sprintf("%t", m.statusSummary.ShowHeap), Help: "Enter toggles live heap allocation bytes in the status summary"},
		{Field: "Include GC/sec", Value: fmt.Sprintf("%t", m.statusSummary.ShowGC), Help: "Enter toggles GC cadence in the status summary"},
		{Field: "Include goroutines", Value: fmt.Sprintf("%t", m.statusSummary.ShowGorout), Help: "Enter toggles goroutine count in the status summary"},
		// 7-12: pprof server config
		{Field: "Enable profiler HTTP server", Value: pprofState, Help: "Enter toggles Go's built-in pprof server. Required for all browser viewer endpoints below. Profiles measure CPU, memory, goroutines, and GC."},
		{Field: "Profiler listen addr", Value: m.pprof.Addr, Help: fmt.Sprintf("Enter cycles between %s and %s. Restart server to apply.", pprofDefaultAddr, pprofAltAddr)},
		{Field: "go tool pprof UI addr", Value: m.pprof.ToolUIAddr, Help: fmt.Sprintf("Enter cycles between %s and %s (address used when launching go tool pprof -http).", pprofDefaultToolUI, pprofAltToolUI)},
		{Field: "Pprof view mode", Value: m.pprof.ViewMode, Help: "Enter cycles: builtin (browser, no deps) → go-tool (go tool pprof -http, needs Go in PATH) → graphviz (graph view, also needs 'dot')"},
		{Field: "CPU capture duration (secs)", Value: secs, Help: "Enter increments by 1 s. Used by 'Capture CPU profile' and the CPU stream browser endpoint."},
		{Field: "Output dir", Value: m.pprof.OutputDir, Help: "Heap/CPU snapshot files are written here. Analyze with: go tool pprof <file>"},
		// 13-14: capture actions (no server required)
		{Field: "Write heap snapshot", Value: "Enter to run", ActionOnly: true, Help: "Saves a heap profile (.pprof) to Output dir right now. No server required. Useful for offline analysis."},
		{Field: "Capture CPU profile", Value: fmt.Sprintf("Enter to run (%ss)", secs), ActionOnly: true, Help: "Records CPU samples for the configured duration. Blocks the UI during capture. Analyze with: go tool pprof <file>"},
		// 15: section header
		{Field: "── Browser viewer (profiler HTTP must be enabled above) ──", SectionOnly: true},
		// 16-25: built-in browser endpoints (no external dependencies)
		{Field: "Profile index page", Value: "Open", ActionOnly: true, Help: "All available profile types as a human-readable HTML index."},
		{Field: "Heap — allocation counts (debug=1)", Value: "Open", ActionOnly: true, Help: "Live heap: per-allocation stack traces. Best starting point for finding memory leaks."},
		{Field: "Heap — all live objects (debug=2)", Value: "Open", ActionOnly: true, Help: "Every live heap object with full stacks. Very verbose; use debug=1 first."},
		{Field: "Goroutines — deduplicated (debug=1)", Value: "Open", ActionOnly: true, Help: "One line per unique goroutine stack. Easy to scan; good for spotting goroutine leaks."},
		{Field: "Goroutines — all stacks (debug=2)", Value: "Open", ActionOnly: true, Help: "Every running goroutine with its full stack. Shows exact counts; confirms goroutine leaks."},
		{Field: "Allocations profile (debug=1)", Value: "Open", ActionOnly: true, Help: "Heap allocations since program start (includes freed objects). Great for allocation hot-path analysis."},
		{Field: "Block profile (debug=1)", Value: "Open", ActionOnly: true, Help: "Where goroutines block on synchronization (channels, mutexes). Needs runtime.SetBlockProfileRate."},
		{Field: "Mutex profile (debug=1)", Value: "Open", ActionOnly: true, Help: "Mutex contention report. Needs runtime.SetMutexProfileFraction."},
		{Field: "CPU profile stream", Value: fmt.Sprintf("Open (%ss)", secs), ActionOnly: true, Help: "Browser downloads an N-second CPU profile. Analyze offline with: go tool pprof <file>"},
		{Field: "Execution trace stream (5s)", Value: "Open", ActionOnly: true, Help: "Browser downloads a 5-second execution trace. View with: go tool trace <file>"},
		// 26: section header
		{Field: "── go tool pprof -http (Go in PATH required; graph view needs Graphviz 'dot') ──", SectionOnly: true},
		// 27-29: go tool pprof -http (needs Go toolchain; graph view needs Graphviz)
		{Field: "Open pprof UI — last saved file", Value: "Run", ActionOnly: true, Help: "Launches 'go tool pprof -http' for the most recently saved .pprof file. Flame/top/source views need no Graphviz; graph view does."},
		{Field: "Open pprof UI — live heap", Value: "Run", ActionOnly: true, Help: "Launches 'go tool pprof -http' fetching live heap from the profiler server (must be enabled above)."},
		{Field: "Open pprof UI — live CPU", Value: "Run", ActionOnly: true, Help: fmt.Sprintf("Launches 'go tool pprof -http' capturing %s seconds of live CPU (UI appears after capture finishes).", secs)},
		// 30: info
		{Field: "Server", Value: serverState},
	}
}

func (m *InspectorModel) handleSettingsKey(km tea.KeyPressMsg) tea.Cmd {
	items := m.settingsRows()
	if len(items) == 0 {
		return nil
	}
	switch km.Code {
	case tea.KeyUp:
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		m.dirty = true
		return nil
	case tea.KeyDown:
		if m.settingsCursor < len(items)-1 {
			m.settingsCursor++
		}
		m.dirty = true
		return nil
	}

	// Only Enter is routed here now; Left/Right fall through to tab switching.
	if km.Code != tea.KeyEnter {
		m.dirty = true
		return nil
	}

	base := strings.TrimRight(m.pprof.ServerURL, "/")
	secs := strconv.Itoa(max(1, m.pprof.CPUCaptureSecs))

	requiresServer := func() bool {
		if m.pprof.ServerURL != "" {
			return true
		}
		m.settingsMessage = "pprof server is not running — enable 'Enable profiler HTTP server' first"
		m.dirty = true
		return false
	}

	switch settingsRowIndex(m.settingsCursor) {
	// --- display settings ---
	case settingsRowLatestRefresh:
		m.latestValueInterval = time.Duration(max(100, int((m.latestValueInterval+100*time.Millisecond)/time.Millisecond))) * time.Millisecond
	case settingsRowStatsRefresh:
		m.statsRefreshInterval = time.Duration(max(200, int((m.statsRefreshInterval+100*time.Millisecond)/time.Millisecond))) * time.Millisecond
	case settingsRowStatusSummary:
		m.statusSummary.Enabled = !m.statusSummary.Enabled
	case settingsRowShowTerm:
		m.statusSummary.ShowTerm = !m.statusSummary.ShowTerm
	case settingsRowShowHeap:
		m.statusSummary.ShowHeap = !m.statusSummary.ShowHeap
	case settingsRowShowGC:
		m.statusSummary.ShowGC = !m.statusSummary.ShowGC
	case settingsRowShowGoroutines:
		m.statusSummary.ShowGorout = !m.statusSummary.ShowGorout
	// --- pprof server config ---
	case settingsRowPprofEnabled:
		m.pprof.Enabled = !m.pprof.Enabled
		if m.pprof.Enabled {
			m.dirty = true
			return m.startPprofServerCmd()
		}
		m.dirty = true
		return m.stopPprofServerCmd()
	case settingsRowPprofAddr:
		if m.pprof.Addr == pprofDefaultAddr {
			m.pprof.Addr = pprofAltAddr
		} else {
			m.pprof.Addr = pprofDefaultAddr
		}
	case settingsRowPprofToolAddr:
		if m.pprof.ToolUIAddr == pprofDefaultToolUI {
			m.pprof.ToolUIAddr = pprofAltToolUI
		} else {
			m.pprof.ToolUIAddr = pprofDefaultToolUI
		}
	case settingsRowPprofViewMode: // cycle: builtin → go-tool → graphviz → builtin
		switch m.pprof.ViewMode {
		case "builtin":
			m.pprof.ViewMode = "go-tool"
		case "go-tool":
			m.pprof.ViewMode = "graphviz"
		default:
			m.pprof.ViewMode = "builtin"
		}
	case settingsRowCPUSecs:
		m.pprof.CPUCaptureSecs = max(1, m.pprof.CPUCaptureSecs+1)
	// settingsRowOutputDir: read-only display, no action
	// --- capture actions ---
	case settingsRowWriteHeap:
		m.dirty = true
		return m.writeProfileSnapshotCmd()
	case settingsRowCaptureCPU:
		m.dirty = true
		return m.captureCPUProfileCmd()
	// settingsRowBuiltinHeader: section header — not interactive
	case settingsRowPprofIndex:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/")
		}
	case settingsRowHeapDebug1:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/heap?debug=1")
		}
	case settingsRowHeapDebug2:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/heap?debug=2")
		}
	case settingsRowGoroutineDebug1:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/goroutine?debug=1")
		}
	case settingsRowGoroutineDebug2:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/goroutine?debug=2")
		}
	case settingsRowAllocsDebug1:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/allocs?debug=1")
		}
	case settingsRowBlockDebug1:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/block?debug=1")
		}
	case settingsRowMutexDebug1:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/mutex?debug=1")
		}
	case settingsRowCPUStream:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/profile?seconds=" + secs)
		}
	case settingsRowTraceStream:
		if requiresServer() {
			m.dirty = true
			return openBrowserCmd(base + "/trace?seconds=5")
		}
	// settingsRowGotoolHeader: section header — not interactive
	case settingsRowGotoolLatest:
		m.dirty = true
		return m.openGoToolPprofLatestCmd()
	case settingsRowGotoolLiveHeap:
		m.dirty = true
		return m.openGoToolPprofLiveHeapCmd()
	case settingsRowGotoolLiveCPU:
		m.dirty = true
		return m.openGoToolPprofLiveCPUCmd()
		// settingsRowServerState: read-only display
	}
	m.dirty = true
	return nil
}

func (m *InspectorModel) startPprofServerCmd() tea.Cmd {
	addr := m.pprof.Addr
	return func() tea.Msg {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", netpprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", netpprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", netpprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", netpprof.Trace)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return pprofServerStartedMsg{Err: err}
		}
		srv := &http.Server{Handler: mux}
		go func() { _ = srv.Serve(ln) }()
		return pprofServerStartedMsg{Server: srv, URL: "http://" + ln.Addr().String() + "/debug/pprof/"}
	}
}

func (m *InspectorModel) stopPprofServerCmd() tea.Cmd {
	srv := m.pprof.server
	if srv == nil {
		return func() tea.Msg { return pprofServerStoppedMsg{} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return pprofServerStoppedMsg{Err: srv.Shutdown(ctx)}
	}
}

func (m *InspectorModel) writeProfileSnapshotCmd() tea.Cmd {
	outDir := m.pprof.OutputDir
	return func() tea.Msg {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return pprofActionMsg{Kind: "snapshot", Err: err}
		}
		ts := time.Now().Format("20060102-150405")
		heapPath := filepath.Join(outDir, "heap-"+ts+".pprof")
		f, err := os.Create(heapPath)
		if err != nil {
			return pprofActionMsg{Kind: "snapshot", Err: err}
		}
		runtime.GC()
		err = runtimepprof.WriteHeapProfile(f)
		_ = f.Close()
		if err != nil {
			return pprofActionMsg{Kind: "snapshot", Err: err}
		}
		text := "heap snapshot saved: " + heapPath + " | open via built-in browser endpoints or go tool pprof UI actions in settings"
		return pprofActionMsg{Kind: "snapshot", Path: heapPath, Text: text}
	}
}

func (m *InspectorModel) captureCPUProfileCmd() tea.Cmd {
	outDir := m.pprof.OutputDir
	secs := m.pprof.CPUCaptureSecs
	return func() tea.Msg {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return pprofActionMsg{Kind: "cpu profile", Err: err}
		}
		ts := time.Now().Format("20060102-150405")
		cpuPath := filepath.Join(outDir, "cpu-"+ts+".pprof")
		f, err := os.Create(cpuPath)
		if err != nil {
			return pprofActionMsg{Kind: "cpu profile", Err: err}
		}
		if err := runtimepprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return pprofActionMsg{Kind: "cpu profile", Err: err}
		}
		time.Sleep(time.Duration(secs) * time.Second)
		runtimepprof.StopCPUProfile()
		_ = f.Close()
		text := fmt.Sprintf("cpu profile (%ds) saved: %s | open via go tool pprof UI or CPU stream action in settings", secs, cpuPath)
		return pprofActionMsg{Kind: "cpu profile", Path: cpuPath, Text: text}
	}
}

// openGoToolPprofLatestCmd launches "go tool pprof -http=ToolUIAddr <lastFile>".
// The web UI has flamegraph/top/source views without Graphviz; graph view requires dot.
func (m *InspectorModel) openGoToolPprofLatestCmd() tea.Cmd {
	profilePath := m.pprof.LastProfilePath
	uiAddr := m.pprof.ToolUIAddr
	return func() tea.Msg {
		if strings.TrimSpace(profilePath) == "" {
			return pprofActionMsg{Kind: "go tool pprof", Err: fmt.Errorf("no saved profile yet — use 'write heap snapshot' first")}
		}
		cmd := exec.Command("go", "tool", "pprof", "-http="+uiAddr, profilePath)
		if err := cmd.Start(); err != nil {
			return pprofActionMsg{Kind: "go tool pprof", Err: fmt.Errorf("go tool pprof: %w (is Go toolchain in PATH?)", err)}
		}
		url := "http://" + uiAddr
		return pprofActionMsg{Kind: "go tool pprof", Text: "go pprof UI started: " + url + " (flamegraph/top/source views need no Graphviz; graph view needs dot)"}
	}
}

// openGoToolPprofLiveHeapCmd launches "go tool pprof -http=ToolUIAddr <heapEndpoint>".
func (m *InspectorModel) openGoToolPprofLiveHeapCmd() tea.Cmd {
	serverURL := m.pprof.ServerURL
	uiAddr := m.pprof.ToolUIAddr
	return func() tea.Msg {
		if strings.TrimSpace(serverURL) == "" {
			return pprofActionMsg{Kind: "go tool pprof", Err: fmt.Errorf("pprof HTTP server is not running — enable pprof HTTP first")}
		}
		heapURL := strings.TrimRight(serverURL, "/") + "/heap"
		cmd := exec.Command("go", "tool", "pprof", "-http="+uiAddr, heapURL)
		if err := cmd.Start(); err != nil {
			return pprofActionMsg{Kind: "go tool pprof", Err: fmt.Errorf("go tool pprof: %w (is Go toolchain in PATH?)", err)}
		}
		url := "http://" + uiAddr
		return pprofActionMsg{Kind: "go tool pprof", Text: "go pprof UI started: " + url + " (source: " + heapURL + ")"}
	}
}

// openGoToolPprofLiveCPUCmd launches "go tool pprof -http=ToolUIAddr <cpuProfileEndpoint>".
// The browser will wait CPUCaptureSecs while pprof collects the CPU profile.
func (m *InspectorModel) openGoToolPprofLiveCPUCmd() tea.Cmd {
	serverURL := m.pprof.ServerURL
	uiAddr := m.pprof.ToolUIAddr
	secs := strconv.Itoa(max(1, m.pprof.CPUCaptureSecs))
	return func() tea.Msg {
		if strings.TrimSpace(serverURL) == "" {
			return pprofActionMsg{Kind: "go tool pprof", Err: fmt.Errorf("pprof HTTP server is not running — enable pprof HTTP first")}
		}
		profileURL := strings.TrimRight(serverURL, "/") + "/profile?seconds=" + secs
		cmd := exec.Command("go", "tool", "pprof", "-http="+uiAddr, profileURL)
		if err := cmd.Start(); err != nil {
			return pprofActionMsg{Kind: "go tool pprof", Err: fmt.Errorf("go tool pprof: %w (is Go toolchain in PATH?)", err)}
		}
		url := "http://" + uiAddr
		return pprofActionMsg{Kind: "go tool pprof", Text: fmt.Sprintf("go pprof UI started: %s (capturing %ss CPU profile — UI appears when done)", url, secs)}
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err != nil {
			return pprofActionMsg{Kind: "open browser", Err: err}
		}
		return pprofActionMsg{Kind: "open browser", Text: "opened browser: " + url}
	}
}

// SetStatusSummaryEnabled toggles whether StatusLineSummary returns a non-empty
// compact runtime summary (shown in the status bar when the inspector is closed).
func (m *InspectorModel) SetStatusSummaryEnabled(enabled bool) {
	m.statusSummary.Enabled = enabled
}

// StatusSummaryEnabled reports whether the compact runtime summary is enabled.
func (m *InspectorModel) StatusSummaryEnabled() bool { return m.statusSummary.Enabled }

// StatusLineSummary returns a compact runtime summary suitable for status bar display.
func (m *InspectorModel) StatusLineSummary() string {
	if !m.statusSummary.Enabled {
		return ""
	}
	st := m.stats
	pr := m.prevStats
	parts := make([]string, 0, 4)
	if m.statusSummary.ShowTerm {
		parts = append(parts, fmt.Sprintf("term %dx%d", m.Width(), m.Height()))
	}
	if m.statusSummary.ShowHeap {
		parts = append(parts, "heap "+formatBytes(st.HeapAllocBytes))
	}
	if m.statusSummary.ShowGC {
		dt := st.CapturedAt.Sub(pr.CapturedAt).Seconds()
		if dt <= 0 {
			dt = 1
		}
		gcPerSec := 0.0
		if st.NumGC >= pr.NumGC {
			gcPerSec = float64(st.NumGC-pr.NumGC) / dt
		}
		if gcPerSec < 0.1 {
			if gcPerSec == 0 {
				parts = append(parts, "gc idle")
			} else {
				parts = append(parts, fmt.Sprintf("gc %.1fs", 1.0/gcPerSec))
			}
		} else {
			parts = append(parts, fmt.Sprintf("gc %.1f/s", gcPerSec))
		}
	}
	if m.statusSummary.ShowGorout {
		parts = append(parts, fmt.Sprintf("gor %d", st.Goroutines))
	}
	return strings.Join(parts, " • ")
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

func truncate(s string, maxW int) string {
	r := []rune(s)
	if len(r) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return string(r[:maxW-1]) + "…"
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

var _ theme.ColorAware = (*InspectorModel)(nil)

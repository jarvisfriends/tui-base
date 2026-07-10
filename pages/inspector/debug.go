package inspector

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"math"
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
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
	"github.com/jarvisfriends/snap/gate"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/page"
	"github.com/jarvisfriends/tui-base/theme"
	tint "github.com/lrstanley/bubbletint/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	defaultLatestValueRenderInterval = 500 * time.Millisecond
	defaultStatsRefreshInterval      = time.Second
)

// saturatingDuration converts a uint64 nanosecond count to time.Duration,
// clamping at math.MaxInt64 to prevent overflow on pathological values.
func saturatingDuration(ns uint64) time.Duration {
	if ns > math.MaxInt64 {
		return math.MaxInt64
	}
	return time.Duration(ns)
}

// AccessibilityTabGate is the feature-gate name controlling whether the
// inspector shows its Accessibility tab. The router registers it with
// Default:false (hidden) when the app has not defined it, so the tab is
// opt-in: flip it at runtime on the settings page (Feature Flags section)
// or at startup via the <APPNAME>_GATE_INSPECTOR_ACCESSIBILITY_TAB env var.
const AccessibilityTabGate = "Inspector Accessibility Tab"

const (
	debugTabTitleAccessibility = "Accessibility"
	pprofViewModeBuiltin       = "builtin"
	settingsColMetric          = "Metric"
	settingsColValue           = "Value"
	settingsActionOpen         = "Open"
	settingsActionRun          = "Run"
	pprofKindSnapshot          = "snapshot"
	pprofKindCPUProfile        = "cpu profile"
	pprofKindGoTool            = "go tool pprof"
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

// DebugKeyMap holds key bindings for the debug inspector page. All bindings
// are exported so consumers can rebind them without forking the package.
type DebugKeyMap struct {
	Highlight     key.Binding // toggle mouse-highlight overlay
	NotifyInfo    key.Binding // fire a test info notification
	NotifyWarning key.Binding // fire a test warning notification
	NotifyError   key.Binding // fire a test error notification
	ExportLog     key.Binding // export inspector log to file
	LevelFilter   key.Binding // toggle the Log tab's WARN+ only filter

	// NextTab/PrevTab cycle the inspector tabs. The router syncs them with the
	// application's NextPage/PreviousPage bindings (SetNavKeys) so switching
	// inspector tabs feels identical to switching pages — including custom
	// rebinds. ←/→ and 1-7 keep working as inspector-local shortcuts.
	NextTab key.Binding
	PrevTab key.Binding

	// Help-only composite bindings
	TabSwitch key.Binding
	Scroll    key.Binding
	EnterRun  key.Binding
}

// DefaultDebugKeys returns the default key bindings for the debug inspector.
func DefaultDebugKeys() DebugKeyMap {
	return DebugKeyMap{
		Highlight: key.NewBinding(
			key.WithKeys("h", "H"),
			key.WithHelp("h", "highlight toggle"),
		),
		NotifyInfo: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "test info notification"),
		),
		NotifyWarning: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "test warning notification"),
		),
		NotifyError: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "test error notification"),
		),
		ExportLog:   key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "export log to file")),
		LevelFilter: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter WARN+ only")),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		TabSwitch: key.NewBinding(
			key.WithKeys("tab", "shift+tab", "left", "right", "1", "2", "3", "4", "5", "6", "7"),
			key.WithHelp("tab/←/→ 1-7", "switch tab"),
		),
		Scroll:   key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "scroll")),
		EnterRun: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "change/run")),
	}
}

// dataTableKeyMap returns the key bindings for the inspector's data tables
// (Runtime, Input, Disks). Standardized navigation keys only — no vim
// fallbacks (ADR-011), and no half-page bindings (they would shadow other
// inspector shortcuts).
func dataTableKeyMap() table.KeyMap {
	return table.KeyMap{
		LineUp:     key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		LineDown:   key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		PageUp:     key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PageDown:   key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		GotoTop:    key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "first")),
		GotoBottom: key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "last")),
	}
}

type debugTab int

const (
	debugTabRuntime debugTab = iota
	debugTabInput
	debugTabDisks
	debugTabTerminal
	debugTabAccessibility
	debugTabLog
	debugTabSettings
)

var debugTabTitles = []string{
	"Runtime",
	"Input",
	"Disks",
	"Terminal",
	debugTabTitleAccessibility,
	"Log",
	"Settings",
}

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
	settingsRowShowLink                                // 7
	settingsRowPprofEnabled                            // 8
	settingsRowPprofAddr                               // 9
	settingsRowPprofToolAddr                           // 10
	settingsRowPprofViewMode                           // 11
	settingsRowCPUSecs                                 // 12
	settingsRowOutputDir                               // 13 — read-only display
	settingsRowWriteHeap                               // 14
	settingsRowCaptureCPU                              // 15
	settingsRowBuiltinHeader                           // 16 — SectionOnly
	settingsRowPprofIndex                              // 17
	settingsRowHeapDebug1                              // 18
	settingsRowHeapDebug2                              // 19
	settingsRowGoroutineDebug1                         // 20
	settingsRowGoroutineDebug2                         // 21
	settingsRowAllocsDebug1                            // 22
	settingsRowBlockDebug1                             // 23
	settingsRowMutexDebug1                             // 24
	settingsRowCPUStream                               // 25
	settingsRowTraceStream                             // 26
	settingsRowGotoolHeader                            // 27 — SectionOnly
	settingsRowGotoolLatest                            // 28
	settingsRowGotoolLiveHeap                          // 29
	settingsRowGotoolLiveCPU                           // 30
	settingsRowServerState                             // 31 — read-only display
)

type summaryFlags struct {
	Enabled    bool
	ShowTerm   bool
	ShowHeap   bool
	ShowGC     bool
	ShowGorout bool
	// ShowLink includes the estimated remote-link Tx/Rx rates (the router
	// injects the text via SetLinkRateSummary; collection follows demand).
	ShowLink bool
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
	// logWarnPlus filters the Log tab to WARN+ entries only when true (I-6).
	// Intercepted (non-level) messages are hidden while it is active.
	logWarnPlus bool
	// Accessibility panel — shown when its tab is active
	acPanel *AccessibilityPanel
	// gates controls visibility of gated tabs (currently the Accessibility
	// tab via AccessibilityTabGate). Nil means "no gates": gated tabs stay
	// hidden, matching the gate's Default:false registration.
	gates *gate.GateRegistry
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
	linkSummary     func() string
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

	// tableActive records, per tab, whether the last render used the tab's
	// bubbles table (vs. the flat narrow-terminal fallback). Navigation keys
	// go to the table's row cursor only when it is actually on screen.
	tableActive map[debugTab]bool

	// providers are the registered custom tabs (E-5), rendered after the
	// built-ins; providerRefreshed tracks each provider's last render time so
	// RefreshInterval can force re-renders from the stats tick.
	providers         []MetricsProvider
	providerRefreshed map[string]time.Time

	// logCapacity overrides the message-log ring size; 0 = default (I-7).
	logCapacity int

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

// ToggleVisible shows/hides the inspector overlay. Registered tab providers
// run only while the inspector is on screen (Q-8: startable and stoppable).
func (m *InspectorModel) ToggleVisible() {
	m.visible = !m.visible
	for _, p := range m.providers {
		if m.visible {
			p.Start()
		} else {
			p.Stop()
		}
	}
}

// ShortHelp implements [help.KeyMap]. Returns a compact list of bindings for
// the current tab shown in the status bar one-liner.
func (m *InspectorModel) ShortHelp() []key.Binding {
	switch m.activeTab {
	case debugTabSettings:
		return []key.Binding{m.keys.TabSwitch, m.keys.Scroll, m.keys.EnterRun}
	case debugTabLog:
		return []key.Binding{
			m.keys.TabSwitch, m.keys.Scroll, m.keys.LevelFilter,
			m.keys.NotifyInfo, m.keys.NotifyWarning, m.keys.NotifyError, m.keys.ExportLog,
		}
	case debugTabRuntime, debugTabInput, debugTabDisks, debugTabTerminal, debugTabAccessibility:
		return []key.Binding{m.keys.TabSwitch, m.keys.Scroll, m.keys.Highlight}
	}
	return []key.Binding{m.keys.TabSwitch, m.keys.Scroll, m.keys.Highlight}
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

// SetNavKeys aligns the inspector's tab cycling with the application's page
// navigation bindings (typically AppKeyMap.NextPage/PreviousPage). The router
// calls this at startup and again whenever the user rebinds keys, so the
// inspector always honors the same next/previous keys as the main navigation.
func (m *InspectorModel) SetNavKeys(next, prev key.Binding) {
	m.keys.NextTab = key.NewBinding(
		key.WithKeys(next.Keys()...),
		key.WithHelp(next.Help().Key, "next tab"),
	)
	m.keys.PrevTab = key.NewBinding(
		key.WithKeys(prev.Keys()...),
		key.WithHelp(prev.Help().Key, "prev tab"),
	)
	tabKeys := append([]string{}, next.Keys()...)
	tabKeys = append(tabKeys, prev.Keys()...)
	tabKeys = append(tabKeys, "left", "right", "1", "2", "3", "4", "5", "6", "7")
	m.keys.TabSwitch = key.NewBinding(
		key.WithKeys(tabKeys...),
		key.WithHelp(next.Help().Key+"/←/→ 1-7", "switch tab"),
	)
}

// SetGates hands the inspector the app's feature-gate registry so gated tabs
// (the Accessibility tab) can show or hide live. The router calls this once at
// startup; the registry pointer is shared, so later gate flips are visible
// immediately without re-wiring.
func (m *InspectorModel) SetGates(g *gate.GateRegistry) {
	m.gates = g
	m.OnGatesChanged()
}

// OnGatesChanged re-derives gate-dependent state after a feature gate flips at
// runtime (the router calls it on settings.GatesChangedMsg). If the active tab
// just became hidden, the inspector snaps to the Runtime tab so the user is
// never left on an invisible tab.
func (m *InspectorModel) OnGatesChanged() {
	if !m.tabVisible(m.activeTab) {
		m.saveActiveTabScroll()
		m.activeTab = debugTabRuntime
		m.syncAcPanelVisibility()
		m.restoreActiveTabScroll()
	}
	m.dirty = true
}

// activeDataTable returns the bubbles table backing the active tab, or nil
// when the active tab is not table-based or its last render fell back to the
// flat layout (narrow terminals), in which case keys scroll the viewport.
func (m *InspectorModel) activeDataTable() *table.Model {
	if !m.tableActive[m.activeTab] {
		return nil
	}
	switch m.activeTab { //nolint:exhaustive // non-table tabs fall through to nil
	case debugTabRuntime:
		return &m.runtimeTbl
	case debugTabInput:
		return &m.inputDbgTbl
	case debugTabDisks:
		return &m.diskTbl
	default:
		return nil
	}
}

// setTableActive records whether a tab's last render used its bubbles table
// (vs. the flat fallback), which decides where navigation keys are routed.
func (m *InspectorModel) setTableActive(tab debugTab, active bool) {
	if m.tableActive == nil {
		m.tableActive = make(map[debugTab]bool)
	}
	m.tableActive[tab] = active
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
	if tab < 0 || int(tab) >= m.tabCount() || tab == m.activeTab || !m.tabVisible(tab) {
		return
	}
	m.saveActiveTabScroll()
	m.activeTab = tab
	// Ensure Accessibility panel visibility follows the active tab.
	if m.acPanel != nil {
		m.syncAcPanelVisibility()
	}
	m.restoreActiveTabScroll()
	m.dirty = true
}

func (m *InspectorModel) syncAcPanelVisibility() {
	want := m.activeTab == debugTabAccessibility
	if want != m.acPanel.IsVisible() {
		m.acPanel.Toggle()
	}
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
			ShowLink:   true,
		},
		pprof: pprofConfig{
			Enabled:        false,
			Addr:           pprofDefaultAddr,
			ToolUIAddr:     pprofDefaultToolUI,
			ViewMode:       pprofViewModeBuiltin,
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
			m.runtimeColumns[i] = table.Column{Title: settingsColMetric, Width: -1}
		} else {
			m.runtimeColumns[i] = table.Column{Title: settingsColValue, Width: -1}
		}
	}

	m.inputDbgColumns = make([]table.Column, 6)
	for i := range m.inputDbgColumns {
		if i%2 == 0 {
			m.inputDbgColumns[i] = table.Column{Title: settingsColMetric, Width: -1}
		} else {
			m.inputDbgColumns[i] = table.Column{Title: settingsColValue, Width: -1}
		}
	}
	m.inputDbgColMaxW = make([]int, 6)

	m.inputDbgTbl = table.New(
		table.WithColumns(m.inputDbgColumns),
		table.WithRows(nil),
		table.WithFocused(true),
		table.WithHeight(2),
		table.WithWidth(60),
		table.WithKeyMap(dataTableKeyMap()),
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
		table.WithFocused(true),
		table.WithHeight(2),
		table.WithWidth(80),
		table.WithKeyMap(dataTableKeyMap()),
	)
	m.diskTbl = table.New(
		table.WithColumns(m.diskHeader),
		table.WithRows(nil),
		table.WithFocused(true),
		table.WithHeight(2),
		table.WithWidth(80),
		table.WithKeyMap(dataTableKeyMap()),
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
		switch {
		case msg.Err != nil:
			m.settingsMessage = msg.Kind + " failed: " + msg.Err.Error()
		case msg.Text != "":
			m.settingsMessage = msg.Text
		default:
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
		// When the accessibility tab is active, let the panel handle keys —
		// except the tab-switching keys (next/prev nav plus left/right), which
		// always switch tabs so the user can never get stuck on this tab.
		// (The panel claims 1/2/3 for CVD filters, so these are the reliable
		// way out.)
		if m.activeTab == debugTabAccessibility && m.acPanel != nil {
			press, isPress := msg.(tea.KeyPressMsg)
			if !isPress || (press.Code != tea.KeyLeft && press.Code != tea.KeyRight &&
				!key.Matches(press, m.keys.NextTab) && !key.Matches(press, m.keys.PrevTab)) {
				model, cmd := m.acPanel.Update(msg)
				if p, ok := model.(*AccessibilityPanel); ok {
					m.acPanel = p
				}
				return m, tea.Batch(preCmd, cmd)
			}
		}
		switch km := msg.(type) {
		case tea.KeyPressMsg:
			switch {
			case m.activeTab == debugTabSettings:
				// Settings tab: Up/Down navigate rows, Enter toggles/runs.
				// Left/Right fall through so they always switch tabs.
				switch km.Code {
				case tea.KeyUp, tea.KeyDown, tea.KeyEnter:
					return m, tea.Batch(preCmd, m.handleSettingsKey(km))
				}
			case m.activeDataTable() != nil:
				// Table tabs (Runtime/Input/Disks): navigation keys move the
				// table's row cursor; the table scrolls internally when its
				// rows exceed the section height.
				switch km.Code {
				case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
					tbl := m.activeDataTable()
					*tbl, _ = tbl.Update(km)
					m.dirty = true
					return m, preCmd
				}
			default:
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
			if km.Code == tea.KeyLeft || key.Matches(km, m.keys.PrevTab) {
				m.stepTab(-1)
				return m, preCmd
			}
			if km.Code == tea.KeyRight || key.Matches(km, m.keys.NextTab) {
				m.stepTab(1)
				return m, preCmd
			}
			if km.Text >= "1" && km.Text <= "9" {
				// Digits address the tabs as displayed (hidden tabs don't
				// consume a number), matching the numbers on the tab bar.
				if vis := m.visibleTabs(); int(km.Text[0]-'1') < len(vis) {
					m.switchTab(vis[km.Text[0]-'1'])
				}
				return m, preCmd
			}
			switch {
			// Accessibility key removed; no-op
			case key.Matches(km, m.keys.Highlight):
				m.ShowHighlight = !m.ShowHighlight
				m.dirty = true
				return m, preCmd
			case key.Matches(km, m.keys.NotifyInfo):
				return m, tea.Batch(preCmd, func() tea.Msg {
					return notifications.AddMsg{
						Content:  "Test: Info notification from Inspector",
						Severity: notifications.SeverityInfo,
						TTL:      notifications.SeverityInfo.DefaultTTL(),
					}
				})
			case key.Matches(km, m.keys.NotifyWarning):
				return m, tea.Batch(preCmd, func() tea.Msg {
					return notifications.AddMsg{
						Content:  "Test: Warning notification from Inspector",
						Severity: notifications.SeverityWarning,
						TTL:      notifications.SeverityWarning.DefaultTTL(),
					}
				})
			case key.Matches(km, m.keys.NotifyError):
				return m, tea.Batch(preCmd, func() tea.Msg {
					return notifications.AddMsg{
						Content:  "Test: Error notification from Inspector",
						Severity: notifications.SeverityError,
						TTL:      notifications.SeverityError.DefaultTTL(),
					}
				})
			case key.Matches(km, m.keys.ExportLog):
				return m, tea.Batch(preCmd, m.exportLogCmd())
			case key.Matches(km, m.keys.LevelFilter):
				m.logWarnPlus = !m.logWarnPlus
				m.scrollToBottom = true
				m.dirty = true
				return m, preCmd
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
		m.tickProviderRefresh()
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
			if msgContent != "" {
				msgContent += "\n  "
			}
			if pair := strings.SplitN(kv, "=", 2); len(pair) == 2 {
				msgContent += fmt.Sprintf("Key: %s  Value: %s", pair[0], pair[1])
			} else {
				msgContent += "Env: " + kv
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

	m.trimLogs()
	m.scrollToBottom = true
	return nil
}

// logCapacityDefault is the message-log ring size when SetLogCapacity was
// never called.
const logCapacityDefault = 50

// SetLogCapacity sets how many deduplicated entries the inspector's message
// log retains (I-7). Values below 10 are clamped to 10; 0 restores the
// default.
func (m *InspectorModel) SetLogCapacity(n int) {
	switch {
	case n == 0:
		m.logCapacity = 0
	case n < 10:
		m.logCapacity = 10
	default:
		m.logCapacity = n
	}
	m.trimLogs()
}

// trimLogs drops the oldest entries beyond the configured capacity.
func (m *InspectorModel) trimLogs() {
	limit := m.logCapacity
	if limit <= 0 {
		limit = logCapacityDefault
	}
	if len(m.Logs) > limit {
		m.Logs = m.Logs[len(m.Logs)-limit:]
	}
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
	m.trimLogs()
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
		PauseTotal:      saturatingDuration(ms.PauseTotalNs),
		GcCPUFraction:   ms.GCCPUFraction,
		Disks:           cachedDriveStats(now),
		Launch:          launchOnce,
	}
	if ms.NumGC > 0 {
		snap.LastPause = saturatingDuration(ms.PauseNs[(ms.NumGC+255)%256])
	}
	return snap
}

func (m *InspectorModel) View() tea.View {
	if !m.dirty {
		return m.view
	}

	c := m.Colors()
	availW := max(m.Width()-4, 20)
	runtimeRows := m.buildRuntimeRows(c)
	inputRows := m.buildInputRows(c)
	m.updateRuntimeColumnWidths(runtimeRows)
	m.updateInputColumnWidths(inputRows)
	tblStyles := m.baseTableStyles(c)
	runtimeSection := m.renderRuntimeSection(c, tblStyles, runtimeRows, availW)
	logContent := m.renderLogContent(c)
	rawTabsLine := m.buildTabsLine(c)
	sectionTitle, sectionContent := m.sectionForActiveTab(
		c,
		availW,
		tblStyles,
		runtimeSection,
		inputRows,
		logContent,
	)

	titleText := sectionTitle + " (Inspector)"
	titleLine := lipgloss.PlaceHorizontal(
		availW,
		lipgloss.Center,
		c.Styles.Title.Bold(true).Render(titleText),
	)
	sep := c.Styles.Title.Render(strings.Repeat("─", availW))
	tabsLine := ansi.Truncate(rawTabsLine, availW, "…")

	topH := lipgloss.Height(titleLine) + lipgloss.Height(tabsLine) + lipgloss.Height(sep)
	m.sectionOriginX = 0
	// tabsOriginY: the inner content layout is titleLine → sep → tabsLine, so
	// tabs start after both title AND separator lines.
	m.tabsOriginY = lipgloss.Height(titleLine) + lipgloss.Height(sep)
	m.tabsHeight = lipgloss.Height(tabsLine)
	m.sectionOriginY = topH
	m.sectionHeight = max(1, m.Height()-topH)
	m.logViewport.SetWidth(max(availW, 1))
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
		m.sectionViewport.SetWidth(max(availW, 1))
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
			m.handleWheel(wheel.Mouse())
			return nil
		}
		if rel, ok := mm.(tea.MouseReleaseMsg); ok && rel.Mouse().Button == tea.MouseLeft {
			my := rel.Mouse().Y
			if my >= m.tabsOriginY && my < m.tabsOriginY+m.tabsHeight {
				m.selectTabByX(rel.Mouse().X)
				return func() tea.Msg { return mm }
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

// handleWheel routes a mouse-wheel event: horizontal wheel (tilt, or the
// shift+wheel chord terminals emit for horizontal scrolling) switches tabs
// like ←/→; a vertical wheel moves the table row cursor on table tabs and
// scrolls the section viewport elsewhere.
func (m *InspectorModel) handleWheel(me tea.Mouse) {
	shifted := me.Mod&tea.ModShift != 0
	switch {
	case me.Button == tea.MouseWheelLeft || (shifted && me.Button == tea.MouseWheelUp):
		m.stepTab(-1)
		return
	case me.Button == tea.MouseWheelRight || (shifted && me.Button == tea.MouseWheelDown):
		m.stepTab(1)
		return
	}
	if tbl := m.activeDataTable(); tbl != nil {
		if me.Button == tea.MouseWheelUp {
			tbl.MoveUp(1)
		} else {
			tbl.MoveDown(1)
		}
		m.dirty = true
		return
	}
	if me.Button == tea.MouseWheelUp {
		m.scrollActiveSection(-3)
	} else {
		m.scrollActiveSection(3)
	}
}

// termRow is one aligned key/value line in the Terminal & Theme tab; warn rows
// render the value in the theme's warning style.
type termRow struct {
	k, v string
	warn bool
}

// termSection is a titled group of rows in the Terminal & Theme tab.
type termSection struct {
	title string
	rows  []termRow
}

// buildTermSection renders the terminal environment and active theme
// diagnostics as titled category sections with a single aligned key column —
// the same header convention as the settings overview.
func (m *InspectorModel) buildTermSection(c *theme.AppStyle, width int) string {
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
	if m.termDiagSet && m.termDiag != nil {
		bgDetStr = theme.ColorHex(m.termDiag.DetectedBg)
		isDarkStr = strconv.FormatBool(m.termDiag.BgIsDark)
		bgSwatch := lipgloss.NewStyle().
			Background(m.termDiag.DetectedBg).
			Foreground(m.termDiag.DetectedBg).
			Render("■■")
		bgDetStr = bgSwatch + " " + bgDetStr
	}

	// Active tint key colors
	var activeTint *tint.Tint
	func() {
		// tint.Current() and tint.Tints() both panic before the registry is
		// initialized; guard with recover so the inspector still renders.
		defer func() {
			if r := recover(); r != nil {
				activeTint = nil // registry not ready; display "(none)"
			}
		}()
		activeTint = tint.Current()
	}()
	tintID := "(none)"
	bgHex, fgHex, accentHex, selBgHex := "?", "?", "?", "?"
	if activeTint != nil {
		tintID = activeTint.ID
		if activeTint.Bg != nil {
			col := activeTint.Bg
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
			col := activeTint.SelectionBg
			selBgHex = activeTint.SelectionBg.Hex()
			sw := lipgloss.NewStyle().Background(col).Foreground(col).Render("■")
			selBgHex = sw + " " + selBgHex
		}
	}

	colorRows := []termRow{
		{k: "Color Profile", v: profStr},
		{k: "Dark Background", v: isDarkStr},
		{k: "OSC11 Bg", v: bgDetStr},
	}
	// Remedy hint: colors look washed-out / wrong over SSH when the profile
	// downsamples 24-bit theme colors. Surface the fix when we detect the
	// classic signature (ANSI256 + SSH + no override + COLORTERM unset).
	if prof == colorprofile.ANSI256 && isSSH && profileOverride == "" && colorterm == "(not set)" {
		colorRows = append(
			colorRows,
			termRow{
				k:    "Color hint",
				v:    "24-bit colors quantized to ANSI256 (COLORTERM not forwarded over SSH)",
				warn: true,
			},
			termRow{
				k:    "Fix",
				v:    "set COLORTERM=truecolor on the remote, or run with " + envName + "=truecolor",
				warn: true,
			},
		)
	}

	st := m.stats
	sections := []termSection{
		{title: "Terminal Environment", rows: []termRow{
			{k: "TERM", v: termEnv},
			{k: "COLORTERM", v: colorterm},
			{k: "TERM_PROGRAM", v: termProg},
			{k: "SSH", v: sshStr, warn: isSSH},
		}},
		{title: "Colors & Profile", rows: colorRows},
		{title: "Active Theme", rows: []termRow{
			{k: "Tint", v: tintID},
			{k: "Background", v: bgHex},
			{k: "Foreground", v: fgHex},
			{k: "Accent", v: accentHex},
			{k: "Selection Bg", v: selBgHex},
		}},
		{title: "Process & Launch", rows: []termRow{
			{k: "Executable", v: st.Launch.Executable},
			{k: "Args", v: strings.Join(st.Launch.Args, " ")},
			{k: "Working Dir", v: st.Launch.WorkDir},
			{k: "User@Host", v: st.Launch.Username + "@" + st.Launch.Hostname},
		}},
	}

	// One aligned key column across all sections keeps values in a straight
	// vertical line down the whole tab.
	keyW := 0
	for _, sec := range sections {
		for _, r := range sec.rows {
			keyW = max(keyW, lipgloss.Width(r.k))
		}
	}

	headerStyle := c.Styles.Subtitle.Bold(true)
	keyStyle := c.Styles.Item.Width(keyW)
	warnKeyStyle := c.Styles.Dim.Width(keyW)
	valStyle := c.Styles.TextOnBg
	warnValStyle := c.Styles.Warning

	var out []string
	for i, sec := range sections {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, headerStyle.Render(sec.title))
		for _, r := range sec.rows {
			if r.warn {
				out = append(out, "  "+warnKeyStyle.Render(r.k)+"  "+warnValStyle.Render(r.v))
				continue
			}
			out = append(out, "  "+keyStyle.Render(r.k)+"  "+valStyle.Render(r.v))
		}
	}

	joined := strings.Join(out, "\n")
	return c.Styles.TextOnBg.Width(width).Render(joined)
}

func (m *InspectorModel) renderSettingsSection(c *theme.AppStyle) string {
	items := m.settingsRows()
	availW := max(m.Width()-4, 28)
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
	// Use the theme's semantic selection colors (SelectionBg/Fg), matching the
	// table, sidebar, and settings page, rather than the tab-hover affordance.
	// Every segment of the selected row — indicator, field, separator, value —
	// carries the selection background (the same technique as the main
	// settings page's renderOverview) so the highlight is one continuous bar
	// with no unstyled gaps between the columns.
	selectedField := c.Styles.SelectedItem.Width(fieldW)
	selectedValue := c.Styles.TextOnBg.Foreground(c.SelectionFg).
		Background(c.SelectionBg).
		Width(valueW)
	selectedRow := c.Styles.Row.Background(c.SelectionBg).Width(availW)
	indicatorStyle := lipgloss.NewStyle().Foreground(c.SelectionFg).Background(c.SelectionBg)
	spaceStyle := lipgloss.NewStyle().Background(c.SelectionBg)

	// Category headers use the same style as the Terminal tab's sections (and
	// the settings page's category headers).
	sectionHeader := c.Styles.Subtitle.Bold(true)

	var out []string
	for i, row := range items {
		if row.SectionOnly {
			out = append(out, sectionHeader.Render(row.Field))
			continue
		}
		field := ansi.Truncate(row.Field, fieldW, "…")
		value := ansi.Truncate(row.Value, valueW, "…")
		if i == m.settingsCursor {
			prefix := "▶ "
			if row.ActionOnly {
				prefix = "↵ "
			}
			line := indicatorStyle.Render(prefix) + selectedField.Render(field) +
				spaceStyle.Render("   ") + selectedValue.Render(value)
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
		{
			Field: "Latest-value refresh",
			Value: fmt.Sprintf("%dms", m.latestValueInterval/time.Millisecond),
			Help:  "Enter increases cadence by 100 ms (mouse/key telemetry redraw interval)",
		},
		{
			Field: "Runtime tick refresh",
			Value: fmt.Sprintf("%dms", m.statsRefreshInterval/time.Millisecond),
			Help:  "Enter increases cadence by 100 ms (runtime snapshot update interval)",
		},
		{
			Field: "Status summary on close",
			Value: strconv.FormatBool(m.statusSummary.Enabled),
			Help:  "Enter toggles compact runtime summary in status bar when inspector is closed",
		},
		{
			Field: "Include terminal size",
			Value: strconv.FormatBool(m.statusSummary.ShowTerm),
			Help:  "Enter toggles terminal dimensions in the status summary",
		},
		{
			Field: "Include heap size",
			Value: strconv.FormatBool(m.statusSummary.ShowHeap),
			Help:  "Enter toggles live heap allocation bytes in the status summary",
		},
		{
			Field: "Include GC/sec",
			Value: strconv.FormatBool(m.statusSummary.ShowGC),
			Help:  "Enter toggles GC cadence in the status summary",
		},
		{
			Field: "Include goroutines",
			Value: strconv.FormatBool(m.statusSummary.ShowGorout),
			Help:  "Enter toggles goroutine count in the status summary",
		},
		{
			Field: "Include link rate",
			Value: strconv.FormatBool(m.statusSummary.ShowLink),
			Help:  "Enter toggles estimated remote Tx/Rx rates in the status summary (keeps the link meter collecting while the inspector is closed)",
		},
		// 8-13: pprof server config
		{
			Field: "Enable profiler HTTP server",
			Value: pprofState,
			Help:  "Enter toggles Go's built-in pprof server. Required for all browser viewer endpoints below. Profiles measure CPU, memory, goroutines, and GC.",
		},
		{
			Field: "Profiler listen addr",
			Value: m.pprof.Addr,
			Help: fmt.Sprintf(
				"Enter cycles between %s and %s. Restart server to apply.",
				pprofDefaultAddr,
				pprofAltAddr,
			),
		},
		{
			Field: "go tool pprof UI addr",
			Value: m.pprof.ToolUIAddr,
			Help: fmt.Sprintf(
				"Enter cycles between %s and %s (address used when launching go tool pprof -http).",
				pprofDefaultToolUI,
				pprofAltToolUI,
			),
		},
		{
			Field: "Pprof view mode",
			Value: m.pprof.ViewMode,
			Help:  "Enter cycles: builtin (browser, no deps) → go-tool (go tool pprof -http, needs Go in PATH) → graphviz (graph view, also needs 'dot')",
		},
		{
			Field: "CPU capture duration (secs)",
			Value: secs,
			Help:  "Enter increments by 1 s. Used by 'Capture CPU profile' and the CPU stream browser endpoint.",
		},
		{
			Field: "Output dir",
			Value: m.pprof.OutputDir,
			Help:  "Heap/CPU snapshot files are written here. Analyze with: go tool pprof <file>",
		},
		// 13-14: capture actions (no server required)
		{
			Field:      "Write heap snapshot",
			Value:      "Enter to run",
			ActionOnly: true,
			Help:       "Saves a heap profile (.pprof) to Output dir right now. No server required. Useful for offline analysis.",
		},
		{
			Field:      "Capture CPU profile",
			Value:      fmt.Sprintf("Enter to run (%ss)", secs),
			ActionOnly: true,
			Help:       "Records CPU samples for the configured duration. Blocks the UI during capture. Analyze with: go tool pprof <file>",
		},
		// 15: section header
		{Field: "── Browser viewer (profiler HTTP must be enabled above) ──", SectionOnly: true},
		// 16-25: built-in browser endpoints (no external dependencies)
		{
			Field:      "Profile index page",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "All available profile types as a human-readable HTML index.",
		},
		{
			Field:      "Heap — allocation counts (debug=1)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "Live heap: per-allocation stack traces. Best starting point for finding memory leaks.",
		},
		{
			Field:      "Heap — all live objects (debug=2)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "Every live heap object with full stacks. Very verbose; use debug=1 first.",
		},
		{
			Field:      "Goroutines — deduplicated (debug=1)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "One line per unique goroutine stack. Easy to scan; good for spotting goroutine leaks.",
		},
		{
			Field:      "Goroutines — all stacks (debug=2)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "Every running goroutine with its full stack. Shows exact counts; confirms goroutine leaks.",
		},
		{
			Field:      "Allocations profile (debug=1)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "Heap allocations since program start (includes freed objects). Great for allocation hot-path analysis.",
		},
		{
			Field:      "Block profile (debug=1)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "Where goroutines block on synchronization (channels, mutexes). Needs runtime.SetBlockProfileRate.",
		},
		{
			Field:      "Mutex profile (debug=1)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "Mutex contention report. Needs runtime.SetMutexProfileFraction.",
		},
		{
			Field:      "CPU profile stream",
			Value:      fmt.Sprintf("Open (%ss)", secs),
			ActionOnly: true,
			Help:       "Browser downloads an N-second CPU profile. Analyze offline with: go tool pprof <file>",
		},
		{
			Field:      "Execution trace stream (5s)",
			Value:      settingsActionOpen,
			ActionOnly: true,
			Help:       "Browser downloads a 5-second execution trace. View with: go tool trace <file>",
		},
		// 26: section header
		{
			Field:       "── go tool pprof -http (Go in PATH required; graph view needs Graphviz 'dot') ──",
			SectionOnly: true,
		},
		// 27-29: go tool pprof -http (needs Go toolchain; graph view needs Graphviz)
		{
			Field:      "Open pprof UI — last saved file",
			Value:      settingsActionRun,
			ActionOnly: true,
			Help:       "Launches 'go tool pprof -http' for the most recently saved .pprof file. Flame/top/source views need no Graphviz; graph view does.",
		},
		{
			Field:      "Open pprof UI — live heap",
			Value:      settingsActionRun,
			ActionOnly: true,
			Help:       "Launches 'go tool pprof -http' fetching live heap from the profiler server (must be enabled above).",
		},
		{
			Field:      "Open pprof UI — live CPU",
			Value:      settingsActionRun,
			ActionOnly: true,
			Help: fmt.Sprintf(
				"Launches 'go tool pprof -http' capturing %s seconds of live CPU (UI appears after capture finishes).",
				secs,
			),
		},
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
		m.latestValueInterval = time.Duration(
			max(100, int((m.latestValueInterval+100*time.Millisecond)/time.Millisecond)),
		) * time.Millisecond
	case settingsRowStatsRefresh:
		m.statsRefreshInterval = time.Duration(
			max(200, int((m.statsRefreshInterval+100*time.Millisecond)/time.Millisecond)),
		) * time.Millisecond
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
	case settingsRowShowLink:
		m.statusSummary.ShowLink = !m.statusSummary.ShowLink
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
		case pprofViewModeBuiltin:
			m.pprof.ViewMode = "go-tool"
		case "go-tool":
			m.pprof.ViewMode = "graphviz"
		default:
			m.pprof.ViewMode = pprofViewModeBuiltin
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
	case settingsRowOutputDir,
		settingsRowBuiltinHeader,
		settingsRowGotoolHeader,
		settingsRowServerState:
		// read-only display rows and section headers — no interactive action
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
		srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() { _ = srv.Serve(ln) }()
		return pprofServerStartedMsg{
			Server: srv,
			URL:    "http://" + ln.Addr().String() + "/debug/pprof/",
		}
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
		if err := os.MkdirAll(outDir, 0o750); err != nil {
			return pprofActionMsg{Kind: pprofKindSnapshot, Err: err}
		}
		ts := time.Now().Format("20060102-150405")
		heapPath := filepath.Join(outDir, "heap-"+ts+".pprof")
		f, err := os.Create(filepath.Clean(heapPath))
		if err != nil {
			return pprofActionMsg{Kind: pprofKindSnapshot, Err: err}
		}
		runtime.GC()
		err = runtimepprof.WriteHeapProfile(f)
		_ = f.Close()
		if err != nil {
			return pprofActionMsg{Kind: pprofKindSnapshot, Err: err}
		}
		text := "heap snapshot saved: " + heapPath + " | open via built-in browser endpoints or go tool pprof UI actions in settings"
		return pprofActionMsg{Kind: pprofKindSnapshot, Path: heapPath, Text: text}
	}
}

func (m *InspectorModel) captureCPUProfileCmd() tea.Cmd {
	outDir := m.pprof.OutputDir
	secs := m.pprof.CPUCaptureSecs
	return func() tea.Msg {
		if err := os.MkdirAll(outDir, 0o750); err != nil {
			return pprofActionMsg{Kind: pprofKindCPUProfile, Err: err}
		}
		ts := time.Now().Format("20060102-150405")
		cpuPath := filepath.Join(outDir, "cpu-"+ts+".pprof")
		f, err := os.Create(filepath.Clean(cpuPath))
		if err != nil {
			return pprofActionMsg{Kind: pprofKindCPUProfile, Err: err}
		}
		if err := runtimepprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return pprofActionMsg{Kind: pprofKindCPUProfile, Err: err}
		}
		time.Sleep(time.Duration(secs) * time.Second)
		runtimepprof.StopCPUProfile()
		_ = f.Close()
		text := fmt.Sprintf(
			"cpu profile (%ds) saved: %s | open via go tool pprof UI or CPU stream action in settings",
			secs,
			cpuPath,
		)
		return pprofActionMsg{Kind: pprofKindCPUProfile, Path: cpuPath, Text: text}
	}
}

// openGoToolPprofLatestCmd launches "go tool pprof -http=ToolUIAddr <lastFile>".
// The web UI has flamegraph/top/source views without Graphviz; graph view requires dot.
func (m *InspectorModel) openGoToolPprofLatestCmd() tea.Cmd {
	profilePath := m.pprof.LastProfilePath
	uiAddr := m.pprof.ToolUIAddr
	return func() tea.Msg {
		if strings.TrimSpace(profilePath) == "" {
			return pprofActionMsg{
				Kind: pprofKindGoTool,
				Err:  errors.New("no saved profile yet — use 'write heap snapshot' first"),
			}
		}
		cmd := exec.Command("go", "tool", "pprof", "-http="+uiAddr, profilePath)
		if err := cmd.Start(); err != nil {
			return pprofActionMsg{
				Kind: pprofKindGoTool,
				Err:  fmt.Errorf("go tool pprof: %w (is Go toolchain in PATH?)", err),
			}
		}
		url := "http://" + uiAddr
		return pprofActionMsg{
			Kind: pprofKindGoTool,
			Text: "go pprof UI started: " + url + " (flamegraph/top/source views need no Graphviz; graph view needs dot)",
		}
	}
}

// openGoToolPprofLiveHeapCmd launches "go tool pprof -http=ToolUIAddr <heapEndpoint>".
func (m *InspectorModel) openGoToolPprofLiveHeapCmd() tea.Cmd {
	serverURL := m.pprof.ServerURL
	uiAddr := m.pprof.ToolUIAddr
	return func() tea.Msg {
		if strings.TrimSpace(serverURL) == "" {
			return pprofActionMsg{
				Kind: pprofKindGoTool,
				Err:  errors.New("pprof HTTP server is not running — enable pprof HTTP first"),
			}
		}
		heapURL := strings.TrimRight(serverURL, "/") + "/heap"
		cmd := exec.Command("go", "tool", "pprof", "-http="+uiAddr, heapURL)
		if err := cmd.Start(); err != nil {
			return pprofActionMsg{
				Kind: pprofKindGoTool,
				Err:  fmt.Errorf("go tool pprof: %w (is Go toolchain in PATH?)", err),
			}
		}
		url := "http://" + uiAddr
		return pprofActionMsg{
			Kind: pprofKindGoTool,
			Text: "go pprof UI started: " + url + " (source: " + heapURL + ")",
		}
	}
}

// exportLogCmd writes a snapshot of the current inspector log to a temporary file
// and triggers an info notification with the resulting path.
func (m *InspectorModel) exportLogCmd() tea.Cmd {
	logs := make([]MsgLog, len(m.Logs))
	copy(logs, m.Logs)

	return func() tea.Msg {
		outDir := filepath.Join(os.TempDir(), "tui-base", "logs")
		if err := os.MkdirAll(outDir, 0o750); err != nil {
			return notifications.AddMsg{
				Content:  "Failed to create log dir: " + err.Error(),
				Severity: notifications.SeverityError,
				TTL:      notifications.SeverityError.DefaultTTL(),
			}
		}

		ts := time.Now().Format("20060102-150405")
		logPath := filepath.Join(outDir, "inspector-"+ts+".log")
		f, err := os.Create(filepath.Clean(logPath))
		if err != nil {
			return notifications.AddMsg{
				Content:  "Failed to create log file: " + err.Error(),
				Severity: notifications.SeverityError,
				TTL:      notifications.SeverityError.DefaultTTL(),
			}
		}
		defer func() { _ = f.Close() }()

		for _, l := range logs {
			_, _ = fmt.Fprintf(
				f,
				"[%s] %s: %s (count: %d)\n",
				l.Timestamp.Format(time.RFC3339),
				l.Type,
				l.Content,
				l.Count,
			)
		}

		return notifications.AddMsg{
			Content:  "Inspector log exported to " + logPath,
			Severity: notifications.SeverityInfo,
			TTL:      notifications.SeverityInfo.DefaultTTL(),
		}
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
			return pprofActionMsg{
				Kind: pprofKindGoTool,
				Err:  errors.New("pprof HTTP server is not running — enable pprof HTTP first"),
			}
		}
		profileURL := strings.TrimRight(serverURL, "/") + "/profile?seconds=" + secs
		cmd := exec.Command("go", "tool", "pprof", "-http="+uiAddr, profileURL)
		if err := cmd.Start(); err != nil {
			return pprofActionMsg{
				Kind: pprofKindGoTool,
				Err:  fmt.Errorf("go tool pprof: %w (is Go toolchain in PATH?)", err),
			}
		}
		url := "http://" + uiAddr
		return pprofActionMsg{
			Kind: pprofKindGoTool,
			Text: fmt.Sprintf(
				"go pprof UI started: %s (capturing %ss CPU profile — UI appears when done)",
				url,
				secs,
			),
		}
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

// SetLinkRateSummary injects the compact link-rate text (e.g. "tx 12 B/s rx
// 1.2 KiB/s") shown in the status summary when "Include link rate" is on.
// The router installs this; nil removes the part.
func (m *InspectorModel) SetLinkRateSummary(fn func() string) {
	m.linkSummary = fn
}

// StatusSummaryLinkEnabled reports whether the status summary wants link-rate
// text right now — the router uses it to keep the link meter collecting while
// the inspector is closed.
func (m *InspectorModel) StatusSummaryLinkEnabled() bool {
	return m.statusSummary.Enabled && m.statusSummary.ShowLink
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
		parts = append(parts, "heap "+humanize.IBytes(st.HeapAllocBytes))
	}
	if m.statusSummary.ShowGC {
		parts = append(parts, gcSummary(st, pr))
	}
	if m.statusSummary.ShowGorout {
		parts = append(parts, fmt.Sprintf("gor %d", st.Goroutines))
	}
	if m.statusSummary.ShowLink && m.linkSummary != nil {
		if link := m.linkSummary(); link != "" {
			parts = append(parts, link)
		}
	}
	return strings.Join(parts, " • ")
}

func gcSummary(st, pr runtimeStatsSnapshot) string {
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
			return "gc idle"
		}
		return fmt.Sprintf("gc %.1fs", 1.0/gcPerSec)
	}
	return fmt.Sprintf("gc %.1f/s", gcPerSec)
}

func getEnvOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
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

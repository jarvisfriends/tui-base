package router

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/colorprofile"
	cfg "github.com/jarvisfriends/tui-base/config"
	"github.com/jarvisfriends/tui-base/gate"
	"github.com/jarvisfriends/tui-base/keys"
	log "github.com/jarvisfriends/tui-base/logging"
	"github.com/jarvisfriends/tui-base/navigation"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/pages/home"
	"github.com/jarvisfriends/tui-base/pages/inspector"
	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/status"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// DefaultAppName is the fallback application name used when Options.AppName is empty.
const DefaultAppName = "TUI Base"

const (
	navStyleTabs   = "tabs"
	navStyleTopnav = "topnav"

	pageTitleHome     = "Home"
	pageTitleSettings = "Settings"

	konamiKeyDown  = "down"
	konamiKeyUp    = "up"
	konamiKeyLeft  = "left"
	konamiKeyRight = "right"
)

// RegisteredPage describes one application page to be added to the router.
// Only Title and Model are required; the navigation ID is derived automatically
// from the Title so callers never need to manage IDs separately.
type RegisteredPage struct {
	Title string
	Model tea.Model
}

var (
	globalRegistry   []RegisteredPage
	globalRegistryMu sync.Mutex
)

// RegisterPage adds a page to the global registry. This is typically called
// from a page package's init() function so pages can self-register.
func RegisterPage(title string, model tea.Model) {
	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	globalRegistry = append(globalRegistry, RegisteredPage{
		Title: title,
		Model: model,
	})
}

// RegisteredPages returns a copy of all dynamically registered pages.
func RegisteredPages() []RegisteredPage {
	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	out := make([]RegisteredPage, len(globalRegistry))
	copy(out, globalRegistry)
	return out
}

// ClearRegisteredPages clears the global page registry (primarily useful for tests).
func ClearRegisteredPages() {
	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	globalRegistry = nil
}

// ReplaceAppPagesMsg replaces the app-provided pages at runtime.
// Router-managed pages (Inspector and Settings) are preserved automatically.
//
// ActiveIndex is preferred when it is in range; otherwise ActiveTitle is used.
// If neither matches, the first app page becomes active.
type ReplaceAppPagesMsg struct {
	Pages       []RegisteredPage
	ActiveTitle string
	ActiveIndex int
}

// Options controls router startup behavior for embedding applications.
type Options struct {
	// AppName is the display name shown in the terminal window title and the
	// info modal (ℹ overlay). Defaults to "TUI Base" when empty.
	AppName string
	// AppVersion overrides the version string shown in the info modal.
	// When empty, common.AppVersion() (set via -ldflags at build time) is used.
	AppVersion string
	// ConfigDirName is the subdirectory name used under os.UserConfigDir() for
	// storing notifications and other persistent state. Defaults to AppName
	// (lowercased) when empty, and further falls back to "tui-base".
	ConfigDirName string
	// ConfigDir directly overrides the settings configuration directory.
	// If set, this path is used directly.
	ConfigDir  string
	ExtraPages []RegisteredPage
	// DefaultPage is the Title of the page to activate on startup.
	// When empty the first page in the list is shown.
	DefaultPage      string
	InitialLogLevel  string
	SettingsSections []cfg.Section[string]
	KeyMap           *keys.AppKeyMap
	Gates            *gate.GateRegistry
}

// pageIDFromTitle derives a stable navigation ID from a human-readable title
// by lower-casing and replacing spaces with hyphens (e.g. "My Page" , "my-page").
func pageIDFromTitle(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "-"))
}

// screamingSnake converts a human-readable app name to a SCREAMING_SNAKE_CASE
// env-var prefix, e.g. "TUI Base", "TUI_BASE", "My Cool App", "MY_COOL_APP".
// Non-alphanumeric runes are collapsed to underscores and duplicates removed.
func screamingSnake(name string) string {
	var b strings.Builder
	prev := '_'
	for _, r := range strings.ToUpper(name) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = r
		} else if prev != '_' {
			b.WriteByte('_')
			prev = '_'
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "TUI_BASE"
	}
	return result
}

// var assertion is at the bottom of this file.

type RouterModel struct {
	nav     navigation.Navigator
	appName string
	// appEnvPrefix is the SCREAMING_SNAKE_CASE env-var prefix derived from the
	// app name (e.g. "TUI Base" , "TUI_BASE"). Used to derive env var names
	// so consumer apps get branded env vars instead of the framework defaults.
	appEnvPrefix       string
	colorProfileEnvVar string
	debugEnvVar        string

	pages    []tea.Model
	appPages []RegisteredPage

	// inspector is a dedicated debug model that receives all messages for
	// logging/stats and is rendered as an overlay (Ctrl+D).
	inspector *inspector.InspectorModel
	// settingsPage is kept as a stable pointer so app-page replacement can
	// preserve the settings model and its internal state.
	settingsPage *settings.SettingsModel
	// homePage is used when no app pages are supplied.
	homePage tea.Model

	status *status.BarModel
	keys   *keys.AppKeyMap

	// infoModal is the centered full-screen dependency/version info overlay.
	infoModal *status.InfoModal

	// overlays is the Z-ordered stack of floating overlays (toast, notification
	// history, inspector, info modal). The router drives them through generic
	// loops in View/Update/OnMouse rather than a hardcoded block per overlay.
	// Stored ascending by Z; iterate in reverse for top-down input priority.
	overlays []Overlay

	// colors is a shared AppColors pointer. All child components hold this
	// pointer so updating *colors here propagates immediately on the next render.
	colors *theme.AppStyle

	notifMgr         *notifications.Manager
	notifPersistPath string // default persist path; empty when config dir unavailable

	navigationVisible bool

	// sidebarFocused tracks whether the sidebar has keyboard focus (vs. the
	// active page). When true, key events are routed to the sidebar instead.
	sidebarFocused bool

	// navShowNumbers is the user's preference for showing a leading per-item
	// number prefix on number-capable navs (the minimal top nav). Re-applied
	// whenever the nav is (re)built so it survives a nav-style switch.
	navShowNumbers bool

	// navNumberSelect gates the number-key (1–9) page-selection shortcut on
	// top-docked navs. Disabled by default; toggled live from the Settings page
	// via NavNumberSelectMsg. Decoupled from navShowNumbers so the prefix and the
	// shortcut can be enabled independently.
	navNumberSelect bool

	// konamiProgress tracks how far the user is through the secret key sequence.
	// The sequence is observed passively (keys still do their normal job); only
	// completing it fires the hidden easter egg.
	konamiProgress int

	width  int
	height int

	// colorProfile is the active rendering profile (forced or detected),
	// cached at construction. Used to convert OSC-emitted colors (the View
	// background/foreground) so they match SGR-rendered content over
	// downsampling profiles like ANSI256.
	colorProfile colorprofile.Profile

	// startup synchronization flags to keep terminal default colors deterministic.
	startupBgSeen    bool
	startupColorSync bool
}

type syncTerminalColorsMsg struct{}

func New() *RouterModel {
	return NewWithOptions(Options{})
}

// NewWithRegisteredPages creates a router with the built-in pages and app
// provided pages appended after them.
func NewWithRegisteredPages(extraPages []RegisteredPage) *RouterModel {
	return NewWithOptions(Options{ExtraPages: extraPages})
}

// NewWithOptions creates a router with built-in pages and optional app pages.
// When DefaultPageID is set and found, that page is selected on startup.
func NewWithOptions(opts Options) *RouterModel {
	// Resolve app name and config dir name early — both are used below.
	appName := opts.AppName
	if appName == "" {
		appName = DefaultAppName
	}
	configDirName := opts.ConfigDirName
	if configDirName == "" {
		configDirName = strings.ToLower(appName)
	}

	// Set the logging prefix so log files are named after the embedding app.
	log.SetAppName(configDirName)

	// Persist settings (tui_settings.json) under the per-app OS config directory
	// rather than the current working directory, so settings survive regardless
	// of where the binary is launched from. Falls back to a temp directory if
	// the OS config dir is unavailable or we are running in tests.
	appConfigDir := opts.ConfigDir
	if appConfigDir == "" {
		if flag.Lookup("test.v") != nil {
			appConfigDir = filepath.Join(os.TempDir(), fmt.Sprintf("tui-base-tests-%s-%d", configDirName, time.Now().UnixNano()))
		} else if base, err := os.UserConfigDir(); err == nil {
			appConfigDir = filepath.Join(base, configDirName)
		} else {
			appConfigDir = filepath.Join(os.TempDir(), configDirName)
		}
	}
	settings.SetConfigDir(appConfigDir)

	// create settings first so we can pick the initial navigation style
	settingsModel := settings.NewWithOptions(settings.Options{
		ExtraSections: opts.SettingsSections,
		DefaultKeys:   opts.KeyMap,
		Gates:         opts.Gates,
	})
	// On first run (no persisted settings file), settings.New applies first-run
	// defaults (tabs nav). Persist them now so the choice is stable next launch.
	if !settingsModel.LoadedFromFile() {
		if err := settingsModel.Save(); err != nil {
			log.Warnf("could not write initial settings file: %v", err)
		}
	}
	theme.SetThemePreferences(settingsModel.ThemeMode, settingsModel.AccessibilityColors, theme.StylePreset(settingsModel.StylePreset))
	settingsModel.ColorThemeID = theme.ResolveTintIDForMode(settingsModel.ColorThemeID, settingsModel.ThemeMode)
	if settingsModel.ColorThemeID != "" {
		_ = theme.SetCurrentTint(settingsModel.ColorThemeID)
	}
	// initialize logging from settings (writes to temp dir by default)
	if _, err := log.InitFromSettings(settingsModel.LogOutput, settingsModel.LogPath); err != nil {
		// best-effort: print to stderr if log init fails
		_ = err
	}
	// ensure the settings model reflects the currently-open log file and level
	if p := log.CurrentLogFile(); p != "" {
		settingsModel.LogPath = p
	}
	settingsModel.LogLevel = log.GetLevel()
	if opts.InitialLogLevel != "" {
		if err := log.SetLevel(opts.InitialLogLevel); err == nil {
			settingsModel.LogLevel = log.GetLevel()
		}
	}
	// choose nav implementation from persisted settings
	var nav navigation.Navigator
	switch settingsModel.NavStyle {
	case navStyleTabs:
		nav = navigation.NewTabs()
	case navStyleTopnav:
		nav = navigation.NewMinimalTopNav()
	default:
		nav = navigation.New()
	}
	if nl, ok := nav.(navigation.NumberLabeled); ok {
		nl.SetShowNumbers(settingsModel.NavShowNumbers)
	}

	// Compute the initial palette from the active tint and store it as a shared
	// pointer. All child components receive this pointer; updating the value in
	// place (see ThemeMsg handler) propagates instantly without re-wiring.
	initialColors := theme.Active()

	// create router with chosen nav

	// Derive env-var prefix before struct construction so colorProfile can
	// honor the app-specific override (e.g. MY_APP_COLOR_PROFILE).
	appPrefix := screamingSnake(appName)
	appColorProfileEnvVar := appPrefix + "_COLOR_PROFILE"
	appDebugEnvVar := appPrefix + "_DEBUG"
	initialColorProfile := effectiveColorProfileForEnvVar(appColorProfileEnvVar)

	appKeys := opts.KeyMap
	if appKeys == nil {
		appKeys = keys.DefaultKeyMap()
	}
	if settingsModel.CustomKeys != nil {
		appKeys.ApplyCustomizations(settingsModel.CustomKeys)
	}

	m := &RouterModel{
		nav:               nav,
		appName:           appName,
		status:            status.New(),
		keys:              appKeys,
		colors:            initialColors,
		navigationVisible: true,
		navShowNumbers:    settingsModel.NavShowNumbers,
		navNumberSelect:   settingsModel.NavNumberSelect,
		colorProfile:      initialColorProfile,
		settingsPage:      settingsModel,
		homePage:          home.New(),
	}
	// create a single inspector instance and keep a pointer to it so we can
	// forward messages to it even when it's not the active page.
	m.inspector = inspector.New()

	// Derive env-var names from the app name so consumers get branded vars
	// (e.g. "My App", MY_APP_COLOR_PROFILE, MY_APP_DEBUG) instead of the
	// generic TUI_BASE_* names.
	m.appEnvPrefix = appPrefix
	m.colorProfileEnvVar = appColorProfileEnvVar
	m.debugEnvVar = appDebugEnvVar
	m.inspector.SetColorProfileEnvVar(m.colorProfileEnvVar)

	// Also expose the app's env var name via the program helper so that
	// NewProgram(m) picks up the correct override if a consumer sets it.
	// The router exposes ColorProfileEnvVar() for callers who need the name.

	// subscribe inspector to log events so runtime logs appear in the UI
	log.RegisterSubscriber(func(level string, ts time.Time, msg string) {
		m.inspector.AddLog(level, ts, msg)
	})
	// Collect valid extra pages. When the caller supplies extra pages they come
	// first in the nav list (Home is omitted — the app provides its own landing
	// page). Settings is always appended last. Inspector is available globally
	// as an overlay via Ctrl+D.
	// When no extra pages are supplied the default is Home + Settings.
	m.status.SetKeys(m.keys)
	allPages := append([]RegisteredPage{}, opts.ExtraPages...)
	allPages = append(allPages, RegisteredPages()...)
	m.replaceAppPages(allPages, opts.DefaultPage, -1)
	// create notification manager and load persisted entries (best-effort)
	m.notifMgr = notifications.NewManager()
	if appConfigDir != "" {
		_ = m.notifMgr.Load(appConfigDir)
		defaultPersistPath := filepath.Join(appConfigDir, "notifications.json")
		m.notifPersistPath = defaultPersistPath
		// honor the persisted setting: only activate file persistence when enabled
		if settingsModel.NotificationsPersist {
			m.notifMgr.SetEnabled(settingsModel.NotificationsEnabled)
			m.notifMgr.SetPersistPath(defaultPersistPath)
		} else {
			m.notifMgr.SetEnabled(settingsModel.NotificationsEnabled)
		}
	}
	m.status.SetNotifManager(m.notifMgr)
	m.infoModal = status.NewInfoModal()
	m.infoModal.SetKeys(m.keys)
	m.infoModal.SetAppName(m.appName)
	if opts.AppVersion != "" {
		m.infoModal.SetVersion(opts.AppVersion)
	}
	m.buildOverlays()
	// Surface the inspector's compact runtime summary in the status bar's right
	// segment, but only while the inspector overlay is closed (when open, the
	// full inspector is already on screen). Evaluated on every status render.
	m.status.SetSummaryProvider(func() string {
		if m.inspector != nil && !m.inspector.IsVisible() {
			return m.inspector.StatusLineSummary()
		}
		return ""
	})
	m.applyColors()
	m.updatePageKeys()
	return m
}

// updatePageKeys checks whether the active page implements navigation.PageKeyProvider
// and, if so, pushes its key bindings to the status bar so the bar shows
// page-specific hints instead of the global router shortcuts.
func (m *RouterModel) updatePageKeys() {
	// If a visible modal overlay provides key bindings, they take precedence.
	for _, o := range slices.Backward(m.overlays) {
		if o.Visible() {
			if km, ok := o.(help.KeyMap); ok {
				m.status.SetPageBindings(km)
				return
			}
		}
	}

	idx := 0
	if m.nav != nil {
		idx = m.nav.GetActiveIndex()
	}
	if idx >= 0 && idx < len(m.pages) {
		if kp, ok := m.pages[idx].(help.KeyMap); ok {
			m.status.SetPageBindings(kp)
			return
		}
	}
	m.status.SetPageBindings(nil) // revert to global key map
}

// applyColors distributes the router's shared colors pointer to all child
// components that implement theme.ColorAware. Call this after construction and
// whenever the nav component is replaced.
func (m *RouterModel) applyColors() {
	if m.nav != nil {
		if ca, ok := m.nav.(theme.ColorAware); ok {
			ca.SetColors(m.colors)
		}
	}
	for _, p := range m.pages {
		if ca, ok := p.(theme.ColorAware); ok {
			ca.SetColors(m.colors)
		}
	}
	if m.inspector != nil {
		if ca, ok := any(m.inspector).(theme.ColorAware); ok {
			ca.SetColors(m.colors)
		}
	}
	if ca, ok := any(m.status).(theme.ColorAware); ok {
		ca.SetColors(m.colors)
	}
}

// activeWindowTitle returns the terminal window title for the current state,
// formatted as "AppName - PageTitle" (e.g. "aSettings - Aliases").
func (m *RouterModel) activeWindowTitle() string {
	if m.nav == nil {
		return m.appName
	}
	idx := m.nav.GetActiveIndex()
	pages := m.nav.GetPages()
	if idx >= 0 && idx < len(pages) && pages[idx].Title != "" {
		return m.appName + " - " + pages[idx].Title
	}
	return m.appName
}

func (m *RouterModel) activatePageByID(id string) bool {
	if m.nav == nil {
		return false
	}
	for i, p := range m.nav.GetPages() {
		if p.ID != id {
			continue
		}
		m.nav.SetActiveIndex(i)
		m.updatePageKeys()
		return true
	}
	return false
}

// replaceAppPages rebuilds the router page list from app-provided pages while
// preserving router-owned pages (Settings).
func (m *RouterModel) replaceAppPages(extraPages []RegisteredPage, activeTitle string, activeIndex int) {
	m.appPages = extraPages
	var navPages []navigation.Page
	var pageModels []tea.Model

	for _, rp := range extraPages {
		if rp.Model == nil || rp.Title == "" {
			continue
		}
		navPages = append(navPages, navigation.Page{ID: pageIDFromTitle(rp.Title), Title: rp.Title})
		pageModels = append(pageModels, rp.Model)
	}

	if len(pageModels) == 0 {
		navPages = append(navPages, navigation.Page{ID: navigation.PageIDHome, Title: pageTitleHome})
		pageModels = append(pageModels, m.homePage)
	}

	navPages = append(
		navPages,
		navigation.Page{ID: navigation.PageIDSettings, Title: pageTitleSettings},
	)
	pageModels = append(pageModels, m.settingsPage)

	m.pages = pageModels
	if m.nav != nil {
		m.nav.SetPages(navPages)
		selected := 0
		if activeIndex >= 0 && activeIndex < len(navPages) {
			selected = activeIndex
		} else if activeTitle != "" {
			for i, p := range navPages {
				if p.Title == activeTitle {
					selected = i
					break
				}
			}
		}
		m.nav.SetActiveIndex(selected)
	}
	m.applyColors()
	m.updatePageKeys()
}

// RegisterPage adds a page to the router at runtime. It initializes the new page's model,
// updates the navigation layout, and returns the model's Init command.
func (m *RouterModel) RegisterPage(title string, model tea.Model) tea.Cmd {
	m.appPages = append(m.appPages, RegisteredPage{Title: title, Model: model})
	activeIdx := 0
	if m.nav != nil {
		activeIdx = m.nav.GetActiveIndex()
	}
	m.replaceAppPages(m.appPages, "", activeIdx)
	return model.Init()
}

func (m *RouterModel) Init() tea.Cmd {
	pgInits := make([]tea.Cmd, len(m.pages))
	for i, p := range m.pages {
		pgInits[i] = p.Init()
	}
	var inspectorInit tea.Cmd
	if m.inspector != nil {
		inspectorInit = m.inspector.Init()
	}
	return tea.Batch(
		m.nav.Init(),
		m.status.Init(),
		inspectorInit,
		tea.Batch(pgInits...),
		// Request an explicit initial size so the first full-frame render is
		// deterministic across terminals/SSH transports.
		tea.RequestWindowSize,
		// T-3: request terminal background color on startup so the router can
		// auto-switch between dark and light themes without requiring the user
		// to manually pick a tint.
		tea.RequestBackgroundColor,
	)
}

func (m *RouterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Forward to active components
	var cmds []tea.Cmd
	// The inspector receives non-key messages for its message log and runtime
	// stats even when closed. Key messages are NOT forwarded here: when the
	// inspector overlay is open the overlay key handler (overlayHandleKey) routes
	// keys to it explicitly, and when it is closed it must not act on its own
	// keybindings (e.g. test-toast keys) at all. Forwarding keys here would both
	// double-dispatch while open and fire inspector shortcuts while closed.
	_, isKeyMsg := msg.(tea.KeyMsg)
	if m.inspector != nil && !isKeyMsg {
		_, cmd := m.inspector.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	idx := 0
	if m.nav != nil {
		idx = min(len(m.pages)-1, max(0, m.nav.GetActiveIndex()))
	}

	// Check whether the active page is claiming exclusive keyboard focus.
	// We do this before the main switch so the result is available inside the
	// tea.KeyMsg case without a second index lookup.
	activeCapturesKeys := false
	if m.nav != nil {
		activeIdx := m.nav.GetActiveIndex()
		if activeIdx >= 0 && activeIdx < len(m.pages) {
			if kc, ok := m.pages[activeIdx].(navigation.KeyCapturer); ok {
				activeCapturesKeys = kc.CapturesKeys()
			}
		}
	}

	switch msg := msg.(type) {
	case ReplaceAppPagesMsg:
		m.replaceAppPages(msg.Pages, msg.ActiveTitle, msg.ActiveIndex)
		var initCmds []tea.Cmd
		for _, p := range m.pages {
			if p == nil {
				continue
			}
			initCmds = append(initCmds, p.Init())
		}
		return m, tea.Batch(append(initCmds, m.handleResizeCmd())...)

	case settings.NavStyleMsg:
		// Switch navigation presentation
		var oldPages []navigation.Page
		var oldIdx int
		if m.nav != nil {
			oldPages = m.nav.GetPages()
			oldIdx = m.nav.GetActiveIndex()
		}
		switch msg.Style {
		case "tabs":
			m.nav = navigation.NewTabs()
		case "topnav":
			m.nav = navigation.NewMinimalTopNav()
		default:
			m.nav = navigation.New()
		}
		if nl, ok := m.nav.(navigation.NumberLabeled); ok {
			nl.SetShowNumbers(m.navShowNumbers)
		}
		if m.nav != nil {
			if oldPages != nil {
				m.nav.SetPages(oldPages)
			}
			if oldIdx >= 0 {
				m.nav.SetActiveIndex(oldIdx)
			} else {
				m.nav.SetActiveIndex(0)
			}
			// Wire the shared colors pointer to the new nav component.
			if ca, ok := m.nav.(theme.ColorAware); ok {
				ca.SetColors(m.colors)
			}
		}
		return m, m.handleResizeCmd()

	case settings.NavShowNumbersMsg:
		// Persist the preference and apply it to the current nav if it supports
		// number prefixes (the minimal top nav).
		m.navShowNumbers = msg.Show
		if nl, ok := m.nav.(navigation.NumberLabeled); ok {
			nl.SetShowNumbers(msg.Show)
		}
		return m, m.handleResizeCmd()

	case settings.NavNumberSelectMsg:
		// Enable/disable the number-key (1–9) page-selection shortcut. No layout
		// change, so no resize is needed.
		m.navNumberSelect = msg.Enabled
		return m, nil

	case notifications.AddMsg, notifications.DismissMsg, notifications.DismissKeyMsg, notifications.DismissAllMsg, notifications.ExpireMsg:
		if cmd := m.notifMgr.Handle(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.handleResizeCmd())
		return m, tea.Batch(cmds...)

	case settings.NotificationsSettingsMsg:
		// Apply notification enabled/persist settings to the shared manager.
		m.notifMgr.SetEnabled(msg.Enabled)
		if msg.Persist && m.notifPersistPath != "" {
			m.notifMgr.SetPersistPath(m.notifPersistPath)
		} else {
			m.notifMgr.SetPersistPath("") // disable file persistence
		}
		return m, m.handleResizeCmd()

	case settings.KeybindingsChangedMsg:
		m.keys.ApplyCustomizations(msg.CustomKeys)
		m.status.SetKeys(m.keys)
		m.infoModal.SetKeys(m.keys)
		m.updatePageKeys()
		return m, m.handleResizeCmd()

	case tea.BackgroundColorMsg:
		// T-3: terminal reported its background color on startup (or when the
		// user changes their terminal theme). Auto-switch dark/light mode so the
		// palette stays readable without a manual tint selection.
		mode := theme.ThemeModeLight
		if msg.IsDark() {
			mode = theme.ThemeModeDark
		}

		// Forward terminal diagnostics to the inspector so it can display
		// the detected background, color profile, and dark/light result.
		if m.inspector != nil {
			prof := colorprofile.Detect(os.Stdout, os.Environ())
			diagMsg := inspector.TermDiagMsg{
				DetectedBg: msg.Color,
				BgIsDark:   msg.IsDark(),
				Profile:    prof,
			}
			_, _ = m.inspector.Update(diagMsg)
		}

		m.startupBgSeen = true

		prefs := theme.ThemePreferencesSnapshot()
		if prefs.Mode != mode {
			theme.SetThemePreferences(mode, prefs.Accessibility, prefs.Style)
			resolvedID := theme.ResolveTintIDForMode("", mode)
			_ = theme.SetCurrentTint(resolvedID)
			newColors := theme.Active()
			if m.colors == nil {
				m.colors = newColors
			} else {
				*m.colors = *newColors
			}
			m.applyColors()
			m.startupColorSync = true
			return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd())
		}
		m.startupColorSync = true
		return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd())

	case settings.ThemeMsg:
		// Apply the selected tint globally and refresh the shared colors pointer.
		// All child components hold *m.colors so they see the new palette on the
		// next render without any additional wiring.
		if m.debugEnabled() {
			log.Debugf("Router.Update: received ThemeMsg id=%s router size=%dx%d", msg.ID, m.width, m.height)
		}
		if msg.ApplyPreferences {
			theme.SetThemePreferences(msg.Mode, msg.Accessibility, theme.StylePreset(msg.Style))
		}
		resolvedID := msg.ID
		if msg.ApplyPreferences {
			resolvedID = theme.ResolveTintIDForMode(msg.ID, msg.Mode)
		}
		_ = theme.SetCurrentTint(resolvedID)
		newColors := theme.Active()
		if m.colors == nil {
			m.colors = newColors
		} else {
			*m.colors = *newColors
		}
		m.applyColors()
		if m.debugEnabled() {
			log.Debugf("Router.Update: applied theme id=%s", msg.ID)
		}
		// Force a resize pass so children receive the correct content dimensions
		// immediately after a theme change. This avoids temporary re-renders
		// using an out-of-date width (which can make center vs left-aligned
		// rendering appear briefly).
		return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd())
	case tea.KeyPressMsg:
		keyMsg := msg
		// Observe the secret key sequence. Intermediate keys are NOT consumed
		// (they still navigate, etc.); only completing the sequence fires the
		// hidden easter egg and consumes that final key.
		if cmd := m.advanceKonami(keyMsg.String()); cmd != nil {
			return m, cmd
		}

		// A visible modal overlay (inspector, info modal, history panel)
		// intercepts all keys. The topmost visible KeyConsumer wins; passive
		// overlays (the toast) are not KeyConsumers and never block keys.
		if cmd, ok := m.overlayHandleKey(keyMsg); ok {
			return m, cmd
		}

		// Layout-toggle shortcuts are always active, even when a form has focus.
		switch {
		case key.Matches(keyMsg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.ToggleNav):
			switch {
			case !m.navigationVisible:
				// Hidden, show (unfocused).
				m.navigationVisible = true
			case !m.sidebarFocused:
				// Visible, unfocused, focus it so keyboard can navigate.
				m.sidebarFocused = true
				m.setNavFocused(true)
			default:
				// Visible and focused, hide it and drop focus.
				m.navigationVisible = false
				m.sidebarFocused = false
				m.setNavFocused(false)
			}
			return m, m.handleResizeCmd()
		case key.Matches(keyMsg, m.keys.ToggleFullHelp):
			m.status.ToggleFullHelpVisible()
			return m, m.handleResizeCmd()
		case key.Matches(keyMsg, m.keys.OpenSettings):
			if m.activatePageByID("settings") {
				return m, m.handleResizeCmd()
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.ToggleStatus):
			m.status.ToggleVisible()
			return m, m.handleResizeCmd()
		case key.Matches(keyMsg, m.keys.Debug):
			// Only reached when the inspector is not already visible (a visible
			// inspector consumes keys via overlayHandleKey above), so this opens it.
			m.inspector.ToggleVisible()
			m.updatePageKeys()
			return m, m.handleResizeCmd()
		}
		// When the active page has captured keyboard focus, bypass global
		// shortcuts (region/focus moves, page-cycling) so every key reaches
		// the form.
		if !activeCapturesKeys {
			if cmd, handled := m.handleNavKeys(keyMsg); handled {
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		// This is the actual terminal size. Update router state and
		// synchronously forward computed child sizes so children can
		// render immediately.
		m.width = msg.Width
		m.height = msg.Height
		m.infoModal.Resize(m.width, m.height)
		if !m.startupColorSync && !m.startupBgSeen {
			// Fallback path for terminals that don't respond to OSC 11.
			m.startupColorSync = true
			return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd())
		}
		return m, m.handleResizeCmd()

	case syncTerminalColorsMsg:
		return m, m.syncTerminalColorsCmd()

	case navigation.SelectedMsg:
		// A child navigation item was selected (via click or key). Switch pages.

		if msg.PageIndex < 0 || m.nav == nil || msg.PageIndex >= len(m.nav.GetPages()) {
			// Defensive: SelectedMsg with invalid index still triggers a resize
			return m, m.handleResizeCmd()
		}
		m.nav.SetActiveIndex(msg.PageIndex)
		// Keyboard focus is NOT changed here: navigating the sidebar with Up/Down
		// keeps focus on the sidebar (live page switch). Focus moves to the page
		// content only on an explicit Right/Enter/Tab (see the key handler). A
		// mouse click manages focus via NavFocusMsg separately.
		// Update status bar key hints to reflect the newly active page.
		m.updatePageKeys()
		// Schedule a resize for children but continue to forward the
		// SelectedMsg into the inspector and children so it is logged.
		cmds = append(cmds, m.handleResizeCmd())

	case status.CloseInfoModalMsg:
		m.infoModal.Close()
		return m, m.handleResizeCmd()

	case navigation.NavFocusMsg:
		m.sidebarFocused = msg.Focused
		if m.nav != nil {
			_, cmd := m.nav.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(append(cmds, m.handleResizeCmd())...)

	case navigation.CollapseToggleMsg:
		if m.nav != nil {
			_, cmd := m.nav.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(append(cmds, m.handleResizeCmd())...)

	case status.InfoModalScrollMsg:
		if msg.Up {
			m.infoModal.ScrollUp()
		} else {
			m.infoModal.ScrollDown()
		}
		return m, m.handleResizeCmd()

	case status.ClickRegionMsg:
		// Click on an interactive region in the status bar (e.g., settings icon).
		name := msg.Name
		// If notifications icon was clicked, toggle the notifications panel.
		if name == status.NotificationsRegionName {
			return m, tea.Batch(m.status.ToggleNotifications(), m.handleResizeCmd())
		}
		// If info icon was clicked, toggle the dependency/version info overlay.
		if name == status.InfoRegionName {
			m.infoModal.Toggle(m.width, m.height)
			return m, m.handleResizeCmd()
		}
		if m.nav != nil {
			if m.activatePageByID(name) {
				return m, m.handleResizeCmd()
			}
		}
		return m, nil
	}

	_, isKey := msg.(tea.KeyMsg)
	_, isMouse := msg.(tea.MouseMsg)

	modalVisible := false
	if isKey {
		for _, o := range m.overlays {
			if o.Visible() {
				if _, ok := o.(KeyConsumer); ok {
					modalVisible = true
					break
				}
			}
		}
	}

	// Nav: always receives non-key messages (WindowSizeMsg, etc.);
	// receives key messages only when the sidebar is focused AND the active
	// page is not claiming exclusive keyboard focus AND no modal overlay is visible.
	if m.inspector.IsVisible() {
		ow, oh := m.inspectorOverlayInnerSize()
		_, inspectorCmd := m.inspector.Update(tea.WindowSizeMsg{Width: ow, Height: oh})
		cmds = append(cmds, inspectorCmd)
	}
	if m.navigationVisible && m.nav != nil {
		if !isKey || (!modalVisible && m.sidebarFocused && !activeCapturesKeys) {
			_, cmd := m.nav.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	// For non-interactive messages (background cmd results, data msgs, etc.)
	// forward to every page so that pages that are not currently active can
	// still receive the results of their Init/Cmd-produced messages (e.g. a
	// scan that finishes while the user is viewing a different page).
	if !isKey && !isMouse {
		for i, p := range m.pages {
			if i == idx {
				continue // handled separately below
			}
			_, cmd := p.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Active page: receives all messages EXCEPT key events that were claimed
	// by the sidebar (i.e. sidebar focused and page not capturing keys) or
	// intercepted by a modal overlay.
	if !isKey || (!modalVisible && (!m.sidebarFocused || activeCapturesKeys)) {
		_, cmd := m.pages[idx].Update(msg)
		cmds = append(cmds, cmd)
	}
	_, cmd := m.status.Update(msg)
	cmds = append(cmds, cmd)

	// Refresh status bar key hints in case the active page's mode changed
	// (e.g. a page entering or leaving a text-input/search mode).
	m.updatePageKeys()

	return m, tea.Batch(cmds...)
}

// navReservedWidth returns the columns a visible left-docked sidebar reserves
// (0 for tabs or when navigation is hidden). Overlays use it to avoid covering
// the sidebar.
// setNavFocused applies keyboard focus to the active nav when it supports focus
// (the sidebar). Navigators without a focus concept (tabs) are left unchanged.
func (m *RouterModel) setNavFocused(focused bool) {
	if f, ok := m.nav.(navigation.Focusable); ok {
		f.SetFocused(focused)
	}
}

func (m *RouterModel) navReservedWidth() int {
	if m.navigationVisible && m.nav != nil && m.nav.Dock() == navigation.DockLeft {
		return m.nav.Width()
	}
	return 0
}

func (m *RouterModel) inspectorOverlayOuterSize() (w, h int) {
	w = min(max(m.width-6, 40), m.width)
	h = min(max(m.height-4, 12), m.height)
	return max(w, 1), max(h, 1)
}

func (m *RouterModel) inspectorOverlayInnerSize() (w, h int) {
	ow, oh := m.inspectorOverlayOuterSize()
	return max(ow-2, 1), max(oh-2, 1)
}

// cyclePage advances the active page by delta (wrapping in both directions),
// moves keyboard focus back to the page content, and refreshes status-bar key
// hints. Used by Tab (delta=+1) and Shift+Tab (delta=-1).
func (m *RouterModel) cyclePage(delta int) tea.Cmd {
	var pages []navigation.Page
	if m.nav != nil {
		pages = m.nav.GetPages()
	}
	if len(pages) == 0 {
		return nil
	}
	cur := m.nav.GetActiveIndex()
	next := ((cur+delta)%len(pages) + len(pages)) % len(pages)
	m.nav.SetActiveIndex(next)
	m.sidebarFocused = false
	m.setNavFocused(false)
	m.updatePageKeys()
	return m.handleResizeCmd()
}

// cyclePageTo switches directly to an absolute page index, moving keyboard focus
// back to the page content. Used by the top-nav number-key shortcuts.
func (m *RouterModel) cyclePageTo(index int) tea.Cmd {
	if m.nav == nil || index < 0 || index >= len(m.nav.GetPages()) {
		return nil
	}
	m.nav.SetActiveIndex(index)
	m.sidebarFocused = false
	m.setNavFocused(false)
	m.updatePageKeys()
	return m.handleResizeCmd()
}

// navDigitIndex maps a "1".."9" key press to a zero-based page index.
func navDigitIndex(keyMsg tea.KeyPressMsg) (int, bool) {
	s := keyMsg.Text
	if lipgloss.Width(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		return int(s[0] - '1'), true
	}
	return 0, false
}

// StatusBarContent reports the status bar's currently rendered text and whether
// it is visible. Exposed as an introspection seam so conformance tests
// (testutil.CheckStatusBarVisible) can assert the status bar is present in every
// rendered frame — on every page and with overlays open. Satisfies
// testutil.StatusProvider structurally.
func (m *RouterModel) StatusBarContent() (string, bool) {
	return m.status.View().Content, m.status.IsVisible()
}

// konamiSequence is the classic cheat code. Completing it triggers the hidden
// "secret menu" — for now just a fun notification; games or other surprises may
// live here later.
var konamiSequence = []string{konamiKeyUp, konamiKeyUp, konamiKeyDown, konamiKeyDown, konamiKeyLeft, konamiKeyRight, konamiKeyLeft, konamiKeyRight, "b", "a"}

var konamiMessages = []string{
	"🕹️  Konami code accepted! 30 extra lives… just kidding. (Secret menu coming soon.)",
	"⬆⬆⬇⬇⬅➡⬅➡🅱🅰 — you found the secret menu! It's gloriously empty for now.",
	"🎮 Achievement unlocked: pressed 10 keys in a very specific order.",
	"👾 The cake is a lie, but the easter egg is real. Stay tuned…",
}

var konamiPick int

// advanceKonami observes one key press toward the secret sequence. It returns a
// non-nil command (the easter-egg notification) only when the full sequence has
// just completed; otherwise nil, so the key continues to its normal handling.
// A wrong key restarts progress (and still counts if it's the sequence's first key).
func (m *RouterModel) advanceKonami(k string) tea.Cmd {
	switch k {
	case konamiSequence[m.konamiProgress]:
		m.konamiProgress++
	case konamiSequence[0]:
		m.konamiProgress = 1 // wrong key, but it restarts the sequence
	default:
		m.konamiProgress = 0
	}
	if m.konamiProgress < len(konamiSequence) {
		return nil
	}
	m.konamiProgress = 0
	content := konamiMessages[konamiPick%len(konamiMessages)]
	konamiPick++
	return func() tea.Msg {
		return notifications.AddMsg{
			Content:  content,
			Severity: notifications.SeverityInfo,
			TTL:      notifications.SeverityInfo.DefaultTTL(),
		}
	}
}

// debugEnabled reports whether verbose router debug logging is active. The env
// var is read live (not cached) so it can be toggled at runtime, as documented
// on DebugEnvVar.
func (m *RouterModel) debugEnabled() bool {
	return os.Getenv(m.debugEnvVar) == "1"
}

func (m *RouterModel) GetActivePage() tea.Model {
	idx := 0
	if m.nav != nil {
		idx = m.nav.GetActiveIndex()
	}
	if idx < 0 || idx >= len(m.pages) {
		idx = 0
	}
	return m.pages[idx]
}

func (m *RouterModel) handleResizeCmd() tea.Cmd {
	var cmds []tea.Cmd
	_, cmd := m.status.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	cmds = append(cmds, cmd)
	removeHeight := m.status.Height() // status bar knows if its active and what its height is
	removeWidth := 0
	if m.debugEnabled() {
		log.Debugf("handleResizeCmd: router size=%dx%d statusHeight=%d", m.width, m.height, removeHeight)
	}
	if m.navigationVisible && m.nav != nil {
		// Let the active nav compute its preferred size based on the full terminal width and available height.
		_, navCmd := m.nav.Update(tea.WindowSizeMsg{Width: m.width - removeWidth, Height: m.height - removeHeight})
		cmds = append(cmds, navCmd)
		// A left-docked nav (sidebar) reserves width; a top-docked nav (tabs)
		// reserves height. Driven by Dock() so a new nav style needs no router change.
		if m.nav.Dock() == navigation.DockLeft {
			removeWidth += m.nav.Width()
		} else {
			removeHeight += m.nav.Height()
		}
		if m.debugEnabled() {
			log.Debugf("handleResizeCmd: after nav type=%T removeWidth=%d removeHeight=%d", m.nav, removeWidth, removeHeight)
		}
	}
	if m.debugEnabled() {
		log.Debugf("handleResizeCmd: active page will get size=%dx%d", m.width-removeWidth, m.height-removeHeight)
	}
	_, cmd = m.GetActivePage().Update(tea.WindowSizeMsg{Width: m.width - removeWidth, Height: m.height - removeHeight})
	cmds = append(cmds, cmd)
	// Return a single internal resize message so Update can forward child
	// sizes without overwriting the router's terminal width/height.
	return tea.Batch(cmds...)
}

func (m *RouterModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		// Emit terminal defaults even before the first WindowSizeMsg so startup
		// uses the same terminal color path as runtime theme switches.
		return tea.View{
			Content:         "",
			AltScreen:       true,
			MouseMode:       tea.MouseModeCellMotion,
			BackgroundColor: m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetBackground()),
			ForegroundColor: m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetForeground()),
			WindowTitle:     m.activeWindowTitle(),
		}
	}
	activePage := m.GetActivePage()
	activePageView := activePage.View()
	statusView := m.status.View()
	statusContent := statusView.Content
	statusHeight := 0
	if m.status.IsVisible() {
		statusHeight = lipgloss.Height(statusContent)
	}

	var navView tea.View
	if m.navigationVisible && m.nav != nil {
		navView = m.nav.View()
	}

	var layout string
	if navView.Content != "" {
		if m.nav.Dock() == navigation.DockLeft {
			layout = lipgloss.JoinHorizontal(lipgloss.Top, navView.Content, activePageView.Content)
		} else {
			// top-docked nav (tabs) renders above the content
			layout = lipgloss.JoinVertical(lipgloss.Left, navView.Content, activePageView.Content)
		}
	} else {
		layout = activePageView.Content
	}

	// Keep the main content area pinned to the available viewport height so the
	// status bar is always anchored at the terminal bottom. Without this, short
	// pages make the status bar float upward and oversized pages can push it off
	// screen.
	//
	// Background must be explicit: over SSH, OSC 11 (v.BackgroundColor) is often
	// stripped, so any unstyled padding rows would expose the terminal's own
	// default background (typically black) instead of the theme color.
	mainHeight := max(m.height-statusHeight, 0)
	layout = lipgloss.NewStyle().
		Background(m.colors.Styles.TextOnBg.GetBackground()).
		Width(m.width).
		Height(mainHeight).
		MaxHeight(mainHeight).
		Render(layout)
	// Child components can emit ANSI resets mid-line; over SSH those resets
	// expose the terminal default background in unstyled gaps. Re-apply the
	// page background after each reset for the shared main layout area.
	layout = theme.ReapplyBg(layout, m.colors.Styles.TextOnBg.GetBackground())

	contentStr := layout
	if statusHeight > 0 {
		contentStr = lipgloss.JoinVertical(lipgloss.Left, layout, statusContent)
	}

	// Composite every visible overlay (toast, history panel, inspector, info
	// modal) bottom-up by Z. Each overlay owns its placement and bounds; the
	// router no longer special-cases them here. NewCompositor (not NewCanvas)
	// overlays directly onto the source so the nav sidebar and status bar behind
	// the overlay are never blanked.
	contentStr = m.renderOverlays(contentStr, statusHeight)

	// Bubble Tea emits BackgroundColor/ForegroundColor as OSC sequences that the
	// color-profile writer passes through UNCHANGED (it only downsamples SGR).
	// Convert them through the active profile ourselves so the terminal-default
	// fill matches the quantized SGR backgrounds of the rendered content. Without
	// this, over ANSI256 (e.g. SSH) the OSC background stays exact 24-bit while
	// content cells are quantized — two visibly different shades of one color.
	v := tea.View{
		Content:         contentStr,
		AltScreen:       true,
		MouseMode:       tea.MouseModeCellMotion,
		BackgroundColor: m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetBackground()),
		ForegroundColor: m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetForeground()),
		WindowTitle:     m.activeWindowTitle(),
	}
	// Dispatch mouse events into child views by converting global mouse
	// coordinates into child-relative coordinates and then calling the
	// child's OnMouse handler. Bubble Tea only calls the top-level view's
	// OnMouse, so we must route manually here.
	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		// A visible modal overlay intercepts the mouse: events inside its bounds
		// route to the overlay, a release outside closes it, and everything else
		// is consumed. Passive overlays (the toast) are transparent and fall
		// through to the page routing below.
		if cmd, ok := m.overlayHandleMouse(mm); ok {
			return cmd
		}

		// helper to route a mouse message into a child view with offsets.
		// Always emit a MouseHighlightMsg so the inspector can visualize where
		// the router considered the event to be, even if the child has no
		// OnMouse handler.
		route := func(child tea.View, offX, offY int, childName string) tea.Cmd {
			switch ev := mm.(type) {
			case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg, tea.MouseWheelMsg:
				mEvent := ev.Mouse()
				nm := tea.Mouse{X: mEvent.X - offX, Y: mEvent.Y - offY, Button: mEvent.Button, Mod: mEvent.Mod}
				var childCmd tea.Cmd
				if child.OnMouse != nil {
					switch ev.(type) {
					case tea.MouseClickMsg:
						childCmd = child.OnMouse(tea.MouseClickMsg(nm))
					case tea.MouseReleaseMsg:
						childCmd = child.OnMouse(tea.MouseReleaseMsg(nm))
					case tea.MouseMotionMsg:
						childCmd = child.OnMouse(tea.MouseMotionMsg(nm))
					case tea.MouseWheelMsg:
						childCmd = child.OnMouse(tea.MouseWheelMsg(nm))
					}
				}
				return tea.Batch(childCmd, func() tea.Msg {
					return inspector.MouseHighlightMsg{GlobalX: mEvent.X, GlobalY: mEvent.Y, Child: childName, OffX: offX, OffY: offY}
				})
			default:
				return nil
			}
		}

		// compute status height and main layout height
		mainHeight := max(m.height-statusHeight, 0)

		// route based on nav layout
		if cmd := m.routeMouseToNav(mm, mainHeight, navView, activePageView, route); cmd != nil {
			return cmd
		}

		// status area (at bottom) — delegate entirely to the status view's own
		// OnMouse handler which uses pre-computed lipgloss.Width regions and the
		// correct row index. Avoids parsing ANSI-encoded rendered strings with
		// strings.Index which is unreliable when lipgloss injects resets mid-glyph.
		mmPos := mm.Mouse()
		if mmPos.Y >= mainHeight && mmPos.Y < mainHeight+statusHeight {
			return route(statusView, 0, mainHeight, "status")
		}

		return nil
	}
	return v
}

type routeFn func(child tea.View, offX, offY int, childName string) tea.Cmd

func (m *RouterModel) routeMouseToNav(mm tea.MouseMsg, mainHeight int, navView, activePageView tea.View, route routeFn) tea.Cmd {
	if m.navigationVisible && m.nav != nil {
		return m.routeMouseWithNavVisible(mm, mainHeight, navView, activePageView, route)
	}
	// nav hidden -> content occupies main area
	mmPos := mm.Mouse()
	if mmPos.Y < mainHeight {
		return route(activePageView, 0, 0, "content")
	}
	return nil
}

func (m *RouterModel) routeMouseWithNavVisible(mm tea.MouseMsg, mainHeight int, navView, activePageView tea.View, route routeFn) tea.Cmd {
	mmPos := mm.Mouse()
	if m.nav.Dock() == navigation.DockLeft {
		navW := m.nav.Width()
		if mmPos.Y >= mainHeight {
			return nil
		}
		if mmPos.X < navW {
			return route(navView, 0, 0, "sidebar")
		}
		// Content area click: release nav keyboard focus so the border
		// and highlight reset immediately on the next render.
		if m.sidebarFocused {
			m.sidebarFocused = false
			m.setNavFocused(false)
		}
		return route(activePageView, navW, 0, "content")
	}
	navH := m.nav.Height()
	if mmPos.Y < navH {
		return route(navView, 0, 0, "tabs")
	}
	if mmPos.Y < mainHeight {
		return route(activePageView, 0, navH, "content")
	}
	return nil
}

func (m *RouterModel) handleNavKeys(keyMsg tea.KeyPressMsg) (tea.Cmd, bool) {
	_, isSidebar := m.nav.(navigation.Focusable)
	if isSidebar {
		return m.handleSidebarNavKey(keyMsg)
	}
	// Top-docked nav (tabs / minimal top nav): Tab/Shift+Tab cycle
	// pages. Number keys 1–9 jump directly only when the user has
	// enabled "Number Key Select" in Settings (off by default), and
	// independently of whether the nav shows a number prefix.
	switch {
	case key.Matches(keyMsg, m.keys.NextPage):
		return m.cyclePage(1), true
	case key.Matches(keyMsg, m.keys.PreviousPage):
		return m.cyclePage(-1), true
	}
	if m.navNumberSelect && m.nav != nil && m.nav.Dock() == navigation.DockTop {
		if i, ok := navDigitIndex(keyMsg); ok && i < len(m.nav.GetPages()) {
			return m.cyclePageTo(i), true
		}
	}
	return nil, false
}

// handleSidebarNavKey handles key events when the active nav is a sidebar
// (implements navigation.Focusable). It manages focus transitions between the
// sidebar region and the page region without cycling pages via Tab/Shift+Tab.
func (m *RouterModel) handleSidebarNavKey(keyMsg tea.KeyPressMsg) (tea.Cmd, bool) {
	// Region focus model for the sidebar (the recommended UX):
	//   • Up/Down navigate within the sidebar and never move focus
	//     out of it (handled by the sidebar when focused).
	//   • Right / Enter / Tab / Shift+Tab move focus to the page.
	//   • Left / Esc / Tab / Shift+Tab return focus to the sidebar.
	// Tab/Shift+Tab therefore toggle focus between the two regions
	// rather than cycling pages (page selection is Up/Down in the
	// sidebar). Pages that need Left/Esc must implement
	// KeyCapturer so those keys reach them instead.
	switch {
	case keyMsg.Code == tea.KeyRight || keyMsg.Code == tea.KeyEnter || keyMsg.Code == tea.KeyTab || keyMsg.String() == "shift+tab":
		if m.sidebarFocused {
			m.sidebarFocused = false
			m.setNavFocused(false)
			m.updatePageKeys()
			return m.handleResizeCmd(), true
		}
	case keyMsg.Code == tea.KeyLeft || keyMsg.Code == tea.KeyEscape:
		if !m.sidebarFocused {
			m.sidebarFocused = true
			m.setNavFocused(true)
			m.updatePageKeys()
			return m.handleResizeCmd(), true
		}
	}
	// Tab/Shift+Tab while the page is focused return to the sidebar
	// (the "prev/next region" half not covered above).
	if !m.sidebarFocused {
		if key.Matches(keyMsg, m.keys.NextPage) || key.Matches(keyMsg, m.keys.PreviousPage) {
			m.sidebarFocused = true
			m.setNavFocused(true)
			m.updatePageKeys()
			return m.handleResizeCmd(), true
		}
	}
	// Up/Down (and Esc while focused) fall through to the sidebar
	// via the normal key-forwarding path below.
	return nil, false
}

// syncTerminalColorsCmd force-applies terminal default foreground/background
// colors via OSC, even when the renderer thinks the values are unchanged.
// This keeps terminal frame/tab edge colors in sync with the active theme.
func (m *RouterModel) syncTerminalColorsCmd() tea.Cmd {
	bg := theme.ColorHex(m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetBackground()))
	fg := theme.ColorHex(m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetForeground()))
	seq := ansi.SetBackgroundColor(bg) + ansi.SetForegroundColor(fg)
	return tea.Raw(seq)
}

func (m *RouterModel) syncTerminalColorsAfterCmd() tea.Cmd {
	return tea.Tick(20*time.Millisecond, func(time.Time) tea.Msg {
		return syncTerminalColorsMsg{}
	})
}

var _ tea.Model = (*RouterModel)(nil)

// ColorProfileEnvVar returns the env-var name the router uses to honor a
// forced color profile override. For an app named "My App" this will be
// "MY_APP_COLOR_PROFILE". Pass this name to NewProgram so the program and the
// router agree on which variable to read:
//
//	m := router.NewWithOptions(router.Options{AppName: "My App"})
//	p := router.NewProgram(m, m.ColorProfileEnvVar())
func (m *RouterModel) ColorProfileEnvVar() string { return m.colorProfileEnvVar }

// DebugEnvVar returns the env-var name the router uses to enable verbose debug
// logging (e.g. "MY_APP_DEBUG"). Set it to "1" at runtime to activate.
func (m *RouterModel) DebugEnvVar() string { return m.debugEnvVar }

package router

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	cfg "github.com/jarvisfriends/tui-base/config"
	"github.com/jarvisfriends/tui-base/keys"
	log "github.com/jarvisfriends/tui-base/logging"
	"github.com/jarvisfriends/tui-base/navigation"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/pages/debug"
	"github.com/jarvisfriends/tui-base/pages/home"
	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/status"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	tint "github.com/lrstanley/bubbletint/v2"
)

// DefaultAppName is the fallback application name used when Options.AppName is empty.
const DefaultAppName = "TUI Base"

// RegisteredPage describes one application page to be added to the router.
// Only Title and Model are required; the navigation ID is derived automatically
// from the Title so callers never need to manage IDs separately.
type RegisteredPage struct {
	Title string
	Model tea.Model
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
	ExtraPages    []RegisteredPage
	// DefaultPage is the Title of the page to activate on startup.
	// When empty the first page in the list is shown.
	DefaultPage      string
	InitialLogLevel  string
	SettingsSections []cfg.Section
}

// pageIDFromTitle derives a stable navigation ID from a human-readable title
// by lower-casing and replacing spaces with hyphens (e.g. "My Page" → "my-page").
func pageIDFromTitle(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "-"))
}

// screamingSnake converts a human-readable app name to a SCREAMING_SNAKE_CASE
// env-var prefix, e.g. "TUI Base" → "TUI_BASE", "My Cool App" → "MY_COOL_APP".
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
	// app name (e.g. "TUI Base" → "TUI_BASE"). Used to derive env var names
	// so consumer apps get branded env vars instead of the framework defaults.
	appEnvPrefix        string
	colorProfileEnvVar  string
	debugEnvVar         string

	pages []tea.Model

	// inspector is a dedicated debug model that receives all messages for
	// logging/stats and is rendered as an overlay (Ctrl+D).
	inspector *debug.Model
	// settingsPage is kept as a stable pointer so app-page replacement can
	// preserve the settings model and its internal state.
	settingsPage *settings.Model
	// homePage is used when no app pages are supplied.
	homePage tea.Model

	status *status.BarModel
	keys   *keys.AppKeyMap

	// infoModal is the centered full-screen dependency/version info overlay.
	infoModal *status.InfoModal

	// historyOverlayBounds caches [x, y, w, h] of the last-rendered notification
	// history panel so the OnMouse handler can detect outside clicks without
	// re-parsing ANSI output.
	historyOverlayBounds [4]int

	// inspectorOverlayBounds caches [x, y, w, h] of the last-rendered
	// inspector overlay for hit-testing in OnMouse.
	inspectorOverlayBounds [4]int

	// colors is a shared AppColors pointer. All child components hold this
	// pointer so updating *colors here propagates immediately on the next render.
	colors *theme.AppStyle

	notifMgr         *notifications.Manager
	notifPersistPath string // default persist path; empty when config dir unavailable

	navigationVisible bool

	// sidebarFocused tracks whether the sidebar has keyboard focus (vs. the
	// active page). When true, key events are routed to the sidebar instead.
	sidebarFocused bool

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
	// of where the binary is launched from. Falls back to CWD if the OS config
	// dir is unavailable.
	appConfigDir := ""
	if base, err := os.UserConfigDir(); err == nil {
		appConfigDir = filepath.Join(base, configDirName)
	}
	settings.SetConfigDir(appConfigDir)

	// create settings first so we can pick the initial navigation style
	settingsModel := settings.New(opts.SettingsSections...)
	// On first run (no persisted settings file), settings.New applies first-run
	// defaults (tabs nav). Persist them now so the choice is stable next launch.
	if !settingsModel.LoadedFromFile() {
		if err := settingsModel.Save(); err != nil {
			log.Warnf("could not write initial settings file: %v", err)
		}
	}
	theme.SetThemePreferences(settingsModel.ThemeMode, settingsModel.AccessibilityColors)
	settingsModel.ColorThemeID = theme.ResolveTintIDForMode(settingsModel.ColorThemeID, settingsModel.ThemeMode)
	if settingsModel.ColorThemeID != "" {
		tint.SetTintID(settingsModel.ColorThemeID) //nolint:errcheck
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
	if settingsModel.NavStyle == "tabs" {
		nav = navigation.NewTabs()
	} else {
		nav = navigation.New()
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

	m := &RouterModel{
		nav:               nav,
		appName:           appName,
		status:            status.New(),
		keys:              keys.DefaultKeyMap(),
		colors:            initialColors,
		navigationVisible: true,
		colorProfile:      initialColorProfile,
		settingsPage:      settingsModel,
		homePage:          home.New(),
	}
	// create a single inspector instance and keep a pointer to it so we can
	// forward messages to it even when it's not the active page.
	m.inspector = debug.New()

	// Derive env-var names from the app name so consumers get branded vars
	// (e.g. "My App" → MY_APP_COLOR_PROFILE, MY_APP_DEBUG) instead of the
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
	m.replaceAppPages(opts.ExtraPages, opts.DefaultPage, -1)
	// create notification manager and load persisted entries (best-effort)
	m.notifMgr = notifications.NewManager()
	if configDir, err := os.UserConfigDir(); err == nil {
		persistDir := filepath.Join(configDir, configDirName)
		_ = m.notifMgr.Load(persistDir)
		defaultPersistPath := filepath.Join(persistDir, "notifications.json")
		m.notifPersistPath = defaultPersistPath
		// honour the persisted setting: only activate file persistence when enabled
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
	m.applyColors()
	m.updatePageKeys()
	return m
}

// ColorAware is implemented by any component that accepts a shared color palette pointer.
type ColorAware interface {
	SetColors(c *theme.AppStyle)
}

// updatePageKeys checks whether the active page implements navigation.PageKeyProvider
// and, if so, pushes its key bindings to the status bar so the bar shows
// page-specific hints instead of the global router shortcuts.
func (m *RouterModel) updatePageKeys() {
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
// components that implement colorAware. Call this after construction and
// whenever the nav component is replaced.
func (m *RouterModel) applyColors() {
	if m.nav != nil {
		if ca, ok := m.nav.(ColorAware); ok {
			ca.SetColors(m.colors)
		}
	}
	for _, p := range m.pages {
		if ca, ok := p.(ColorAware); ok {
			ca.SetColors(m.colors)
		}
	}
	if m.inspector != nil {
		if ca, ok := any(m.inspector).(ColorAware); ok {
			ca.SetColors(m.colors)
		}
	}
	if ca, ok := any(m.status).(ColorAware); ok {
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
		navPages = append(navPages, navigation.Page{ID: "home", Title: "Home"})
		pageModels = append(pageModels, m.homePage)
	}

	navPages = append(navPages,
		navigation.Page{ID: "settings", Title: "Settings"},
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
	if m.inspector != nil {
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
		if msg.Style == "tabs" {
			m.nav = navigation.NewTabs()
		} else {
			m.nav = navigation.New()
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
			if ca, ok := m.nav.(ColorAware); ok {
				ca.SetColors(m.colors)
			}
		}
		return m, m.handleResizeCmd()
	case notifications.AddMsg, notifications.DismissMsg, notifications.DismissAllMsg, notifications.ExpireMsg:
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

	case tea.BackgroundColorMsg:
		// T-3: terminal reported its background colour on startup (or when the
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
			diagMsg := debug.TermDiagMsg{
				DetectedBg: msg.Color,
				BgIsDark:   msg.IsDark(),
				Profile:    prof,
			}
			_, _ = m.inspector.Update(diagMsg)
		}

		m.startupBgSeen = true

		prefs := theme.ThemePreferencesSnapshot()
		if prefs.Mode != mode {
			theme.SetThemePreferences(mode, prefs.Accessibility)
			resolvedID := theme.ResolveTintIDForMode("", mode)
			tint.SetTintID(resolvedID) //nolint:errcheck
			newColors := theme.Active()
			if m.colors == nil {
				m.colors = newColors
			} else {
				*m.colors = *newColors
			}
			m.applyColors()
			m.startupColorSync = true
			return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd(20*time.Millisecond))
		}
		m.startupColorSync = true
		return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd(20*time.Millisecond))

	case settings.ThemeMsg:
		// Apply the selected tint globally and refresh the shared colors pointer.
		// All child components hold *m.colors so they see the new palette on the
		// next render without any additional wiring.
		if os.Getenv(m.debugEnvVar) == "1" {
			log.Debugf("Router.Update: received ThemeMsg id=%s router size=%dx%d", msg.ID, m.width, m.height)
		}
		if msg.ApplyPreferences {
			theme.SetThemePreferences(msg.Mode, msg.Accessibility)
		}
		resolvedID := msg.ID
		if msg.ApplyPreferences {
			resolvedID = theme.ResolveTintIDForMode(msg.ID, msg.Mode)
		}
		tint.SetTintID(resolvedID) //nolint:errcheck
		newColors := theme.Active()
		if m.colors == nil {
			m.colors = newColors
		} else {
			*m.colors = *newColors
		}
		m.applyColors()
		if os.Getenv(m.debugEnvVar) == "1" {
			log.Debugf("Router.Update: applied theme id=%s", msg.ID)
		}
		// Force a resize pass so children receive the correct content dimensions
		// immediately after a theme change. This avoids temporary re-renders
		// using an out-of-date width (which can make center vs left-aligned
		// rendering appear briefly).
		return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd(20*time.Millisecond))
	case tea.KeyMsg:
		switch keyMsg := msg.(type) {
		case tea.KeyPressMsg:
			// Inspector overlay intercepts keys when open.
			if m.inspector.IsVisible() {
				switch {
				case key.Matches(keyMsg, m.keys.Debug), key.Matches(keyMsg, m.keys.Dismiss):
					m.inspector.ToggleVisible()
					m.inspectorOverlayBounds = [4]int{}
					return m, m.handleResizeCmd()
				default:
					if m.inspector != nil {
						_, cmd := m.inspector.Update(msg)
						return m, tea.Batch(cmd, m.handleResizeCmd())
					}
					return m, nil
				}
			}

			// Info modal intercepts all keys when open.
			if m.infoModal.IsVisible() {
				if _, cmd := m.infoModal.Update(msg); cmd != nil {
					return m, cmd
				}
				return m, m.handleResizeCmd() // consume all other keys
			}

			// Notification history panel intercepts all keys when open.
			if m.status.IsHistoryVisible() {
				notifCount := 0
				if m.notifMgr != nil {
					notifCount = len(m.notifMgr.Active())
				}
				switch {
				case key.Matches(keyMsg, m.keys.Quit):
					m.status.CloseHistory()
					return m, m.handleResizeCmd()
				case key.Matches(keyMsg, m.keys.Up):
					m.status.NotifHistoryCursorUp()
					return m, m.handleResizeCmd()
				case key.Matches(keyMsg, m.keys.Down):
					m.status.NotifHistoryCursorDown(notifCount)
					return m, m.handleResizeCmd()
				case key.Matches(keyMsg, m.keys.Select):
					cursor := m.status.HistoryCursor()
					if m.notifMgr != nil {
						active := m.notifMgr.Active()
						if cursor >= 0 && cursor < len(active) {
							m.notifMgr.Dismiss(active[cursor].ID)
						}
					}
					return m, m.handleResizeCmd()
				case key.Matches(keyMsg, m.keys.DismissAll):
					if m.notifMgr != nil {
						m.notifMgr.DismissAll(nil)
					}
					return m, m.handleResizeCmd()
				case key.Matches(keyMsg, m.keys.Dismiss):
					if m.notifMgr != nil {
						m.notifMgr.DismissAll(nil)
					}
					return m, m.handleResizeCmd()
				}
				return m, nil // consume all other keys
			}

			// Layout-toggle shortcuts are always active, even when a form has focus.
			switch {
			case key.Matches(keyMsg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(keyMsg, m.keys.ToggleNav):
				if !m.navigationVisible {
					// Hidden → show (unfocused).
					m.navigationVisible = true
				} else if !m.sidebarFocused {
					// Visible, unfocused → focus it so keyboard can navigate.
					m.sidebarFocused = true
					if sb, ok := m.nav.(*navigation.Sidebar); ok {
						sb.SetFocused(true)
					}
				} else {
					// Visible and focused → hide it and drop focus.
					m.navigationVisible = false
					m.sidebarFocused = false
					if sb, ok := m.nav.(*navigation.Sidebar); ok {
						sb.SetFocused(false)
					}
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
				m.inspector.ToggleVisible()
				if !m.inspector.IsVisible() {
					m.inspectorOverlayBounds = [4]int{}
				}
				return m, m.handleResizeCmd()
			}
			// When the active page has captured keyboard focus, bypass global
			// shortcuts (quit, page-cycling) so every key reaches the form.
			if !activeCapturesKeys {
				switch {
				case key.Matches(keyMsg, m.keys.Tab):
					// Cycle forward through pages regardless of nav implementation.
					pages := []navigation.Page{}
					if m.nav != nil {
						pages = m.nav.GetPages()
					}
					if len(pages) == 0 {
						return m, nil
					}
					cur := 0
					if m.nav != nil {
						cur = m.nav.GetActiveIndex()
					}
					next := (cur + 1) % len(pages)
					if m.nav != nil {
						m.nav.SetActiveIndex(next)
					}
					// Tab always moves focus back to the page content area.
					m.sidebarFocused = false
					if sb, ok := m.nav.(*navigation.Sidebar); ok {
						sb.SetFocused(false)
					}
					m.updatePageKeys()
					return m, m.handleResizeCmd()
				case key.Matches(keyMsg, m.keys.ShiftTab):
					pages := []navigation.Page{}
					if m.nav != nil {
						pages = m.nav.GetPages()
					}
					if len(pages) == 0 {
						return m, nil
					}
					cur := 0
					if m.nav != nil {
						cur = m.nav.GetActiveIndex()
					}
					prev := (cur - 1 + len(pages)) % len(pages)
					if m.nav != nil {
						m.nav.SetActiveIndex(prev)
					}
					// ShiftTab always moves focus back to the page content area.
					m.sidebarFocused = false
					if sb, ok := m.nav.(*navigation.Sidebar); ok {
						sb.SetFocused(false)
					}
					m.updatePageKeys()
					return m, m.handleResizeCmd()
				}
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
			return m, tea.Batch(m.handleResizeCmd(), m.syncTerminalColorsAfterCmd(20*time.Millisecond))
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
	// Nav: always receives non-key messages (WindowSizeMsg, etc.);
	// receives key messages only when the sidebar is focused AND the active
	// page is not claiming exclusive keyboard focus.
	if m.inspector.IsVisible() {
		ow, oh := m.inspectorOverlayInnerSize()
		_, inspectorCmd := m.inspector.Update(tea.WindowSizeMsg{Width: ow, Height: oh})
		cmds = append(cmds, inspectorCmd)
	}
	if m.navigationVisible && m.nav != nil {
		if !isKey || (m.sidebarFocused && !activeCapturesKeys) {
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
	// by the sidebar (i.e. sidebar focused and page not capturing keys).
	if !isKey || !m.sidebarFocused || activeCapturesKeys {
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

func (m *RouterModel) inspectorOverlayOuterSize() (int, int) {
	w := min(max(m.width-6, 40), m.width)
	h := min(max(m.height-4, 12), m.height)
	return max(w, 1), max(h, 1)
}

func (m *RouterModel) inspectorOverlayInnerSize() (int, int) {
	ow, oh := m.inspectorOverlayOuterSize()
	return max(ow-2, 1), max(oh-2, 1)
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
	if os.Getenv(m.debugEnvVar) == "1" {
		log.Debugf("handleResizeCmd: router size=%dx%d statusHeight=%d", m.width, m.height, removeHeight)
	}
	if m.navigationVisible && m.nav != nil {
		// Let the active nav compute its preferred size based on the full terminal width and available height.
		_, cmd := m.nav.Update(tea.WindowSizeMsg{Width: m.width - removeWidth, Height: m.height - removeHeight})
		cmds = append(cmds, cmd)
		switch m.nav.(type) {
		case *navigation.Sidebar:
			removeWidth += m.nav.Width()
		case *navigation.Tabs:
			// tabs expect full width and provide a nav height
			removeHeight += m.nav.Height()
		default:
			removeWidth += m.nav.Width()
			removeHeight += m.nav.Height()
		}
		if os.Getenv(m.debugEnvVar) == "1" {
			log.Debugf("handleResizeCmd: after nav type=%T removeWidth=%d removeHeight=%d", m.nav, removeWidth, removeHeight)
		}
	}
	if os.Getenv(m.debugEnvVar) == "1" {
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
		switch m.nav.(type) {
		case *navigation.Sidebar:
			layout = lipgloss.JoinHorizontal(lipgloss.Top, navView.Content, activePageView.Content)
		case *navigation.Tabs:
			// render tabs above content
			layout = lipgloss.JoinVertical(lipgloss.Left, navView.Content, activePageView.Content)
		default:
			// render unknown nav above content
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
	// default background (typically black) instead of the theme colour.
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
	layout = reapplyBg(layout, m.colors.Styles.TextOnBg.GetBackground())

	contentStr := layout
	if statusHeight > 0 {
		contentStr = lipgloss.JoinVertical(lipgloss.Left, layout, statusContent)
	}

	// Canvas-based notification history panel overlay: composited in the
	// bottom-right corner, just above the status bar. Mutually exclusive with
	// the toast. Height is capped so the nav and most content stay visible.
	if m.status.IsHistoryVisible() {
		// Determine nav width so the panel never overlaps the sidebar.
		navW := 0
		if m.navigationVisible && m.nav != nil {
			if sb, ok := m.nav.(*navigation.Sidebar); ok {
				navW = sb.Width()
			}
		}
		// Cap height to ~1/3 of the content area (max 12 rows) so nav and
		// content remain clearly visible behind the panel.
		contentH := m.height - statusHeight
		maxPanelH := max(min(contentH/3, 12), 4)
		// Limit width to the space available to the right of the nav sidebar.
		maxPanelW := m.width - navW
		panelStr := m.status.RenderHistoryOverlay(maxPanelW, maxPanelH)
		if panelStr != "" {
			pw, ph := lipgloss.Size(panelStr)
			// Align to bottom-right above the status bar, right of any nav sidebar.
			panelX := max(m.width-pw, navW)
			panelY := max(m.height-statusHeight-ph, 0)
			m.historyOverlayBounds = [4]int{panelX, panelY, pw, ph}
			// Use NewCompositor (not NewCanvas) so the base content is overlaid
			// directly — no blank grid that would erase the nav sidebar or status bar.
			contentStr = lipgloss.NewCompositor(
				lipgloss.NewLayer(contentStr),
				lipgloss.NewLayer(panelStr).X(panelX).Y(panelY).Z(1),
			).Render()
		}
	} else {
		m.historyOverlayBounds = [4]int{}
	}

	// Canvas-based toast overlay: show the most-recent active notification
	// in the lower-right corner when the history panel is not open.
	if m.notifMgr != nil && !m.status.IsHistoryVisible() {
		active := m.notifMgr.Active()
		if len(active) > 0 {
			toast := active[0]
			borderColor := lipgloss.Color(notifications.ColorForSeverity(toast.Severity))
			toastStyle := m.colors.Styles.OverlayBorder.
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Background(m.colors.Styles.TextOnBg.GetBackground()).
				Foreground(m.colors.Styles.TextOnBg.GetForeground()).
				Padding(0, 1)
			msg := toast.Content
			if len([]rune(msg)) > 40 {
				msg = string([]rune(msg)[:39]) + "…"
			}
			toastStr := toastStyle.Render(msg)
			tw, th := lipgloss.Size(toastStr)
			toastX := max(m.width-tw, 0)
			toastY := max(m.height-statusHeight-th, 0)
			// Use NewCompositor (not NewCanvas) so the base content is overlaid
			// directly — no blank grid that would erase the nav sidebar or status bar.
			contentStr = lipgloss.NewCompositor(
				lipgloss.NewLayer(contentStr),
				lipgloss.NewLayer(toastStr).X(toastX).Y(toastY).Z(1),
			).Render()
		}
	}

	if m.inspector.IsVisible() {
		ow, oh := m.inspectorOverlayOuterSize()
		ox := max((m.width-ow)/2, 0)
		oy := max((m.height-oh)/2, 0)
		iw, ih := m.inspectorOverlayInnerSize()
		_, _ = m.inspector.Update(tea.WindowSizeMsg{Width: iw, Height: ih})
		inspectorStr := m.inspector.View().Content
		panel := m.colors.Styles.OverlayBorder.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.colors.Styles.SelectedItem.GetForeground()).
			Background(m.colors.Styles.TextOnBg.GetBackground()).
			Foreground(m.colors.Styles.TextOnBg.GetForeground()).
			Width(ow).
			MaxHeight(oh).
			Height(oh).
			Render(inspectorStr)
		m.inspectorOverlayBounds = [4]int{ox, oy, ow, oh}
		contentStr = lipgloss.NewCompositor(
			lipgloss.NewLayer(contentStr),
			lipgloss.NewLayer(panel).X(ox).Y(oy).Z(2),
		).Render()
	} else {
		m.inspectorOverlayBounds = [4]int{}
	}

	// Info modal: centered full-screen overlay. Shown on top of everything,
	// including the toast. Built on its own canvas so all background rows are
	// painted (no transparent holes in the base content).
	if m.infoModal.IsVisible() {
		modalStr := m.infoModal.View()
		if modalStr.Content != "" {
			bx, by, _, _ := m.infoModal.Bounds()
			contentStr = lipgloss.NewCompositor(lipgloss.NewLayer(contentStr),
				lipgloss.NewLayer(modalStr.Content).
					X(max(0, bx)).
					Y(max(0, by)).Z(1)).Render()

		}
	}

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
		// Inspector overlay intercept: click outside closes; inside routes to
		// the inspector child view (wheel/drag/click handling).
		if m.inspector.IsVisible() {
			bx, by, bw, bh := m.inspectorOverlayBounds[0], m.inspectorOverlayBounds[1], m.inspectorOverlayBounds[2], m.inspectorOverlayBounds[3]
			me := mm.Mouse()
			inside := me.X >= bx && me.X < bx+bw && me.Y >= by && me.Y < by+bh
			if rel, ok := mm.(tea.MouseReleaseMsg); ok && !inside {
				_ = rel
				m.inspector.ToggleVisible()
				m.inspectorOverlayBounds = [4]int{}
				return m.handleResizeCmd()
			}
			if inside {
				iv := m.inspector.View()
				if iv.OnMouse != nil {
					offX, offY := bx+1, by+1
					nm := tea.Mouse{X: me.X - offX, Y: me.Y - offY, Button: me.Button, Mod: me.Mod}
					switch mm.(type) {
					case tea.MouseClickMsg:
						return iv.OnMouse(tea.MouseClickMsg(nm))
					case tea.MouseReleaseMsg:
						return iv.OnMouse(tea.MouseReleaseMsg(nm))
					case tea.MouseMotionMsg:
						return iv.OnMouse(tea.MouseMotionMsg(nm))
					case tea.MouseWheelMsg:
						return iv.OnMouse(tea.MouseWheelMsg(nm))
					}
				}
				return nil
			}
			return nil
		}

		// Notification history panel intercept: when the panel is open, clicks
		// outside it close the panel; wheel events inside scroll the list.
		if m.status.IsHistoryVisible() {
			switch ev := mm.(type) {
			case tea.MouseReleaseMsg:
				me := ev.Mouse()
				bx, by, bw, bh := m.historyOverlayBounds[0], m.historyOverlayBounds[1], m.historyOverlayBounds[2], m.historyOverlayBounds[3]
				if me.X < bx || me.X >= bx+bw || me.Y < by || me.Y >= by+bh {
					return tea.Batch(
						m.status.ToggleNotifications(),
						m.handleResizeCmd(),
					)
				}
				// Inside panel — let status bar's own handler process it.
				return nil
			case tea.MouseWheelMsg:
				me := ev.Mouse()
				bx, by, bw, bh := m.historyOverlayBounds[0], m.historyOverlayBounds[1], m.historyOverlayBounds[2], m.historyOverlayBounds[3]
				if me.X >= bx && me.X < bx+bw && me.Y >= by && me.Y < by+bh {
					if me.Button == tea.MouseWheelUp {
						m.status.NotifHistoryCursorUp()
					} else {
						count := 0
						if m.notifMgr != nil {
							count = len(m.notifMgr.Active())
						}
						m.status.NotifHistoryCursorDown(count)
					}
					return m.handleResizeCmd()
				}
				return nil
			}
			return nil
		}

		// Info modal intercept: when the modal is open, only mouse events
		// inside its bounding box are passed through (as scroll commands);
		// a release outside the box sends CloseInfoModalMsg.
		if m.infoModal.IsVisible() {
			switch ev := mm.(type) {
			case tea.MouseReleaseMsg:
				me := ev.Mouse()
				bx, by, bw, bh := m.infoModal.Bounds()
				if me.X < bx || me.X >= bx+bw || me.Y < by || me.Y >= by+bh {
					// Click was outside the modal — close it.
					return func() tea.Msg { return status.CloseInfoModalMsg{} }
				}
				// Inside the modal — consume without routing to children.
				return nil
			case tea.MouseWheelMsg:
				me := ev.Mouse()
				bx, by, bw, bh := m.infoModal.Bounds()
				if me.X >= bx && me.X < bx+bw && me.Y >= by && me.Y < by+bh {
					up := me.Button == tea.MouseWheelUp
					return func() tea.Msg { return status.InfoModalScrollMsg{Up: up} }
				}
				return nil
			}
			return nil // block all other mouse events from children
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
					return debug.MouseHighlightMsg{GlobalX: mEvent.X, GlobalY: mEvent.Y, Child: childName, OffX: offX, OffY: offY}
				})
			default:
				return nil
			}
		}

		// compute status height and main layout height
		mainHeight := max(m.height-statusHeight, 0)

		// route based on nav layout
		if m.navigationVisible && m.nav != nil {
			switch nav := m.nav.(type) {
			case *navigation.Sidebar:
				navW := nav.Width()
				// click in main layout area
				mmPos := mm.Mouse()
				if mmPos.Y < mainHeight {
					if mmPos.X < navW {
						return route(navView, 0, 0, "sidebar")
					}
					// Content area click: release sidebar focus so the border
					// and highlight reset immediately on the next render.
					if m.sidebarFocused {
						m.sidebarFocused = false
						nav.SetFocused(false)
					}
					return route(activePageView, navW, 0, "content")
				}
			case *navigation.Tabs:
				navH := nav.Height()
				mmPos := mm.Mouse()
				if mmPos.Y < navH {
					return route(navView, 0, 0, "tabs")
				}
				if mmPos.Y < mainHeight {
					return route(activePageView, 0, navH, "content")
				}
			default:
				// unknown nav layout: route everything in main area to active page
				mmPos := mm.Mouse()
				if mmPos.Y < mainHeight {
					return route(activePageView, 0, 0, "content")
				}
			}
		} else {
			// nav hidden -> content occupies main area
			mmPos := mm.Mouse()
			if mmPos.Y < mainHeight {
				return route(activePageView, 0, 0, "content")
			}
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

// reapplyBg replaces every ANSI reset (\x1b[m or \x1b[0m) with that reset
// immediately followed by the given background escape code.
func reapplyBg(s string, bg color.Color) string {
	bgCode := firstEscapeFromStyle(lipgloss.NewStyle().Background(bg).Render("X"))
	if bgCode == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+bgCode)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+bgCode)
	return s
}

// firstEscapeFromStyle extracts the first ANSI escape sequence from a lipgloss
// render result.
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

// syncTerminalColorsCmd force-applies terminal default foreground/background
// colors via OSC, even when the renderer thinks the values are unchanged.
// This keeps terminal frame/tab edge colors in sync with the active theme.
func (m *RouterModel) syncTerminalColorsCmd() tea.Cmd {
	bg := colorHex(m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetBackground()))
	fg := colorHex(m.colorProfile.Convert(m.colors.Styles.TextOnBg.GetForeground()))
	seq := ansi.SetBackgroundColor(bg) + ansi.SetForegroundColor(fg)
	return tea.Raw(seq)
}

func (m *RouterModel) syncTerminalColorsAfterCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return syncTerminalColorsMsg{}
	})
}

func colorHex(c color.Color) string {
	if c == nil {
		return "#000000"
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
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

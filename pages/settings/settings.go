package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/tui-base/config"
	"github.com/jarvisfriends/tui-base/logging"
	"github.com/jarvisfriends/tui-base/overlay"
	"github.com/jarvisfriends/tui-base/page"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

const defaultSettingsFile = "tui_settings.json"

// initRegistryOnce ensures tint.NewDefaultRegistry is called exactly once
// across the process lifetime. NewDefaultRegistry writes to a module-level
// global inside bubbletint; calling it from concurrent goroutines (e.g.
// parallel tests that each call settings.New) produces a data race.
var initRegistryOnce sync.Once

// configDirMu guards the configDir package-level variable. Tests that call
// settings.New in parallel each invoke SetConfigDir, which would otherwise
// produce a write-write data race on the unguarded string.
var configDirMu sync.RWMutex

// configDir is the directory tui_settings.json is read from and written to.
// Empty means the current working directory (legacy behavior). The router sets
// this to the per-app OS config directory via SetConfigDir before settings.New
// so settings persist in a stable, user-appropriate location rather than
// wherever the binary happened to be launched from.
var configDir string

// SetConfigDir sets the directory used to persist tui_settings.json. Call this
// before settings.New. An empty string restores current-working-directory
// behavior. The directory is created on first save if it does not exist.
func SetConfigDir(dir string) {
	configDirMu.Lock()
	configDir = dir
	configDirMu.Unlock()
}

// settingsFilePath returns the absolute (or CWD-relative) path to the settings
// JSON file, honoring any directory set via SetConfigDir.
func settingsFilePath() string {
	configDirMu.RLock()
	dir := configDir
	configDirMu.RUnlock()
	if dir == "" {
		if base, err := os.UserConfigDir(); err == nil {
			dir = filepath.Join(base, "tui-base")
		} else {
			dir = filepath.Join(os.TempDir(), "tui-base")
		}
	}
	return filepath.Join(dir, defaultSettingsFile)
}

// NavStyleMsg is emitted when the user selects a different navigation style.
type NavStyleMsg struct{ Style string }

// NavShowNumbersMsg is emitted when the user toggles the leading per-item number
// prefix on number-capable navs (the minimal top nav).
type NavShowNumbersMsg struct{ Show bool }

// NavNumberSelectMsg is emitted when the user toggles number-key page selection
// (pressing 1–9 to jump directly to a page) on top-docked navs. It is disabled
// by default; the router only honors digit shortcuts while it is enabled.
type NavNumberSelectMsg struct{ Enabled bool }

// ThemeMsg is emitted when the user selects a different color theme.
type ThemeMsg struct {
	ID               string
	Mode             string
	Style            string
	Accessibility    bool
	ApplyPreferences bool
}

// NotificationsSettingsMsg is emitted when the user saves notification settings
// so the router can apply them to the shared *notifications.Manager at runtime.
type NotificationsSettingsMsg struct {
	Enabled bool
	Persist bool
}

// settingItem describes one row in the compact overview and how to edit it.
type settingItem struct {
	category  string
	title     string
	value     func() string    // returns current display value for the overview row
	buildForm func() *huh.Form // builds a single-field overlay form for this setting
	leftTrunc bool             // if true, show tail of value with leading … (useful for paths)
	apply     func() error     // optional callback after submit
}

type settingsCategory struct {
	title      string
	itemIdxSet []int
}

type overviewEntry struct {
	header    string
	itemIndex int
	isHeader  bool
}

type overviewLayout struct {
	entries      []overviewEntry
	columns      int
	rowsPerCol   int
	gap          int
	colWidth     int
	listTopY     int
	cursorEntry  int
	visibleCount int
}

type Keys struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Dismiss key.Binding
}

func (k *Keys) ShortHelp() []key.Binding {
	if k == nil {
		return nil
	}
	return []key.Binding{k.Up, k.Down, k.Select}
}

func (k *Keys) FullHelp() [][]key.Binding {
	if k == nil {
		return nil
	}
	return [][]key.Binding{{k.Up, k.Down}, {k.Select}}
}

func DefaultKeys() *Keys {
	return &Keys{
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "dismiss modal/overlay"),
		),
	}
}

// SettingsModel is the settings page. It has two modes:
//
//   - Overview: a compact list showing every setting on one line with its current
//     value. Up/Down moves the cursor; Enter or click opens an edit overlay.
//
//   - Editing: a centred huh form (one field) is composited over the overview using
//     the lipgloss Compositor. Submitting or aborting the form returns to overview.
type SettingsModel struct {
	page.Base

	// Persisted fields (exported so JSON encoding works).
	NavStyle             string `json:"nav_style"`
	NavShowNumbers       bool   `json:"nav_show_numbers"`
	NavNumberSelect      bool   `json:"nav_number_select"`
	ColorThemeID         string `json:"theme_id"`
	ThemeMode            string `json:"theme_mode"`
	StylePreset          string `json:"style_preset"`
	AccessibilityColors  bool   `json:"accessibility_colors"`
	LogOutput            string `json:"log_output"`
	LogPath              string `json:"log_path"`
	LogLevel             string `json:"log_level"`
	NotificationsEnabled bool   `json:"notifications_enabled"`
	NotificationsPersist bool   `json:"notifications_persist"`
	// defaultTerminal holds the current machine default terminal handling on
	// Windows. It is intentionally unexported so it's not persisted to the
	// JSON settings file; the value is always read from / written to the
	// registry directly.
	defaultTerminal string

	// intermediate string forms for huh selects (not persisted directly)
	notifEnabledStr    string
	notifPersistStr    string
	accessibilityStr   string
	navNumbersStr      string
	navNumberSelectStr string

	extraSections []config.Section
	items         []settingItem
	categories    []settingsCategory

	// loadedFromFile reports whether a persisted settings file was found and
	// read at startup. When false, the router applies first-run defaults (e.g.
	// tabs navigation) instead of the struct defaults.
	loadedFromFile bool

	// Overview state.
	cursor int
	// scrollTop is the first visible overview entry in the flattened
	// category + item list used by the responsive overview layout.
	scrollTop int

	// editOverlay manages the centered huh form shown when the user opens a
	// setting for editing. FormOverlayHost handles sizing, compositing, and
	// outside-click bounds — no manual geom.Rect or compositor calls needed.
	editOverlay overlay.FormOverlayHost
	editIndex   int

	keys *Keys
}

// LoadedFromFile reports whether a persisted settings file was found and read
// at startup. The router uses this to detect a first run (and persist defaults).
func (m *SettingsModel) LoadedFromFile() bool { return m.loadedFromFile }

// Save persists the current settings synchronously to the configured path.
// Use this for one-off saves (e.g. writing first-run defaults at startup).
func (m *SettingsModel) Save() error {
	if err := m.SaveToFile(settingsFilePath()); err != nil {
		return err
	}
	m.loadedFromFile = true
	return nil
}

// New creates a settings model. Pass extra config.Sections contributed by
// Configurable components; they appear after the built-in rows.
func New(extraSections ...config.Section) *SettingsModel {
	m := &SettingsModel{
		NavStyle:             "sidebar",
		NavShowNumbers:       false,
		NavNumberSelect:      false,
		ColorThemeID:         "dracula_plus",
		ThemeMode:            theme.ThemeModeDark,
		StylePreset:          string(theme.DefaultStylePreset),
		AccessibilityColors:  false,
		LogOutput:            "temp",
		LogPath:              "",
		LogLevel:             "ERROR",
		NotificationsEnabled: true,
		NotificationsPersist: false,
		defaultTerminal:      "let_windows",
		extraSections:        extraSections,
		keys:                 DefaultKeys(),
	}
	if err := m.LoadFromFile(settingsFilePath()); err == nil {
		m.loadedFromFile = true
	} else {
		// First run (no persisted settings): default to tabs navigation, which
		// is friendlier than the sidebar for a brand-new app. Theme mode defaults
		// to dark; the router's BackgroundColorMsg handler detects the terminal
		// background at startup and flips to light live when appropriate. We do
		// NOT query the terminal here: a synchronous query in New() blocks on
		// stdin in non-interactive contexts (tests, pipes) and can race the
		// program's own background query.
		m.NavStyle = "tabs"
	}

	// Initialize the bubbletint global registry exactly once. The library is not
	// goroutine-safe; calling NewDefaultRegistry from concurrent goroutines races
	// on the module-level registry pointer.
	initRegistryOnce.Do(tint.NewDefaultRegistry)
	if m.ThemeMode == "" {
		m.ThemeMode = theme.ThemeModeDark
	}
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.StylePreset = string(theme.NormalizePreset(m.StylePreset))
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
	theme.SetThemePreferences(m.ThemeMode, m.AccessibilityColors, theme.StylePreset(m.StylePreset))
	if m.ColorThemeID != "" {
		_ = theme.SetCurrentTint(m.ColorThemeID)
	}

	// Always detect the current system default terminal so the UI reflects
	// the real machine state. Detection is a no-op on non-Windows builds.
	if det, err := detectDefaultTerminal(); err == nil && det != "" {
		m.defaultTerminal = det
	}

	m.buildItems()
	return m
}

// buildItems constructs the settingItem slice. Call this once in New() and
// again after LoadFromFile (pointer addresses stay stable; only values change).
func (m *SettingsModel) buildItems() {
	m.categories = nil
	m.items = nil

	addItem := func(category string, item settingItem) {
		item.category = category
		idx := len(m.items)
		m.items = append(m.items, item)
		for i := range m.categories {
			if m.categories[i].title == category {
				m.categories[i].itemIdxSet = append(m.categories[i].itemIdxSet, idx)
				return
			}
		}
		m.categories = append(m.categories, settingsCategory{title: category, itemIdxSet: []int{idx}})
	}

	// sync intermediate strings from persisted bools
	if m.NotificationsEnabled {
		m.notifEnabledStr = "true"
	} else {
		m.notifEnabledStr = "false"
	}
	if m.NotificationsPersist {
		m.notifPersistStr = "true"
	} else {
		m.notifPersistStr = "false"
	}
	if m.AccessibilityColors {
		m.accessibilityStr = "true"
	} else {
		m.accessibilityStr = "false"
	}
	if m.NavShowNumbers {
		m.navNumbersStr = "true"
	} else {
		m.navNumbersStr = "false"
	}
	if m.NavNumberSelect {
		m.navNumberSelectStr = "true"
	} else {
		m.navNumberSelectStr = "false"
	}
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.StylePreset = string(theme.NormalizePreset(m.StylePreset))
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
	navOpts := []huh.Option[string]{
		huh.NewOption("Sidebar - vertical panel on the left", "sidebar"),
		huh.NewOption("Tabs    - horizontal bar at the top", "tabs"),
		huh.NewOption("Top Nav - minimal horizontal bar (inspector-style)", "topnav"),
	}
	navNumbersOpts := []huh.Option[string]{
		huh.NewOption("Off", "false"),
		huh.NewOption("On", "true"),
	}
	modeOpts := []huh.Option[string]{
		huh.NewOption("Dark", theme.ThemeModeDark),
		huh.NewOption("Light", theme.ThemeModeLight),
	}
	styleOpts := make([]huh.Option[string], 0, len(theme.StylePresets()))
	for _, p := range theme.StylePresets() {
		styleOpts = append(styleOpts, huh.NewOption(p.DisplayName(), string(p)))
	}
	accessibilityOpts := []huh.Option[string]{
		huh.NewOption("Off", "false"),
		huh.NewOption("On", "true"),
	}
	logOpts := []huh.Option[string]{
		huh.NewOption("Temporary directory (default)", "temp"),
		huh.NewOption("Fixed directory", "dir"),
		huh.NewOption("Fixed file", "file"),
	}
	levelOpts := []huh.Option[string]{
		huh.NewOption("Debug (verbose)", "DEBUG"),
		huh.NewOption("Info (normal)", "INFO"),
		huh.NewOption("Warn (warnings only)", "WARN"),
		huh.NewOption("Error (errors only)", "ERROR"),
	}

	// Terminal options only apply on Windows. On other platforms we omit the
	// setting entirely to avoid confusing users with irrelevant OS controls.
	var terminalOpts []huh.Option[string]
	if runtime.GOOS == "windows" {
		terminalOpts = []huh.Option[string]{
			huh.NewOption("Let Windows Decide (system default)", "let_windows"),
			huh.NewOption("Windows Console Host (Classic ConHost) — legacy", "classic"),
			huh.NewOption("Windows Terminal (Modern) — recommended", "modern"),
		}
	}

	for _, sec := range m.extraSections {
		cat := sec.Title
		if cat == "" {
			cat = "Other"
		}
		for _, def := range sec.Fields {
			addItem(cat, m.itemFromDef(def))
		}
	}

	addItem("Navigation", settingItem{
		title: "Navigation Style",
		value: func() string { return labelFor(m.NavStyle, navOpts) },
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Navigation Style").
					Description("How the navigation chrome is displayed").
					Options(navOpts...).
					Value(&m.NavStyle),
			).WithTheme(theme.HuhThemeFunc()))
		},
	})
	addItem("Navigation", settingItem{
		title: "Show Nav Numbers",
		value: func() string {
			if m.NavShowNumbers {
				return "On"
			}
			return "Off"
		},
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Show Nav Numbers").
					Description("Show a leading number on each Top Nav item (pairs well with Number Key Select)").
					Options(navNumbersOpts...).
					Value(&m.navNumbersStr),
			).WithTheme(theme.HuhThemeFunc()))
		},
	})
	addItem("Navigation", settingItem{
		title: "Number Key Select",
		value: func() string {
			if m.NavNumberSelect {
				return "On"
			}
			return "Off"
		},
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Number Key Select").
					Description("Press 1–9 to jump directly to a Tabs / Top Nav page (off by default)").
					Options(navNumbersOpts...).
					Value(&m.navNumberSelectStr),
			).WithTheme(theme.HuhThemeFunc()))
		},
	})
	addItem("Logging", settingItem{
		title: "Log Destination",
		value: func() string { return labelFor(m.LogOutput, logOpts) },
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Log Destination").
					Description("Where runtime logs are written").
					Options(logOpts...).
					Value(&m.LogOutput),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)
	addItem("Logging", settingItem{
		title:     "Log Path",
		leftTrunc: true,
		value: func() string {
			if m.LogPath == "" {
				return "(system temp)"
			}
			return m.LogPath
		},
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewFilePicker().
					Title("Log Path").
					Description("Directory or file, ignored when destination is Temporary").
					DirAllowed(true).
					FileAllowed(true).
					Value(&m.LogPath),
			).WithTheme(theme.HuhThemeFunc()))
		},
	})
	addItem("Logging", settingItem{
		title: "Log Level",
		value: func() string { return labelFor(m.LogLevel, levelOpts) },
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Log Level").
					Description("Minimum severity recorded to file and shown in inspector").
					Options(levelOpts...).
					Value(&m.LogLevel),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)
	addItem("Appearance", settingItem{
		title: "Theme Mode",
		value: func() string { return labelFor(m.ThemeMode, modeOpts) },
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Theme Mode").
					Description("Choose between dark and light themes for the theme picker").
					Options(modeOpts...).
					Value(&m.ThemeMode),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)
	addItem("Appearance", settingItem{
		title: "Color Theme",
		value: func() string { return tintDisplayName(m.ColorThemeID) },
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Color Theme").
					Description("Up/Down to browse - applied immediately as you scroll").
					Options(buildThemeOptions(m.ThemeMode)...).
					Height(14).
					Value(&m.ColorThemeID),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)
	addItem("Appearance", settingItem{
		title: "Form Style",
		value: func() string { return labelFor(m.StylePreset, styleOpts) },
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Form Style").
					Description("Border, prefix, and indicator style for forms - colors come from the Color Theme").
					Options(styleOpts...).
					Value(&m.StylePreset),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)
	addItem("Appearance", settingItem{
		title: "Accessibility Colors",
		value: func() string {
			if m.AccessibilityColors {
				return "On"
			}
			return "Off"
		},
		buildForm: func() *huh.Form {
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Accessibility Colors").
					Description("Apply accessibility adjustments to foreground colors app-wide").
					Options(accessibilityOpts...).
					Value(&m.accessibilityStr),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)
	addItem("Notifications", settingItem{
		title: "Bell Notifications",
		value: func() string {
			if m.NotificationsEnabled {
				return "Enabled 🔔"
			}
			return "Disabled 🔕"
		},
		buildForm: func() *huh.Form {
			notifOpts := []huh.Option[string]{
				huh.NewOption("Enabled", "true"),
				huh.NewOption("Disabled", "false"),
			}
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Bell Notifications").
					Description("Show toast pop-ups for info, warnings, and errors").
					Options(notifOpts...).
					Value(&m.notifEnabledStr),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)
	addItem("Notifications", settingItem{
		title: "Notification Persistence",
		value: func() string {
			if m.NotificationsPersist {
				return "On"
			}
			return "Off"
		},
		buildForm: func() *huh.Form {
			persistOpts := []huh.Option[string]{
				huh.NewOption("Off (session only)", "false"),
				huh.NewOption("On (saved to disk)", "true"),
			}
			return huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Notification Persistence").
					Description("Store notifications in config dir across restarts").
					Options(persistOpts...).
					Value(&m.notifPersistStr),
			).WithTheme(theme.HuhThemeFunc()))
		},
	},
	)

	if runtime.GOOS == "windows" {
		addItem("System", settingItem{
			title: "Default Terminal",
			value: func() string { return labelFor(m.defaultTerminal, terminalOpts) },
			buildForm: func() *huh.Form {
				return huh.NewForm(huh.NewGroup(
					huh.NewSelect[string]().
						Title("Default Terminal").
						Description("Choose the OS default terminal delegation (Windows only).").
						Options(terminalOpts...).
						Value(&m.defaultTerminal),
				).WithTheme(theme.HuhThemeFunc()))
			},
			apply: func() error {
				switch m.defaultTerminal {
				case "let_windows":
					return applyTerminalSetting("{00000000-0000-0000-0000-000000000000}", "{00000000-0000-0000-0000-000000000000}")
				case "classic":
					return applyTerminalSetting("{B23D10C0-E52E-411E-9D5B-C09FDF709C7D}", "{B23D10C0-E52E-411E-9D5B-C09FDF709C7D}")
				case "modern":
					return applyTerminalSetting("{2EACA947-7F5F-4CFA-BA87-8F7FBEEFBE69}", "{E12CFF52-A866-4C77-9A90-F570A7AA2C6B}")
				default:
					return nil
				}
			},
		})
	}
}

// itemFromDef builds a settingItem from a config.FieldDef for extra sections.
func (m *SettingsModel) itemFromDef(def config.FieldDef) settingItem {
	return settingItem{
		title:     def.Title,
		leftTrunc: def.Kind == config.FieldFilePicker,
		value: func() string {
			if def.Value == nil {
				return ""
			}
			// For Select fields, try to look up the human-readable label.
			if def.Kind == config.FieldSelect {
				return labelFor(*def.Value, def.Options)
			}
			return *def.Value
		},
		buildForm: func() *huh.Form {
			f := m.huhFieldFromDef(def)
			if f == nil {
				return nil
			}
			return huh.NewForm(huh.NewGroup(f).WithTheme(theme.HuhThemeFunc()))
		},
		apply: func() error {
			if def.Apply == nil || def.Value == nil {
				return nil
			}
			return def.Apply(*def.Value)
		},
	}
}

// huhFieldFromDef creates a huh.Field from a config.FieldDef.
func (m *SettingsModel) huhFieldFromDef(def config.FieldDef) huh.Field {
	switch def.Kind {
	case config.FieldSelect:
		s := huh.NewSelect[string]().
			Title(def.Title).
			Description(def.Description).
			Options(def.Options...).
			Value(def.Value)
		if def.Height > 0 {
			s = s.Height(def.Height)
		}
		if def.Validate != nil {
			s = s.Validate(def.Validate)
		}
		return s
	case config.FieldText:
		ti := huh.NewInput().
			Title(def.Title).
			Description(def.Description).
			Value(def.Value)
		if def.Validate != nil {
			ti = ti.Validate(def.Validate)
		}
		return ti
	case config.FieldFilePicker:
		return huh.NewFilePicker().
			Title(def.Title).
			Description(def.Description).
			DirAllowed(def.DirAllowed).
			FileAllowed(def.FileAllowed).
			Value(def.Value)
	}
	return nil
}

// CapturesKeys returns true while an edit overlay is open so the router hands
// all keystrokes directly to the form.
func (m *SettingsModel) CapturesKeys() bool { return m.editOverlay.IsOpen() }

// ShortHelp implements help.KeyMap for the status bar.
func (m *SettingsModel) ShortHelp() []key.Binding {
	if m == nil || m.keys == nil {
		return nil
	}
	if m.editOverlay.IsOpen() {
		return []key.Binding{m.keys.Dismiss, m.keys.Select}
	}
	return m.keys.ShortHelp()
}

// FullHelp implements help.KeyMap for the status bar.
func (m *SettingsModel) FullHelp() [][]key.Binding {
	if m == nil || m.keys == nil {
		return nil
	}
	if m.editOverlay.IsOpen() {
		return [][]key.Binding{{m.keys.Dismiss, m.keys.Select}}
	}
	return m.keys.FullHelp()
}

func (m *SettingsModel) Init() tea.Cmd { return nil }

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(wMsg.Width, wMsg.Height)
		m.editOverlay.OnResize(m.Width(), m.Height())
	}
	if m.Width() == 0 {
		return m, nil
	}
	if m.editOverlay.IsOpen() {
		return m.updateEditing(msg)
	}
	return m.updateOverview(msg)
}

func (m *SettingsModel) updateOverview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		keyMsg := msg
		switch {
		case key.Matches(keyMsg, m.keys.Up):
			m.cursor = max(m.cursor-1, 0)
			m.ensureCursorVisible()
		case key.Matches(keyMsg, m.keys.Down):
			m.cursor = min(m.cursor+1, max(len(m.items)-1, 0))
			m.ensureCursorVisible()
		case key.Matches(keyMsg, m.keys.Select):
			return m, m.startEdit()
		}
	case tea.MouseWheelMsg:
		if msg.Mouse().Button == tea.MouseWheelUp {
			m.cursor = max(m.cursor-1, 0)
		} else {
			m.cursor = min(m.cursor+1, max(len(m.items)-1, 0))
		}
		m.ensureCursorVisible()
	}
	return m, nil
}

func (m *SettingsModel) startEdit() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	m.editIndex = m.cursor
	// For the system default terminal, refresh the value from the OS so the
	// edit form always reflects the current registry state.
	if m.items[m.cursor].title == "Default Terminal" {
		if det, err := detectDefaultTerminal(); err == nil && det != "" {
			m.defaultTerminal = det
		}
	}
	f := m.items[m.cursor].buildForm()
	if f == nil {
		return nil
	}
	return m.editOverlay.Open(f, m.Width(), m.Height())
}

// abortEdit reverts to the last persisted state and closes the overlay.
func (m *SettingsModel) abortEdit() tea.Cmd {
	m.editOverlay.Close()
	_ = m.LoadFromFile(settingsFilePath())
	m.buildItems()
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.StylePreset = string(theme.NormalizePreset(m.StylePreset))
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
	id := m.ColorThemeID
	mode := m.ThemeMode
	style := m.StylePreset
	accessibility := m.AccessibilityColors
	return func() tea.Msg {
		return ThemeMsg{ID: id, Mode: mode, Style: style, Accessibility: accessibility, ApplyPreferences: true}
	}
}

func (m *SettingsModel) updateEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Esc closes the overlay and reverts any unsaved/live-preview changes.
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, m.keys.Dismiss) {
		return m, m.abortEdit()
	}

	// Translate mouse wheel events to key presses for the edit overlay so
	// users can scroll select controls with a mouse wheel even though huh
	// lacks mouse support. Wheel up -> KeyUp, Wheel down -> KeyDown.
	if wm, ok := msg.(tea.MouseWheelMsg); ok {
		if wm.Mouse().Button == tea.MouseWheelUp {
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		} else {
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		}
	}

	prevTheme := m.ColorThemeID
	prevThemeMode := m.ThemeMode
	prevStyle := m.StylePreset
	prevAccessibility := m.AccessibilityColors
	prevLevel := m.LogLevel
	prevNav := m.NavStyle
	prevNavNumbers := m.NavShowNumbers
	prevNavNumberSelect := m.NavNumberSelect

	state, cmd := m.editOverlay.Update(msg)
	var cmds []tea.Cmd
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.AccessibilityColors = m.accessibilityStr == "true"
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.StylePreset = string(theme.NormalizePreset(m.StylePreset))
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)

	// Live theme preview: fires while editing theme-related options.
	if m.ColorThemeID != prevTheme || m.ThemeMode != prevThemeMode || m.StylePreset != prevStyle || m.AccessibilityColors != prevAccessibility {
		id := m.ColorThemeID
		mode := m.ThemeMode
		style := m.StylePreset
		accessibility := m.AccessibilityColors
		cmds = append(cmds, func() tea.Msg {
			return ThemeMsg{ID: id, Mode: mode, Style: style, Accessibility: accessibility, ApplyPreferences: true}
		})
	}
	if m.LogLevel != prevLevel {
		_ = logging.SetLevel(m.LogLevel)
	}
	if m.NavStyle != prevNav {
		nav := m.NavStyle
		cmds = append(cmds, func() tea.Msg { return NavStyleMsg{Style: nav} })
	}
	m.NavShowNumbers = m.navNumbersStr == "true"
	if m.NavShowNumbers != prevNavNumbers {
		show := m.NavShowNumbers
		cmds = append(cmds, func() tea.Msg { return NavShowNumbersMsg{Show: show} })
	}
	m.NavNumberSelect = m.navNumberSelectStr == "true"
	if m.NavNumberSelect != prevNavNumberSelect {
		enabled := m.NavNumberSelect
		cmds = append(cmds, func() tea.Msg { return NavNumberSelectMsg{Enabled: enabled} })
	}

	switch state {
	case huh.StateCompleted:
		if m.editIndex >= 0 && m.editIndex < len(m.items) {
			if apply := m.items[m.editIndex].apply; apply != nil {
				if err := apply(); err != nil {
					logging.Errorf("Settings: failed applying field %q: %v", m.items[m.editIndex].title, err)
				}
			}
		}
		// sync notification bool fields from intermediate string values
		m.NotificationsEnabled = m.notifEnabledStr == "true"
		m.NotificationsPersist = m.notifPersistStr == "true"
		m.AccessibilityColors = m.accessibilityStr == "true"
		m.NavShowNumbers = m.navNumbersStr == "true"
		m.NavNumberSelect = m.navNumberSelectStr == "true"
		m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
		m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
		// propagate notification settings to the shared manager at runtime
		notifEnabled, notifPersist := m.NotificationsEnabled, m.NotificationsPersist
		cmds = append(cmds, func() tea.Msg {
			return NotificationsSettingsMsg{Enabled: notifEnabled, Persist: notifPersist}
		})
		// S-13: persist asynchronously so a slow disk never blocks Update. The
		// snapshot is taken now (on the UI goroutine) so the background write
		// sees a consistent copy even if the user keeps editing.
		cmds = append(cmds, m.saveCmd())
		m.loadedFromFile = true
		if path, err := logging.InitFromSettings(m.LogOutput, m.LogPath); err == nil {
			m.LogPath = path
			logging.Infof("Settings: log path updated to %s", path)
		} else {
			logging.Errorf("Settings: failed to initialise logging: %v", err)
		}
		_ = logging.SetLevel(m.LogLevel)
		m.editOverlay.Close()

	case huh.StateAborted:
		// huh's own abort key (ctrl+c inside the form) — same revert path.
		cmds = append(cmds, m.abortEdit())
	}

	return m, tea.Batch(cmds...)
}

func (m *SettingsModel) View() tea.View {
	c := m.Colors()
	overview := m.renderOverview()
	content := m.editOverlay.Composite(overview, c.Styles.OverlayBorder)

	v := tea.NewView(content)
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	// Click anywhere on the overview to jump the cursor to that row and open
	// its edit overlay. Coordinates here are child-relative (router translates
	// them before calling OnMouse).
	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		click, ok := mm.(tea.MouseClickMsg)
		if !ok {
			return nil
		}
		if m.editOverlay.IsOpen() {
			// Click outside the overlay aborts and returns to overview.
			if m.editOverlay.IsOutsideClick(click.X, click.Y) {
				return m.abortEdit()
			}
			return nil
		}
		layout := m.overviewLayout()
		if click.Y < layout.listTopY || click.Y >= layout.listTopY+layout.rowsPerCol {
			return nil
		}
		relY := click.Y - layout.listTopY
		cellW := layout.colWidth + layout.gap
		if cellW <= 0 {
			return nil
		}
		col := click.X / cellW
		if col < 0 || col >= layout.columns {
			return nil
		}
		if (click.X % cellW) >= layout.colWidth {
			return nil
		}
		visibleIdx := col*layout.rowsPerCol + relY
		entryIdx := m.scrollTop + visibleIdx
		if entryIdx < 0 || entryIdx >= len(layout.entries) {
			return nil
		}
		entry := layout.entries[entryIdx]
		if entry.isHeader {
			return nil
		}
		m.cursor = entry.itemIndex
		if m.cursor >= 0 && m.cursor < len(m.items) {
			m.ensureCursorVisible()
			return m.startEdit()
		}
		return nil
	}

	return v
}

// renderOverview renders the compact one-line-per-setting list.
func (m *SettingsModel) renderOverview() string {
	c := m.Colors()
	layout := m.overviewLayout()

	labelW := min(24, max(12, layout.colWidth/2))
	valueW := max(layout.colWidth-labelW-3, 1)

	// The selected row uses the theme's semantic, contrast-guarded selection
	// colors (SelectionBg/Fg) so it matches the sidebar and table widgets, rather
	// than borrowing the tab-hover affordance background (which is a hover hint,
	// not a selection indicator).
	selBg := c.SelectionBg
	selFg := c.SelectionFg
	normalLabel := c.Styles.TextOnBg.Width(labelW)
	normalValue := c.Styles.Subtitle.Width(valueW)
	cursorLabel := c.Styles.SelectedItem.Width(labelW)
	cursorValue := c.Styles.TextOnBg.Foreground(selFg).Background(selBg).Width(valueW)
	cursorBg := c.Styles.Row.Background(selBg).Width(layout.colWidth)
	headerStyle := c.Styles.Subtitle.Bold(true).Width(layout.colWidth)
	emptyRow := lipgloss.NewStyle().Width(layout.colWidth).Render("")
	titleStyle := c.Styles.Title

	lines := make([]string, 0, 4)
	lines = append(lines, titleStyle.Render("Settings"))
	lines = append(lines, "") // blank separator — part of headerLines

	m.ensureCursorVisible()
	layout = m.overviewLayout()
	visible := layout.visibleEntries(m.scrollTop)
	cols := make([]string, 0, layout.columns)
	for col := 0; col < layout.columns; col++ {
		colLines := make([]string, 0, layout.rowsPerCol)
		for row := 0; row < layout.rowsPerCol; row++ {
			i := col*layout.rowsPerCol + row
			if i >= len(visible) {
				colLines = append(colLines, emptyRow)
				continue
			}
			entry := visible[i]
			if entry.isHeader {
				colLines = append(colLines, headerStyle.Render(entry.header))
				continue
			}
			item := m.items[entry.itemIndex]
			lbl := ansi.Truncate(item.title, labelW, "…")
			v := item.value()
			val := ansi.Truncate(v, valueW, "…")
			if item.leftTrunc && ansi.StringWidth(v) > valueW {
				// Keep the tail (e.g. a file path's end) when it overflows.
				val = ansi.TruncateLeft(v, ansi.StringWidth(v)-valueW+1, "…")
			}
			if entry.itemIndex == m.cursor {
				indicatorStyle := lipgloss.NewStyle().Foreground(selFg).Background(selBg)
				spaceStyle := lipgloss.NewStyle().Background(selBg)
				rowText := indicatorStyle.Render("▶ ") + cursorLabel.Render(lbl) + spaceStyle.Render(" ") + cursorValue.Render(val)
				colLines = append(colLines, cursorBg.Render(rowText))
				continue
			}
			colLines = append(colLines, "  "+normalLabel.Render(lbl)+" "+normalValue.Render(val))
		}
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Top, colLines...))
	}
	if len(cols) > 0 {
		if layout.gap > 0 && len(cols) > 1 {
			withGap := make([]string, 0, len(cols)*2-1)
			gapBlock := lipgloss.NewStyle().Width(layout.gap).Render("")
			for i, col := range cols {
				if i > 0 {
					withGap = append(withGap, gapBlock)
				}
				withGap = append(withGap, col)
			}
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, withGap...))
		} else {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, cols...))
		}
	}

	return c.Styles.TextOnBg.
		Width(m.Width()).
		Height(m.Height()).
		Padding(1, 0).
		Render(lipgloss.JoinVertical(lipgloss.Top, lines...))
}

func (m *SettingsModel) ensureCursorVisible() {
	if len(m.items) == 0 {
		m.cursor = 0
		m.scrollTop = 0
		return
	}
	m.cursor = max(m.cursor, 0)
	m.cursor = min(m.cursor, len(m.items)-1)
	layout := m.overviewLayout()
	if layout.visibleCount <= 0 {
		m.scrollTop = 0
		return
	}
	cursorEntry := layout.cursorEntry
	m.scrollTop = max(m.scrollTop, 0)
	maxStart := max(len(layout.entries)-layout.visibleCount, 0)
	m.scrollTop = min(cursorEntry, min(m.scrollTop, maxStart))
	if cursorEntry >= m.scrollTop+layout.visibleCount {
		m.scrollTop = cursorEntry - layout.visibleCount + 1
	}
	m.scrollTop = max(m.scrollTop, 0)
	m.scrollTop = min(m.scrollTop, maxStart)
}

func (m *SettingsModel) overviewLayout() overviewLayout {
	entries := m.flattenedOverviewEntries()
	cursorEntry := 0
	for i, e := range entries {
		if !e.isHeader && e.itemIndex == m.cursor {
			cursorEntry = i
			break
		}
	}

	innerW := max(m.Width()-4, 20)
	innerH := max(m.Height()-2, 1)
	listTopY := 3 // top padding + title + blank separator
	listHeight := max(innerH-2, 1)

	gap := 2
	colPref := m.preferredColumnWidth()
	columns := 1
	maxCols := min(3, len(entries))
	for c := 2; c <= maxCols; c++ {
		needed := c*colPref + (c-1)*gap
		if needed <= innerW {
			columns = c
		}
	}
	colWidth := max((innerW-(columns-1)*gap)/columns, 20)
	visibleCount := max(listHeight*columns, 1)

	return overviewLayout{
		entries:      entries,
		columns:      columns,
		rowsPerCol:   listHeight,
		gap:          gap,
		colWidth:     colWidth,
		listTopY:     listTopY,
		cursorEntry:  cursorEntry,
		visibleCount: visibleCount,
	}
}

func (l overviewLayout) visibleEntries(scrollTop int) []overviewEntry {
	if len(l.entries) == 0 {
		return nil
	}
	start := min(max(scrollTop, 0), len(l.entries))
	end := min(start+l.visibleCount, len(l.entries))
	if start >= end {
		return nil
	}
	return l.entries[start:end]
}

func (m *SettingsModel) flattenedOverviewEntries() []overviewEntry {
	if len(m.categories) == 0 {
		return nil
	}
	entries := make([]overviewEntry, 0, len(m.items)+len(m.categories))
	for _, cat := range m.categories {
		if len(cat.itemIdxSet) == 0 {
			continue
		}
		entries = append(entries, overviewEntry{header: cat.title, isHeader: true})
		for _, idx := range cat.itemIdxSet {
			entries = append(entries, overviewEntry{itemIndex: idx})
		}
	}
	return entries
}

func (m *SettingsModel) preferredColumnWidth() int {
	maxLabel := 0
	maxValue := 0
	for _, item := range m.items {
		maxLabel = max(maxLabel, lipgloss.Width(item.title))
		if item.title == "Log Path" {
			continue
		}
		maxValue = max(maxValue, lipgloss.Width(item.value()))
	}
	labelW := min(max(maxLabel, 12), 28)
	valueW := min(max(maxValue, 12), 40)
	return labelW + valueW + 3 // cursor prefix + separator
}

// SettingsSavedMsg is emitted after an async settings save completes. Err is
// non-nil when the write failed. Apps may ignore it; the router does not
// require handling.
type SettingsSavedMsg struct {
	Path string
	Err  error
}

// saveCmd returns a tea.Cmd that persists the current settings to disk off the
// UID goroutine. It snapshots the JSON synchronously (cheap, and guarantees a
// consistent view) and performs only the file write in the background.
func (m *SettingsModel) saveCmd() tea.Cmd {
	path := settingsFilePath()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return func() tea.Msg { return SettingsSavedMsg{Path: path, Err: err} }
	}
	return func() tea.Msg {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return SettingsSavedMsg{Path: path, Err: mkErr}
			}
		}
		writeErr := os.WriteFile(path, append(data, '\n'), 0o644)
		return SettingsSavedMsg{Path: path, Err: writeErr}
	}
}

// SaveToFile writes settings to the given filename as JSON, synchronously.
// Prefer the async path (saveCmd) inside Update; this remains for callers that
// need a blocking save (tests, explicit export).
func (m *SettingsModel) SaveToFile(filename string) error {
	if dir := filepath.Dir(filename); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// LoadFromFile loads settings from the given filename if it exists.
func (m *SettingsModel) LoadFromFile(filename string) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, m)
}

// labelFor returns the human-readable Key for the given value in an option
// slice, falling back to the raw value if not found.
func labelFor(val string, opts []huh.Option[string]) string {
	for _, o := range opts {
		if o.Value == val {
			return o.Key
		}
	}
	return val
}

// tintDisplayName returns the display name for a tint ID.
func tintDisplayName(id string) string {
	for _, t := range tint.Tints() {
		if t.ID == id {
			return t.DisplayName
		}
	}
	return id
}

// buildThemeOptions generates one huh.Option per registered tint.
func buildThemeOptions(mode string) []huh.Option[string] {
	tints := tint.Tints()
	wantDark := theme.NormalizeMode(mode) == theme.ThemeModeDark
	opts := make([]huh.Option[string], 0, len(tints))
	baseSwatch := theme.Active().Styles.SwatchDot
	for _, t := range tints {
		if t.Dark != wantDark {
			continue
		}
		opts = append(opts, huh.NewOption(themeOptionKey(t, baseSwatch), t.ID))
	}
	return opts
}

// themeOptionKey builds the display string for one tint entry showing the
// theme name followed by color swatches for at-a-glance palette preview.
func themeOptionKey(t *tint.Tint, baseSwatch lipgloss.Style) string {
	swatch := func(dotStyle lipgloss.Style, fg *tint.Color) string {
		hex := "#444444"
		if fg == nil {
			return dotStyle.Foreground(lipgloss.Color(hex)).Render("  ")
		}
		return dotStyle.Foreground(fg).Render("● ")
	}
	sBgColor := t.Bg
	if t.SelectionBg != nil {
		sBgColor = t.SelectionBg
	}

	name := fmt.Sprintf("%-30s", t.DisplayName)
	currentTheme := baseSwatch.Foreground(t.Fg).Background(t.Bg)
	idName := currentTheme.Render(name, " ")

	dotsStyle := baseSwatch.Background(sBgColor)
	idDots := lipgloss.JoinHorizontal(lipgloss.Right,
		swatch(dotsStyle, t.Cursor),
		" ",
		swatch(dotsStyle, t.BrightBlack),
		swatch(dotsStyle, t.Black),
		swatch(dotsStyle, t.BrightBlue),
		swatch(dotsStyle, t.Blue),
		swatch(dotsStyle, t.BrightCyan),
		swatch(dotsStyle, t.Cyan),
		swatch(dotsStyle, t.BrightGreen),
		" ",
		swatch(dotsStyle, t.Green),
		swatch(dotsStyle, t.BrightPurple),
		swatch(dotsStyle, t.Purple),
		swatch(dotsStyle, t.BrightRed),
		swatch(dotsStyle, t.Red),
		swatch(dotsStyle, t.BrightWhite),
		swatch(dotsStyle, t.White),
		swatch(dotsStyle, t.Yellow),
		swatch(dotsStyle, t.BrightYellow),
		" ")

	return lipgloss.JoinHorizontal(lipgloss.Left, idName, idDots)
}

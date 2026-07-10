package settings

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/snap/datepicker"
	"github.com/jarvisfriends/snap/gate"
	"github.com/jarvisfriends/snap/keys"
	"github.com/jarvisfriends/snap/page"
	"github.com/jarvisfriends/snap/pickers"
	"github.com/jarvisfriends/snap/timepicker"
	"github.com/jarvisfriends/tui-base/common"
	"github.com/jarvisfriends/tui-base/config"
	"github.com/jarvisfriends/tui-base/envpath"
	"github.com/jarvisfriends/tui-base/logging"
	"github.com/jarvisfriends/tui-base/overlay"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

const defaultSettingsFile = "tui_settings.json"

const (
	boolStrTrue   = "true"
	boolStrFalse  = "false"
	settingValOff = "Off"
	settingValOn  = "On"

	// Option keys for the "Default Terminal" setting; the winterm mapping
	// lives in terminal.go.
	defTerminalLetWindows = "let_windows"
	defTerminalClassic    = "classic"
	defTerminalModern     = "modern"

	itemTitleLogPath = "Log Path"

	// Log destination values persisted in LogOutput.
	logOutputTemp = "temp"
	logOutputDir  = "dir"
	logOutputFile = "file"
)

// initRegistryOnce ensures tint.NewDefaultRegistry is called exactly once
// across the process lifetime. NewDefaultRegistry writes to a module-level
// global inside bubbletint; calling it from concurrent goroutines (e.g.
// parallel tests that each call settings.NewWithOptions) produces a data race.
var initRegistryOnce sync.Once

// configDirMu guards the configDir package-level variable. Tests that call
// settings.NewWithOptions in parallel each invoke SetConfigDir, which would otherwise
// produce a write-write data race on the unguarded string.
var configDirMu sync.RWMutex

// configDir is the directory tui_settings.json is read from and written to.
// Empty means the current working directory (legacy behavior). The router sets
// this to the per-app OS config directory via SetConfigDir before settings.NewWithOptions
// so settings persist in a stable, user-appropriate location rather than
// wherever the binary happened to be launched from.
var configDir string

// SetConfigDir sets the directory used to persist tui_settings.json. Call this
// before settings.NewWithOptions. An empty string restores current-working-directory
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

// KeybindingsChangedMsg is emitted when the user modifies custom keybindings
// so the router can apply them to the shared *keys.AppKeyMap at runtime.
type KeybindingsChangedMsg struct {
	CustomKeys map[string]string
}

// GatesChangedMsg is emitted when the user flips a feature gate on the
// settings page (Feature Flags section). The shared *gate.GateRegistry is
// already updated when this fires — the message is the "re-derive your
// gate-dependent UI now" signal so changes show immediately; Values is a
// snapshot for convenience. Gate values are runtime-only and are never
// persisted to the settings file: startup state comes from each gate's
// Default and the <APPNAME>_GATE_<NAME> environment overrides.
type GatesChangedMsg struct {
	Values map[string]bool
}

// A settingItem represents a single configurable value.
type settingItem struct {
	category   string
	title      string
	leftTrunc  bool
	value      func() string
	setValue   func(string)
	buildForm  func() *huh.Form
	buildModel func() tea.Model
	apply      func() error
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
//   - Editing: a centered huh form (one field) is composited over the overview using
//     the lipgloss Compositor. Submitting or aborting the form returns to overview.
type SettingsModel struct {
	page.Base

	// Persisted fields (exported so JSON encoding works).
	NavStyle             string            `json:"nav_style,omitempty"`
	NavShowNumbers       bool              `json:"nav_show_numbers,omitempty"`
	NavNumberSelect      bool              `json:"nav_number_select,omitempty"`
	ColorThemeID         string            `json:"theme_id,omitempty"`
	ThemeMode            string            `json:"theme_mode,omitempty"`
	StylePreset          string            `json:"style_preset,omitempty"`
	AccessibilityColors  bool              `json:"accessibility_colors,omitempty"`
	LogOutput            string            `json:"log_output,omitempty"`
	LogPath              string            `json:"log_path,omitempty"`
	LogLevel             string            `json:"log_level,omitempty"`
	NotificationsEnabled bool              `json:"notifications_enabled,omitempty"`
	NotificationsPersist bool              `json:"notifications_persist,omitempty"`
	CustomKeys           map[string]string `json:"custom_keys,omitempty"`
	// defaultTerminal holds the current machine default terminal handling on
	// Windows. It is intentionally unexported so it's not persisted to the
	// JSON settings file; the value is always read from / written to the
	// registry directly.
	defaultTerminal string

	// gatesChanged records that a feature-gate apply mutated the registry
	// during the current edit; the StateCompleted handler consumes it to emit
	// a single GatesChangedMsg. Gates are runtime-only and never persisted.
	gatesChanged bool
	// gateEditVals holds each Feature Flags item's live edit binding (the
	// string the huh select writes into). Keyed by gate name; rebuilt with the
	// items. Kept on the model so tests can drive the commit path the same way
	// the form does.
	gateEditVals map[string]*string

	// intermediate string forms for huh selects (not persisted directly)
	notifEnabledStr    string
	notifPersistStr    string
	accessibilityStr   string
	navNumbersStr      string
	navNumberSelectStr string

	extraSections []config.Section[string]
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
	// outside clicks.
	editOverlay  overlay.FormOverlayHost
	modelOverlay overlay.ModelOverlayHost

	// editIndex is the index of the setting currently being edited, or -1.
	editIndex int

	keys *Keys
	opts Options
}

// Options provides configuration for the settings UI.
type Options struct {
	ExtraSections []config.Section[string]
	DefaultKeys   *keys.AppKeyMap
	Gates         *gate.GateRegistry
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

// NewWithOptions creates a settings model with advanced configuration.
func NewWithOptions(opts Options) *SettingsModel {
	m := &SettingsModel{
		NavStyle:             "sidebar",
		NavShowNumbers:       false,
		NavNumberSelect:      false,
		ColorThemeID:         "dracula_plus",
		ThemeMode:            theme.ThemeModeDark,
		StylePreset:          string(theme.DefaultStylePreset),
		AccessibilityColors:  false,
		LogOutput:            logOutputTemp,
		LogPath:              "",
		LogLevel:             "ERROR",
		NotificationsEnabled: true,
		NotificationsPersist: false,
		CustomKeys:           make(map[string]string),
		defaultTerminal:      defTerminalLetWindows,
		extraSections:        opts.ExtraSections,
		keys:                 DefaultKeys(),
		opts:                 opts,
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
	// T-4: user-authored YAML themes load from <config-dir>/themes into the
	// registry, appearing in the Theme selector next to the built-ins. One bad
	// file never hides the rest; problems land in the log.
	themesDir := filepath.Join(filepath.Dir(settingsFilePath()), "themes")
	if n, errs := theme.RegisterYAMLTints(themesDir); n > 0 || len(errs) > 0 {
		logging.Infof("Settings: loaded %d custom theme(s) from %s", n, themesDir)
		for _, e := range errs {
			logging.Warnf("Settings: custom theme skipped: %v", e)
		}
	}
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

	if m.CustomKeys == nil {
		m.CustomKeys = make(map[string]string)
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
		m.categories = append(
			m.categories,
			settingsCategory{title: category, itemIdxSet: []int{idx}},
		)
	}

	// sync intermediate strings from persisted bools
	if m.NotificationsEnabled {
		m.notifEnabledStr = boolStrTrue
	} else {
		m.notifEnabledStr = boolStrFalse
	}
	if m.NotificationsPersist {
		m.notifPersistStr = boolStrTrue
	} else {
		m.notifPersistStr = boolStrFalse
	}
	if m.AccessibilityColors {
		m.accessibilityStr = boolStrTrue
	} else {
		m.accessibilityStr = boolStrFalse
	}
	if m.NavShowNumbers {
		m.navNumbersStr = boolStrTrue
	} else {
		m.navNumbersStr = boolStrFalse
	}
	if m.NavNumberSelect {
		m.navNumberSelectStr = boolStrTrue
	} else {
		m.navNumberSelectStr = boolStrFalse
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
		huh.NewOption(settingValOff, boolStrFalse),
		huh.NewOption(settingValOn, boolStrTrue),
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
		huh.NewOption(settingValOff, boolStrFalse),
		huh.NewOption(settingValOn, boolStrTrue),
	}
	logOpts := []huh.Option[string]{
		huh.NewOption("Temporary directory (default)", logOutputTemp),
		huh.NewOption("Fixed directory", logOutputDir),
		huh.NewOption("Fixed file", logOutputFile),
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
			huh.NewOption("Let Windows Decide (system default)", defTerminalLetWindows),
			huh.NewOption("Windows Console Host (Classic ConHost) — legacy", defTerminalClassic),
			huh.NewOption("Windows Terminal (Modern) — recommended", defTerminalModern),
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
				return settingValOn
			}
			return settingValOff
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
				return settingValOn
			}
			return settingValOff
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
	addItem(
		"Logging", settingItem{
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
		title:     itemTitleLogPath,
		leftTrunc: true,
		value: func() string {
			if m.LogPath == "" {
				return "(system temp)"
			}
			return m.LogPath
		},
		setValue: func(val string) { m.LogPath = val },
		// Fixed-directory destination: browse directories only (files hidden).
		buildModel: func() tea.Model {
			if m.LogOutput != logOutputDir {
				return nil
			}
			dp := newThemedDirPicker(m.LogPath)
			dp.Width, dp.Height = m.Width(), m.Height()
			return dp
		},
		buildForm: func() *huh.Form {
			// Temporary destination ignores the path — nothing to edit, so
			// don't open a picker at all.
			if m.LogOutput != logOutputFile {
				return nil
			}
			return huh.NewForm(huh.NewGroup(
				huh.NewFilePicker().
					Title(itemTitleLogPath).
					Description("Log file to write to").
					DirAllowed(false).
					FileAllowed(true).
					// Open directly in browse mode and fill most of the page:
					// the embedded picker otherwise defaults to a one-row list.
					Picking(true).
					Height(overlay.FormHeight(m.Height())).
					Value(&m.LogPath),
			).WithTheme(theme.HuhThemeFunc())).WithKeyMap(pickers.FilePickerKeyMap())
		},
	})
	addItem(
		"Logging", settingItem{
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
	addItem(
		"Appearance", settingItem{
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
	addItem(
		"Appearance", settingItem{
			title: "Color Theme",
			value: func() string { return tintDisplayName(m.ColorThemeID) },
			buildForm: func() *huh.Form {
				return huh.NewForm(huh.NewGroup(
					huh.NewSelect[string]().
						Title("Color Theme").
						Description("↑/↓ to browse - applied immediately as you scroll").
						Options(buildThemeOptions(m.ThemeMode)...).
						Height(14).
						Value(&m.ColorThemeID),
				).WithTheme(theme.HuhThemeFunc()))
			},
		},
	)
	addItem(
		"Appearance", settingItem{
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
	addItem(
		"Appearance", settingItem{
			title: "Accessibility Colors",
			value: func() string {
				if m.AccessibilityColors {
					return settingValOn
				}
				return settingValOff
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
	addItem(
		"Notifications", settingItem{
			title: "Bell Notifications",
			value: func() string {
				if m.NotificationsEnabled {
					return "Enabled 🔔"
				}
				return "Disabled 🔕"
			},
			buildForm: func() *huh.Form {
				notifOpts := []huh.Option[string]{
					huh.NewOption("Enabled", boolStrTrue),
					huh.NewOption("Disabled", boolStrFalse),
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
	addItem(
		"Notifications", settingItem{
			title: "Notification Persistence",
			value: func() string {
				if m.NotificationsPersist {
					return settingValOn
				}
				return settingValOff
			},
			buildForm: func() *huh.Form {
				persistOpts := []huh.Option[string]{
					huh.NewOption("Off (session only)", boolStrFalse),
					huh.NewOption("On (saved to disk)", boolStrTrue),
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

	var keyOptions []keys.BindingDef
	if m.opts.DefaultKeys != nil {
		keyOptions = m.opts.DefaultKeys.BindingDefs()
	} else {
		keyOptions = keys.DefaultKeyMap().BindingDefs()
	}

	for _, kOpt := range keyOptions {
		// Create a local copy for the closures
		opt := kOpt
		addItem("Keybindings", settingItem{
			title: opt.Title,
			value: func() string {
				val, exists := m.CustomKeys[opt.ID]
				if !exists {
					return opt.Def
				}
				if val == "" {
					return "(none)"
				}
				return val
			},
			buildModel: func() tea.Model {
				val, exists := m.CustomKeys[opt.ID]
				if !exists {
					val = opt.Def
				}
				kr := NewKeyRecorder(val)
				kr.Validate = func(keys []string) error {
					seen := make(map[string]bool)
					for _, k := range keys {
						kNorm := strings.ToLower(strings.TrimSpace(k))
						if kNorm == "" {
							continue
						}
						if seen[kNorm] {
							return fmt.Errorf("duplicate key %q within this shortcut", k)
						}
						seen[kNorm] = true
					}

					for _, other := range keyOptions {
						if other.ID == opt.ID {
							continue
						}
						// If the other shortcut has this key mapped, it's a conflict
						val, exists := m.CustomKeys[other.ID]
						if !exists {
							val = other.Def
						}
						var otherKeys []string
						parts := strings.SplitSeq(val, ",")
						for p := range parts {
							pTrim := strings.ToLower(strings.TrimSpace(p))
							if pTrim != "" && pTrim != "(none)" {
								otherKeys = append(otherKeys, pTrim)
							}
						}
						for _, k := range keys {
							kNorm := strings.ToLower(strings.TrimSpace(k))
							if kNorm == "" {
								continue
							}
							if slices.Contains(otherKeys, kNorm) {
								return fmt.Errorf(
									"key %q is already assigned to %q",
									k,
									other.Title,
								)
							}
						}
					}
					return nil
				}
				kr.validate()
				return kr
			},
			setValue: func(val string) {
				m.CustomKeys[opt.ID] = val
			},
		})
	}

	if runtime.GOOS == "windows" {
		addItem("System", settingItem{
			title: "Default Terminal",
			value: func() string { return labelFor(m.defaultTerminal, terminalOpts) },
			buildForm: func() *huh.Form {
				return huh.NewForm(huh.NewGroup(
					huh.NewSelect[string]().
						Title("Default Terminal").
						Description("Choose the OS default terminal delegation (Windows only). " +
							"Takes effect for newly opened console windows.").
						Options(terminalOpts...).
						Value(&m.defaultTerminal),
				).WithTheme(theme.HuhThemeFunc()))
			},
			apply: func() error {
				return applyTerminalSetting(m.defaultTerminal)
			},
		})
	}

	if m.opts.Gates != nil {
		m.gateEditVals = make(map[string]*string, len(m.opts.Gates.Defs()))
		for _, gateDef := range m.opts.Gates.Defs() {
			gDef := gateDef // capture
			// editVal outlives buildForm so the huh select's pointer binding
			// still holds the user's choice when apply runs on completion.
			// (Binding to a local inside buildForm silently discards the edit.)
			editVal := new(string)
			m.gateEditVals[gDef.Name] = editVal
			addItem("Feature Flags", settingItem{
				title: gDef.Name,
				value: func() string {
					if m.opts.Gates.Value(gDef.Name) {
						return "Enabled"
					}
					return "Disabled"
				},
				buildForm: func() *huh.Form {
					opts := []huh.Option[string]{
						huh.NewOption("Disabled", boolStrFalse),
						huh.NewOption("Enabled", boolStrTrue),
					}
					*editVal = boolStrFalse
					if m.opts.Gates.Value(gDef.Name) {
						*editVal = boolStrTrue
					}
					return huh.NewForm(huh.NewGroup(
						huh.NewSelect[string]().
							Title(gDef.Name).
							Description(gDef.Description).
							Options(opts...).
							Value(editVal),
					).WithTheme(theme.HuhThemeFunc()))
				},
				apply: func() error {
					enabled := *editVal == boolStrTrue
					if enabled != m.opts.Gates.Value(gDef.Name) {
						m.opts.Gates.Set(gDef.Name, enabled)
						// Flag for the StateCompleted handler, which emits one
						// GatesChangedMsg after all commit work is done.
						m.gatesChanged = true
					}
					return nil
				},
			})
		}
	}
}

// itemFromDef builds a settingItem from a config.FieldDef for extra sections.
func (m *SettingsModel) itemFromDef(def config.FieldDef[string]) settingItem {
	return settingItem{
		title:     def.Title,
		leftTrunc: def.Kind == config.FieldFilePicker,
		value: func() string {
			if def.Value == nil {
				return ""
			}
			val := *def.Value
			if def.Kind == config.FieldSelect {
				return labelFor(val, def.Options)
			}
			if isSecret(def.Title) {
				runes := []rune(val)
				if len(runes) > 4 {
					return "****" + string(runes[len(runes)-4:])
				} else if len(runes) > 0 {
					return strings.Repeat("*", len(runes))
				}
				return ""
			}
			if def.Kind == config.FieldFilePicker || def.Kind == config.FieldMultiFilePicker ||
				strings.Contains(strings.ToLower(def.Title), "path") {
				return envpath.Collapse(val)
			}
			if def.Kind == config.FieldCustom && def.CustomFieldText != "" {
				return def.CustomFieldText
			}
			return val
		},
		setValue: func(val string) {
			if def.Value != nil {
				*def.Value = val
			}
		},
		buildForm: func() *huh.Form {
			if def.Kind == config.FieldMultiFilePicker || def.Kind == config.FieldDuration ||
				def.Kind == config.FieldDate ||
				def.Kind == config.FieldCustom {
				return nil
			}
			f := m.huhFieldFromDef(def)
			if f == nil {
				return nil
			}
			form := huh.NewForm(huh.NewGroup(f).WithTheme(theme.HuhThemeFunc()))
			if def.Kind == config.FieldFilePicker {
				form = form.WithKeyMap(pickers.FilePickerKeyMap())
			}
			return form
		},
		buildModel: func() tea.Model {
			switch def.Kind {
			case config.FieldFilePicker:
				// Directory-only pickers browse with files hidden entirely;
				// mixed or file pickers stay on the huh form (buildForm).
				if !def.DirAllowed || def.FileAllowed {
					return nil
				}
				dp := newThemedDirPicker(*def.Value)
				dp.Width, dp.Height = m.Width(), m.Height()
				return dp
			case config.FieldMultiFilePicker:
				e := newThemedMultiFileEditor(*def.Value)
				// A directory-only field browses with the DirPicker (folders
				// and drives, no files) instead of the mixed file browser.
				e.DirsOnly = def.DirAllowed && !def.FileAllowed
				// Seed the current page size: ModelOverlayHost only sends
				// WindowSizeMsg on later resizes, and the editor needs the
				// height up front to size its file-picker form.
				e.Width, e.Height = m.Width(), m.Height()
				return e
			case config.FieldDuration:
				d, _ := time.ParseDuration(*def.Value)
				tp := timepicker.New(d)
				applyTimePickerTheme(tp, m.Colors())
				return tp
			case config.FieldDate:
				t, _ := time.Parse("2006-01-02", *def.Value)
				if t.IsZero() {
					t = time.Now()
				}
				dp := datepicker.New(t)
				dp.Styles = themedDatePickerStyles(m.Colors())
				return dp
			case config.FieldCustom:
				if def.CustomModelBuilder != nil {
					return def.CustomModelBuilder()
				}
			case config.FieldSelect, config.FieldText:
				// handled via huh form, not a model overlay
			}
			return nil
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
func (m *SettingsModel) huhFieldFromDef(def config.FieldDef[string]) huh.Field {
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
		if isSecret(def.Title) {
			ti = ti.EchoMode(huh.EchoModePassword)
		}
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
			// Open directly in browse mode and fill most of the page: the
			// embedded picker otherwise defaults to a one-row list.
			Picking(true).
			Height(overlay.FormHeight(m.Height())).
			Value(def.Value)
	case config.FieldMultiFilePicker, config.FieldDate, config.FieldDuration, config.FieldCustom:
		// handled via model overlay, not a huh field
	}
	return nil
}

func isSecret(title string) bool {
	t := strings.ToLower(title)
	return strings.Contains(t, "password") || strings.Contains(t, "pass") ||
		strings.Contains(t, "token") ||
		strings.Contains(t, "secret")
}

// CapturesKeys returns true while an edit overlay is open so the router hands
// all keystrokes directly to the form.
func (m *SettingsModel) CapturesKeys() bool { return m.editOverlay.IsOpen() || m.modelOverlay.IsOpen() }

// ShortHelp implements help.KeyMap for the status bar.
func (m *SettingsModel) ShortHelp() []key.Binding {
	if m == nil || m.keys == nil {
		return nil
	}
	if m.editOverlay.IsOpen() || m.modelOverlay.IsOpen() {
		return []key.Binding{m.keys.Dismiss, m.keys.Select}
	}
	return m.keys.ShortHelp()
}

// FullHelp implements help.KeyMap for the status bar.
func (m *SettingsModel) FullHelp() [][]key.Binding {
	if m == nil || m.keys == nil {
		return nil
	}
	if m.editOverlay.IsOpen() || m.modelOverlay.IsOpen() {
		return [][]key.Binding{{m.keys.Dismiss, m.keys.Select}}
	}
	return m.keys.FullHelp()
}

func (m *SettingsModel) Init() tea.Cmd { return nil }

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(wMsg.Width, wMsg.Height)
		m.editOverlay.OnResize(m.Width(), m.Height())
		m.modelOverlay.OnResize(m.Width(), m.Height())
	}
	if m.Width() == 0 {
		return m, nil
	}
	if m.editOverlay.IsOpen() || m.modelOverlay.IsOpen() {
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
	if m.items[m.cursor].buildModel != nil {
		customModel := m.items[m.cursor].buildModel()
		if customModel != nil {
			return m.modelOverlay.Open(customModel, m.Width(), m.Height())
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
	m.modelOverlay.Close()
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
		return ThemeMsg{
			ID:               id,
			Mode:             mode,
			Style:            style,
			Accessibility:    accessibility,
			ApplyPreferences: true,
		}
	}
}

func (m *SettingsModel) updateActiveOverlay(msg tea.Msg) (huh.FormState, tea.Cmd) {
	if !m.modelOverlay.IsOpen() {
		return m.editOverlay.Update(msg)
	}
	cmd := m.modelOverlay.Update(msg)
	mod := m.modelOverlay.Model()
	switch v := mod.(type) {
	case *pickers.MultiFileEditor:
		if v.Done {
			if m.editIndex >= 0 && m.items[m.editIndex].setValue != nil {
				m.items[m.editIndex].setValue(v.Value())
			}
			return huh.StateCompleted, cmd
		} else if v.Aborted {
			return huh.StateAborted, cmd
		}
	case *KeyRecorder:
		if v.Done {
			if m.editIndex >= 0 && m.items[m.editIndex].setValue != nil {
				m.items[m.editIndex].setValue(v.Value())
			}
			return huh.StateCompleted, cmd
		} else if v.Aborted {
			return huh.StateAborted, cmd
		}
	case *pickers.DirPicker:
		if v.Done {
			if m.editIndex >= 0 && m.items[m.editIndex].setValue != nil {
				m.items[m.editIndex].setValue(v.Value())
			}
			return huh.StateCompleted, cmd
		} else if v.Aborted {
			return huh.StateAborted, cmd
		}
	case *timepicker.TimePickerModel:
		if v.Done {
			if m.editIndex >= 0 && m.items[m.editIndex].setValue != nil {
				m.items[m.editIndex].setValue(v.Duration.String())
			}
			return huh.StateCompleted, cmd
		} else if v.Aborted {
			return huh.StateAborted, cmd
		}
	case *datepicker.DatePickerModel:
		if v.Selected {
			if m.editIndex >= 0 && m.items[m.editIndex].setValue != nil {
				m.items[m.editIndex].setValue(v.Time.Format("2006-01-02"))
			}
			return huh.StateCompleted, cmd
		} else if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, v.KeyMap.Quit) {
			return huh.StateAborted, cmd
		}
	case interface {
		IsDone() bool
		IsAborted() bool
	}:
		if v.IsDone() {
			return huh.StateCompleted, cmd
		} else if v.IsAborted() {
			return huh.StateAborted, cmd
		}
	}
	return huh.StateNormal, cmd
}

func (m *SettingsModel) updateEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Esc closes the overlay and reverts any unsaved/live-preview changes.
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, m.keys.Dismiss) {
		return m, m.abortEdit()
	}

	// Mouse routing while overlays are open: hosted model overlays receive
	// mouse exclusively via View.OnMouse (translated coordinates); letting the
	// raw page-relative event through here would double-deliver it with wrong
	// coordinates. The huh edit overlay has no mouse support, so its wheel
	// events become arrow keys instead.
	if _, isMouse := msg.(tea.MouseMsg); isMouse && m.modelOverlay.IsOpen() {
		return m, nil
	}
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

	var state huh.FormState
	var cmd tea.Cmd
	var cmds []tea.Cmd

	state, cmd = m.updateActiveOverlay(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.AccessibilityColors = m.accessibilityStr == boolStrTrue
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.StylePreset = string(theme.NormalizePreset(m.StylePreset))
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)

	// Live theme preview: fires while editing theme-related options.
	if m.ColorThemeID != prevTheme || m.ThemeMode != prevThemeMode || m.StylePreset != prevStyle ||
		m.AccessibilityColors != prevAccessibility {
		id := m.ColorThemeID
		mode := m.ThemeMode
		style := m.StylePreset
		accessibility := m.AccessibilityColors
		cmds = append(cmds, func() tea.Msg {
			return ThemeMsg{
				ID:               id,
				Mode:             mode,
				Style:            style,
				Accessibility:    accessibility,
				ApplyPreferences: true,
			}
		})
	}
	if m.LogLevel != prevLevel {
		_ = logging.SetLevel(m.LogLevel)
	}
	if m.NavStyle != prevNav {
		nav := m.NavStyle
		cmds = append(cmds, func() tea.Msg { return NavStyleMsg{Style: nav} })
	}
	m.NavShowNumbers = m.navNumbersStr == boolStrTrue
	if m.NavShowNumbers != prevNavNumbers {
		show := m.NavShowNumbers
		cmds = append(cmds, func() tea.Msg { return NavShowNumbersMsg{Show: show} })
	}
	m.NavNumberSelect = m.navNumberSelectStr == boolStrTrue
	if m.NavNumberSelect != prevNavNumberSelect {
		enabled := m.NavNumberSelect
		cmds = append(cmds, func() tea.Msg { return NavNumberSelectMsg{Enabled: enabled} })
	}

	switch state {
	case huh.StateCompleted:
		if m.editIndex >= 0 && m.editIndex < len(m.items) {
			if apply := m.items[m.editIndex].apply; apply != nil {
				if err := apply(); err != nil {
					logging.Errorf(
						"Settings: failed applying field %q: %v",
						m.items[m.editIndex].title,
						err,
					)
				}
			}
		}
		// sync notification bool fields from intermediate string values
		m.NotificationsEnabled = m.notifEnabledStr == boolStrTrue
		m.NotificationsPersist = m.notifPersistStr == boolStrTrue
		m.AccessibilityColors = m.accessibilityStr == boolStrTrue
		m.NavShowNumbers = m.navNumbersStr == boolStrTrue
		m.NavNumberSelect = m.navNumberSelectStr == boolStrTrue
		m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
		m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
		// propagate notification settings to the shared manager at runtime
		notifEnabled, notifPersist := m.NotificationsEnabled, m.NotificationsPersist
		customKeys := make(map[string]string)
		maps.Copy(customKeys, m.CustomKeys)
		// S-13: persist asynchronously so a slow disk never blocks Update. The
		// snapshot is taken now (on the UI goroutine) so the background write
		// sees a consistent copy even if the user keeps editing.
		cmds = append(
			cmds,
			func() tea.Msg { return NotificationsSettingsMsg{Enabled: notifEnabled, Persist: notifPersist} },
			func() tea.Msg { return KeybindingsChangedMsg{CustomKeys: customKeys} },
			m.saveCmd(),
		)
		// A feature-gate flip broadcasts immediately so gated UI (pages,
		// inspector tabs) reacts without waiting for other interaction. The
		// registry itself was already updated in the item's apply.
		if m.gatesChanged && m.opts.Gates != nil {
			m.gatesChanged = false
			values := m.opts.Gates.Snapshot()
			cmds = append(cmds, func() tea.Msg { return GatesChangedMsg{Values: values} })
		}
		m.loadedFromFile = true
		if path, err := logging.InitFromSettings(m.LogOutput, m.LogPath); err == nil {
			m.LogPath = path
			logging.Infof("Settings: log path updated to %s", path)
		} else {
			logging.Errorf("Settings: failed to initialize logging: %v", err)
		}
		_ = logging.SetLevel(m.LogLevel)
		m.editOverlay.Close()
		m.modelOverlay.Close()

	case huh.StateAborted:
		// huh's own abort key (ctrl+c inside the form) — same revert path.
		cmds = append(cmds, m.abortEdit())
	case huh.StateNormal:
		// form still in progress — no action
	}

	return m, tea.Batch(cmds...)
}

func (m *SettingsModel) View() tea.View {
	c := m.Colors()
	overview := m.renderOverview()

	var content string
	if m.modelOverlay.IsOpen() {
		content = m.modelOverlay.Composite(overview, c.Styles.OverlayBorder)
	} else {
		content = m.editOverlay.Composite(overview, c.Styles.OverlayBorder)
	}

	v := tea.NewView(content)
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	// Click anywhere on the overview to jump the cursor to that row and open
	// its edit overlay. Coordinates here are child-relative (router translates
	// them before calling OnMouse).
	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		// A hosted model overlay (dir picker, date/time picker, ...) gets the
		// mouse first: outside clicks dismiss it, everything else is forwarded
		// with coordinates translated into the hosted content's space — that
		// is what makes snap's click/wheel hit zones line up live, not just in
		// unit tests. Mouse reaches hosted models ONLY through this path (the
		// Update path drops mouse while the overlay is open, mirroring the
		// router's own OnMouse/Update modality gate).
		if m.modelOverlay.IsOpen() {
			if click, ok := mm.(tea.MouseClickMsg); ok &&
				m.modelOverlay.IsOutsideClick(click.X, click.Y) {
				return m.abortEdit()
			}
			return m.modelOverlay.ForwardMouse(mm)
		}
		click, ok := mm.(tea.MouseClickMsg)
		if !ok {
			return nil
		}
		if m.editOverlay.IsOpen() {
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
	lines = append(
		lines,
		titleStyle.Render("Settings"),
		"",
	) // blank separator — part of headerLines

	m.ensureCursorVisible()
	layout = m.overviewLayout()
	visible := layout.visibleEntries(m.scrollTop)
	cols := make([]string, 0, layout.columns)
	for col := range layout.columns {
		colLines := make([]string, 0, layout.rowsPerCol)
		for row := range layout.rowsPerCol {
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
				rowText := indicatorStyle.Render(
					"▶ ",
				) + cursorLabel.Render(
					lbl,
				) + spaceStyle.Render(
					" ",
				) + cursorValue.Render(
					val,
				)
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
		if item.title == itemTitleLogPath {
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
			if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
				return SettingsSavedMsg{Path: path, Err: mkErr}
			}
		}
		writeErr := common.WriteFileAtomic(path, append(data, '\n'), 0o600)
		return SettingsSavedMsg{Path: path, Err: writeErr}
	}
}

// SaveToFile writes settings to the given filename as JSON, synchronously and
// atomically (temp file + rename). Prefer the async path (saveCmd) inside
// Update; this remains for callers that need a blocking save (tests, explicit
// export).
func (m *SettingsModel) SaveToFile(filename string) error {
	if dir := filepath.Dir(filename); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return common.WriteFileAtomic(filename, append(data, '\n'), 0o600)
}

// LoadFromFile loads settings from the given filename if it exists.
func (m *SettingsModel) LoadFromFile(filename string) error {
	b, err := os.ReadFile(filepath.Clean(filename))
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

package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jarvisfriends/tui-base/config"
	"github.com/jarvisfriends/tui-base/logging"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

const defaultSettingsFile = "tui_settings.json"

// configDir is the directory tui_settings.json is read from and written to.
// Empty means the current working directory (legacy behavior). The router sets
// this to the per-app OS config directory via SetConfigDir before settings.New
// so settings persist in a stable, user-appropriate location rather than
// wherever the binary happened to be launched from.
var configDir string

// SetConfigDir sets the directory used to persist tui_settings.json. Call this
// before settings.New. An empty string restores current-working-directory
// behavior. The directory is created on first save if it does not exist.
func SetConfigDir(dir string) { configDir = dir }

// settingsFilePath returns the absolute (or CWD-relative) path to the settings
// JSON file, honoring any directory set via SetConfigDir.
func settingsFilePath() string {
	if configDir == "" {
		return defaultSettingsFile
	}
	return filepath.Join(configDir, defaultSettingsFile)
}

// headerLines is the number of rendered lines above the first item row in the
// overview: 1 (top padding) + 1 (title) + 1 (blank separator) = 3.
const headerLines = 3
const footerLines = 3

// NavStyleMsg is emitted when the user selects a different navigation style.
type NavStyleMsg struct{ Style string }

// ThemeMsg is emitted when the user selects a different color theme.
type ThemeMsg struct {
	ID               string
	Mode             string
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
	title     string
	value     func() string    // returns current display value for the overview row
	buildForm func() *huh.Form // builds a single-field overlay form for this setting
	leftTrunc bool             // if true, show tail of value with leading … (useful for paths)
	apply     func() error     // optional callback after submit
}

type Keys struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Dismiss key.Binding
}

func DefaultKeys() *Keys {
	return &Keys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
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

// Model is the settings page. It has two modes:
//
//   - Overview: a compact list showing every setting on one line with its current
//     value. Up/Down moves the cursor; Enter or click opens an edit overlay.
//
//   - Editing: a centred huh form (one field) is composited over the overview using
//     the lipgloss Compositor. Submitting or aborting the form returns to overview.
type Model struct {
	width, height int
	colors        *theme.AppStyle

	// Persisted fields (exported so JSON encoding works).
	NavStyle             string `json:"nav_style"`
	ColorThemeID         string `json:"theme_id"`
	ThemeMode            string `json:"theme_mode"`
	AccessibilityColors  bool   `json:"accessibility_colors"`
	LogOutput            string `json:"log_output"`
	LogPath              string `json:"log_path"`
	LogLevel             string `json:"log_level"`
	NotificationsEnabled bool   `json:"notifications_enabled"`
	NotificationsPersist bool   `json:"notifications_persist"`

	// intermediate string forms for huh selects (not persisted directly)
	notifEnabledStr  string
	notifPersistStr  string
	accessibilityStr string

	extraSections []config.Section
	items         []settingItem

	// loadedFromFile reports whether a persisted settings file was found and
	// read at startup. When false, the router applies first-run defaults (e.g.
	// tabs navigation) instead of the struct defaults.
	loadedFromFile bool

	// Overview state.
	cursor int
	// scrollTop is the first visible item in the overview list.
	scrollTop int

	// Overlay state.
	editing   bool
	editForm  *huh.Form
	editIndex int

	// Overlay geometry cached by View() so OnMouse can hit-test click coordinates.
	overlayX, overlayY, overlayW, overlayH int

	keys *Keys
}

// LoadedFromFile reports whether a persisted settings file was found and read
// at startup. The router uses this to detect a first run (and persist defaults).
func (m *Model) LoadedFromFile() bool { return m.loadedFromFile }

// Save persists the current settings synchronously to the configured path.
// Use this for one-off saves (e.g. writing first-run defaults at startup).
func (m *Model) Save() error {
	if err := m.SaveToFile(settingsFilePath()); err != nil {
		return err
	}
	m.loadedFromFile = true
	return nil
}

// SetColors stores a shared AppColors pointer.
func (m *Model) SetColors(c *theme.AppStyle) { m.colors = c }

func (m *Model) resolveColors() *theme.AppStyle {
	if m.colors != nil {
		return m.colors
	}
	return theme.Active()
}

// New creates a settings model. Pass extra config.Sections contributed by
// Configurable components; they appear after the built-in rows.
func New(extraSections ...config.Section) *Model {
	m := &Model{
		NavStyle:             "sidebar",
		ColorThemeID:         "dracula_plus",
		ThemeMode:            theme.ThemeModeDark,
		AccessibilityColors:  false,
		LogOutput:            "temp",
		LogPath:              "",
		LogLevel:             "INFO",
		NotificationsEnabled: true,
		NotificationsPersist: false,
		extraSections:        extraSections,
		keys:                 DefaultKeys(),
	}
	if err := m.LoadFromFile(settingsFilePath()); err == nil {
		m.loadedFromFile = true
	} else {
		// First run (no persisted settings): default to tabs navigation, which
		// is friendlier than the sidebar for a brand-new app.
		m.NavStyle = "tabs"
	}

	tint.NewDefaultRegistry()
	if m.ThemeMode == "" {
		m.ThemeMode = theme.ThemeModeDark
	}
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
	theme.SetThemePreferences(m.ThemeMode, m.AccessibilityColors)
	if m.ColorThemeID != "" {
		tint.SetTintID(m.ColorThemeID) //nolint:errcheck
	}

	m.buildItems()
	return m
}

// buildItems constructs the settingItem slice. Call this once in New() and
// again after LoadFromFile (pointer addresses stay stable; only values change).
func (m *Model) buildItems() {
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
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
	navOpts := []huh.Option[string]{
		huh.NewOption("Sidebar \u2013 vertical panel on the left", "sidebar"),
		huh.NewOption("Tabs    \u2013 horizontal bar at the top", "tabs"),
	}
	modeOpts := []huh.Option[string]{
		huh.NewOption("Dark", theme.ThemeModeDark),
		huh.NewOption("Light", theme.ThemeModeLight),
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

	m.items = []settingItem{
		{
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
		},
		{
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
		{
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
						Description("Directory or file  \u00b7  ignored when destination is Temporary").
						DirAllowed(true).
						FileAllowed(true).
						Value(&m.LogPath),
				).WithTheme(theme.HuhThemeFunc()))
			},
		},
		{
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
		{
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
		{
			title: "Color Theme",
			value: func() string { return tintDisplayName(m.ColorThemeID) },
			buildForm: func() *huh.Form {
				return huh.NewForm(huh.NewGroup(
					huh.NewSelect[string]().
						Title("Color Theme").
						Description("Up/Down to browse \u2014 applied immediately as you scroll").
						Options(buildThemeOptions(m.ThemeMode)...).
						Height(14).
						Value(&m.ColorThemeID),
				).WithTheme(theme.HuhThemeFunc()))
			},
		},
		{
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
		{
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
		{
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
	}

	for _, sec := range m.extraSections {
		for _, def := range sec.Fields {
			m.items = append(m.items, m.itemFromDef(def))
		}
	}
}

// itemFromDef builds a settingItem from a config.FieldDef for extra sections.
func (m *Model) itemFromDef(def config.FieldDef) settingItem {
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
func (m *Model) huhFieldFromDef(def config.FieldDef) huh.Field {
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
func (m *Model) CapturesKeys() bool { return m.editing }

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = wMsg.Width, wMsg.Height
		// Resize the active overlay form immediately so the compositor never
		// tries to paint a form that is wider than the terminal.
		if m.editing && m.editForm != nil {
			m.editForm.WithWidth(max(30, min(m.width-14, 120)))
		}
	}
	if m.width == 0 {
		return m, nil
	}
	if m.editing {
		return m.updateEditing(msg)
	}
	return m.updateOverview(msg)
}

func (m *Model) updateOverview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keyMsg := msg.(type) {
		case tea.KeyPressMsg:
			switch {
			case key.Matches(keyMsg, m.keys.Up):
				if m.cursor > 0 {
					m.cursor--
				}
				m.ensureCursorVisible()
			case key.Matches(keyMsg, m.keys.Down):
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
				m.ensureCursorVisible()
			case key.Matches(keyMsg, m.keys.Select):
				return m, m.startEdit()
			}
		}
	case tea.MouseWheelMsg:
		if msg.Mouse().Button == tea.MouseWheelUp {
			if m.cursor > 0 {
				m.cursor--
			}
		} else {
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
		m.ensureCursorVisible()
	}
	return m, nil
}

func (m *Model) startEdit() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	m.editIndex = m.cursor
	f := m.items[m.cursor].buildForm()
	if f == nil {
		return nil
	}
	// Use as much horizontal space as available so option labels don't wrap.
	// The border+padding box around the form adds 6 cols (1+1 border, 2+2 pad),
	// so leave at least a 4-col gutter on each side: form width = w-14.
	// Cap at 120 and floor at 30 for narrow terminals.
	formW := max(30, min(m.width-14, 120))
	m.editForm = f.WithWidth(formW)
	m.editing = true
	return m.editForm.Init()
}

// abortEdit reverts to the last persisted state and closes the overlay.
func (m *Model) abortEdit() tea.Cmd {
	_ = m.LoadFromFile(settingsFilePath())
	m.buildItems()
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
	id := m.ColorThemeID
	mode := m.ThemeMode
	accessibility := m.AccessibilityColors
	m.editing = false
	m.editForm = nil
	return func() tea.Msg {
		return ThemeMsg{ID: id, Mode: mode, Accessibility: accessibility, ApplyPreferences: true}
	}
}

func (m *Model) updateEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Esc closes the overlay and reverts any unsaved/live-preview changes.
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, m.keys.Dismiss) {
		return m, m.abortEdit()
	}

	prevTheme := m.ColorThemeID
	prevThemeMode := m.ThemeMode
	prevAccessibility := m.AccessibilityColors
	prevLevel := m.LogLevel
	prevNav := m.NavStyle

	_, cmd := m.editForm.Update(msg)
	var cmds []tea.Cmd
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.AccessibilityColors = m.accessibilityStr == "true"
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)

	// Live theme preview: fires while editing theme-related options.
	if m.ColorThemeID != prevTheme || m.ThemeMode != prevThemeMode || m.AccessibilityColors != prevAccessibility {
		id := m.ColorThemeID
		mode := m.ThemeMode
		accessibility := m.AccessibilityColors
		cmds = append(cmds, func() tea.Msg {
			return ThemeMsg{ID: id, Mode: mode, Accessibility: accessibility, ApplyPreferences: true}
		})
	}
	if m.LogLevel != prevLevel {
		_ = logging.SetLevel(m.LogLevel)
	}
	if m.NavStyle != prevNav {
		nav := m.NavStyle
		cmds = append(cmds, func() tea.Msg { return NavStyleMsg{Style: nav} })
	}

	switch m.editForm.State {
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
		m.editing = false
		m.editForm = nil

	case huh.StateAborted:
		// huh's own abort key (ctrl+c inside the form) — same revert path.
		cmds = append(cmds, m.abortEdit())
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() tea.View {
	c := m.resolveColors()
	overview := m.renderOverview()

	var content string
	if m.editing && m.editForm != nil {
		// Wrap the form in a rounded border box and composite it centred over
		// the overview using the lipgloss layer compositor.
		formBox := c.Styles.OverlayBorder.
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Render(m.editForm.View())

		overlayW, overlayH := lipgloss.Size(formBox)
		overlayX := max(0, (m.width-overlayW)/2)
		overlayY := max(0, (m.height-overlayH)/2)

		// Cache geometry so OnMouse can hit-test clicks against the overlay.
		m.overlayX, m.overlayY = overlayX, overlayY
		m.overlayW, m.overlayH = overlayW, overlayH

		base := lipgloss.NewLayer(overview)
		overlay := lipgloss.NewLayer(formBox).X(overlayX).Y(overlayY).Z(1)
		content = lipgloss.NewCompositor(base, overlay).Render()
	} else {
		content = overview
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
		click, ok := mm.(tea.MouseClickMsg)
		if !ok {
			return nil
		}
		if m.editing {
			// Click outside the overlay aborts and returns to overview.
			inside := click.X >= m.overlayX && click.X < m.overlayX+m.overlayW &&
				click.Y >= m.overlayY && click.Y < m.overlayY+m.overlayH
			if !inside {
				return m.abortEdit()
			}
			return nil
		}
		idx := m.scrollTop + click.Y - headerLines
		if idx >= 0 && idx < len(m.items) {
			m.cursor = idx
			m.ensureCursorVisible()
			return m.startEdit()
		}
		return nil
	}

	return v
}

// renderOverview renders the compact one-line-per-setting list.
func (m *Model) renderOverview() string {
	c := m.resolveColors()

	// Padding(1,2) consumes 2+2 cols of total width.
	innerW := max(m.width-4, 20)
	labelW := min(28, innerW/2)
	// 3 = 2 cols for cursor prefix "▶ "/"  " + 1 col separator between label and value.
	valueW := max(innerW-labelW-3, 1)

	normalLabel := c.Styles.TextOnBg.Width(labelW)
	normalValue := c.Styles.Subtitle.Width(valueW)
	cursorLabel := c.Styles.Title.Width(labelW)
	cursorValue := c.Styles.TextOnBg.Width(valueW)
	cursorBg := c.Styles.Row.Background(c.Styles.TabHover.GetBackground()).Width(innerW)
	titleStyle := c.Styles.Title
	helpStyle := c.Styles.Dim

	lines := make([]string, 0, 3+len(m.items)+2)
	lines = append(lines, titleStyle.Render("Settings"))
	lines = append(lines, "") // blank separator — part of headerLines

	m.ensureCursorVisible()
	start, end := m.visibleItemRange()
	for i := start; i < end; i++ {
		item := m.items[i]
		lbl := truncate(item.title, labelW)
		var val string
		if item.leftTrunc {
			val = truncateLeft(item.value(), valueW)
		} else {
			val = truncate(item.value(), valueW)
		}
		if i == m.cursor {
			row := "▶ " + cursorLabel.Render(lbl) + " " + cursorValue.Render(val)
			lines = append(lines, cursorBg.Render(row))
		} else {
			lines = append(lines, "  "+normalLabel.Render(lbl)+" "+normalValue.Render(val))
		}
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("\u2191\u2193 navigate  enter edit"))

	return c.Styles.TextOnBg.
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) visibleItemRange() (int, int) {
	innerH := max(m.height-2, 1) // account for top/bottom padding in renderOverview style
	viewHeight := max(innerH-headerLines-footerLines, 1)
	start := m.scrollTop
	if start < 0 {
		start = 0
	}
	if start > len(m.items) {
		start = len(m.items)
	}
	end := start + viewHeight
	if end > len(m.items) {
		end = len(m.items)
	}
	return start, end
}

func (m *Model) ensureCursorVisible() {
	if len(m.items) == 0 {
		m.cursor = 0
		m.scrollTop = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	innerH := max(m.height-2, 1)
	viewHeight := max(innerH-headerLines-footerLines, 1)
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
	maxStart := max(len(m.items)-viewHeight, 0)
	if m.scrollTop > maxStart {
		m.scrollTop = maxStart
	}
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
	if m.cursor >= m.scrollTop+viewHeight {
		m.scrollTop = m.cursor - viewHeight + 1
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
	if m.scrollTop > maxStart {
		m.scrollTop = maxStart
	}
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
func (m *Model) saveCmd() tea.Cmd {
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
func (m *Model) SaveToFile(filename string) error {
	if dir := filepath.Dir(filename); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// LoadFromFile loads settings from the given filename if it exists.
func (m *Model) LoadFromFile(filename string) error {
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

// truncate shortens s to at most maxW runes, appending "…" if cut.
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

// truncateLeft shortens s to at most maxW runes keeping the tail, prepending
// "…" if cut. Use this for file paths so the filename/end remains visible.
func truncateLeft(s string, maxW int) string {
	r := []rune(s)
	if len(r) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return "…" + string(r[len(r)-(maxW-1):])
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
		return dotStyle.Foreground(fg).Render("\u25cf ")
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

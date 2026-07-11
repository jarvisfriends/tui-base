package tuibase

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/gate"
	"github.com/jarvisfriends/snap/keys"
	"github.com/jarvisfriends/tui-base/config"
)

// Option is a functional option for building an app, Bubble Tea-style (SP-11,
// shape per Q-21): the struct [Options] and With* options coexist — pass a
// struct, a list of options, or both. All options are applied first, then
// defaults fill anything left unset, exactly like tea.NewProgram:
//
//	err := tuibase.Run(tuibase.Options{},
//	    tuibase.WithAppName("My App"),
//	    tuibase.WithPages(tuibase.RegisteredPage{Title: "Dashboard", Model: dash}),
//	    tuibase.WithDebugOverlay(myInspector),
//	)
type Option func(*Options)

// WithAppName sets the display name used in the terminal window title, the
// info modal, and the derived env-var prefix.
func WithAppName(name string) Option {
	return func(o *Options) { o.AppName = name }
}

// WithAppVersion overrides the version string shown in the info modal.
func WithAppVersion(version string) Option {
	return func(o *Options) { o.AppVersion = version }
}

// WithConfigDirName sets the subdirectory name under os.UserConfigDir() used
// for persistent state.
func WithConfigDirName(name string) Option {
	return func(o *Options) { o.ConfigDirName = name }
}

// WithConfigDir overrides the settings configuration directory outright.
func WithConfigDir(dir string) Option {
	return func(o *Options) { o.ConfigDir = dir }
}

// WithPages appends application pages (order preserved).
func WithPages(pages ...RegisteredPage) Option {
	return func(o *Options) { o.ExtraPages = append(o.ExtraPages, pages...) }
}

// WithDefaultPage selects the page (by Title) shown on startup.
func WithDefaultPage(title string) Option {
	return func(o *Options) { o.DefaultPage = title }
}

// WithInitialLogLevel sets the log level applied at startup.
func WithInitialLogLevel(level string) Option {
	return func(o *Options) { o.InitialLogLevel = level }
}

// WithSettingsSections appends app-defined sections to the settings page.
func WithSettingsSections(sections ...config.Section[string]) Option {
	return func(o *Options) { o.SettingsSections = append(o.SettingsSections, sections...) }
}

// WithKeyMap replaces the default key map.
func WithKeyMap(km *keys.AppKeyMap) Option {
	return func(o *Options) { o.KeyMap = km }
}

// WithGates supplies the feature-gate registry (see docs/feature-gates.md).
func WithGates(g *gate.GateRegistry) Option {
	return func(o *Options) { o.Gates = g }
}

// WithWatchSettingsFile enables live reload of tui_settings.json (FW-1).
func WithWatchSettingsFile() Option {
	return func(o *Options) { o.WatchSettingsFile = true }
}

// WithoutTerminalRelaunch disables the automatic relaunch into Windows
// Terminal when started under the legacy console.
func WithoutTerminalRelaunch() Option {
	return func(o *Options) { o.DisableTerminalRelaunch = true }
}

// WithDebugOverlay stores a model that replaces the built-in inspector as the
// Ctrl+D debug pop-up (Q-22): while the model is non-nil, tui-base owns the
// Ctrl+D toggle and presents this model in the inspector's overlay box —
// pairing with the standalone jarvisfriends/inspector, which delivers itself
// as a plain tea.Model. The model receives the overlay's inner size as a
// WindowSizeMsg, all keys while visible (Ctrl+D/Esc close), and mouse events
// when its View sets OnMouse.
func WithDebugOverlay(m tea.Model) Option {
	return func(o *Options) { o.DebugOverlay = m }
}

// applyOptions applies options on top of base, in order.
func applyOptions(base Options, options []Option) Options {
	for _, opt := range options {
		if opt != nil {
			opt(&base)
		}
	}
	return base
}

// Package tuibase is the consumer-facing entry point for building multi-page
// terminal applications on the tui-base framework.
//
// It wraps the router package (the root model that owns navigation, theming,
// the status bar, notifications, and the Ctrl+D inspector) so a full app needs
// only one import:
//
//	err := tuibase.Run(tuibase.Options{
//	    AppName: "My App",
//	    ExtraPages: []tuibase.RegisteredPage{
//	        {Title: "Dashboard", Model: dashboard.New()},
//	    },
//	})
//
// Apps that need to customize the Bubble Tea program (extra options, custom
// signal handling) can build the pieces themselves via NewWithOptions and the
// router package; Run is the batteries-included path.
//
// The runnable reference application lives in cmd/tui-base.
package tuibase

import (
	"context"
	"os"

	"github.com/jarvisfriends/tui-base/router"
)

type (
	// Options re-exports [router.Options]: startup configuration for the app
	// (name, version, config dir, pages, settings sections).
	Options = router.Options
	// RegisteredPage re-exports [router.RegisteredPage]: one application page
	// (title + model) to add to the router.
	RegisteredPage = router.RegisteredPage
	// RouterModel re-exports [router.RouterModel], the root tea.Model.
	RouterModel = router.RouterModel
	// WindowsTerminalProfile re-exports [router.WindowsTerminalProfile]: the
	// description of a Windows Terminal profile fragment (branded new-tab entry).
	WindowsTerminalProfile = router.WindowsTerminalProfile
)

// InstallWindowsTerminalProfile re-exports
// [router.InstallWindowsTerminalProfile]: it registers the app as a Windows
// Terminal profile (branded name + icon in the new-tab dropdown). Call it from
// an installer or a setup flag, not on every launch.
func InstallWindowsTerminalProfile(p WindowsTerminalProfile) (fragmentFile string, err error) {
	return router.InstallWindowsTerminalProfile(p)
}

// UninstallWindowsTerminalProfile re-exports
// [router.UninstallWindowsTerminalProfile]: it removes the fragment previously
// installed for appName.
func UninstallWindowsTerminalProfile(appName string) error {
	return router.UninstallWindowsTerminalProfile(appName)
}

// EnsureWindowsTerminal relaunches the process inside Windows Terminal when it
// was started under the legacy Windows console (conhost) and, having done so,
// exits the original process. Call it as the very first statement in main —
// before any other setup — so apps that bootstrap their own state before
// tuibase.Run still get the modern terminal without doing that work twice:
//
//	func main() {
//	    tuibase.EnsureWindowsTerminal()
//	    // ... app setup, then tuibase.Run(...) ...
//	}
//
// It does nothing (and returns) on non-Windows platforms, when already in a
// modern terminal, in a non-interactive session, when wt.exe is missing, or
// when opted out via TUI_BASE_NO_WT_RELAUNCH. tuibase.Run/RunContext already
// perform this check, so calling it here is only needed for apps that run
// setup before Run and want it moved as early as possible. See
// [router.MaybeRelaunchInWindowsTerminal] for the full policy.
func EnsureWindowsTerminal() {
	if relaunched, _ := router.MaybeRelaunchInWindowsTerminal(
		router.TerminalRelaunchConfig{},
	); relaunched {
		os.Exit(0)
	}
}

// New returns a router with the built-in pages only (Home and Settings, with
// the inspector available as the Ctrl+D overlay).
func New() *RouterModel { return router.New() }

// NewWithOptions returns a router configured for the embedding application.
func NewWithOptions(opts Options) *RouterModel { return router.NewWithOptions(opts) }

// Run builds the router for opts, wraps it in a Bubble Tea program with
// tui-base's standard options (including the app-derived color-profile env
// var), and blocks until the program exits.
func Run(opts Options) error {
	return RunContext(context.Background(), opts)
}

// RunContext is Run bound to ctx: cancel it (e.g. from
// signal.NotifyContext on SIGINT/SIGTERM) and the program shuts down
// cleanly with the terminal restored — the graceful-shutdown path for
// services and wrappers embedding a tui-base app.
func RunContext(ctx context.Context, opts Options) error {
	// Move into Windows Terminal before building anything when running under the
	// legacy console; if a relaunch was started, this process is done.
	// ProfileName defaults to the app name so a profile installed via
	// InstallWindowsTerminalProfile(AppName: ...) brands the relaunched tab
	// automatically (it is skipped when no such profile is installed).
	if relaunched, _ := router.MaybeRelaunchInWindowsTerminal(router.TerminalRelaunchConfig{
		AppName:     opts.AppName,
		ProfileName: opts.AppName,
		Disable:     opts.DisableTerminalRelaunch,
	}); relaunched {
		return nil
	}
	m := router.NewWithOptions(opts)
	defer m.Close()
	p := router.NewProgramWithContext(ctx, m, m.ColorProfileEnvVar())
	_, err := p.Run()
	return err
}

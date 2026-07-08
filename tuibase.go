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
)

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
	m := router.NewWithOptions(opts)
	defer m.Close()
	p := router.NewProgramWithContext(ctx, m, m.ColorProfileEnvVar())
	_, err := p.Run()
	return err
}

package tuibase_test

import (
	tuibase "github.com/jarvisfriends/tui-base/tuibase"
)

// ExampleRun shows the minimal bootstrap for an application built on
// tui-base: one call wires the router, theming, status bar, notifications,
// and the Ctrl+D inspector around your pages.
//
// It has no Output comment on purpose: the example is compile-checked but not
// executed, because Run blocks on a live terminal.
func ExampleRun() {
	err := tuibase.Run(tuibase.Options{
		AppName:    "My App",
		ExtraPages: []tuibase.RegisteredPage{
			// {Title: "Dashboard", Model: dashboard.New()},
		},
	})
	if err != nil {
		panic(err)
	}
}

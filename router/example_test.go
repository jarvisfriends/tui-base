// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router_test

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/router"
)

// dashboardModel is a stand-in for an application page.
type dashboardModel struct{}

func (dashboardModel) Init() tea.Cmd                       { return nil }
func (dashboardModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return dashboardModel{}, nil }
func (dashboardModel) View() tea.View                      { return tea.NewView("dashboard") }

// ExampleNewWithOptions shows the standard construction path for an embedding
// application: options in, program out. Compile-checked but not executed —
// constructing a router reads config and initializes logging, and running the
// program needs a live terminal.
func ExampleNewWithOptions() {
	m := router.NewWithOptions(router.Options{
		AppName:     "My App",
		DefaultPage: "Dashboard",
		ExtraPages: []router.RegisteredPage{
			{Title: "Dashboard", Model: dashboardModel{}},
		},
	})
	p := router.NewProgramWithEnvVar(m, m.ColorProfileEnvVar())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}

// ExampleRouterModel_RegisterPage shows runtime page registration: the
// returned command initializes the new page's model and must be dispatched.
// Compile-checked but not executed (see ExampleNewWithOptions).
func ExampleRouterModel_RegisterPage() {
	m := router.New()
	initCmd := m.RegisterPage("Reports", dashboardModel{})
	_ = initCmd // return it from your Update (or batch it) so it runs
}

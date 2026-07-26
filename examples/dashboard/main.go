// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

// Command dashboard shows a single custom page with themed widgets: a
// bubbles table styled via theme.TableStyles, live status bar segments, and
// a notification fired from a key press.
//
//	go run ./examples/dashboard
package main

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/notifications"
	"github.com/jarvisfriends/snap/page"
	"github.com/jarvisfriends/tui-base/router"
	"github.com/jarvisfriends/tui-base/theme"
)

// dashboard is a page: a tea.Model embedding page.Base for nil-safe theme
// access and size bookkeeping.
type dashboard struct {
	page.Base
	tbl  table.Model
	keys dashboardKeys
}

type dashboardKeys struct {
	Notify key.Binding
}

func newDashboard() *dashboard {
	tbl := table.New(
		table.WithColumns([]table.Column{
			{Title: "Service", Width: 16},
			{Title: "State", Width: 10},
			{Title: "Uptime", Width: 10},
		}),
		table.WithRows([]table.Row{
			{"api", "healthy", "12d"},
			{"worker", "healthy", "12d"},
			{"scheduler", "degraded", "3h"},
		}),
		table.WithFocused(true),
		table.WithHeight(6),
	)
	return &dashboard{
		tbl: tbl,
		keys: dashboardKeys{
			Notify: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "notify")),
		},
	}
}

func (d *dashboard) Init() tea.Cmd { return nil }

func (d *dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.SetSize(msg.Width, msg.Height)
		return d, nil
	case tea.KeyPressMsg:
		if key.Matches(msg, d.keys.Notify) {
			return d, func() tea.Msg {
				return notifications.AddMsg{
					Content:  "Hello from the dashboard",
					Severity: notifications.SeverityInfo,
					TTL:      5 * time.Second,
				}
			}
		}
	}
	var cmd tea.Cmd
	d.tbl, cmd = d.tbl.Update(msg)
	return d, cmd
}

func (d *dashboard) View() tea.View {
	c := d.Colors()
	// Re-apply theme styles each render so live theme switching works.
	d.tbl.SetStyles(theme.TableStyles(c))
	body := c.Styles.Title.Render("Services") + "\n\n" + d.tbl.View() +
		"\n\n" + c.Styles.Subtitle.Render("press n for a toast • Ctrl+D for the inspector")
	v := tea.NewView(c.Styles.TextOnBg.Width(d.Width()).Height(d.Height()).Render(body))
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	return v
}

func main() {
	m := router.NewWithOptions(router.Options{
		AppName: "Dashboard Example",
		ExtraPages: []router.RegisteredPage{
			{Title: "Dashboard", Model: newDashboard()},
		},
	})

	// Live right-aligned status segment (E-1).
	start := time.Now()
	m.SetStatusSegment("uptime", func() string {
		return "up " + time.Since(start).Round(time.Second).String()
	})

	p := router.NewProgramWithEnvVar(m, m.ColorProfileEnvVar())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

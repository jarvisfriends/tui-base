// Command multipage shows several app pages with custom settings sections
// and graceful SIGINT/SIGTERM shutdown via RunContext.
//
//	go run ./examples/multipage
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/page"
	tuibase "github.com/jarvisfriends/tui-base"
	"github.com/jarvisfriends/tui-base/config"
)

// textPage is a trivial page rendering a fixed body.
type textPage struct {
	page.Base
	title, body string
}

func (p *textPage) Init() tea.Cmd { return nil }

func (p *textPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		p.SetSize(ws.Width, ws.Height)
	}
	return p, nil
}

func (p *textPage) View() tea.View {
	c := p.Colors()
	content := c.Styles.Title.Render(p.title) + "\n\n" + c.Styles.TextOnBg.Render(p.body)
	v := tea.NewView(c.Styles.TextOnBg.Width(p.Width()).Height(p.Height()).Render(content))
	v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
	return v
}

const pageOverview = "Overview"

func main() {
	// Ctrl+C / SIGTERM cancels the context; RunContext restores the terminal
	// and returns (A-2).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	apiURL := "https://api.example.com"
	err := tuibase.RunContext(ctx, tuibase.Options{
		AppName:     "Multipage Example",
		DefaultPage: pageOverview,
		ExtraPages: []tuibase.RegisteredPage{
			{
				Title: pageOverview,
				Model: &textPage{title: pageOverview, body: "Tab / Shift+Tab cycle pages."},
			},
			{Title: "Reports", Model: &textPage{title: "Reports", body: "A second app page."}},
		},
		// App-specific settings appear on the built-in Settings page and
		// persist alongside the framework's own values.
		SettingsSections: []config.Section[string]{{
			Title: "Multipage Example",
			Fields: []config.FieldDef[string]{{
				Kind:  config.FieldText,
				Title: "API URL",
				Value: &apiURL,
			}},
		}},
	})
	stop()
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

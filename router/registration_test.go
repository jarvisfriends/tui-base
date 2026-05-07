package router

import (
	"testing"

	log "github.com/jarvisfriends/tui-base/logging"

	tea "charm.land/bubbletea/v2"
)

type stubPage struct{}

func (stubPage) Init() tea.Cmd                           { return nil }
func (stubPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return stubPage{}, nil }
func (stubPage) View() tea.View                          { return tea.NewView("stub") }

func TestNewWithRegisteredPages_AppendsExtraPages(t *testing.T) {
	t.Parallel()

	r := NewWithRegisteredPages([]RegisteredPage{
		{
			Title: "App",
			Model: stubPage{},
		},
	})

	// When extra pages are supplied: app pages come first, Home is omitted,
	// Inspector and Settings are appended at the end.
	pages := r.nav.GetPages()
	if len(pages) != 2 {
		t.Fatalf("nav pages len = %d; want 2 (app, settings)", len(pages))
	}
	if pages[0].ID != "app" || pages[0].Title != "App" {
		t.Fatalf("pages[0] = %+v; want app/App", pages[0])
	}
	if pages[1].ID != "settings" {
		t.Fatalf("pages[1] = %+v; want settings/Settings", pages[1])
	}

	if len(r.pages) != 2 {
		t.Fatalf("router model pages len = %d; want 2", len(r.pages))
	}
}

func TestNewWithRegisteredPages_SkipsInvalidEntries(t *testing.T) {
	t.Parallel()

	r := NewWithRegisteredPages([]RegisteredPage{
		// nil model — invalid
		{Title: "NoModel"},
		// empty title — invalid
		{Model: stubPage{}},
	})

	// Both entries are invalid; fallback to default standalone mode (3 pages).
	if got := len(r.nav.GetPages()); got != 2 {
		t.Fatalf("nav pages len = %d; want 2", got)
	}
	if got := len(r.pages); got != 2 {
		t.Fatalf("router pages len = %d; want 2", got)
	}
}

func TestNewWithOptions_DefaultPageSelection(t *testing.T) {
	t.Parallel()

	r := NewWithOptions(Options{
		ExtraPages: []RegisteredPage{{
			Title: "aSettings",
			Model: stubPage{},
		}},
		DefaultPage: "aSettings",
	})

	idx := r.nav.GetActiveIndex()
	pages := r.nav.GetPages()
	if idx < 0 || idx >= len(pages) {
		t.Fatalf("active index out of range: %d", idx)
	}
	if pages[idx].Title != "aSettings" {
		t.Fatalf("active page title = %q; want %q", pages[idx].Title, "aSettings")
	}
}

func TestNewWithOptions_InitialLogLevel(t *testing.T) {
	prev := log.GetLevel()
	defer func() {
		_ = log.SetLevel(prev)
	}()

	r := NewWithOptions(Options{InitialLogLevel: "ERROR"})
	if r == nil {
		t.Fatal("router should not be nil")
	}
	if got := log.GetLevel(); got != "ERROR" {
		t.Fatalf("log level = %q; want %q", got, "ERROR")
	}
}

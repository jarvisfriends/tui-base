package router

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type mockPageModel struct {
	title string
}

func (m mockPageModel) Init() tea.Cmd {
	return func() tea.Msg { return nil }
}

func (m mockPageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m mockPageModel) View() tea.View {
	return tea.View{
		Content: "Mock Page: " + m.title,
	}
}

// TestExtraPagesRegistration verifies construction-time page registration via
// Options.ExtraPages — the explicit replacement for the removed global
// RegisterPage()/init() registry (A-7/Q-5).
func TestExtraPagesRegistration(t *testing.T) {
	t.Parallel()
	m := NewWithOptions(Options{ExtraPages: []RegisteredPage{
		{Title: "Extra Page A", Model: mockPageModel{title: "A"}},
		{Title: "Extra Page B", Model: mockPageModel{title: "B"}},
	}})

	pages := m.nav.GetPages()
	// Extra Page A, Extra Page B, Settings = 3 pages (no default Home page)
	if len(pages) != 3 {
		t.Fatalf("expected 3 registered pages, got %d", len(pages))
	}

	if pages[0].Title != "Extra Page A" {
		t.Errorf("expected page 0 to be Extra Page A, got %q", pages[0].Title)
	}
	if pages[1].Title != "Extra Page B" {
		t.Errorf("expected page 1 to be Extra Page B, got %q", pages[1].Title)
	}
	if pages[2].Title != pageTitleSettings {
		t.Errorf("expected page 2 to be Settings, got %q", pages[2].Title)
	}
}

func TestRuntimeDynamicRegistration(t *testing.T) {
	t.Parallel()
	m := New()

	// Initial check: only Home + Settings = 2 pages
	initialPages := m.nav.GetPages()
	if len(initialPages) != 2 {
		t.Fatalf("expected 2 initial pages, got %d", len(initialPages))
	}

	// Active index should be Home (0)
	if m.nav.GetActiveIndex() != 0 {
		t.Fatalf("expected active index to be 0, got %d", m.nav.GetActiveIndex())
	}

	// Register a page at runtime
	cmd := m.RegisterPage("Runtime Dynamic Page", mockPageModel{title: "Dynamic"})
	if cmd == nil {
		t.Fatal("expected non-nil initialization command from dynamic page registration")
	}

	// Check if navigation pages contains the dynamic page and settings (no Home page)
	pages := m.nav.GetPages()
	if len(pages) != 2 {
		t.Fatalf("expected 2 registered pages after dynamic registration, got %d", len(pages))
	}
	if pages[0].Title != "Runtime Dynamic Page" {
		t.Errorf("expected page 0 to be Runtime Dynamic Page, got %q", pages[0].Title)
	}
	if pages[1].Title != pageTitleSettings {
		t.Errorf("expected page 1 to be Settings, got %q", pages[1].Title)
	}

	// Verify the active page index was preserved (still at 0)
	if m.nav.GetActiveIndex() != 0 {
		t.Errorf("expected active page index to be preserved at 0, got %d", m.nav.GetActiveIndex())
	}

	// Verify the newly registered page's view content at index 0
	activeView := m.pages[0].View().Content
	if !strings.Contains(activeView, "Mock Page: Dynamic") {
		t.Errorf("expected view content to contain 'Mock Page: Dynamic', got %q", activeView)
	}
}

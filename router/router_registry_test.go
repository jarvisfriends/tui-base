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

func TestStaticSelfRegistration(t *testing.T) {
	// 1. Clear any existing global registry entries
	ClearRegisteredPages()

	// 2. Register mock pages statically
	RegisterPage("Global Page A", mockPageModel{title: "A"})
	RegisterPage("Global Page B", mockPageModel{title: "B"})

	// 3. Instantiate router
	m := New()

	// 4. Verify pages are registered in correct order
	pages := m.nav.GetPages()
	// Global Page A, Global Page B, Settings = 3 pages (no default Home page)
	if len(pages) != 3 {
		t.Fatalf("expected 3 registered pages, got %d", len(pages))
	}

	if pages[0].Title != "Global Page A" {
		t.Errorf("expected page 0 to be Global Page A, got %q", pages[0].Title)
	}
	if pages[1].Title != "Global Page B" {
		t.Errorf("expected page 1 to be Global Page B, got %q", pages[1].Title)
	}
	if pages[2].Title != "Settings" {
		t.Errorf("expected page 2 to be Settings, got %q", pages[2].Title)
	}

	// 5. Cleanup
	ClearRegisteredPages()
}

func TestRuntimeDynamicRegistration(t *testing.T) {
	ClearRegisteredPages()
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
	if pages[1].Title != "Settings" {
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

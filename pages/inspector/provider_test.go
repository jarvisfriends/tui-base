package inspector

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/theme"
)

// fakeProvider records its lifecycle and renders fixed rows.
type fakeProvider struct {
	name           string
	starts, stops  int
	rows           []string
	refreshEvery   time.Duration
	buildRowsCalls int
}

func (f *fakeProvider) TabName() string { return f.name }

func (f *fakeProvider) BuildRows(*theme.AppStyle) []string {
	f.buildRowsCalls++
	return f.rows
}
func (f *fakeProvider) RefreshInterval() time.Duration { return f.refreshEvery }
func (f *fakeProvider) Start()                         { f.starts++ }
func (f *fakeProvider) Stop()                          { f.stops++ }

// TestInspectorProviderTabLifecycle covers the E-5 contract: registration adds
// a switchable tab rendering the provider's rows, visibility drives
// Start/Stop, replacement stops the old provider, and removal falls back to a
// built-in tab.
func TestInspectorProviderTabLifecycle(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	p := &fakeProvider{name: "Queue", rows: []string{"depth 42"}}
	m.RegisterTab(p)
	if p.starts != 0 {
		t.Fatalf("provider started while inspector hidden; starts=%d", p.starts)
	}

	// Opening the inspector starts providers.
	m.ToggleVisible()
	if p.starts != 1 {
		t.Fatalf("starts=%d after open; want 1", p.starts)
	}

	// The custom tab is switchable and renders the provider's rows.
	customTab := debugTab(len(debugTabTitles))
	m.switchTab(customTab)
	if m.activeTab != customTab {
		t.Fatalf("switchTab rejected the custom tab index %d", customTab)
	}
	view := m.View().Content
	if !strings.Contains(view, "depth 42") {
		t.Fatal("provider rows not rendered on its tab")
	}
	if !strings.Contains(view, "Queue") {
		t.Fatal("provider tab title not in the tabs line")
	}

	// Closing stops providers.
	m.ToggleVisible()
	if p.stops != 1 {
		t.Fatalf("stops=%d after close; want 1", p.stops)
	}

	// Replacing by name stops the old instance; registering on a visible
	// inspector starts immediately.
	m.ToggleVisible()
	p2 := &fakeProvider{name: "Queue", rows: []string{"depth 43"}}
	m.RegisterTab(p2)
	if p.stops != 2 {
		t.Fatalf("old provider stops=%d after replacement; want 2", p.stops)
	}
	if p2.starts != 1 {
		t.Fatalf("replacement starts=%d; want 1 (registered while visible)", p2.starts)
	}

	// Removal stops it and the active tab falls back to a built-in.
	m.switchTab(customTab)
	m.RemoveTab("Queue")
	if p2.stops != 1 {
		t.Fatalf("removed provider stops=%d; want 1", p2.stops)
	}
	if int(m.activeTab) >= m.tabCount() {
		t.Fatalf("active tab %d out of range after removal", m.activeTab)
	}
}

// TestInspectorProviderRefreshInterval verifies the stats tick marks the view
// dirty once the provider's interval has elapsed.
func TestInspectorProviderRefreshInterval(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	p := &fakeProvider{name: "Fast", rows: []string{"x"}, refreshEvery: time.Nanosecond}
	m.RegisterTab(p)
	m.ToggleVisible()
	m.switchTab(debugTab(len(debugTabTitles)))
	_ = m.View() // records the provider's render timestamp
	m.dirty = false

	time.Sleep(2 * time.Millisecond)
	m.tickProviderRefresh()
	if !m.dirty {
		t.Fatal("elapsed RefreshInterval did not mark the inspector dirty")
	}
}

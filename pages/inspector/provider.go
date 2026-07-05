package inspector

import (
	"strings"
	"time"

	"github.com/jarvisfriends/tui-base/theme"
)

// MetricsProvider is the developer-facing extension point for the inspector
// (E-5): an implementation appears as an additional numbered tab after the
// built-in tabs and renders whatever rows it builds.
//
// This is a diagnostics surface for developers, not an application UI
// primitive — build user-facing screens as pages, not inspector tabs.
//
// Lifecycle: Start is called when the provider becomes active (registered on
// a visible inspector, or when the inspector opens); Stop is called when the
// inspector closes or the provider is removed/replaced. Both must be
// idempotent — the inspector may toggle rapidly. Do any collection in
// goroutines or tea.Cmds you own; BuildRows runs on the UI goroutine and must
// only format already-collected state.
type MetricsProvider interface {
	// TabName returns the short, stable tab label (also the registry key).
	TabName() string
	// BuildRows returns the tab's content lines, styled against c. Called on
	// every render of the tab — keep it allocation-light.
	BuildRows(c *theme.AppStyle) []string
	// RefreshInterval is how often the inspector forces a re-render while
	// this tab is active, checked at the inspector's stats-tick cadence.
	// Return <= 0 for no forced refresh (the tab still re-renders whenever
	// the inspector redraws).
	RefreshInterval() time.Duration
	// Start begins collection; Stop ends it. See the lifecycle note above.
	Start()
	Stop()
}

// RegisterTab adds a custom inspector tab, replacing (and stopping) any
// registered provider with the same TabName. If the inspector is currently
// visible the provider is started immediately.
func (m *InspectorModel) RegisterTab(p MetricsProvider) {
	if p == nil {
		return
	}
	name := p.TabName()
	for i, existing := range m.providers {
		if existing.TabName() != name {
			continue
		}
		existing.Stop()
		m.providers[i] = p
		if m.visible {
			p.Start()
		}
		m.dirty = true
		return
	}
	m.providers = append(m.providers, p)
	if m.visible {
		p.Start()
	}
	m.dirty = true
}

// RemoveTab stops and removes the provider registered under name. Removing
// the active tab falls back to the Runtime tab.
func (m *InspectorModel) RemoveTab(name string) {
	for i, p := range m.providers {
		if p.TabName() != name {
			continue
		}
		p.Stop()
		m.providers = append(m.providers[:i], m.providers[i+1:]...)
		if int(m.activeTab) >= m.tabCount() {
			m.activeTab = debugTabRuntime
		}
		m.dirty = true
		return
	}
}

// tabCount returns the number of tabs: built-ins plus registered providers.
func (m *InspectorModel) tabCount() int {
	return len(debugTabTitles) + len(m.providers)
}

// tabTitles returns the built-in tab titles followed by provider tab names.
func (m *InspectorModel) tabTitles() []string {
	titles := make([]string, 0, m.tabCount())
	titles = append(titles, debugTabTitles...)
	for _, p := range m.providers {
		titles = append(titles, p.TabName())
	}
	return titles
}

// providerForTab returns the provider backing tab, or nil for built-in tabs.
func (m *InspectorModel) providerForTab(tab debugTab) MetricsProvider {
	idx := int(tab) - len(debugTabTitles)
	if idx < 0 || idx >= len(m.providers) {
		return nil
	}
	return m.providers[idx]
}

// renderProviderSection renders a provider tab's rows and records the refresh
// timestamp used by the stats tick to honor RefreshInterval.
func (m *InspectorModel) renderProviderSection(p MetricsProvider, c *theme.AppStyle) string {
	if m.providerRefreshed == nil {
		m.providerRefreshed = make(map[string]time.Time)
	}
	m.providerRefreshed[p.TabName()] = time.Now()
	return strings.Join(p.BuildRows(c), "\n")
}

// tickProviderRefresh marks the view dirty when the active provider tab's
// RefreshInterval has elapsed; called from the stats tick.
func (m *InspectorModel) tickProviderRefresh() {
	p := m.providerForTab(m.activeTab)
	if p == nil {
		return
	}
	iv := p.RefreshInterval()
	if iv <= 0 {
		return
	}
	if time.Since(m.providerRefreshed[p.TabName()]) >= iv {
		m.dirty = true
	}
}

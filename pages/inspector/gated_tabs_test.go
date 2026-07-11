package inspector

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/gate"
	"github.com/jarvisfriends/tui-base/theme"
)

// newGatesWithAccessibility builds a registry with the accessibility-tab gate
// in the given state, mirroring the router's built-in registration.
func newGatesWithAccessibility(enabled bool) *gate.GateRegistry {
	g := gate.NewGateRegistry()
	g.Register(gate.FeatureGate{Name: AccessibilityTabGate, Default: enabled})
	return g
}

// TestAccessibilityTabHiddenByDefault asserts the gated tab is absent with no
// gate registry (New alone) and with the gate at its router default (false).
func TestAccessibilityTabHiddenByDefault(t *testing.T) {
	t.Parallel()

	for name, m := range map[string]*InspectorModel{
		"no registry":  New(),
		"gate default": func() *InspectorModel { m := New(); m.SetGates(newGatesWithAccessibility(false)); return m }(),
	} {
		if m.tabVisible(debugTabAccessibility) {
			t.Errorf("%s: accessibility tab should be hidden", name)
		}
		for _, tab := range m.visibleTabs() {
			if tab == debugTabAccessibility {
				t.Errorf("%s: visibleTabs contains the hidden accessibility tab", name)
			}
		}
		// The tab bar must not render it either.
		if line := m.buildTabsLine(
			theme.Active(),
		); strings.Contains(
			line,
			debugTabTitleAccessibility,
		) {
			t.Errorf("%s: tab bar renders hidden tab:\n%s", name, line)
		}
	}
}

// TestAccessibilityTabVisibleWhenGateEnabled asserts enabling the gate shows
// the tab in the visible list and on the rendered tab bar.
func TestAccessibilityTabVisibleWhenGateEnabled(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetGates(newGatesWithAccessibility(true))
	found := false
	for _, tab := range m.visibleTabs() {
		if tab == debugTabAccessibility {
			found = true
		}
	}
	if !found {
		t.Fatal("visibleTabs missing accessibility tab with gate enabled")
	}
	if line := m.buildTabsLine(
		theme.Active(),
	); !strings.Contains(
		line,
		debugTabTitleAccessibility,
	) {
		t.Fatalf("tab bar missing enabled tab:\n%s", line)
	}
}

// TestGateFlipAppliesImmediately flips the gate at runtime (as the settings
// page does) and asserts the tab list reacts on the very next read — no
// restart, no reload.
func TestGateFlipAppliesImmediately(t *testing.T) {
	t.Parallel()

	g := newGatesWithAccessibility(false)
	m := New()
	m.SetGates(g)

	if m.tabVisible(debugTabAccessibility) {
		t.Fatal("tab visible before enabling")
	}
	g.Set(AccessibilityTabGate, true)
	if !m.tabVisible(debugTabAccessibility) {
		t.Fatal("enabling the gate did not show the tab immediately")
	}
	g.Set(AccessibilityTabGate, false)
	if m.tabVisible(debugTabAccessibility) {
		t.Fatal("disabling the gate did not hide the tab immediately")
	}
}

// TestGateOffSnapsActiveTab asserts that disabling the gate while the
// accessibility tab is active snaps the inspector back to Runtime (via
// OnGatesChanged, which the router calls on settings.GatesChangedMsg).
func TestGateOffSnapsActiveTab(t *testing.T) {
	t.Parallel()

	g := newGatesWithAccessibility(true)
	m := New()
	m.SetGates(g)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	m.switchTab(debugTabAccessibility)
	if m.activeTab != debugTabAccessibility {
		t.Fatalf("setup: activeTab = %v; want Accessibility", m.activeTab)
	}

	g.Set(AccessibilityTabGate, false)
	m.OnGatesChanged()
	if m.activeTab != debugTabRuntime {
		t.Fatalf("after gate off: activeTab = %v; want Runtime", m.activeTab)
	}
}

// TestHiddenTabSkippedByCyclingAndDigits asserts cycling and digit keys
// address only visible tabs: with the accessibility tab hidden, cycling from
// Terminal goes straight to Log, and the digit that used to be Accessibility
// now selects Log.
func TestHiddenTabSkippedByCyclingAndDigits(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetGates(newGatesWithAccessibility(false))
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	m.switchTab(debugTabTerminal)
	m.stepTab(1)
	if m.activeTab != debugTabLog {
		t.Fatalf("stepTab from Terminal = %v; want Log (skipping hidden tab)", m.activeTab)
	}
	m.stepTab(-1)
	if m.activeTab != debugTabTerminal {
		t.Fatalf("stepTab back = %v; want Terminal", m.activeTab)
	}

	// Visible order: 1:Runtime 2:Input 3:Disks 4:Terminal 5:Log 6:Settings.
	_, _ = m.Update(tea.KeyPressMsg{Code: '5', Text: "5"})
	if m.activeTab != debugTabLog {
		t.Fatalf("digit 5 selected %v; want Log", m.activeTab)
	}

	// switchTab refuses the hidden tab outright.
	m.switchTab(debugTabAccessibility)
	if m.activeTab == debugTabAccessibility {
		t.Fatal("switchTab landed on a hidden tab")
	}
}

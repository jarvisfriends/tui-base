package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jarvisfriends/tui-base/gate"
	"github.com/jarvisfriends/tui-base/pages/inspector"
	"github.com/jarvisfriends/tui-base/pages/settings"
)

// TestRouterRegistersAccessibilityGate asserts the router registers the
// built-in inspector accessibility gate (default off) on an app-provided
// registry, without clobbering an app's own definition.
func TestRouterRegistersAccessibilityGate(t *testing.T) {
	g := gate.NewGateRegistry()
	m := NewWithOptions(Options{AppName: "Gate Test A", Gates: g})
	defer m.Close()

	if !g.Has(inspector.AccessibilityTabGate) {
		t.Fatal("router did not register the built-in accessibility gate")
	}
	if g.Value(inspector.AccessibilityTabGate) {
		t.Fatal("accessibility gate should default to disabled (tab hidden)")
	}

	// An app that pre-registers the gate keeps its own default.
	g2 := gate.NewGateRegistry()
	g2.Register(gate.FeatureGate{Name: inspector.AccessibilityTabGate, Default: true})
	m2 := NewWithOptions(Options{AppName: "Gate Test B", Gates: g2})
	defer m2.Close()
	if !g2.Value(inspector.AccessibilityTabGate) {
		t.Fatal("router clobbered the app's own gate registration")
	}
}

// TestRouterGateEnvOverride asserts the startup env override flips the
// built-in gate: <APPNAME>_GATE_INSPECTOR_ACCESSIBILITY_TAB=1.
func TestRouterGateEnvOverride(t *testing.T) {
	t.Setenv("GATE_ENV_APP_GATE_INSPECTOR_ACCESSIBILITY_TAB", "1")
	g := gate.NewGateRegistry()
	m := NewWithOptions(Options{AppName: "Gate Env App", Gates: g})
	defer m.Close()

	if !g.Value(inspector.AccessibilityTabGate) {
		t.Fatal("env override did not enable the accessibility gate at startup")
	}
}

// TestGatesChangedMsgReachesInspector asserts the router reacts to a runtime
// gate flip by re-deriving inspector state (the active-tab snap is covered by
// inspector tests; here we assert the router wiring dispatches without error
// and keeps running).
func TestGatesChangedMsgReachesInspector(t *testing.T) {
	g := gate.NewGateRegistry()
	m := NewWithOptions(Options{AppName: "Gate Msg App", Gates: g})
	defer m.Close()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	g.Set(inspector.AccessibilityTabGate, true)
	updated, _ := m.Update(settings.GatesChangedMsg{Values: g.Snapshot()})
	if updated == nil {
		t.Fatal("router returned nil model on GatesChangedMsg")
	}
}

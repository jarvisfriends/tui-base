package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jarvisfriends/inspector"
	"github.com/jarvisfriends/snap/gate"
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

// TestInspectorGatesChangedMsgRebroadcasts asserts a gate flip made inside the
// Inspector's own Settings tab (Ctrl+D) — which emits inspector.GatesChangedMsg,
// not settings.GatesChangedMsg — still reaches the app-facing contract: the
// router must re-broadcast it as settings.GatesChangedMsg so a host app's own
// pages that listen for that documented message see gate flips regardless of
// where the toggle happened.
func TestInspectorGatesChangedMsgRebroadcasts(t *testing.T) {
	g := gate.NewGateRegistry()
	m := NewWithOptions(Options{AppName: "Gate Rebroadcast App", Gates: g})
	defer m.Close()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	values := map[string]bool{"Some Gate": true}
	_, cmd := m.Update(inspector.GatesChangedMsg{Values: values})
	if cmd == nil {
		t.Fatal("router returned nil cmd for inspector.GatesChangedMsg")
	}

	// tea.Batch collapses to the single command directly when the other one
	// (the resize cmd) is nil, so accept either a bare settings.GatesChangedMsg
	// or a BatchMsg containing one.
	cmds := []tea.Cmd{cmd}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		cmds = batch
	}
	var found bool
	for _, c := range cmds {
		if sc, ok := c().(settings.GatesChangedMsg); ok {
			found = true
			if !sc.Values["Some Gate"] {
				t.Fatalf("re-broadcast Values = %v; want Some Gate=true", sc.Values)
			}
		}
	}
	if !found {
		t.Fatal("router did not re-broadcast settings.GatesChangedMsg")
	}
}

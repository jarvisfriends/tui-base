package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// recordingDebugModel is a stub Options.DebugOverlay that records what it
// receives.
type recordingDebugModel struct {
	inited bool
	keys   []string
	sized  bool
}

func (m *recordingDebugModel) Init() tea.Cmd { m.inited = true; return nil }

func (m *recordingDebugModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyPressMsg:
		m.keys = append(m.keys, v.String())
	case tea.WindowSizeMsg:
		m.sized = true
	}
	return m, nil
}

func (m *recordingDebugModel) View() tea.View { return tea.NewView("custom debug view") }

// TestWithDebugOverlayOwnsCtrlD asserts the injected debug model takes over
// Ctrl+D from the built-in inspector (Q-22): Ctrl+D opens it, keys are
// forwarded while visible, Ctrl+D closes it again, and the built-in inspector
// never becomes visible.
func TestWithDebugOverlayOwnsCtrlD(t *testing.T) {
	stub := &recordingDebugModel{}
	m := NewWithOptions(Options{AppName: "Debug Overlay App", DebugOverlay: stub})
	defer m.Close()
	_ = m.Init()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	if !stub.inited {
		t.Fatal("router.Init did not Init the injected debug model")
	}

	ctrlD := tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	_, _ = m.Update(ctrlD)
	if !m.debugOverlayVisible {
		t.Fatal("Ctrl+D did not open the injected debug overlay")
	}
	if m.inspector.IsVisible() {
		t.Fatal("built-in inspector opened despite an injected debug overlay")
	}

	// Render once so the overlay lays out and sizes the model.
	_ = m.View()
	if !stub.sized {
		t.Error("debug model never received a WindowSizeMsg from the overlay")
	}

	// A regular key while visible must reach the injected model, not the page.
	_, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if len(stub.keys) == 0 {
		t.Fatal("keys were not forwarded to the visible debug overlay")
	}

	// Ctrl+D again closes it.
	_, _ = m.Update(ctrlD)
	if m.debugOverlayVisible {
		t.Fatal("second Ctrl+D did not close the injected debug overlay")
	}
}

// TestBuiltInInspectorStillOwnsCtrlDWithoutInjection asserts the default path
// is unchanged when no debug overlay is injected.
func TestBuiltInInspectorStillOwnsCtrlDWithoutInjection(t *testing.T) {
	m := NewWithOptions(Options{AppName: "No Injection App"})
	defer m.Close()
	_ = m.Init()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	_, _ = m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if !m.inspector.IsVisible() {
		t.Fatal("Ctrl+D did not open the built-in inspector")
	}
	if m.debugOverlayVisible {
		t.Fatal("custom debug overlay flagged visible with no injected model")
	}
}

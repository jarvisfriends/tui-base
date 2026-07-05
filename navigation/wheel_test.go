package navigation

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// wheelAt builds a horizontal or vertical wheel event at the tab row.
func wheelAt(button tea.MouseButton, mod tea.KeyMod) tea.MouseWheelMsg {
	return tea.MouseWheelMsg(tea.Mouse{X: 1, Y: 0, Button: button, Mod: mod})
}

// runWheel sends the wheel event to the view's OnMouse handler and executes
// the produced command, returning the emitted message (or nil).
func runWheel(t *testing.T, v tea.View, msg tea.MouseWheelMsg) tea.Msg {
	t.Helper()
	if v.OnMouse == nil {
		t.Fatal("view has no OnMouse handler")
	}
	cmd := v.OnMouse(msg)
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestTabsHorizontalWheelCyclesPages(t *testing.T) {
	t.Parallel()

	m := NewTabs()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	msg := runWheel(t, m.View(), wheelAt(tea.MouseWheelRight, 0))
	if sel, ok := msg.(SelectedMsg); !ok || sel.PageIndex != 1 {
		t.Fatalf("wheel-right: got %v; want SelectedMsg{PageIndex:1}", msg)
	}

	// Wheel-left wraps from the first tab to the last.
	m.ActiveIndex = 0
	msg = runWheel(t, m.View(), wheelAt(tea.MouseWheelLeft, 0))
	if sel, ok := msg.(SelectedMsg); !ok || sel.PageIndex != len(m.Pages)-1 {
		t.Fatalf("wheel-left wrap: got %v; want SelectedMsg{PageIndex:%d}", msg, len(m.Pages)-1)
	}

	// Shift+vertical wheel is the common terminal encoding for horizontal.
	m.ActiveIndex = 0
	msg = runWheel(t, m.View(), wheelAt(tea.MouseWheelDown, tea.ModShift))
	if sel, ok := msg.(SelectedMsg); !ok || sel.PageIndex != 1 {
		t.Fatalf("shift+wheel-down: got %v; want SelectedMsg{PageIndex:1}", msg)
	}

	// A plain vertical wheel is NOT horizontal scrolling and must not switch.
	m.ActiveIndex = 0
	if msg = runWheel(t, m.View(), wheelAt(tea.MouseWheelDown, 0)); msg != nil {
		t.Fatalf("plain wheel-down must not switch tabs; got %v", msg)
	}
}

func TestMinimalTopNavHorizontalWheelCyclesPages(t *testing.T) {
	t.Parallel()

	m := NewMinimalTopNav()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	msg := runWheel(t, m.View(), wheelAt(tea.MouseWheelRight, 0))
	if sel, ok := msg.(SelectedMsg); !ok || sel.PageIndex != 1 {
		t.Fatalf("wheel-right: got %v; want SelectedMsg{PageIndex:1}", msg)
	}

	m.ActiveIndex = 0
	msg = runWheel(t, m.View(), wheelAt(tea.MouseWheelUp, tea.ModShift))
	if sel, ok := msg.(SelectedMsg); !ok || sel.PageIndex != len(m.Pages)-1 {
		t.Fatalf("shift+wheel-up wrap: got %v; want SelectedMsg{PageIndex:%d}", msg, len(m.Pages)-1)
	}
}

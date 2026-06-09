package overlay

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/jarvisfriends/tui-base/geom"
)

func TestCenteredBase_Place(t *testing.T) {
	cb := &CenteredBase{}
	content := "hello\nworld" // width=5, height=2
	placed := cb.Place(content, 20, 10)

	if placed != content {
		t.Errorf("Place returned %q; want %q", placed, content)
	}

	expectedBounds := geom.Rect{W: 5, H: 2}.CenteredIn(20, 10)
	if cb.Bounds() != expectedBounds {
		t.Errorf("Bounds() = %+v; want %+v", cb.Bounds(), expectedBounds)
	}
}

func TestFormWidth(t *testing.T) {
	tests := []struct {
		termW int
		want  int
	}{
		{20, 30},   // min width is 30
		{40, 30},   // 40 - 14 = 26 < 30 -> 30
		{80, 66},   // 80 - 14 = 66
		{150, 120}, // max width is 120
	}

	for _, tc := range tests {
		if got := FormWidth(tc.termW); got != tc.want {
			t.Errorf("FormWidth(%d) = %d; want %d", tc.termW, got, tc.want)
		}
	}
}

func TestFormOverlayHost(t *testing.T) {
	h := &FormOverlayHost{}
	if h.IsOpen() {
		t.Error("host should not be open initially")
	}

	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Value(new(string)),
		),
	)

	cmd := h.Open(f, 80, 24)
	if cmd == nil {
		t.Error("expected non-nil Init command on Open")
	}
	if !h.IsOpen() {
		t.Error("host should be open")
	}

	// OnResize
	h.OnResize(100, 30)
	if h.termW != 100 || h.termH != 30 {
		t.Errorf("OnResize did not update dimensions: %d, %d", h.termW, h.termH)
	}

	// Composite
	base := "base content line 1\nbase content line 2\nbase content line 3"
	rendered := h.Composite(base, lipgloss.NewStyle())
	if rendered == base {
		t.Error("Composite should render form over base, not return base unchanged")
	}

	bounds := h.Bounds()
	if bounds.W <= 0 || bounds.H <= 0 {
		t.Errorf("invalid composite bounds: %+v", bounds)
	}

	// IsOutsideClick
	if h.IsOutsideClick(bounds.X+1, bounds.Y+1) {
		t.Error("click inside bounds reported as outside click")
	}
	if !h.IsOutsideClick(bounds.X-1, bounds.Y-1) {
		t.Error("click outside bounds reported as inside click")
	}

	_, _ = h.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	// ctrl+c should abort form
	if h.form.State != huh.StateAborted {
		t.Errorf("expected form state to be aborted, got %v", h.form.State)
	}

	// Close
	h.Close()
	if h.IsOpen() {
		t.Error("host should not be open after Close")
	}
	if h.IsOutsideClick(0, 0) {
		t.Error("IsOutsideClick should be false when closed")
	}

	// Boundary: Update with nil form
	state2, cmd2 := h.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if state2 != huh.StateNormal || cmd2 != nil {
		t.Errorf("expected Normal state and nil cmd when form is nil; got %v, %v", state2, cmd2)
	}

	// Boundary: Composite when not open
	rendered2 := h.Composite(base, lipgloss.NewStyle())
	if rendered2 != base {
		t.Errorf("expected Composite to return base string when closed; got %q", rendered2)
	}
}

package home

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestHomeScrollsOnSmallTerminal verifies the welcome content scrolls (mouse
// wheel and keyboard) when the terminal is too short to show it all, instead of
// silently clipping.
func TestHomeScrollsOnSmallTerminal(t *testing.T) {
	m := New()
	// A height of 3 is far shorter than the bordered welcome box, forcing overflow.
	_, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 3})
	_ = m.View() // syncs viewport content

	if m.vp.TotalLineCount() <= m.vp.VisibleLineCount() {
		t.Fatalf("precondition: content (%d lines) should overflow the %d-line viewport",
			m.vp.TotalLineCount(), m.vp.VisibleLineCount())
	}
	if m.vp.AtBottom() {
		t.Fatal("precondition: viewport should start at the top")
	}

	// Mouse wheel down scrolls.
	before := m.vp.YOffset()
	_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	if m.vp.YOffset() <= before {
		t.Fatalf("mouse wheel did not scroll: YOffset stayed at %d", m.vp.YOffset())
	}

	// Keyboard down also scrolls (does not go backwards).
	mid := m.vp.YOffset()
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.vp.YOffset() < mid {
		t.Fatalf("keyboard down scrolled backwards: %d -> %d", mid, m.vp.YOffset())
	}
}

// TestHomeNoScrollWhenItFits verifies that on a normal terminal the content fits
// and the viewport stays at the top (no spurious scrolling).
func TestHomeNoScrollWhenItFits(t *testing.T) {
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	_ = m.View()
	if !m.vp.AtTop() {
		t.Fatalf("expected viewport at top when content fits, YOffset=%d", m.vp.YOffset())
	}
}

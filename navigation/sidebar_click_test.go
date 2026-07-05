package navigation

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// selectedFrom unwraps a SelectedMsg from a cmd or a BatchMsg.
func selectedFrom(cmd tea.Cmd) (SelectedMsg, bool) {
	if cmd == nil {
		return SelectedMsg{}, false
	}
	switch msg := cmd().(type) {
	case SelectedMsg:
		return msg, true
	case tea.BatchMsg:
		for _, sub := range msg {
			if sub == nil {
				continue
			}
			if sel, ok := sub().(SelectedMsg); ok {
				return sel, true
			}
		}
	}
	return SelectedMsg{}, false
}

// TestSidebarClickSelectsEveryMainItem is a regression test for the bug where a
// blank spacing row between list items (navItemSpacing) made every item after
// the first unreachable by mouse: handleMouse assumed consecutive rows. We click
// each item at its true row (header + idx*navItemStride) and verify it selects.
//
// Clicking by computed row (not by searching rendered text) also avoids the
// fragility where an active item's styling — e.g. underline — splits its title
// with ANSI escapes so a raw substring search misses it.
func TestSidebarClickSelectsEveryMainItem(t *testing.T) {
	t.Parallel()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := m.View()

	numMain := m.numMainItems()
	if numMain < 2 {
		t.Skipf("need at least 2 main nav items to exercise spacing; have %d", numMain)
	}

	for idx := range numMain {
		y := 1 + idx*navItemStride // row 0 is the header; items every stride rows
		cmd := v.OnMouse(tea.MouseReleaseMsg{X: 0, Y: y, Button: tea.MouseLeft})
		sel, ok := selectedFrom(cmd)
		if !ok {
			t.Fatalf(
				"clicking main item %d at y=%d did not emit SelectedMsg; got %T",
				idx,
				y,
				cmd(),
			)
		}
		if sel.PageIndex != idx {
			t.Fatalf(
				"clicking main item %d at y=%d selected PageIndex=%d, want %d",
				idx,
				y,
				sel.PageIndex,
				idx,
			)
		}
		if m.ActiveIndex != idx {
			t.Fatalf("after click, ActiveIndex=%d, want %d", m.ActiveIndex, idx)
		}
	}
}

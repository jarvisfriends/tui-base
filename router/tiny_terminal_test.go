package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/testutil"
)

// TestTinyTerminals drives the router at the cramped sizes from TS-3 —
// including 40x10, smaller than any standard-matrix pair — across page
// switches and the inspector overlay, asserting the frame never exceeds the
// terminal box.
func TestTinyTerminals(t *testing.T) {
	t.Parallel()

	sizes := []struct{ w, h int }{{40, 10}, {60, 20}, {80, 24}}
	for _, size := range sizes {
		m := New()
		_, _ = m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		testutil.AssertBounds(t, m, size.w, size.h)

		// Cycle to the settings page and re-check.
		_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		testutil.AssertBounds(t, m, size.w, size.h)

		// Inspector overlay open.
		_, _ = m.Update(tea.KeyPressMsg{Text: testKeyInspector})
		testutil.AssertBounds(t, m, size.w, size.h)
	}
}

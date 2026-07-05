package router

// Regression test: a CLOSED inspector overlay must not act on its own
// keybindings. Previously the router forwarded key messages to the inspector
// whenever it was NOT visible (the guard only skipped keys while visible), so
// inspector shortcuts — e.g. its test Info/Warn/Error toast keys and tab
// switching — fired while the overlay was hidden. Keys now reach the inspector
// only via the overlay key handler, which runs only when it is visible.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestClosedInspectorIgnoresKeys(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	if m.inspector.IsVisible() {
		t.Fatal("precondition: inspector should start closed")
	}
	before := stripANSI(m.inspector.View().Content)
	if !strings.Contains(before, "Runtime Profiling (Inspector)") {
		t.Fatalf("precondition: inspector should start on the Runtime tab, got:\n%s", before)
	}

	// Drive → through the router while the inspector is closed. A closed
	// inspector must not advance its tab (or fire any other shortcut).
	for range 3 {
		_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}

	after := stripANSI(m.inspector.View().Content)
	if !strings.Contains(after, "Runtime Profiling (Inspector)") {
		t.Fatalf(
			"closed inspector acted on the → key (its tab changed); it must ignore keys while hidden. Got:\n%s",
			after,
		)
	}

	// Sanity: once visible, the overlay path DOES drive the inspector (one tab
	// advance per →), confirming we only gated the closed case.
	m.inspector.ToggleVisible()
	_ = m.handleResizeCmd()
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	visibleAfter := stripANSI(m.inspector.View().Content)
	if strings.Contains(visibleAfter, "Runtime Profiling (Inspector)") {
		t.Fatalf(
			"open inspector did not respond to →; expected it to leave the Runtime tab. Got:\n%s",
			visibleAfter,
		)
	}
}

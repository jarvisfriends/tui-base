package router

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	ov "github.com/jarvisfriends/tui-base/overlay"
)

// consumerOverlay is a minimal external overlay: visible flag, fixed content,
// modal for keys while open (the "implement + append" contract of ADR-013).
type consumerOverlay struct {
	ov.CenteredBase
	open bool
	keys int
}

func (o *consumerOverlay) Name() string  { return "consumer" }
func (o *consumerOverlay) Z() int        { return 90 } // above all built-ins
func (o *consumerOverlay) Visible() bool { return o.open }

func (o *consumerOverlay) Render(ctx ov.Context) string {
	return o.Place("CONSUMER-OVERLAY-CONTENT", ctx.Width, ctx.Height)
}

func (o *consumerOverlay) OverlayKey(k tea.KeyPressMsg) tea.Cmd {
	o.keys++
	if k.Code == tea.KeyEscape {
		o.open = false
	}
	return nil
}

// TestRegisterOverlayExternal verifies the E-2 exposure: a consumer overlay
// registered via RegisterOverlay is composited into the frame, intercepts
// keys while visible (including from the built-in stack's perspective), and
// releases input when closed.
func TestRegisterOverlayExternal(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	o := &consumerOverlay{open: true}
	m.RegisterOverlay(o)

	if !strings.Contains(m.View().Content, "CONSUMER-OVERLAY-CONTENT") {
		t.Fatal("registered overlay content not composited into the frame")
	}

	// Keys go to the overlay, not the page: Tab must NOT cycle pages.
	before := m.nav.GetActiveIndex()
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if o.keys != 1 {
		t.Fatalf("overlay saw %d keys; want 1", o.keys)
	}
	if got := m.nav.GetActiveIndex(); got != before {
		t.Fatalf("page cycled to %d while a modal overlay was open", got)
	}

	// Esc closes it; input returns to the page.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if o.open {
		t.Fatal("overlay still open after Esc")
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if got := m.nav.GetActiveIndex(); got == before {
		t.Fatal("Tab did not cycle pages after the overlay closed")
	}
	if strings.Contains(m.View().Content, "CONSUMER-OVERLAY-CONTENT") {
		t.Fatal("closed overlay still rendered")
	}
}

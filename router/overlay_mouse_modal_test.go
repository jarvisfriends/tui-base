package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/status"

	tea "charm.land/bubbletea/v2"
)

// mouseRecorderPage counts the mouse messages that reach it through Update.
// Pointer receiver so counts survive the router storing returned models.
type mouseRecorderPage struct {
	mouseMsgs int
}

func (p *mouseRecorderPage) Init() tea.Cmd { return nil }

func (p *mouseRecorderPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.MouseMsg); ok {
		p.mouseMsgs++
	}
	return p, nil
}

func (p *mouseRecorderPage) View() tea.View { return tea.NewView("mouse recorder") }

// TestModalOverlayBlocksUpdatePathMouse is the regression test for wheel
// events leaking behind the notification-history and info overlays: Bubble Tea
// delivers mouse events to both View.OnMouse and Update, and the Update path
// used to forward them to the active page even while a modal overlay was open.
func TestModalOverlayBlocksUpdatePathMouse(t *testing.T) {
	t.Parallel()

	rec := &mouseRecorderPage{}
	m := NewWithRegisteredPages([]RegisteredPage{{Title: RecorderPageTitle, Model: rec}})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})

	wheel := func() {
		_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{X: 5, Y: 5, Button: tea.MouseWheelDown}))
	}

	// Baseline: with no overlay open, the active page receives the wheel.
	wheel()
	if rec.mouseMsgs != 1 {
		t.Fatalf("baseline: page saw %d mouse msgs; want 1", rec.mouseMsgs)
	}

	// Notification history open: wheel must NOT reach the page.
	m = updateRouter(m, notifications.AddMsg{
		Content:  "modal-mouse-test",
		Severity: notifications.SeverityInfo,
		TTL:      0,
	})
	_ = m.status.ToggleNotifications()
	if !m.overlayByName("history").Visible() {
		t.Fatal("history overlay did not open")
	}
	wheel()
	if rec.mouseMsgs != 1 {
		t.Fatalf(
			"history open: page saw %d mouse msgs; want 1 (wheel leaked through Update)",
			rec.mouseMsgs,
		)
	}
	_ = m.status.ToggleNotifications() // close

	// Info modal open: wheel must NOT reach the page.
	m.infoModal.Toggle(m.width, m.height)
	if !m.overlayByName("info").Visible() {
		t.Fatal("info overlay did not open")
	}
	wheel()
	if rec.mouseMsgs != 1 {
		t.Fatalf(
			"info open: page saw %d mouse msgs; want 1 (wheel leaked through Update)",
			rec.mouseMsgs,
		)
	}
	m.infoModal.Close()

	// Closed again: the page receives wheels once more.
	wheel()
	if rec.mouseMsgs != 2 {
		t.Fatalf("after close: page saw %d mouse msgs; want 2", rec.mouseMsgs)
	}
}

// TestHistoryOverlayWheelOutsideBoundsScrollsOverlay verifies wheel events are
// treated as positionless scrolling intent: even with the pointer over the
// page area behind the panel, the wheel moves the history cursor — matching
// how keyboard navigation targets the overlay while it is open.
func TestHistoryOverlayWheelOutsideBoundsScrollsOverlay(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	for i := range 5 {
		m = updateRouter(m, notifications.AddMsg{
			Content:  "outside-wheel",
			Severity: notifications.SeverityInfo,
			TTL:      0,
		})
		_ = i
	}
	_ = m.status.ToggleNotifications()

	v := m.View()
	hb := m.overlayByName("history").Bounds()
	if hb.Contains(0, 0) {
		t.Fatal("test expects (0,0) to be outside the history panel bounds")
	}
	before := m.status.HistoryCursor()
	cmd := v.OnMouse(tea.MouseWheelMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseWheelDown}))
	if cmd != nil {
		_ = cmd()
	}
	if got := m.status.HistoryCursor(); got != before+1 {
		t.Fatalf("history cursor = %d after outside-bounds wheel; want %d", got, before+1)
	}
}

// TestInfoOverlayWheelOutsideBoundsScrollsModal verifies the info modal
// receives wheel scrolling regardless of pointer position while open.
func TestInfoOverlayWheelOutsideBoundsScrollsModal(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m.infoModal.Toggle(m.width, m.height)

	v := m.View()
	ib := m.overlayByName("info").Bounds()
	if ib.Contains(0, 0) {
		t.Fatal("test expects (0,0) to be outside the info modal bounds")
	}
	cmd := v.OnMouse(tea.MouseWheelMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseWheelDown}))
	if cmd == nil {
		t.Fatal("expected the info overlay to consume the outside-bounds wheel")
	}
	msg := cmd()
	scroll, ok := msg.(status.InfoModalScrollMsg)
	if !ok {
		t.Fatalf("wheel produced %T; want status.InfoModalScrollMsg", msg)
	}
	if scroll.Up {
		t.Fatal("wheel-down produced an Up scroll msg")
	}
}

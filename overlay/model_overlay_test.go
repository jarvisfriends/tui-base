package overlay

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/snap/geom"
)

// hostedStub is a minimal hosted model that records what the host delivers:
// Init calls, Update msgs, and OnMouse events (with translated coordinates).
type hostedStub struct {
	inits   int
	updates []tea.Msg
	mouse   []tea.Mouse
}

func (m *hostedStub) Init() tea.Cmd { m.inits++; return nil }

func (m *hostedStub) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updates = append(m.updates, msg)
	return m, nil
}

func (m *hostedStub) View() tea.View {
	v := tea.NewView("hosted body")
	v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		m.mouse = append(m.mouse, mm.Mouse())
		return nil
	}
	return v
}

// TestModelOverlayLifecycle: Open shows the overlay and Inits the model,
// Update forwards messages, Close releases everything.
func TestModelOverlayLifecycle(t *testing.T) {
	t.Parallel()

	var h ModelOverlayHost
	if h.IsOpen() {
		t.Fatal("zero-value host should be closed")
	}
	if cmd := h.Update(tea.KeyPressMsg{Code: 'x'}); cmd != nil {
		t.Fatal("Update on a closed host should be a no-op")
	}

	stub := &hostedStub{}
	_ = h.Open(stub, 80, 24)
	if !h.IsOpen() || stub.inits != 1 {
		t.Fatalf("Open: open=%v inits=%d", h.IsOpen(), stub.inits)
	}
	if h.Model() != tea.Model(stub) {
		t.Fatal("Model should return the hosted model")
	}

	h.Update(tea.KeyPressMsg{Code: 'x'})
	if len(stub.updates) != 1 {
		t.Fatalf("Update should forward to the model; got %d msgs", len(stub.updates))
	}
	h.OnResize(100, 30)
	if len(stub.updates) != 2 {
		t.Fatal("OnResize should deliver a WindowSizeMsg")
	}
	if ws, ok := stub.updates[1].(tea.WindowSizeMsg); !ok || ws.Width != 100 || ws.Height != 30 {
		t.Fatalf("resize msg = %+v", stub.updates[1])
	}

	h.Close()
	if h.IsOpen() || h.Model() != nil {
		t.Fatal("Close should release the model")
	}
	if h.Bounds() != (geom.Rect{}) {
		t.Fatalf("Close should clear bounds; got %+v", h.Bounds())
	}
}

// TestModelOverlayCompositeAndClicks: Composite centers the boxed model over
// the base and records bounds; IsOutsideClick answers against those bounds.
func TestModelOverlayCompositeAndClicks(t *testing.T) {
	t.Parallel()

	var h ModelOverlayHost
	base := strings.TrimRight(strings.Repeat(strings.Repeat(".", 40)+"\n", 12), "\n")
	if got := h.Composite(base, lipgloss.NewStyle()); got != base {
		t.Fatal("closed host should return the base unchanged")
	}

	_ = h.Open(&hostedStub{}, 40, 12)
	out := ansi.Strip(h.Composite(base, lipgloss.NewStyle()))
	if !strings.Contains(out, "hosted body") {
		t.Fatalf("composite missing hosted content:\n%s", out)
	}

	b := h.Bounds()
	if b.W == 0 || b.H == 0 {
		t.Fatalf("Composite should record bounds; got %+v", b)
	}
	if h.IsOutsideClick(b.X+1, b.Y+1) {
		t.Fatal("click inside bounds reported as outside")
	}
	if !h.IsOutsideClick(0, 0) {
		t.Fatal("click at the corner should be outside the centered overlay")
	}
}

// TestModelOverlayForwardMouseTranslates: mouse events land on the hosted
// model's View().OnMouse with content-relative coordinates (bounds origin
// plus the border/padding insets subtracted).
func TestModelOverlayForwardMouseTranslates(t *testing.T) {
	t.Parallel()

	var h ModelOverlayHost
	stub := &hostedStub{}
	if cmd := h.ForwardMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 1})); cmd != nil {
		t.Fatal("closed host should not forward mouse")
	}

	base := strings.TrimRight(strings.Repeat(strings.Repeat(".", 40)+"\n", 12), "\n")
	_ = h.Open(stub, 40, 12)
	_ = h.Composite(base, lipgloss.NewStyle()) // establish bounds

	b := h.Bounds()
	click := tea.Mouse{
		X:      b.X + modelOverlayInsetX + 5,
		Y:      b.Y + modelOverlayInsetY + 1,
		Button: tea.MouseLeft,
	}
	_ = h.ForwardMouse(tea.MouseClickMsg(click))
	if len(stub.mouse) != 1 {
		t.Fatalf("expected 1 forwarded event, got %d", len(stub.mouse))
	}
	if got := stub.mouse[0]; got.X != 5 || got.Y != 1 || got.Button != tea.MouseLeft {
		t.Fatalf("translated event = %+v; want X=5 Y=1 left", got)
	}

	// Wheel/motion/release variants forward too; unknown types are dropped.
	_ = h.ForwardMouse(tea.MouseWheelMsg(click))
	_ = h.ForwardMouse(tea.MouseMotionMsg(click))
	_ = h.ForwardMouse(tea.MouseReleaseMsg(click))
	if len(stub.mouse) != 4 {
		t.Fatalf("expected all event kinds forwarded, got %d", len(stub.mouse))
	}
}

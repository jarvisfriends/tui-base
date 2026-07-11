package home

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/snap/styles"
)

// sized returns a home page at a comfortable size with content synced.
func sized(t *testing.T, w, h int) *HomePageModel {
	t.Helper()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	_ = m.View()
	return m
}

// click delivers a click through View().OnMouse at content coordinates.
func click(m *HomePageModel, x, y int) {
	_ = m.View().OnMouse(tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
}

// TestHomePillClickCyclesShape: clicking the pill strip cycles through the
// snap PillShapes; clicking empty space does not.
func TestHomePillClickCyclesShape(t *testing.T) {
	m := sized(t, 100, 40)
	b, ok := m.zones.Bounds(zonePills)
	if !ok {
		t.Fatal("pill zone not registered")
	}

	if m.shape != 0 {
		t.Fatalf("initial shape = %d", m.shape)
	}
	click(m, b.X+1, b.Y)
	if m.shape != 1 {
		t.Fatalf("shape after pill click = %d; want 1", m.shape)
	}
	for range len(styles.PillShapes()) - 1 {
		click(m, b.X+1, b.Y)
	}
	if m.shape != 0 {
		t.Fatalf("shapes should wrap; got %d", m.shape)
	}

	click(m, 0, 0) // far corner: no zone
	if m.shape != 0 {
		t.Fatal("empty-space click should not cycle the shape")
	}
}

// TestHomeChartClickPausesStream: clicking the charts block toggles the demo
// stream; ticks while paused change nothing.
func TestHomeChartClickPausesStream(t *testing.T) {
	m := sized(t, 100, 40)
	_ = m.OnEnter()
	_, _ = m.Update(tickMsg{})
	if len(m.spark.History()) != 1 {
		t.Fatalf("tick should append a sample; history=%d", len(m.spark.History()))
	}

	b, ok := m.zones.Bounds(zoneCharts)
	if !ok {
		t.Fatal("charts zone not registered")
	}
	click(m, b.X+1, b.Y)
	if !m.paused {
		t.Fatal("chart click should pause")
	}
	_, _ = m.Update(tickMsg{})
	if len(m.spark.History()) != 1 {
		t.Fatal("paused tick should not append samples")
	}
	click(m, b.X+1, b.Y)
	if m.paused {
		t.Fatal("second click should resume")
	}
}

// TestHomeLifecycleGatesTicker: OnEnter arms the demo stream and returns the
// tick command; after OnLeave, in-flight ticks neither advance the charts nor
// re-arm (I-1 hooks in action).
func TestHomeLifecycleGatesTicker(t *testing.T) {
	m := sized(t, 100, 40)
	if m.ticking {
		t.Fatal("stream should start disarmed")
	}
	if cmd := m.OnEnter(); cmd == nil {
		t.Fatal("OnEnter should return the tick command")
	}

	_ = m.OnLeave()
	_, cmd := m.Update(tickMsg{})
	if cmd != nil {
		t.Fatal("a tick after OnLeave must not re-arm")
	}
	if len(m.spark.History()) != 0 {
		t.Fatal("a tick after OnLeave must not advance the charts")
	}
}

// TestHomeScrollbarClickJumps: on an overflowing terminal the snap scrollbar
// renders at the right edge and clicking its lower track jumps the viewport;
// dragging keeps tracking until release.
func TestHomeScrollbarClickJumps(t *testing.T) {
	m := sized(t, 40, 4)
	if !m.scrollbarNeeded() {
		t.Fatal("precondition: content should overflow")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "│") {
		t.Fatal("scrollbar should render when content overflows")
	}

	click(m, 39, 3) // bottom of the bar
	if m.vp.YOffset() == 0 {
		t.Fatal("scrollbar click should jump the viewport")
	}
	if !m.dragging {
		t.Fatal("click on the bar should begin a drag")
	}

	_ = m.View().OnMouse(tea.MouseMotionMsg(tea.Mouse{X: 39, Y: 0}))
	if m.vp.YOffset() != 0 {
		t.Fatalf("drag to the top should jump back; YOffset=%d", m.vp.YOffset())
	}
	_ = m.View().OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: 39, Y: 0}))
	if m.dragging {
		t.Fatal("release should end the drag")
	}
	_ = m.View().OnMouse(tea.MouseMotionMsg(tea.Mouse{X: 39, Y: 3}))
	if m.vp.YOffset() != 0 {
		t.Fatal("motion after release must not scroll")
	}
}

// TestHomeUpdateIgnoresMouse pins the input contract: raw mouse messages fed
// to Update are dropped — OnMouse is the only pointer door.
func TestHomeUpdateIgnoresMouse(t *testing.T) {
	m := sized(t, 40, 4)
	_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	if m.vp.YOffset() != 0 {
		t.Fatal("Update must not react to mouse messages")
	}
}

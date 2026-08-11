// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/geom"
	"github.com/jarvisfriends/snap/notifications"
	"github.com/jarvisfriends/snap/status"
	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/theme"
)

// closerOnlyOverlay is an OutsideCloser that is NOT a MouseConsumer, covering
// the modality decisions for overlays that only react to outside clicks.
type closerOnlyOverlay struct {
	z      int
	open   bool
	closed int
}

func (o *closerOnlyOverlay) Name() string          { return "closer-only" }
func (o *closerOnlyOverlay) Z() int                { return o.z }
func (o *closerOnlyOverlay) Visible() bool         { return o.open }
func (o *closerOnlyOverlay) Bounds() Rect          { return geom.Rect{X: 10, Y: 10, W: 10, H: 5} }
func (o *closerOnlyOverlay) Render(Context) string { return "CLOSER" }
func (o *closerOnlyOverlay) CloseOnOutsideClick() tea.Cmd {
	o.open = false
	o.closed++
	return nil
}

func TestRegisterOverlayKeepsZOrder(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Z Order"})

	// Z below every built-in: must be inserted first, not appended.
	low := &closerOnlyOverlay{z: 5}
	m.RegisterOverlay(low)
	if m.overlays[len(m.overlays)-1] == Overlay(low) {
		t.Fatal("low-Z overlay was appended instead of inserted")
	}
	if m.overlayByName("closer-only") == nil {
		t.Fatal("registered overlay not findable by name")
	}
	if m.overlayByName("no-such-overlay") != nil {
		t.Fatal("unknown overlay name should return nil")
	}
	if FormOverlayWidth(120) <= 0 {
		t.Fatal("FormOverlayWidth should be positive")
	}
}

func TestCloserOnlyOverlayMouseModality(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Closer"})
	o := &closerOnlyOverlay{z: 95, open: true}
	// Give it the highest Z so it is the topmost modal surface.
	m.overlays = append(m.overlays, o)

	if !m.mouseModalOverlayVisible() {
		t.Fatal("an open OutsideCloser must be mouse-modal")
	}

	// Inside the bounds: consumed, no close.
	cmd, ok := m.overlayHandleMouse(tea.MouseClickMsg(tea.Mouse{X: 12, Y: 12}))
	if !ok || cmd != nil || o.closed != 0 {
		t.Fatalf("inside click: cmd=%v ok=%v closed=%d", cmd, ok, o.closed)
	}
	// Outside, but not a release: still consumed, no close.
	if _, ok := m.overlayHandleMouse(tea.MouseMotionMsg(tea.Mouse{X: 0, Y: 0})); !ok {
		t.Fatal("outside motion should be consumed by the modal overlay")
	}
	if o.closed != 0 {
		t.Fatal("motion must not close the overlay")
	}
	// Outside release closes.
	if _, ok := m.overlayHandleMouse(tea.MouseReleaseMsg(tea.Mouse{X: 0, Y: 0})); !ok {
		t.Fatal("outside release should be consumed")
	}
	if o.closed != 1 {
		t.Fatal("outside release should close the overlay")
	}

	// With nothing visible, mouse events fall through.
	if _, ok := m.overlayHandleMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 1})); ok {
		t.Fatal("no visible overlay should consume the mouse")
	}
	if m.mouseModalOverlayVisible() {
		t.Fatal("no visible overlay should be mouse-modal")
	}
}

func TestHiddenOverlaysRenderEmpty(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Empty Renders"})
	ctx := m.layoutCtx(1)

	if got := (&toastOverlay{m: m}).Render(ctx); got != "" {
		t.Errorf("toast with no notifications rendered %q", got)
	}
	if got := (&historyOverlay{m: m}).Render(ctx); got != "" {
		t.Errorf("closed history panel rendered %q", got)
	}
	if got := (&infoOverlay{m: m}).Render(ctx); got != "" {
		t.Errorf("closed info modal rendered %q", got)
	}
}

func TestToastRendersProgressBar(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Toast"})
	pct := 40.0
	_, _ = m.notifMgr.AddWithOptions(
		"copying a fairly long file name that gets truncated in the toast body",
		notifications.SeverityInfo, 0, notifications.AddOptions{Key: "cp", Percent: &pct},
	)
	o := &toastOverlay{m: m}
	if !o.Visible() {
		t.Fatal("toast should be visible with an active notification")
	}
	if got := o.Render(m.layoutCtx(1)); got == "" {
		t.Fatal("progress toast should render")
	}
	if o.Bounds().W == 0 {
		t.Fatal("toast bounds should be recorded after render")
	}
}

func TestHistoryOverlayKeysAndMouse(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "History Keys"})

	// One pending (actionable) and one plain notification.
	_, _ = m.notifMgr.AddWithOptions("pending job", notifications.SeverityInfo, 0,
		notifications.AddOptions{Pending: true, Key: "job"})
	_, _ = m.notifMgr.Add("plain", notifications.SeverityInfo, 0)
	drainCmd(m, m.status.ToggleNotifications())
	if !m.status.IsHistoryVisible() {
		t.Fatal("history should be open")
	}
	_ = m.View() // lay the panel out so its bounds are recorded

	o, okCast := m.overlayByName("history").(*historyOverlay)
	if !okCast {
		t.Fatal("history overlay missing")
	}

	// Cursor movement, wheel movement.
	_ = o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyDown})
	_ = o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyUp})
	_ = o.OverlayMouse(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	_ = o.OverlayMouse(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if cmd := o.OverlayMouse(tea.MouseClickMsg(tea.Mouse{})); cmd != nil {
		t.Error("clicks inside the panel are no-ops")
	}

	// A consumer action handler fires on its registered key.
	actioned := false
	m.notifMgr.OnAction("o", func(notifications.Notification) tea.Cmd {
		actioned = true
		return nil
	})
	drainCmd(m, o.OverlayKey(tea.KeyPressMsg{Code: 'o', Text: "o"}))
	if !actioned {
		t.Error("registered action handler was not invoked")
	}

	// Select on a pending notification emits its activation message: walk the
	// cursor onto the pending entry first (history order is manager-defined).
	pendingIdx := -1
	for i, n := range m.notifMgr.Active() {
		if n.Pending {
			pendingIdx = i
		}
	}
	if pendingIdx < 0 {
		t.Fatal("pending notification missing from history")
	}
	for m.status.HistoryCursor() < pendingIdx {
		_ = o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	for m.status.HistoryCursor() > pendingIdx {
		_ = o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyUp})
	}
	sawActivate := false
	collectMsgs(o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter}), func(msg tea.Msg) {
		if _, ok := msg.(notifications.ActivateMsg); ok {
			sawActivate = true
		}
	})
	if !sawActivate {
		t.Error("selecting a pending notification should activate it")
	}

	// Select on a plain notification dismisses it; D dismisses the rest.
	_ = o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = o.OverlayKey(tea.KeyPressMsg{Code: 'D', Text: "D"})

	// Quit/ToggleHistory closes the panel.
	_ = o.OverlayKey(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.status.IsHistoryVisible() {
		t.Fatal("q should close the history panel")
	}
}

func TestInspectorOverlayHelpAndMouse(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Inspector Overlay"})
	m.inspector.ToggleVisible()
	_ = m.View()

	o, okCast := m.overlayByName("inspector").(*inspectorOverlay)
	if !okCast {
		t.Fatal("inspector overlay missing")
	}
	if len(o.ShortHelp()) == 0 {
		t.Error("inspector overlay short help empty")
	}
	if len(o.FullHelp()) == 0 {
		t.Error("inspector overlay full help empty")
	}

	// Mouse forwarding tolerates all event kinds regardless of whether the
	// inspector's current view exposes OnMouse.
	for _, mm := range []tea.MouseMsg{
		tea.MouseClickMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2}),
		tea.MouseReleaseMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2}),
		tea.MouseMotionMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2}),
		tea.MouseWheelMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2, Button: tea.MouseWheelDown}),
	} {
		_ = o.OverlayMouse(mm)
	}

	// Inspector-owned keys route through the overlay; Esc closes it.
	_ = o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyRight})
	drainCmd(m, o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if m.inspector.IsVisible() {
		t.Fatal("Esc should close the inspector overlay")
	}
}

func TestInfoOverlayKeyHandling(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Info Keys"})
	m.infoModal.Toggle(100, 32)
	_ = m.View()

	o, okCast := m.overlayByName("info").(*infoOverlay)
	if !okCast {
		t.Fatal("info overlay missing")
	}
	// A plain key is consumed (relayout); the modal's own close key returns
	// its command instead.
	_ = o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyDown})
	closed := false
	collectMsgs(o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyEscape}), func(msg tea.Msg) {
		if _, ok := msg.(status.CloseInfoModalMsg); ok {
			closed = true
		}
	})
	if !closed && m.infoModal.IsVisible() {
		t.Error("Esc should close the info modal (directly or via message)")
	}
}

// mouseDebugModel is a debug-overlay stub whose view accepts mouse input.
type mouseDebugModel struct {
	events int
}

func (m *mouseDebugModel) Init() tea.Cmd                       { return nil }
func (m *mouseDebugModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m *mouseDebugModel) View() tea.View {
	v := tea.NewView("mouse debug")
	v.OnMouse = func(tea.MouseMsg) tea.Cmd {
		m.events++
		return nil
	}
	return v
}

func TestCustomDebugOverlayMouseAndClose(t *testing.T) {
	stub := &mouseDebugModel{}
	m := newSizedRouter(t, Options{AppName: "Debug Mouse", DebugOverlay: stub})
	_ = m.Init()

	// Ctrl+D opens the injected overlay (not the inspector).
	_, _ = m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if !m.debugOverlayVisible {
		t.Fatal("Ctrl+D should open the injected overlay")
	}
	_ = m.View()

	o, okCast := m.overlayByName("debug-overlay").(*customDebugOverlay)
	if !okCast {
		t.Fatal("custom debug overlay missing")
	}
	for _, mm := range []tea.MouseMsg{
		tea.MouseClickMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2}),
		tea.MouseReleaseMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2}),
		tea.MouseMotionMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2}),
		tea.MouseWheelMsg(tea.Mouse{X: o.rect.X + 2, Y: o.rect.Y + 2, Button: tea.MouseWheelDown}),
	} {
		_ = o.OverlayMouse(mm)
	}
	if stub.events != 4 {
		t.Fatalf("debug model saw %d mouse events; want 4", stub.events)
	}

	// Outside click closes it.
	drainCmd(m, o.CloseOnOutsideClick())
	if m.debugOverlayVisible {
		t.Fatal("outside click should close the debug overlay")
	}

	// Esc-close path through the overlay key handler.
	m.debugOverlayVisible = true
	drainCmd(m, o.OverlayKey(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if m.debugOverlayVisible {
		t.Fatal("Esc should close the debug overlay")
	}

	// Routers without an injected model treat the dispatch as a no-op.
	plain := newSizedRouter(t, Options{AppName: "No Debug"})
	if cmd := plain.updateDebugOverlay(tea.KeyPressMsg{Code: 'x'}); cmd != nil {
		t.Fatal("updateDebugOverlay without a model should be nil")
	}
}

// allMotionPage opts into the AllMotion mouse protocol via its view.
type allMotionPage struct{}

func (allMotionPage) Init() tea.Cmd                         { return nil }
func (p allMotionPage) Update(tea.Msg) (tea.Model, tea.Cmd) { return p, nil }
func (allMotionPage) View() tea.View {
	v := tea.NewView("all motion page")
	v.MouseMode = tea.MouseModeAllMotion
	v.OnMouse = func(tea.MouseMsg) tea.Cmd { return nil }
	return v
}

func TestViewMouseRoutingAcrossLayouts(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Routing"})

	// Zero-size view: the pre-layout startup frame.
	fresh := NewWithOptions(Options{AppName: "Fresh", ConfigDir: t.TempDir()})
	t.Cleanup(fresh.Close)
	if v := fresh.View(); v.Content != "" {
		t.Error("zero-size view should have empty content")
	}

	// Tabs (top-docked) layout: tab row, content row, status row, below.
	v := m.View()
	navH := m.nav.Height()
	statusH := 1
	mainH := 32 - statusH
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 0}))        // tabs
	m.sidebarFocused = true                                        // content click releases focus
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 5, Y: navH + 2})) // content
	if m.sidebarFocused {
		t.Error("content click should release sidebar focus (top dock)")
	}
	_ = v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: 5, Y: navH + 2})) // release routes too
	_ = v.OnMouse(tea.MouseMotionMsg(tea.Mouse{X: 5, Y: navH + 2}))  // motion
	_ = v.OnMouse(tea.MouseWheelMsg(tea.Mouse{X: 5, Y: navH + 2, Button: tea.MouseWheelDown}))
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 5, Y: mainH}))        // status row
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 5, Y: 31 + statusH})) // beyond everything

	// Sidebar (left-docked) layout.
	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleSidebar})
	v = m.View()
	navW := m.nav.Width()
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 2})) // sidebar
	m.sidebarFocused = true
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: navW + 3, Y: 2})) // content: releases focus
	if m.sidebarFocused {
		t.Error("content click should release sidebar focus (left dock)")
	}
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: mainH + 5})) // below main area: nil

	// Hidden nav: content occupies the whole main area.
	m.navigationVisible = false
	v = m.View()
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 5, Y: 5}))  // content
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 5, Y: 40})) // outside: nil
	m.navigationVisible = true

	// A page can opt into the AllMotion mouse mode.
	_, _ = m.Update(ReplaceAppPagesMsg{Pages: []RegisteredPage{{Title: "Motion", Model: allMotionPage{}}}})
	if got := m.View().MouseMode; got != tea.MouseModeAllMotion {
		t.Errorf("MouseMode = %v; want AllMotion", got)
	}
}

func TestLinkRateHelpersAndProvider(t *testing.T) {
	// Formatting helpers, all branches.
	if got := byteCount(3 << 20); got != "3.00 MiB" {
		t.Errorf("byteCount MiB = %q", got)
	}
	if got := byteCount(2 << 10); got != "2.0 KiB" {
		t.Errorf("byteCount KiB = %q", got)
	}
	if got := byteCount(12); got != "12 B" {
		t.Errorf("byteCount B = %q", got)
	}
	if got := rate(12); got != "12 B/s" {
		t.Errorf("rate = %q", got)
	}
	if got := bitRate(1_000_000); got != "8.0 Mbit/s" {
		t.Errorf("bitRate Mbit = %q", got)
	}
	if got := bitRate(1_000); got != "8.0 kbit/s" {
		t.Errorf("bitRate kbit = %q", got)
	}
	if got := bitRate(2); got != "16 bit/s" {
		t.Errorf("bitRate bit = %q", got)
	}
	if got := digits(-1234); got != 4 {
		t.Errorf("digits(-1234) = %d", got)
	}

	// Wire-cost estimation for every message family.
	if got := estimateTxBytes(tea.KeyPressMsg{Text: "é"}); got != 2 {
		t.Errorf("utf-8 key = %d bytes", got)
	}
	if got := estimateTxBytes(tea.KeyPressMsg{Code: tea.KeyUp}); got != 3 {
		t.Errorf("special key = %d bytes", got)
	}
	if got := estimateTxBytes(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}); got != 6 {
		t.Errorf("modified key = %d bytes", got)
	}
	if got := estimateTxBytes(tea.KeyReleaseMsg{Code: 'a'}); got != 8 {
		t.Errorf("key release = %d bytes", got)
	}
	if got := estimateTxBytes(tea.MouseClickMsg(tea.Mouse{X: 10, Y: 5})); got <= 6 {
		t.Errorf("mouse click = %d bytes", got)
	}
	if got := estimateTxBytes(tea.PasteMsg{Content: "hello"}); got != 17 {
		t.Errorf("paste = %d bytes", got)
	}
	if got := estimateTxBytes(struct{}{}); got != 0 {
		t.Errorf("non-input message = %d bytes", got)
	}

	// avgRates over an idle-gapped window.
	buckets := []rateSample{{sec: 100, tx: 10, rx: 20}, {sec: 104, tx: 30, rx: 40}}
	tx, rx := avgRates(buckets, 5)
	if tx != 8 || rx != 12 {
		t.Errorf("avgRates = (%d, %d); want (8, 12)", tx, rx)
	}
	if tx, rx := avgRates(nil, 5); tx != 0 || rx != 0 {
		t.Errorf("avgRates(nil) = (%d, %d)", tx, rx)
	}

	// A meter with finished buckets snapshots last/5s/60s rates.
	l := newLinkRateMeter()
	l.setDemand(demandStatusBar, true)
	l.mu.Lock()
	l.buckets = []rateSample{{sec: time.Now().Unix() - 2, tx: 100, rx: 200}}
	l.mu.Unlock()
	s := l.snapshot()
	if s.txLast != 100 || s.rxLast != 200 {
		t.Errorf("snapshot last = (%d, %d)", s.txLast, s.rxLast)
	}
	if got := l.statusLine(); got == "" {
		t.Error("active meter status line should not be empty")
	}

	// The inspector provider lifecycle plus its rendered rows.
	p := &linkRateProvider{meter: l}
	if p.RefreshInterval() != time.Second {
		t.Errorf("RefreshInterval = %v", p.RefreshInterval())
	}
	p.Start()
	rows := p.BuildRows(theme.Active())
	if len(rows) == 0 {
		t.Fatal("BuildRows returned nothing")
	}
	p.Stop()
}

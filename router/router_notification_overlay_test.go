package router

// Regression tests: toast and history-panel overlays must NOT erase the nav
// sidebar or status bar content visible behind them.
//
// Root cause (now fixed): lipgloss.NewCanvas creates a blank width×height cell
// grid. Any terminal cell not explicitly painted by a layer is filled with the
// default background, so short lines in the base content leave gaps that
// swallow the nav sidebar. The fix is to use lipgloss.NewCompositor, which
// composites directly on the source string without blanking anything (the same
// approach used by the info-modal overlay).

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/jarvisfriends/tui-base/notifications"
	statuspkg "github.com/jarvisfriends/tui-base/status"

	tea "charm.land/bubbletea/v2"
)

// ansiRE strips ANSI SGR escape sequences so we can search for plain text in
// rendered output. We also collapse excess whitespace for robust matching.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// setupRouter returns a router initialised to 100×30 cells with sidebar nav.
func setupRouter(t *testing.T) *RouterModel {
	t.Helper()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

// navShouldContain asserts that the nav sidebar text "Home" is present in
// the stripped view, meaning the nav was not erased by an overlay.
func navShouldContain(t *testing.T, label, content string) {
	t.Helper()
	stripped := stripANSI(content)
	if !strings.Contains(stripped, "Home") {
		t.Errorf("%s: nav sidebar was erased — expected to find \"Home\" in the rendered output", label)
	}
}

// statusShouldContain checks that the status bar is present by looking for
// a ctrl key hint that the status bar always renders.
func statusShouldContain(t *testing.T, label, content string) {
	t.Helper()
	stripped := stripANSI(content)
	// The status bar always renders a ctrl-prefixed key binding (e.g., ctrl+h
	// for help toggle). "ctrl" is a safe, theme-invariant sentinel.
	if !strings.Contains(strings.ToLower(stripped), "ctrl") {
		t.Errorf("%s: status bar content was erased — expected to find \"ctrl\" key hints in the rendered output", label)
	}
}

// TestToastOverlayPreservesNavAndStatus is the regression test for the
// canvas-blanks-background bug on toast notifications.
//
// Failure mode (before fix): View() composited the toast via
// lipgloss.NewCanvas(m.width, m.height), which allocated a blank grid and
// painted the base content on it. Lines shorter than m.width left empty cells
// that rendered as the terminal default background, erasing nav/status.
func TestToastOverlayPreservesNavAndStatus(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	// Baseline: nav and status must appear before any notification.
	baseContent := m.View().Content
	navShouldContain(t, "baseline", baseContent)
	statusShouldContain(t, "baseline", baseContent)

	// Inject a notification of each severity so the toast becomes active.
	for _, tc := range []struct {
		name string
		sev  notifications.Severity
		text string
	}{
		{"info toast", notifications.SeverityInfo, "Info test notification"},
		{"warning toast", notifications.SeverityWarning, "Warn test notification"},
		{"error toast", notifications.SeverityError, "Error test notification"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh router for each sub-test so they are independent.
			mr := setupRouter(t)
			_, cmd := mr.Update(notifications.AddMsg{
				Content:  tc.text,
				Severity: tc.sev,
				TTL:      0, // no auto-dismiss during the test
			})
			_ = cmd

			v := mr.View()
			stripped := stripANSI(v.Content)

			// Toast content must appear.
			if !strings.Contains(stripped, tc.text) {
				t.Errorf("toast content %q not found in rendered output", tc.text)
			}

			// Nav and status must still be visible around the toast.
			navShouldContain(t, tc.name, v.Content)
			statusShouldContain(t, tc.name, v.Content)
		})
	}
}

// TestHistoryPanelOverlayPreservesNavAndStatus is the regression test for the
// canvas-blanks-background bug on the notification history panel.
//
// The history panel was composited with the same lipgloss.NewCanvas approach;
// with many entries the panel grew to near-full height, pushing panelY to ~0
// and covering the sidebar. Even with the new height cap the canvas approach
// still blanked cells around the panel.
func TestHistoryPanelOverlayPreservesNavAndStatus(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	// Add a few notifications so the panel has rows to display.
	for i, sev := range []notifications.Severity{
		notifications.SeverityInfo,
		notifications.SeverityWarning,
		notifications.SeverityError,
	} {
		_, _ = m.Update(notifications.AddMsg{
			Content:  strings.Repeat("x", 10+i*5), // varying lengths
			Severity: sev,
			TTL:      0,
		})
	}

	// Open the history panel via the toggle (simulates bell click).
	_ = m.status.ToggleNotifications()

	v := m.View()
	stripped := stripANSI(v.Content)

	// At least one notification badge must appear (INFO / WARN / ERR).
	if !strings.Contains(stripped, "INFO") && !strings.Contains(stripped, "WARN") && !strings.Contains(stripped, "ERR") {
		t.Error("history panel: no notification badges found in rendered output")
	}

	// Nav and status must remain visible.
	navShouldContain(t, "history panel", v.Content)
	statusShouldContain(t, "history panel", v.Content)
}

// TestTTLExpiryDismissesNotification is a regression test for the bug where
// the router did not route internal expiry messages to notifMgr.Handle().
//
// Failure mode (before fix): notifications.expireMsg was unexported so the
// router's Update switch only matched AddMsg|DismissMsg|DismissAllMsg.
// When the TTL timer fired it delivered an expireMsg that fell through without
// calling notifMgr.Handle(), leaving the notification permanently active.
// updateRouter is a typed Update helper for tests — Update returns tea.Model
// which needs a type assertion to get back *RouterModel.
func updateRouter(m *RouterModel, msg tea.Msg) (*RouterModel, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(*RouterModel), cmd
}

func TestTTLExpiryDismissesNotification(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	// Add a notification with no real timer (TTL=0) so the test controls timing.
	m, _ = updateRouter(m, notifications.AddMsg{
		Content:  "should auto-expire",
		Severity: notifications.SeverityInfo,
		TTL:      0,
	})

	if got := m.notifMgr.Count(); got != 1 {
		t.Fatalf("expected 1 active notification after add, got %d", got)
	}

	// Grab the notification ID so we can synthesise the expiry message.
	all := m.notifMgr.All()
	id := all[0].ID

	// Simulate the TTL timer firing: send the (now-exported) ExpireMsg to the
	// router.  Before the fix the router ignored this type, so Count() stayed 1.
	m, _ = updateRouter(m, notifications.ExpireMsg{ID: id})

	if got := m.notifMgr.Count(); got != 0 {
		t.Errorf("TTL expiry: expected 0 active notifications after ExpireMsg, got %d — ExpireMsg is not being routed to notifMgr.Handle()", got)
	}
}

// TestHistoryPanelDismissAllKey is a regression test for the bug where
// pressing 'd' while the history panel is open did nothing.
//
// Failure mode (before fix): the history-panel key switch only handled
// Quit (esc), Up, Down, Select (enter) and Dismiss (also esc — shadowed by
// Quit).  There was no handler for the 'd' key that the panel footer
// advertises as "dismiss all".
func TestHistoryPanelDismissAllKey(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	// Add two notifications.
	m, _ = updateRouter(m, notifications.AddMsg{Content: "first", Severity: notifications.SeverityInfo, TTL: 0})
	m, _ = updateRouter(m, notifications.AddMsg{Content: "second", Severity: notifications.SeverityWarning, TTL: 0})

	if got := m.notifMgr.Count(); got != 2 {
		t.Fatalf("expected 2 active notifications, got %d", got)
	}

	// Open the history panel.
	_ = m.status.ToggleNotifications()
	if !m.status.IsHistoryVisible() {
		t.Fatal("history panel should be visible after ToggleNotifications()")
	}

	// Press 'd' — the panel footer advertises this as "dismiss all".
	m, _ = updateRouter(m, tea.KeyPressMsg{Text: "d"})

	if got := m.notifMgr.Count(); got != 0 {
		t.Errorf("dismiss-all key 'd': expected 0 active after pressing 'd', got %d — 'd' key is not wired in the history panel handler", got)
	}
}

// TestHistoryPanelEnterDismissRemovesSelected ensures Enter dismisses the
// selected active notification and removes it from the rendered history list.
func TestHistoryPanelEnterDismissRemovesSelected(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	for i := 0; i < 3; i++ {
		m, _ = updateRouter(m, notifications.AddMsg{
			Content:  fmt.Sprintf("dismiss-enter-%d", i),
			Severity: notifications.SeverityInfo,
			TTL:      0,
		})
	}
	_ = m.status.ToggleNotifications()

	before := m.notifMgr.Active()
	if len(before) != 3 {
		t.Fatalf("expected 3 active notifications before Enter dismiss, got %d", len(before))
	}
	selected := before[m.status.HistoryCursor()].Content

	m, _ = updateRouter(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	after := m.notifMgr.Active()
	if len(after) != 2 {
		t.Fatalf("expected active list to shrink to 2 after Enter dismiss, got %d", len(after))
	}
	if strings.Contains(stripANSI(m.View().Content), selected) {
		t.Fatalf("expected dismissed content %q to be removed from history panel", selected)
	}
}

// TestHistoryPanelKeyboardScrollKeepsSelectionVisible ensures arrow-key
// scrolling keeps the selected item inside the panel viewport.
func TestHistoryPanelKeyboardScrollKeepsSelectionVisible(t *testing.T) {
	t.Parallel()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})

	for i := 0; i < 24; i++ {
		m, _ = updateRouter(m, notifications.AddMsg{
			Content:  fmt.Sprintf("kbd-scroll-%02d", i),
			Severity: notifications.SeverityInfo,
			TTL:      0,
		})
	}
	_ = m.status.ToggleNotifications()

	for i := 0; i < 14; i++ {
		m, _ = updateRouter(m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	active := m.notifMgr.Active()
	cur := m.status.HistoryCursor()
	if cur < 0 || cur >= len(active) {
		t.Fatalf("invalid history cursor %d for active len %d", cur, len(active))
	}
	selected := active[cur].Content
	if !strings.Contains(stripANSI(m.View().Content), selected) {
		t.Fatalf("selected item %q is not visible after keyboard scrolling", selected)
	}
}

// TestHistoryPanelWheelScrollKeepsSelectionVisible ensures mouse wheel
// scrolling also keeps the selected item inside the rendered panel viewport.
func TestHistoryPanelWheelScrollKeepsSelectionVisible(t *testing.T) {
	t.Parallel()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})

	for i := 0; i < 24; i++ {
		m, _ = updateRouter(m, notifications.AddMsg{
			Content:  fmt.Sprintf("wheel-scroll-%02d", i),
			Severity: notifications.SeverityInfo,
			TTL:      0,
		})
	}
	_ = m.status.ToggleNotifications()

	v := m.View()
	bx, by, bw, bh := m.historyOverlayBounds[0], m.historyOverlayBounds[1], m.historyOverlayBounds[2], m.historyOverlayBounds[3]
	if bw == 0 || bh == 0 {
		t.Fatal("history overlay bounds were not set")
	}
	// Wheel inside the panel content area.
	x := bx + 1
	y := by + 1
	for i := 0; i < 14; i++ {
		if v.OnMouse == nil {
			t.Fatal("router view OnMouse handler is nil")
		}
		cmd := v.OnMouse(tea.MouseWheelMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseWheelDown}))
		if cmd != nil {
			_ = cmd()
		}
		v = m.View()
	}

	active := m.notifMgr.Active()
	cur := m.status.HistoryCursor()
	if cur < 0 || cur >= len(active) {
		t.Fatalf("invalid history cursor %d for active len %d", cur, len(active))
	}
	selected := active[cur].Content
	if !strings.Contains(stripANSI(m.View().Content), selected) {
		t.Fatalf("selected item %q is not visible after wheel scrolling", selected)
	}
}

// TestDebugPageStillShowsStatusBar ensures the debug page does not push the
// bottom status line out of the viewport.
func TestInspectorOverlayStillShowsStatusBar(t *testing.T) {
	t.Parallel()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.nav == nil {
		t.Fatal("router nav is nil")
	}
	m.inspectorOverlayVisible = true
	_ = m.handleResizeCmd()

	content := stripANSI(m.View().Content)
	if !strings.Contains(content, "MESSAGE INSPECTOR") {
		t.Fatal("inspector overlay missing expected inspector title")
	}
	if !strings.Contains(strings.ToLower(content), "ctrl") {
		t.Fatal("status bar key hints are missing with inspector overlay")
	}
}

// TestHiddenStatusBarDoesNotHandleClicks ensures hidden status content is not
// still hit-tested by router mouse routing.
func TestHiddenStatusBarDoesNotHandleClicks(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)
	if cmd := m.status.ToggleVisible(); cmd != nil {
		_ = cmd
	}
	for i := 0; i < 10; i++ {
		m, _ = updateRouter(m, statuspkg.TickMsg{})
	}
	_ = m.handleResizeCmd()

	v := m.View()
	if v.OnMouse == nil {
		t.Fatal("router OnMouse handler is nil")
	}
	cmd := v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: 2, Y: m.height - 1, Button: tea.MouseLeft}))
	if cmd == nil {
		return
	}
	msg := cmd()
	if _, ok := msg.(statuspkg.ClickRegionMsg); ok {
		t.Fatal("expected hidden status bar click to avoid emitting status ClickRegionMsg")
	}
}

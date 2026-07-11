package router

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/jarvisfriends/snap/keys"
	"github.com/jarvisfriends/snap/navigation"
	"github.com/jarvisfriends/snap/notifications"
	"github.com/jarvisfriends/snap/status"
	log "github.com/jarvisfriends/tui-base/logging"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	testKeyCtrlB        = "ctrl+b"
	testKeyOpenSettings = "ctrl+g"
	testKeyInspector    = "ctrl+d"
	testPageTitle       = "aSettings"
)

func TestTabCyclesPages(t *testing.T) {
	t.Parallel()
	m := New()
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != navigation.PageIDHome {
		t.Fatalf(
			"initial active page = %q; want \"home\"",
			m.nav.GetPages()[m.nav.GetActiveIndex()].ID,
		)
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != navigation.PageIDSettings {
		t.Fatalf(
			"after tab active page = %q; want \"settings\"",
			m.nav.GetPages()[m.nav.GetActiveIndex()].ID,
		)
	}

	// nav highlight should be in sync
	idx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == navigation.PageIDSettings {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("settings page not found in nav")
	}
	if m.nav.GetActiveIndex() != idx {
		t.Fatalf("nav.ActiveIndex = %d; want %d", m.nav.GetActiveIndex(), idx)
	}
}

func TestSelectedMsgSwitchesPage(t *testing.T) {
	t.Parallel()
	m := New()

	// find settings index
	idx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == navigation.PageIDSettings {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("settings page not found in nav")
	}

	_, _ = m.Update(navigation.SelectedMsg{PageIndex: idx})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != navigation.PageIDSettings {
		t.Fatalf("active page = %q; want \"settings\"", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}
	if m.nav.GetActiveIndex() != idx {
		t.Fatalf("nav.ActiveIndex = %d; want %d", m.nav.GetActiveIndex(), idx)
	}
}

// clickStatusRegion simulates a mouse-release on a named status bar region and
// returns the cmd produced by the router's top-level OnMouse handler.
// It uses r.status.Regions() — precomputed with lipgloss.Width — so no
// byte-unsafe ANSI-string parsing is needed.
func clickStatusRegion(t *testing.T, r *RouterModel, regionName string) tea.Cmd {
	t.Helper()

	// Force a resize so regions and the helpView.OnMouse closure are fresh.
	_ = r.handleResizeCmd()

	regions := r.status.Regions()
	var target *status.ClickRegion
	for i := range regions {
		if regions[i].Name == regionName {
			target = &regions[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("region %q not found in status.Regions(): %+v", regionName, regions)
		return nil
	}

	// Click the center of the region for robustness.
	clickX := (target.Start + target.End) / 2
	statusHeight := lipgloss.Height(r.status.View().Content)
	mainHeight := max(r.height-statusHeight, 0)
	// The status bar is the last row of the status view; row index = statusHeight-1.
	globalY := mainHeight + statusHeight - 1

	v := r.View()
	cmd := v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: clickX, Y: globalY}))
	if cmd == nil {
		t.Fatalf("expected non-nil cmd from OnMouse for region %q (clickX=%d globalY=%d)",
			regionName, clickX, globalY)
	}
	return cmd
}

// drainBatch executes cmd(), then for each sub-cmd in a BatchMsg executes
// sub() and feeds non-nil results back into the router. One level of nesting.
func drainBatch(r *RouterModel, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	processSingle := func(m tea.Msg) {
		if m == nil {
			return
		}
		_, cmd2 := r.Update(m)
		if cmd2 != nil {
			if follow := cmd2(); follow != nil {
				_, _ = r.Update(follow)
			}
		}
	}
	switch bm := msg.(type) {
	case tea.BatchMsg:
		for _, sub := range bm {
			if sub != nil {
				processSingle(sub())
			}
		}
	default:
		processSingle(msg)
	}
}

func TestRouter_StatusClick_SettingsNavigates(t *testing.T) {
	r := New()
	r.width = 80
	r.height = 24
	r.status.SetWidth(r.width)

	cmd := clickStatusRegion(t, r, status.SettingsRegionName)
	drainBatch(r, cmd)

	// Verify the nav switched to the settings page.
	settingsIdx := -1
	for i, p := range r.nav.GetPages() {
		if p.ID == navigation.PageIDSettings {
			settingsIdx = i
			break
		}
	}
	if settingsIdx == -1 {
		t.Fatal("settings page not present in nav pages")
	}
	if r.nav.GetActiveIndex() != settingsIdx {
		t.Fatalf("expected active index %d, got %d", settingsIdx, r.nav.GetActiveIndex())
	}
}

func TestSelectedMsgChangesActivePageAndLogs(t *testing.T) {
	m := New()
	// verify initial state
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != navigation.PageIDHome {
		t.Fatalf(
			"expected initial active page 'home'; got %s",
			m.nav.GetPages()[m.nav.GetActiveIndex()].ID,
		)
	}

	// send a WindowSizeMsg to initialize children
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if m.inspector == nil {
		t.Fatal("expected non-nil inspector model")
	}
	dm := m.inspector
	initialLogs := len(dm.Logs)

	// find settings index in nav
	idx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == navigation.PageIDSettings {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("could not find settings page in nav")
	}

	// send selection
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: idx})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != navigation.PageIDSettings {
		t.Fatalf(
			"expected active page 'settings' after selection; got %s",
			m.nav.GetPages()[m.nav.GetActiveIndex()].ID,
		)
	}
	if m.nav.GetActiveIndex() != idx {
		t.Fatalf("expected nav.ActiveIndex=%d; got %d", idx, m.nav.GetActiveIndex())
	}

	if len(dm.Logs) <= initialLogs {
		t.Fatalf(
			"expected debug logs to increase after SelectedMsg; before=%d after=%d",
			initialLogs,
			len(dm.Logs),
		)
	}
}

func TestHandleResizeCmdProducesExpectedSizes(t *testing.T) {
	t.Parallel()
	m := New()
	m.width = 100
	m.height = 40
	m.navigationVisible = true
	m.keys = keys.DefaultKeyMap()
	// Ensure status help view has the correct width so height is computed
	m.status.SetKeys(m.keys)
	m.status.SetWidth(m.width)

	_ = m.handleResizeCmd()

	navWidth := 0
	navHeight := 0
	if m.navigationVisible && m.nav != nil {
		switch m.nav.(type) {
		case *navigation.Tabs:
			navHeight = m.nav.Height()
		default:
			navWidth = m.nav.Width()
			navHeight = m.nav.Height()
		}
	}
	helpHeight := lipgloss.Height(m.status.View().Content)
	expectedContentWidth := m.width - navWidth
	expectedContentHeight := m.height - helpHeight - navHeight

	activePageContent := m.GetActivePage().View().Content

	if lipgloss.Width(activePageContent) != expectedContentWidth {
		t.Fatalf(
			"ContentWidth = %d; want %d",
			lipgloss.Width(activePageContent),
			expectedContentWidth,
		)
	}
	if lipgloss.Height(activePageContent) != expectedContentHeight {
		t.Fatalf(
			"ContentHeight = %d; want %d",
			lipgloss.Height(activePageContent),
			expectedContentHeight,
		)
	}
}

func TestViewContainsHomeAndMouseMode(t *testing.T) {
	t.Parallel()
	m := New()
	m.width = 80
	m.height = 24
	v := m.View()
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected MouseModeCellMotion; got %v", v.MouseMode)
	}
	if v.Content == "" {
		t.Fatal("expected non-empty view content")
	}
}

func TestWindowSizeMsgUpdatesRouterState(t *testing.T) {
	t.Parallel()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Fatalf("router width/height not updated; got %d x %d", m.width, m.height)
	}
	// children should have been updated; ensure views are non-empty
	if m.nav == nil || m.nav.View().Content == "" {
		t.Fatal("expected nav view content after WindowSizeMsg")
	}
	// find home page in pages slice
	homeIdx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == navigation.PageIDHome {
			homeIdx = i
			break
		}
	}
	if homeIdx == -1 {
		t.Fatal("home page not found in nav")
	}
	if homeIdx >= len(m.pages) || m.pages[homeIdx].View().Content == "" {
		t.Fatal("expected home view content after WindowSizeMsg")
	}
}

func TestQuitKeyEmitsCmd(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()

	_, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for quit key")
	}
	// Ensure invoking the cmd returns a message (Quit or similar)
	if msg := cmd(); msg == nil {
		t.Fatalf("expected cmd() to return a non-nil message")
	}
}

func TestTabWithNilNavCyclesPages(t *testing.T) {
	t.Parallel()
	m := New()
	// ensure tab advances the active nav page
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != navigation.PageIDHome {
		t.Fatalf(
			"expected initial active page 'home'; got %q",
			m.nav.GetPages()[m.nav.GetActiveIndex()].ID,
		)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != navigation.PageIDSettings {
		t.Fatalf(
			"expected active page 'settings' after tab; got %q",
			m.nav.GetPages()[m.nav.GetActiveIndex()].ID,
		)
	}
}

// ansiRE strips ANSI SGR escape sequences so we can search for plain text in
// rendered output. We also collapse excess whitespace for robust matching.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// setupRouter returns a router initialized to 100×30 cells with sidebar nav.
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
		t.Errorf(
			"%s: nav sidebar was erased — expected to find \"Home\" in the rendered output",
			label,
		)
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
		t.Errorf(
			"%s: status bar content was erased — expected to find \"ctrl\" key hints in the rendered output",
			label,
		)
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
			t.Parallel()
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
	if !strings.Contains(stripped, "INFO") && !strings.Contains(stripped, "WARN") &&
		!strings.Contains(stripped, "ERR") {
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
func updateRouter(m *RouterModel, msg tea.Msg) *RouterModel {
	next, _ := m.Update(msg)
	r, ok := next.(*RouterModel)
	if !ok {
		panic(fmt.Sprintf("Update returned %T, want *RouterModel", next))
	}
	return r
}

func TestTTLExpiryDismissesNotification(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	// Add a notification with no real timer (TTL=0) so the test controls timing.
	m = updateRouter(m, notifications.AddMsg{
		Content:  "should auto-expire",
		Severity: notifications.SeverityInfo,
		TTL:      0,
	})

	if got := m.notifMgr.Count(); got != 1 {
		t.Fatalf("expected 1 active notification after add, got %d", got)
	}

	// Grab the notification ID so we can synthesize the expiry message.
	all := m.notifMgr.All()
	id := all[0].ID

	// Simulate the TTL timer firing: send the (now-exported) ExpireMsg to the
	// router.  Before the fix the router ignored this type, so Count() stayed 1.
	m = updateRouter(m, notifications.ExpireMsg{ID: id})

	if got := m.notifMgr.Count(); got != 0 {
		t.Errorf(
			"TTL expiry: expected 0 active notifications after ExpireMsg, got %d — ExpireMsg is not being routed to notifMgr.Handle()",
			got,
		)
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
	m = updateRouter(
		m,
		notifications.AddMsg{Content: "first", Severity: notifications.SeverityInfo, TTL: 0},
	)
	m = updateRouter(
		m,
		notifications.AddMsg{Content: "second", Severity: notifications.SeverityWarning, TTL: 0},
	)

	if got := m.notifMgr.Count(); got != 2 {
		t.Fatalf("expected 2 active notifications, got %d", got)
	}

	// Open the history panel.
	_ = m.status.ToggleNotifications()
	if !m.status.IsHistoryVisible() {
		t.Fatal("history panel should be visible after ToggleNotifications()")
	}

	// Press 'd' — the panel footer advertises this as "dismiss all".
	m = updateRouter(m, tea.KeyPressMsg{Text: "d"})

	if got := m.notifMgr.Count(); got != 0 {
		t.Errorf(
			"dismiss-all key 'd': expected 0 active after pressing 'd', got %d — 'd' key is not wired in the history panel handler",
			got,
		)
	}
}

// TestHistoryPanelEnterDismissRemovesSelected ensures Enter dismisses the
// selected active notification and removes it from the rendered history list.
func TestHistoryPanelEnterDismissRemovesSelected(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	for i := range 3 {
		m = updateRouter(m, notifications.AddMsg{
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

	m = updateRouter(m, tea.KeyPressMsg{Code: tea.KeyEnter})

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

	for i := range 24 {
		m = updateRouter(m, notifications.AddMsg{
			Content:  fmt.Sprintf("kbd-scroll-%02d", i),
			Severity: notifications.SeverityInfo,
			TTL:      0,
		})
	}
	_ = m.status.ToggleNotifications()

	for range 14 {
		m = updateRouter(m, tea.KeyPressMsg{Code: tea.KeyDown})
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

	for i := range 24 {
		m = updateRouter(m, notifications.AddMsg{
			Content:  fmt.Sprintf("wheel-scroll-%02d", i),
			Severity: notifications.SeverityInfo,
			TTL:      0,
		})
	}
	_ = m.status.ToggleNotifications()

	v := m.View()
	hb := m.overlayByName("history").Bounds()
	bx, by, bw, bh := hb.X, hb.Y, hb.W, hb.H
	if bw == 0 || bh == 0 {
		t.Fatal("history overlay bounds were not set")
	}
	// Wheel inside the panel content area.
	x := bx + 1
	y := by + 1
	for range 14 {
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
	m.inspector.ToggleVisible() // make inspector visible
	_ = m.handleResizeCmd()

	content := stripANSI(m.View().Content)
	if !strings.Contains(content, "(Inspector)") {
		t.Fatal("inspector overlay missing expected inspector title with (Inspector) marker")
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
	for range 10 {
		m = updateRouter(m, status.TickMsg{})
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
	if _, ok := msg.(status.ClickRegionMsg); ok {
		t.Fatal("expected hidden status bar click to avoid emitting status ClickRegionMsg")
	}
}

// TestInspectorNavKeyNotDoubleDispatched is a regression test for a bug where
// any navigation key pressed while the inspector overlay was open caused the
// inspector to advance twice instead of once.
//
// Root cause: router.Update() forwarded every message — including tea.KeyMsg —
// to m.inspector.Update() unconditionally at the top of the function. The
// tea.KeyPressMsg branch then called m.inspector.Update() a second time for
// the explicit inspector-key routing, producing a double-dispatch. The fix
// guards the top-level forward: key messages are skipped when the inspector is
// visible so only the explicit routing in the KeyPressMsg branch fires.
func TestInspectorNavKeyNotDoubleDispatched(t *testing.T) {
	t.Parallel()
	m := setupRouter(t)

	// Open the inspector overlay.
	m.inspector.ToggleVisible()
	_ = m.handleResizeCmd()

	// Sanity: inspector must be on the first tab (Runtime) before we press anything.
	before := stripANSI(m.inspector.View().Content)
	if !strings.Contains(before, "Runtime Profiling (Inspector)") {
		t.Fatalf(
			"expected inspector to start on 'Runtime Profiling (Inspector)' tab, got:\n%s",
			before,
		)
	}

	// Press → once. With correct routing the active tab advances by exactly one
	// (Runtime → Input). Before the fix it advanced twice (Runtime → Disks)
	// because the key was dispatched to m.inspector.Update() at both the
	// top-of-Update unconditional forward and again inside the KeyPressMsg branch.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})

	after := stripANSI(m.inspector.View().Content)
	switch {
	case strings.Contains(after, "Input Debugging (Inspector)"):
		// correct: exactly one tab advance
	case strings.Contains(after, "Disks (Inspector)"):
		t.Fatal("regression: inspector advanced two tabs on a single → key press (double-dispatch)")
	default:
		t.Fatalf("unexpected inspector title after → key press:\n%s", after)
	}
}

func TestToggleHelpKey(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()
	m.width = 100
	m.height = 40
	m.status.SetKeys(m.keys)
	m.status.SetWidth(m.width)

	before := m.status.IsVisible()
	_, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+j"})
	if m.status.IsVisible() == before {
		t.Fatalf("expected status visibility to toggle")
	}
}

// TestToggleSidebarKey verifies the three-state Ctrl+B cycle:
//
//	visible+unfocused → visible+focused → hidden → visible+unfocused
func TestToggleSidebarKey(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()

	// State: visible, unfocused.
	m.navigationVisible = true
	m.sidebarFocused = false

	// First press: focus the sidebar (not hide it).
	_, _ = m.Update(tea.KeyPressMsg{Text: testKeyCtrlB})
	if !m.navigationVisible {
		t.Fatal("first ctrl+b should focus sidebar, not hide it")
	}
	if !m.sidebarFocused {
		t.Fatal("first ctrl+b should focus sidebar")
	}

	// Second press: hide the sidebar.
	_, _ = m.Update(tea.KeyPressMsg{Text: testKeyCtrlB})
	if m.navigationVisible {
		t.Fatal("second ctrl+b should hide sidebar")
	}
	if m.sidebarFocused {
		t.Fatal("second ctrl+b should clear sidebar focus")
	}

	// Third press: show the sidebar again (unfocused).
	_, _ = m.Update(tea.KeyPressMsg{Text: testKeyCtrlB})
	if !m.navigationVisible {
		t.Fatal("third ctrl+b should show sidebar")
	}
	if m.sidebarFocused {
		t.Fatal("third ctrl+b should leave sidebar unfocused")
	}
}

func TestOpenSettingsKey(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID == navigation.PageIDSettings {
		t.Fatalf("expected test to start on a non-settings page")
	}

	_, _ = m.Update(tea.KeyPressMsg{Text: testKeyOpenSettings})

	if got := m.nav.GetPages()[m.nav.GetActiveIndex()].ID; got != navigation.PageIDSettings {
		t.Fatalf("active page = %q; want settings", got)
	}
}

func TestInvalidSelectedMsgReturnsResizeCmd(t *testing.T) {
	t.Parallel()
	m := New()
	prePageIndex := m.nav.GetActiveIndex()
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: -10})

	newPageIndex := m.nav.GetActiveIndex()
	if newPageIndex != prePageIndex {
		t.Fatalf(
			"expected active page index to remain unchanged; got %d, want %d",
			newPageIndex,
			prePageIndex,
		)
	}
}

// renderedStatus returns the ANSI-stripped status bar content for assertions.
func renderedStatus(m *RouterModel) string {
	return stripANSI(m.status.View().Content)
}

func TestInspectorSummaryShownInStatusBarWhenClosed(t *testing.T) {
	t.Parallel()
	m := New()
	// Enable the opt-in summary and give the router (and inspector) a size so the
	// status bar renders and the summary's "term WxH" segment is populated.
	m.inspector.SetStatusSummaryEnabled(true)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Inspector starts closed: the summary should be in the status bar.
	if m.inspector.IsVisible() {
		t.Fatal("precondition: inspector should start closed")
	}
	want := m.inspector.StatusLineSummary()
	if want == "" {
		t.Fatal("precondition: StatusLineSummary should be non-empty when enabled")
	}
	if got := renderedStatus(m); !strings.Contains(got, "term") {
		t.Fatalf(
			"expected status bar to show inspector summary %q while inspector closed, got:\n%s",
			want,
			got,
		)
	}

	// Open the inspector: the brief summary should yield to the full overlay and
	// disappear from the status bar.
	m.inspector.ToggleVisible()
	_ = m.handleResizeCmd()
	if got := renderedStatus(m); strings.Contains(got, "term ") {
		t.Fatalf("expected status bar summary to be hidden while inspector open, got:\n%s", got)
	}

	// Close the inspector again: the summary must return. This is the exact
	// regression — brief stats not reappearing after closing the inspector.
	m.inspector.ToggleVisible()
	_ = m.handleResizeCmd()
	if got := renderedStatus(m); !strings.Contains(got, "term") {
		t.Fatalf("expected status bar summary to reappear after closing inspector, got:\n%s", got)
	}
}

func TestInspectorSummaryHiddenWhenDisabled(t *testing.T) {
	t.Parallel()
	m := New()
	// Summary is opt-in (disabled by default): nothing extra in the status bar.
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.inspector.StatusSummaryEnabled() {
		t.Fatal("precondition: summary should be disabled by default")
	}
	if got := renderedStatus(m); strings.Contains(got, "term ") {
		t.Fatalf("expected no summary in status bar when disabled, got:\n%s", got)
	}
}

type stubPage struct{}

func (stubPage) Init() tea.Cmd                           { return nil }
func (stubPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return stubPage{}, nil }
func (stubPage) View() tea.View                          { return tea.NewView("stub") }

func TestNewWithRegisteredPages_AppendsExtraPages(t *testing.T) {
	t.Parallel()

	r := NewWithRegisteredPages([]RegisteredPage{
		{
			Title: "App",
			Model: stubPage{},
		},
	})

	// When extra pages are supplied: app pages come first, Home is omitted,
	// Inspector and Settings are appended at the end.
	pages := r.nav.GetPages()
	if len(pages) != 2 {
		t.Fatalf("nav pages len = %d; want 2 (app, settings)", len(pages))
	}
	if pages[0].ID != "app" || pages[0].Title != "App" {
		t.Fatalf("pages[0] = %+v; want app/App", pages[0])
	}
	if pages[1].ID != navigation.PageIDSettings {
		t.Fatalf("pages[1] = %+v; want settings/Settings", pages[1])
	}

	if len(r.pages) != 2 {
		t.Fatalf("router model pages len = %d; want 2", len(r.pages))
	}
}

func TestNewWithRegisteredPages_SkipsInvalidEntries(t *testing.T) {
	t.Parallel()

	r := NewWithRegisteredPages([]RegisteredPage{
		// nil model — invalid
		{Title: "NoModel"},
		// empty title — invalid
		{Model: stubPage{}},
	})

	// Both entries are invalid; fallback to default standalone mode (3 pages).
	if got := len(r.nav.GetPages()); got != 2 {
		t.Fatalf("nav pages len = %d; want 2", got)
	}
	if got := len(r.pages); got != 2 {
		t.Fatalf("router pages len = %d; want 2", got)
	}
}

func TestNewWithOptions_DefaultPageSelection(t *testing.T) {
	t.Parallel()

	r := NewWithOptions(Options{
		ExtraPages: []RegisteredPage{{
			Title: testPageTitle,
			Model: stubPage{},
		}},
		DefaultPage: testPageTitle,
	})

	idx := r.nav.GetActiveIndex()
	pages := r.nav.GetPages()
	if idx < 0 || idx >= len(pages) {
		t.Fatalf("active index out of range: %d", idx)
	}
	if pages[idx].Title != testPageTitle {
		t.Fatalf("active page title = %q; want %q", pages[idx].Title, testPageTitle)
	}
}

func TestNewWithOptions_InitialLogLevel(t *testing.T) {
	prev := log.GetLevel()
	defer func() {
		_ = log.SetLevel(prev)
	}()

	r := NewWithOptions(Options{InitialLogLevel: "ERROR"})
	if r == nil {
		t.Fatal("router should not be nil")
	}
	if got := log.GetLevel(); got != "ERROR" {
		t.Fatalf("log level = %q; want %q", got, "ERROR")
	}
}

func TestRouter_StatusClick_NotificationsTogglesPanel(t *testing.T) {
	r := New()
	r.width = 80
	r.height = 24
	r.status.SetWidth(r.width)

	cmd := clickStatusRegion(t, r, status.NotificationsRegionName)
	drainBatch(r, cmd)

	// After the click the history panel should be visible.
	if !r.status.IsHistoryVisible() {
		t.Fatal("expected notification history panel to be visible after bell click")
	}

	// The history overlay (composited by the router canvas) should include the header.
	overlay := r.status.RenderHistoryOverlay(r.width, r.height)
	if !strings.Contains(overlay, "Notifications") {
		t.Fatalf("expected 'Notifications' in overlay content, got:\n%s", overlay)
	}
}

func BenchmarkRouterViewWithSidebar(b *testing.B) {
	m := New()
	m.navigationVisible = true
	m.keys = keys.DefaultKeyMap()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	b.ReportAllocs()

	for b.Loop() {
		_ = m.View().Content
	}
}

func BenchmarkRouterViewNoSidebar(b *testing.B) {
	m := New()
	m.navigationVisible = false
	m.keys = keys.DefaultKeyMap()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	b.ReportAllocs()

	for b.Loop() {
		_ = m.View().Content
	}
}

// TestSidebarRegionFocusModel verifies the recommended sidebar focus model:
// Up/Down keep focus on the sidebar; Right/Enter/Tab hand focus to the page;
// Left/Esc/(Shift+)Tab return focus to the sidebar. (Tests default to tabs nav
// when no config is present, so this forces the focusable sidebar nav.)
func TestSidebarRegionFocusModel(t *testing.T) {
	m := New()
	m.nav = navigation.New() // focusable sidebar
	if _, ok := m.nav.(navigation.Focusable); !ok {
		t.Fatal("expected a focusable sidebar nav")
	}

	// Focus the sidebar to start.
	m.sidebarFocused = true
	m.setNavFocused(true)

	// Down navigates within the sidebar and must NOT move focus out of it.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if !m.sidebarFocused {
		t.Fatal("Down should keep focus on the sidebar")
	}

	// Right hands focus to the page content.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.sidebarFocused {
		t.Fatal("Right should move focus to the page content")
	}

	// Left returns focus to the sidebar.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.sidebarFocused {
		t.Fatal("Left should return focus to the sidebar")
	}

	// Enter (while focused) → content; Esc → back to the sidebar.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.sidebarFocused {
		t.Fatal("Enter should move focus to the page content")
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.sidebarFocused {
		t.Fatal("Esc should return focus to the sidebar")
	}

	// Tab toggles the focused region: sidebar → content → sidebar.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.sidebarFocused {
		t.Fatal("Tab from the sidebar should move focus to the page content")
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.sidebarFocused {
		t.Fatal("Tab from the page should return focus to the sidebar")
	}
}

package status

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/theme"
)

// newHistoryOverlay returns an open history overlay backed by a manager with
// the given notification contents.
func newHistoryOverlay(t *testing.T, contents ...string) *UserNotificationOverlay {
	t.Helper()
	overlay := NewUserNotificationOverlay()
	nm := notifications.NewManager()
	overlay.SetNotifManager(nm)
	for _, s := range contents {
		nm.Add(s, notifications.SeverityInfo, 5*time.Second)
	}
	overlay.showHistory = true
	return overlay
}

// TestHistoryOverlayInfoModalChrome asserts the panel uses the info-modal
// layout: centered title row, a full-width rule under it and above the footer,
// and a centered footer hint line.
func TestHistoryOverlayInfoModalChrome(t *testing.T) {
	t.Parallel()

	overlay := newHistoryOverlay(t, "chrome-test")
	rendered := overlay.RenderHistoryOverlay(100, 20)
	stripped := stripANSI(rendered)

	if !strings.Contains(stripped, "Notifications (1 active)") {
		t.Errorf("missing title; got:\n%s", stripped)
	}
	if strings.Count(stripped, "───") < 2 {
		t.Errorf("expected two separator rules (below title, above footer); got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Esc close") {
		t.Errorf("missing footer hints; got:\n%s", stripped)
	}
}

// TestHistoryOverlayFitsContentWidth asserts the panel sizes itself to its
// widest element (here the footer hint line) instead of a fixed width, and
// never exceeds the available screen width.
func TestHistoryOverlayFitsContentWidth(t *testing.T) {
	t.Parallel()

	overlay := newHistoryOverlay(t, "short")
	c := theme.Active()
	frameW := c.Styles.OverlayBorder.GetHorizontalFrameSize()
	footerW := lipgloss.Width("↑/↓ navigate • Enter open/dismiss • d dismiss all • Esc close")

	rendered := overlay.RenderHistoryOverlay(120, 20)
	if got := lipgloss.Width(rendered); got != footerW+frameW {
		t.Errorf("panel width = %d; want fit-to-content %d (footer %d + frame %d)",
			got, footerW+frameW, footerW, frameW)
	}

	// Narrow terminal: the panel must respect the available space.
	narrow := overlay.RenderHistoryOverlay(40, 20)
	if got := lipgloss.Width(narrow); got > 40 {
		t.Errorf("panel width = %d exceeds available width 40", got)
	}
}

// TestHistoryOverlayCursorRowUsesSelectionBg asserts the cursor row renders as
// a continuous selection-background bar, matching the app's list convention.
func TestHistoryOverlayCursorRowUsesSelectionBg(t *testing.T) {
	t.Parallel()

	overlay := newHistoryOverlay(t, "first", "second")
	overlay.historyCursor = 1

	c := theme.Active()
	selParams := bgNumericParams(c.SelectionBg)
	if selParams == "" {
		t.Skip("no ANSI background code — running in no-color mode")
	}

	rendered := overlay.RenderHistoryOverlay(100, 20)
	found := false
	for _, line := range nonBlankLines(rendered) {
		if strings.Contains(line, selParams) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no row carries the selection background %q\n%s", selParams, stripANSI(rendered))
	}
}

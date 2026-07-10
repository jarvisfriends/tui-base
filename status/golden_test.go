package status

import (
	"testing"
	"time"

	"github.com/jarvisfriends/snap/rendercheck"
	"github.com/jarvisfriends/tui-base/notifications"
)

// Golden renders of the status surfaces with fixed inputs and the default
// theme (TS-1). Ages render as "0s ago" because the notifications are created
// within the test run.
func TestStatusGoldenRenders(t *testing.T) {
	row, _ := RenderStyled(80, "tab next page • ctrl+g settings", "heap 12MiB", -1, true, 2)
	rendercheck.Golden(t, "statusbar_80w", row)

	overlay := NewUserNotificationOverlay()
	nm := notifications.NewManager()
	overlay.SetNotifManager(nm)
	nm.Add("Deploy finished", notifications.SeverityInfo, time.Hour)
	nm.Add("Disk space low", notifications.SeverityWarning, time.Hour)
	overlay.showHistory = true
	rendered := overlay.RenderHistoryOverlay(100, 20)
	rendercheck.Golden(t, "history_panel_100w", rendered)
	// CF-3: the panel's border must hold its shape (no inner wrapping).
	rendercheck.CheckBorderIntegrityString(t, rendered, "│")
}

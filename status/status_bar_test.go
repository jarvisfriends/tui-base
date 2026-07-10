package status

import (
	"strings"
	"testing"

	"github.com/jarvisfriends/snap/keys"

	tea "charm.land/bubbletea/v2"
)

func TestBarModel_ClickRegions(t *testing.T) {
	b := New()
	b.SetKeys(keys.DefaultKeyMap())
	width := 80
	b.SetWidth(width)

	left := strings.TrimSpace(b.help.View(b.keys))
	statusLine, regions := b.sb.Render(width, left, "")

	// Compute the row index of the status bar line (always 0 now that the
	// overlay is composited externally by the router).
	statusLineRow := 0

	_ = statusLine // used for diagnostics above

	var settingsFound, notifFound bool
	var settingsStart, notifStart int
	for _, r := range regions {
		if r.Name == SettingsRegionName {
			settingsFound = true
			settingsStart = r.Start
		}
		if r.Name == NotificationsRegionName {
			notifFound = true
			notifStart = r.Start
		}
	}
	if !settingsFound {
		t.Fatalf("settings region not found; regions=%+v", regions)
	}
	if !notifFound {
		t.Fatalf("notifications region not found; regions=%+v", regions)
	}

	// Click settings
	cmd := b.helpView.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: settingsStart, Y: statusLineRow}))
	if cmd == nil {
		t.Fatal("expected non-nil cmd for settings click")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected message from settings click cmd")
	}
	crm, ok := msg.(ClickRegionMsg)
	if !ok {
		t.Fatalf("unexpected msg type %T", msg)
	}
	if crm.Name != SettingsRegionName {
		t.Fatalf("expected %q, got %q", SettingsRegionName, crm.Name)
	}

	// Click notifications
	cmd = b.helpView.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: notifStart, Y: statusLineRow}))
	if cmd == nil {
		t.Fatal("expected non-nil cmd for notifications click")
	}
	msg = cmd()
	if msg == nil {
		t.Fatal("expected message from notifications click cmd")
	}
	crm, ok = msg.(ClickRegionMsg)
	if !ok {
		t.Fatalf("unexpected msg type %T for notifications", msg)
	}
	if crm.Name != NotificationsRegionName {
		t.Fatalf("expected %q, got %q", NotificationsRegionName, crm.Name)
	}
}

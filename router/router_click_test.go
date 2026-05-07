package router

import (
	"strings"
	"testing"

	"github.com/jarvisfriends/tui-base/status"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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
	}

	// Click the centre of the region for robustness.
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
		if p.ID == "settings" {
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

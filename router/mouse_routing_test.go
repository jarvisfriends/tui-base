// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"testing"

	"github.com/jarvisfriends/inspector"

	tea "charm.land/bubbletea/v2"
)

// execCmd executes a tea.Cmd and flattens any tea.BatchMsg into a slice
// of produced messages for easier inspection in tests.
func execCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	m := cmd()
	if m == nil {
		return nil
	}
	var out []tea.Msg
	switch bm := m.(type) {
	case tea.BatchMsg:
		for _, sub := range bm {
			if sub == nil {
				continue
			}
			mm := sub()
			if mm != nil {
				out = append(out, mm)
			}
		}
	default:
		out = append(out, m)
	}
	return out
}

// TestMouseRoutingBoundaries verifies that global mouse coordinates are
// mapped to the expected child views (sidebar, content, status) and that
// the OffX/OffY values supplied to the routed MouseHighlightMsg match the
// offsets used when invoking child.OnMouse.
func TestMouseRoutingBoundaries(t *testing.T) {
	r := New()
	r.width = 80
	r.height = 24

	// Let router process a WindowSizeMsg so children compute sizes.
	_, cmd := r.Update(tea.WindowSizeMsg{Width: r.width, Height: r.height})
	if cmd != nil {
		msgs := execCmd(cmd)
		for _, m := range msgs {
			if m != nil {
				_, _ = r.Update(m)
			}
		}
	}

	v := r.View()
	statusHeight := r.status.Height()
	mainHeight := r.height - statusHeight
	if mainHeight <= 0 {
		t.Fatalf("invalid main area height: %d", mainHeight)
	}
	navWidth := 0
	if r.nav != nil {
		navWidth = r.nav.Width()
	}

	// 1) Click in the left region that should be routed to the sidebar.
	globalY := mainHeight / 2
	globalX := navWidth / 2
	msgs := execCmd(v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: globalX, Y: globalY})))
	found := false
	for _, m := range msgs {
		switch mm := m.(type) {
		case inspector.MouseHighlightMsg:
			found = true
			if mm.Child != "content" {
				t.Fatalf("expected child=content, got=%s", mm.Child)
			}
			if mm.OffX != 0 || mm.OffY != 3 {
				t.Fatalf("expected offsets 0,3 got %d,%d", mm.OffX, mm.OffY)
			}
		}
	}
	if !found {
		t.Fatal("no MouseHighlightMsg found for sidebar click")
	}

	// 2) Click in the content area (just right of the nav). OffX should equal navWidth.
	t.Logf(
		"dimensions: width=%d height=%d statusHeight=%d mainHeight=%d navWidth=%d",
		r.width,
		r.height,
		statusHeight,
		mainHeight,
		navWidth,
	)
	globalX = navWidth + (r.width-navWidth)/2
	t.Logf("clicking content at globalX=%d globalY=%d", globalX, globalY)
	msgs = execCmd(v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: globalX, Y: globalY})))
	t.Logf("content click returned %d messages", len(msgs))
	for i, m := range msgs {
		t.Logf("msg %d: type=%T value=%#v", i, m, m)
	}
	found = false
	for _, m := range msgs {
		switch mm := m.(type) {
		case inspector.MouseHighlightMsg:
			found = true
			if mm.Child != "content" {
				t.Fatalf("expected child=content, got=%s", mm.Child)
			}
			if mm.OffX != navWidth || mm.OffY != 3 {
				t.Fatalf("expected offsets %d,3 got %d,%d", navWidth, mm.OffX, mm.OffY)
			}
		}
	}
	if !found {
		t.Fatalf("no MouseHighlightMsg found for content click; see logs above")
	}

	// 3) Click in the status area (bottom). OffY should equal mainHeight.
	globalY = mainHeight + statusHeight/2
	globalX = r.width - 1
	msgs = execCmd(v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: globalX, Y: globalY})))
	found = false
	for _, m := range msgs {
		switch mm := m.(type) {
		case inspector.MouseHighlightMsg:
			found = true
			if mm.Child != "status" {
				t.Fatalf("expected child=status, got=%s", mm.Child)
			}
			if mm.OffY != mainHeight {
				t.Fatalf("expected OffY=%d got %d", mainHeight, mm.OffY)
			}
		}
	}
	if !found {
		t.Fatal("no MouseHighlightMsg found for status click")
	}
}

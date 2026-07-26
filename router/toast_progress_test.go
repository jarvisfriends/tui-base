// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/snap/notifications"
)

// TestProgressToastRendersBar: an AddMsg with Percent renders an HBar under
// the toast message, and a ProgressMsg routed through the router updates it
// in place.
func TestProgressToastRendersBar(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	pct := 50.0
	m = updateRouter(m, notifications.AddMsg{
		Key:      "copy",
		Content:  "copying files",
		Severity: notifications.SeverityInfo,
		Percent:  &pct,
	})

	toast := &toastOverlay{m: m}
	rendered := ansi.Strip(toast.Render(layoutContext{Width: 90, Height: 30}))
	if !strings.Contains(rendered, "copying files") {
		t.Fatalf("toast missing content:\n%s", rendered)
	}
	if !strings.Contains(rendered, "██████████░░░░░░░░░░  50%") {
		t.Errorf("toast missing half-filled bar:\n%s", rendered)
	}

	// ProgressMsg reaches the manager through the router's forwarding list.
	_ = updateRouter(m, notifications.ProgressMsg{Key: "copy", Percent: 100})
	rendered = ansi.Strip(toast.Render(layoutContext{Width: 90, Height: 30}))
	if !strings.Contains(rendered, "████████████████████ 100%") {
		t.Errorf("toast bar not updated by ProgressMsg:\n%s", rendered)
	}
}

// TestPlainToastHasNoBar: toasts without Percent render single-line, no bar.
func TestPlainToastHasNoBar(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m = updateRouter(m, notifications.AddMsg{
		Content:  "plain note",
		Severity: notifications.SeverityInfo,
	})

	toast := &toastOverlay{m: m}
	rendered := ansi.Strip(toast.Render(layoutContext{Width: 90, Height: 30}))
	if strings.Contains(rendered, "░") || strings.Contains(rendered, "%") {
		t.Errorf("plain toast should carry no bar:\n%s", rendered)
	}
}

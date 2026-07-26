// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/notifications"
)

// TestToggleHistoryKey pins I-12: ctrl+n opens the notification history
// panel from the keyboard and closes it again while it is consuming keys as
// a modal.
func TestToggleHistoryKey(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m = updateRouter(m, notifications.AddMsg{
		Content:  "note",
		Severity: notifications.SeverityInfo,
	})
	if m.status.IsHistoryVisible() {
		t.Fatal("history should start closed")
	}

	ctrlN := tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	_, _ = m.Update(ctrlN)
	if !m.status.IsHistoryVisible() {
		t.Fatal("ctrl+n should open the history panel")
	}

	_, _ = m.Update(ctrlN)
	if m.status.IsHistoryVisible() {
		t.Fatal("ctrl+n should close the open history panel")
	}
}

// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/notifications"
)

type actionFiredMsg struct{ id int64 }

// TestNotificationActionHandler verifies E-4: a consumer-registered action
// key runs its handler with the selected notification while the history
// panel is open, built-in keys still win, and unregistered keys stay inert.
func TestNotificationActionHandler(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m = updateRouter(m, notifications.AddMsg{
		Content:  "first",
		Severity: notifications.SeverityInfo,
		TTL:      0,
	})
	m = updateRouter(m, notifications.AddMsg{
		Content:  "second",
		Severity: notifications.SeverityInfo,
		TTL:      0,
	})

	var got notifications.Notification
	m.notifMgr.OnAction("o", func(n notifications.Notification) tea.Cmd {
		got = n
		return func() tea.Msg { return actionFiredMsg{id: n.ID} }
	})

	_ = m.status.ToggleNotifications()
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // select the second row

	_, cmd := m.Update(tea.KeyPressMsg{Text: "o", Code: 'o'})
	if cmd == nil {
		t.Fatal("action key produced no command")
	}
	selected := m.notifMgr.Active()[m.status.HistoryCursor()]
	if got.ID != selected.ID {
		t.Fatalf("handler saw notification %d; selected is %d", got.ID, selected.ID)
	}

	// The handler's cmd is dispatched (inside the returned, possibly nested,
	// batch).
	foundMsg := false
	var scan func(c tea.Cmd, depth int)
	scan = func(c tea.Cmd, depth int) {
		if c == nil || depth > 8 || foundMsg {
			return
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			for _, sub := range msg {
				scan(sub, depth+1)
			}
		case actionFiredMsg:
			foundMsg = true
		}
	}
	scan(cmd, 0)
	if !foundMsg {
		t.Fatal("handler's tea.Cmd was not included in the returned batch")
	}

	// Unregistered keys are consumed but fire nothing.
	got = notifications.Notification{}
	_, _ = m.Update(tea.KeyPressMsg{Text: "z", Code: 'z'})
	if got.ID != 0 {
		t.Fatal("unregistered key invoked a handler")
	}

	// Built-in keys still win: Esc closes the panel rather than reaching a
	// handler registered on the same key.
	escFired := false
	m.notifMgr.OnAction("esc", func(notifications.Notification) tea.Cmd {
		escFired = true
		return nil
	})
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if escFired {
		t.Fatal("built-in Esc was shadowed by a consumer action handler")
	}
}

// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"encoding/json"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/notifications"
	"github.com/jarvisfriends/tui-base/filewatch"
	"github.com/jarvisfriends/tui-base/pages/settings"
)

// drainCmds executes a command tree without a program loop, discarding the
// produced messages (side effects on the model have already happened).
func drainCmds(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			drainCmds(c)
		}
	}
}

// TestSettingsFileEventReloadsAndNotifies drives the FW-1 reload path
// directly: an external edit to tui_settings.json followed by a
// filewatch.Event must update the settings model and add a notification,
// while a no-op event (the app's own save) must stay silent.
func TestSettingsFileEventReloadsAndNotifies(t *testing.T) {
	settings.SetConfigDir(t.TempDir())
	m := NewWithOptions(Options{AppName: "FW1 Test"})
	t.Cleanup(m.Close)

	baseline := len(m.notifMgr.Active())

	// A file-watch event with no on-disk change (self-save echo) is silent.
	if _, cmd := m.Update(filewatch.Event{Path: settings.FilePath(), Op: "WRITE"}); cmd != nil {
		drainCmds(cmd)
	}
	if got := len(m.notifMgr.Active()); got != baseline {
		t.Fatalf("no-op reload raised a notification: %d -> %d", baseline, got)
	}

	// Externally flip a persisted field, then deliver the event.
	raw, err := os.ReadFile(settings.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if unmarshalErr := json.Unmarshal(raw, &doc); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	doc["nav_show_numbers"] = !m.settingsPage.NavShowNumbers
	want, ok := doc["nav_show_numbers"].(bool)
	if !ok {
		t.Fatal("nav_show_numbers is not a bool")
	}
	edited, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings.FilePath(), edited, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, cmd := m.Update(filewatch.Event{Path: settings.FilePath(), Op: "WRITE"}); cmd != nil {
		drainCmds(cmd)
	}
	if m.settingsPage.NavShowNumbers != want {
		t.Fatalf(
			"settings not reloaded: NavShowNumbers = %v, want %v",
			m.settingsPage.NavShowNumbers,
			want,
		)
	}
	if got := len(m.notifMgr.Active()); got != baseline+1 {
		t.Fatalf("external change should add exactly one notification: %d -> %d", baseline, got)
	}
	found := false
	for _, n := range m.notifMgr.Active() {
		if n.Content == "Settings reloaded from disk" && n.Severity == notifications.SeverityInfo {
			found = true
		}
	}
	if !found {
		t.Fatal("reload notification not found in active notifications")
	}
}

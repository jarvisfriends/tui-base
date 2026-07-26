// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/jarvisfriends/snap/notifications"
	"github.com/jarvisfriends/snap/status"
)

// TestColorProfileEnvOverride: TUI_BASE_COLOR_PROFILE forces the profile
// (case-insensitive, several aliases); unset or garbage falls back to
// detection, and NewProgram builds a program either way.
func TestColorProfileEnvOverride(t *testing.T) {
	for val, want := range map[string]colorprofile.Profile{
		"truecolor": colorprofile.TrueColor,
		"24bit":     colorprofile.TrueColor,
	} {
		t.Setenv(ColorProfileEnvVar, val)
		got, ok := ForcedColorProfile()
		if !ok || got != want {
			t.Errorf("%q: got %v ok=%v; want %v", val, got, ok, want)
		}
		if EffectiveColorProfile() != want {
			t.Errorf("%q: EffectiveColorProfile should honor the override", val)
		}
	}

	t.Setenv(ColorProfileEnvVar, "not-a-profile")
	if _, ok := ForcedColorProfile(); ok {
		t.Error("garbage value should not force a profile")
	}

	t.Setenv(ColorProfileEnvVar, "truecolor")
	if p := NewProgram(New()); p == nil {
		t.Fatal("NewProgram returned nil")
	}
}

// TestOverlayOutsideClickClosers: each modal overlay's CloseOnOutsideClick
// actually closes its surface — history panel, inspector, and info modal.
func TestOverlayOutsideClickClosers(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})

	// History panel.
	m = updateRouter(m, notifications.AddMsg{Content: "n", Severity: notifications.SeverityInfo})
	_ = m.status.ToggleNotifications()
	if !m.status.IsHistoryVisible() {
		t.Fatal("history should be open")
	}
	drainCmd(m, (&historyOverlay{m: m}).CloseOnOutsideClick())
	if m.status.IsHistoryVisible() {
		t.Fatal("outside click should close the history panel")
	}

	// Inspector.
	m.inspector.ToggleVisible()
	if !m.inspector.IsVisible() {
		t.Fatal("inspector should be open")
	}
	drainCmd(m, (&inspectorOverlay{m: m}).CloseOnOutsideClick())
	if m.inspector.IsVisible() {
		t.Fatal("outside click should close the inspector")
	}

	// Info modal: the closer emits CloseInfoModalMsg for the router.
	m.infoModal.Toggle(90, 30)
	drainCmd(m, (&infoOverlay{m: m}).CloseOnOutsideClick())
	if m.infoModal.IsVisible() {
		t.Fatal("outside click should close the info modal")
	}
}

// TestInfoOverlayScrollWheel: wheel events over the info modal turn into
// InfoModalScrollMsg with the right direction.
func TestInfoOverlayScrollWheel(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	o := &infoOverlay{m: m}

	cmd := o.OverlayMouse(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if cmd == nil {
		t.Fatal("wheel should produce a scroll command")
	}
	if msg, ok := cmd().(status.InfoModalScrollMsg); !ok || !msg.Up {
		t.Fatalf("wheel-up cmd = %#v", cmd())
	}
	if cmd := o.OverlayMouse(tea.MouseClickMsg(tea.Mouse{})); cmd != nil {
		t.Fatal("non-wheel mouse should be ignored")
	}
}

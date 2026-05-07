package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/keys"

	tea "charm.land/bubbletea/v2"
)

func TestQuitKeyEmitsCmd(t *testing.T) {
	t.Parallel()
	m := New()
	m.keys = keys.DefaultKeyMap()

	_, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for quit key")
	}
	// Ensure invoking the cmd returns a message (Quit or similar)
	if msg := cmd(); msg == nil {
		t.Fatalf("expected cmd() to return a non-nil message")
	}
}

func TestTabWithNilNavCyclesPages(t *testing.T) {
	t.Parallel()
	m := New()
	// ensure tab advances the active nav page
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != "home" {
		t.Fatalf("expected initial active page 'home'; got %q", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != "settings" {
		t.Fatalf("expected active page 'settings' after tab; got %q", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}
}

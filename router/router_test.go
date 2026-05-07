package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/navigation"

	tea "charm.land/bubbletea/v2"
)

func TestTabCyclesPages(t *testing.T) {
	t.Parallel()
	m := New()
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != "home" {
		t.Fatalf("initial active page = %q; want \"home\"", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != "settings" {
		t.Fatalf("after tab active page = %q; want \"settings\"", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}

	// nav highlight should be in sync
	idx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == "settings" {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("settings page not found in nav")
	}
	if m.nav.GetActiveIndex() != idx {
		t.Fatalf("nav.ActiveIndex = %d; want %d", m.nav.GetActiveIndex(), idx)
	}
}

func TestSelectedMsgSwitchesPage(t *testing.T) {
	t.Parallel()
	m := New()

	// find settings index
	idx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == "settings" {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("settings page not found in nav")
	}

	_, _ = m.Update(navigation.SelectedMsg{PageIndex: idx})
	if m.nav.GetPages()[m.nav.GetActiveIndex()].ID != "settings" {
		t.Fatalf("active page = %q; want \"settings\"", m.nav.GetPages()[m.nav.GetActiveIndex()].ID)
	}
	if m.nav.GetActiveIndex() != idx {
		t.Fatalf("nav.ActiveIndex = %d; want %d", m.nav.GetActiveIndex(), idx)
	}
}

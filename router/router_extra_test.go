package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/keys"
	"github.com/jarvisfriends/tui-base/navigation"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestHandleResizeCmdProducesExpectedSizes(t *testing.T) {
	t.Parallel()
	m := New()
	m.width = 100
	m.height = 40
	m.navigationVisible = true
	m.keys = keys.DefaultKeyMap()
	// Ensure status help view has the correct width so height is computed
	m.status.SetKeys(m.keys)
	m.status.SetWidth(m.width)

	_ = m.handleResizeCmd()

	navWidth := 0
	navHeight := 0
	if m.navigationVisible && m.nav != nil {
		switch m.nav.(type) {
		case *navigation.Tabs:
			navHeight = m.nav.Height()
		default:
			navWidth = m.nav.Width()
			navHeight = m.nav.Height()
		}
	}
	helpHeight := lipgloss.Height(m.status.View().Content)
	expectedContentWidth := m.width - navWidth
	expectedContentHeight := m.height - helpHeight - navHeight

	activePageContent := m.GetActivePage().View().Content

	if lipgloss.Width(activePageContent) != expectedContentWidth {
		t.Fatalf("ContentWidth = %d; want %d", lipgloss.Width(activePageContent), expectedContentWidth)
	}
	if lipgloss.Height(activePageContent) != expectedContentHeight {
		t.Fatalf("ContentHeight = %d; want %d", lipgloss.Height(activePageContent), expectedContentHeight)
	}
}

func TestViewContainsHomeAndMouseMode(t *testing.T) {
	t.Parallel()
	m := New()
	m.width = 80
	m.height = 24
	v := m.View()
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected MouseModeCellMotion; got %v", v.MouseMode)
	}
	if v.Content == "" {
		t.Fatal("expected non-empty view content")
	}
}

func TestWindowSizeMsgUpdatesRouterState(t *testing.T) {
	t.Parallel()
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Fatalf("router width/height not updated; got %d x %d", m.width, m.height)
	}
	// children should have been updated; ensure views are non-empty
	if m.nav == nil || m.nav.View().Content == "" {
		t.Fatal("expected nav view content after WindowSizeMsg")
	}
	// find home page in pages slice
	homeIdx := -1
	for i, p := range m.nav.GetPages() {
		if p.ID == "home" {
			homeIdx = i
			break
		}
	}
	if homeIdx == -1 {
		t.Fatal("home page not found in nav")
	}
	if homeIdx >= len(m.pages) || m.pages[homeIdx].View().Content == "" {
		t.Fatal("expected home view content after WindowSizeMsg")
	}
}

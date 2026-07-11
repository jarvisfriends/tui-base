package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/navigation"
)

// keyRecorderPage records every key press routed to it, so tests can prove
// whether the router delivered a key to the active page.
type keyRecorderPage struct {
	keys *[]string
}

func (p keyRecorderPage) Init() tea.Cmd { return nil }
func (p keyRecorderPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		*p.keys = append(*p.keys, k.String())
	}
	return p, nil
}
func (p keyRecorderPage) View() tea.View { return tea.NewView(RecorderPageTitle) }

// TestLetterKeysReachPageWhileSidebarFocused reproduces the "page action key
// does nothing" trap: Esc/← silently move focus to the sidebar, and the
// sidebar only uses arrows/tab/enter/esc — so a page action key like `m`
// pressed afterwards was dropped entirely. Keys the sidebar doesn't use
// must fall through to the active page.
func TestLetterKeysReachPageWhileSidebarFocused(t *testing.T) {
	var received []string
	m := NewWithRegisteredPages([]RegisteredPage{
		{Title: RecorderPageTitle, Model: keyRecorderPage{keys: &received}},
	})
	m.nav = navigation.New() // focusable sidebar
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// The user presses Esc on the page: focus silently moves to the sidebar.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.sidebarFocused {
		t.Fatal("precondition: Esc should focus the sidebar")
	}

	// Stage 1: a page action key (like the spelling page's `m` promote) must
	// still reach the page even though the sidebar has key focus.
	received = received[:0]
	_, _ = m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if len(received) == 0 {
		t.Fatal("stage 1: `m` was dropped — sidebar focus swallowed a key the sidebar doesn't use")
	}

	// Stage 2: keys the sidebar DOES use (Down) must stay with the sidebar,
	// not leak into the page.
	received = received[:0]
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if len(received) != 0 {
		t.Fatalf("stage 2: sidebar navigation key leaked to the page: %v", received)
	}
	if !m.sidebarFocused {
		t.Fatal("stage 2: Down must keep focus on the sidebar")
	}
}

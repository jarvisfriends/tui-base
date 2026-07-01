package navigation

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestSidebarContentFillsToBorder renders the expanded sidebar at several
// widths and asserts every line's border rune sits exactly at innerWidth()
// (0-indexed) — i.e. there is no unstyled gap column between the sidebar's
// content and its border. innerWidth() is derived from sidebarFrame's real
// GetHorizontalFrameSize() rather than a hardcoded literal specifically so
// this holds regardless of what border sidebarFrame ends up using; a
// hand-counted constant that doesn't match the border actually drawn (as
// this package previously had: -2 for a border that only occupies the right
// edge) wastes a column of content on every render without overflowing,
// so CheckNoLineOverflow-style tests can't catch it.
func TestSidebarContentFillsToBorder(t *testing.T) {
	t.Parallel()
	for _, w := range []int{20, 30, 40, 60, 100, 160} {
		m, _ := New().Update(tea.WindowSizeMsg{Width: w, Height: 20})
		sb, ok := m.(*Sidebar)
		if !ok {
			t.Fatalf("width=%d: Update returned %T, want *Sidebar", w, m)
		}
		want := sb.innerWidth()

		v := sb.View()
		for i, line := range strings.Split(v.Content, "\n") {
			stripped := ansi.Strip(line)
			runes := []rune(stripped)
			if len(runes) == 0 {
				continue
			}
			if want >= len(runes) || runes[want] != '│' {
				t.Fatalf("width=%d line %d: expected border '│' at col %d (innerWidth), got %q",
					w, i, want, stripped)
			}
		}
	}
}

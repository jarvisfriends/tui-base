package navigation

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// tortureTitles exercises the width edge cases that break naive len()-based
// layout math (TS-2): CJK double-width, emoji ZWJ sequences, flags, and
// combining marks.
var tortureTitles = []string{
	"日本語ページ",         // CJK double-width
	"👨‍👩‍👧‍👦 Family", // ZWJ sequence
	"🇺🇸 Flags",       // regional indicator pair
	"café àè",        // combining/accented
	"Ｗｉｄｅ",           // fullwidth latin
}

func torturePages() []Page {
	pages := make([]Page, 0, len(tortureTitles))
	for i, title := range tortureTitles {
		pages = append(pages, Page{ID: string(rune('a' + i)), Title: title})
	}
	return pages
}

// assertLinesFit fails if any rendered line exceeds width display cells.
func assertLinesFit(t *testing.T, content string, width int, label string) {
	t.Helper()
	for i, line := range strings.Split(content, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("%s: line %d is %d cells wide; exceeds %d: %q", label, i, got, width, line)
		}
	}
}

func TestTabsUnicodeTitlesFitWidth(t *testing.T) {
	t.Parallel()

	m := NewTabs()
	m.SetPages(torturePages())
	for _, w := range []int{30, 60, 100} {
		_, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
		assertLinesFit(t, m.View().Content, w, "tabs")
	}
}

func TestMinimalTopNavUnicodeTitlesRenderConsistently(t *testing.T) {
	t.Parallel()

	m := NewMinimalTopNav()
	m.SetPages(torturePages())
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	v := m.View()

	// Click ranges must be contiguous and derived from display width — a
	// byte-based measurement would leave gaps or overlaps between tabs.
	for i := 1; i < len(m.starts); i++ {
		if m.starts[i] != m.ends[i-1]+1 {
			t.Errorf("click ranges not contiguous at %d: end[%d]=%d start[%d]=%d",
				i, i-1, m.ends[i-1], i, m.starts[i])
		}
	}
	total := lipgloss.Width(v.Content)
	if last := m.ends[len(m.ends)-1]; last != total-1 {
		t.Errorf("last click range ends at %d; rendered row is %d cells", last, total)
	}
}

func TestSidebarUnicodeTitlesFitWidth(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetPages(torturePages())
	_, _ = m.Update(tea.WindowSizeMsg{Width: 24, Height: 20})
	assertLinesFit(t, m.View().Content, 24, "sidebar")
}

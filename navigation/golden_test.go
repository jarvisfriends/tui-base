package navigation

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/rendercheck"
)

// Golden renders of the three navigators with the default page set and theme
// (TS-1): catches alignment, border, and styling regressions that width/
// height assertions cannot see. Regenerate with UPDATE_GOLDEN=1 after an
// intentional visual change and review the diff.
func TestNavigatorGoldenRenders(t *testing.T) {
	tabs := NewTabs()
	_, _ = tabs.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	rendercheck.Golden(t, "tabs_60w", tabs.View().Content)

	// Overflowing tab strip: window + arrows.
	wide := NewTabs()
	wide.SetPages(torturePages())
	wide.SetActiveIndex(2)
	_, _ = wide.Update(tea.WindowSizeMsg{Width: 30, Height: 24})
	rendercheck.Golden(t, "tabs_overflow_30w", wide.View().Content)

	top := NewMinimalTopNav()
	top.SetShowNumbers(true)
	_, _ = top.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	rendercheck.Golden(t, "topnav_numbers_60w", top.View().Content)

	sb := New()
	_, _ = sb.Update(tea.WindowSizeMsg{Width: 24, Height: 12})
	rendercheck.Golden(t, "sidebar_24w", sb.View().Content)
}

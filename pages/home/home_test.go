package home_test

import (
	"testing"

	home "github.com/jarvisfriends/tui-base/pages/home"
	"github.com/jarvisfriends/tui-base/testutil"

	tea "charm.land/bubbletea/v2"
)

func TestHomeResizeAndView(t *testing.T) {
	m := home.New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := m.View()
	if v.Content == "" {
		t.Fatal("home View content should not be empty")
	}
	if !v.AltScreen {
		t.Fatal("expected AltScreen true on home View")
	}
}

// TestHomeNeverOverflows asserts that the home page View never produces a line
// wider than the terminal width, at a range of terminal sizes.
func TestHomeNeverOverflows(t *testing.T) {
	testutil.CheckNoLineOverflow(t, home.New(), testutil.StandardWidths)
}

// TestHomeNarrowWidths asserts the home page does not crash or overflow at very
// narrow terminal widths where the bordered box might be wider than the screen.
func TestHomeNarrowWidths(t *testing.T) {
	testutil.CheckNoBorderOverflow(t, home.New(), 20, 24)
}

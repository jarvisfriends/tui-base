package theme

import (
	"testing"
)

// TestBubblesHelpersDeriveFromTheme locks the TC-1 contract: the bubbles
// widget styles come from the active theme's semantic slots, not widget
// defaults.
func TestBubblesHelpersDeriveFromTheme(t *testing.T) {
	c := Active()

	ts := TableStyles(c)
	if got := ts.Selected.GetBackground(); got != c.SelectionBg {
		t.Errorf("table Selected bg = %v; want SelectionBg %v", got, c.SelectionBg)
	}
	if got := ts.Selected.GetForeground(); got != c.SelectionFg {
		t.Errorf("table Selected fg = %v; want SelectionFg %v", got, c.SelectionFg)
	}

	ls := ListDelegateStyles(c)
	if got := ls.SelectedTitle.GetForeground(); got != c.Accent {
		t.Errorf("list SelectedTitle fg = %v; want Accent %v", got, c.Accent)
	}
	if got := ls.NormalDesc.GetForeground(); got != c.Muted {
		t.Errorf("list NormalDesc fg = %v; want Muted %v", got, c.Muted)
	}

	if got := SpinnerStyle(c).GetForeground(); got != c.Accent {
		t.Errorf("spinner fg = %v; want Accent %v", got, c.Accent)
	}

	from, to := ProgressGradient(c)
	if from == "" || to == "" || from == to {
		t.Errorf("progress gradient (%q, %q) should be two distinct theme colors", from, to)
	}
}

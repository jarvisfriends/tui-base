package testutil

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// CheckBorderIntegrity renders a single-box model at every standard size and
// asserts the border-character invariant (CF-3): each rendered line that
// contains the vertical border glyph contains it exactly twice (left and
// right edge). A line with more occurrences means inner content wrapped and
// pushed a border glyph inward; fewer means an edge was clipped — both are
// sharper wrap signals than a plain width check, which passes as long as the
// wrapped result still fits.
//
// The glyph is a parameter so callers whose content legitimately contains the
// default "│" can restyle the box border to a sentinel rune for the test.
// Only use this on models that render one bordered box (overlays, modals);
// multi-column layouts have legitimate higher counts.
func CheckBorderIntegrity(t *testing.T, m tea.Model, vertical string) {
	t.Helper()
	for i, w := range StandardWidths {
		h := 24
		if i < len(StandardHeights) {
			h = StandardHeights[i]
		}
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		CheckBorderIntegrityString(t, m.View().Content, vertical)
	}
}

// CheckBorderIntegrityString applies the CF-3 invariant to an already
// rendered frame (for overlay strings produced outside a tea.Model View).
func CheckBorderIntegrityString(t *testing.T, content, vertical string) {
	t.Helper()
	for i, line := range strings.Split(StripANSI(content), "\n") {
		n := strings.Count(line, vertical)
		if n != 0 && n != 2 {
			t.Errorf(
				"border integrity: line %d contains %d %q glyphs (want 0 or 2) — inner content likely wrapped\n  %q",
				i,
				n,
				vertical,
				line,
			)
		}
	}
}

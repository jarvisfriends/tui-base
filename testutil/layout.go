// Package testutil provides shared test helpers for layout correctness tests
// across tui-base and applications built on it.
//
// Typical usage in a page test:
//
//	func TestMyPageNeverOverflows(t *testing.T) {
//	    m := mypage.New()
//	    testutil.CheckNoLineOverflow(t, m, testutil.StandardWidths)
//	}
package testutil

import (
	"image/color"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
)

// StandardWidths is the set of terminal widths every page is tested against.
var StandardWidths = []int{40, 60, 80, 100, 120, 160, 200}

// StandardHeights is the set of terminal heights paired with StandardWidths.
var StandardHeights = []int{12, 20, 24, 30, 40, 50, 50}

// CheckNoLineOverflow renders a tea.Model at each width in widths (paired with
// height 24) and asserts that no rendered line exceeds the terminal width in
// display cells. Use this for every page that renders tabular or variable
// content.
func CheckNoLineOverflow(t *testing.T, m tea.Model, widths []int) {
	t.Helper()
	for _, w := range widths {
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
		v := m.View()
		for i, line := range strings.Split(v.Content, "\n") {
			got := lipgloss.Width(line)
			if got > w {
				t.Errorf("width=%d line %d overflows by %d cell(s): %q",
					w, i, got-w, StripANSI(line))
			}
		}
	}
}

// CheckNoLineOverflowAtSizes is like CheckNoLineOverflow but tests each
// (width, height) pair from StandardWidths and StandardHeights.
func CheckNoLineOverflowAtSizes(t *testing.T, m tea.Model) {
	t.Helper()
	for i, w := range StandardWidths {
		h := 24
		if i < len(StandardHeights) {
			h = StandardHeights[i]
		}
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		v := m.View()
		for lineIdx, line := range strings.Split(v.Content, "\n") {
			got := lipgloss.Width(line)
			if got > w {
				t.Errorf("width=%d height=%d line %d overflows by %d: %q",
					w, h, lineIdx, got-w, StripANSI(line))
			}
		}
	}
}

// CheckNoBorderOverflow renders a component at narrow widths (down to minWidth)
// and asserts that bordered boxes do not wrap their border characters to a new
// line (which would happen if inner content is wider than available space).
func CheckNoBorderOverflow(t *testing.T, m tea.Model, minWidth, height int) {
	t.Helper()
	for w := minWidth; w <= 80; w += 5 {
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: height})
		v := m.View()
		for i, line := range strings.Split(v.Content, "\n") {
			got := lipgloss.Width(line)
			if got > w {
				t.Errorf("narrow width=%d line %d overflows by %d: %q",
					w, i, got-w, StripANSI(line))
			}
		}
	}
}

// CheckNoBackgroundHoles asserts that every non-blank line of rendered contains
// the ANSI background parameter sequence for wantBg. Lines without the
// background code expose the terminal default, causing visual "holes".
//
// Blank lines (containing only whitespace after ANSI stripping) are skipped
// because they may legitimately carry no background code when the terminal
// BackgroundColor OSC fills them.
func CheckNoBackgroundHoles(t *testing.T, rendered string, wantBg color.Color, label string) {
	t.Helper()
	params := bgNumericParams(wantBg)
	if params == "" {
		t.Skip("no ANSI background code — running in no-color mode")
	}
	for i, line := range NonBlankLines(rendered) {
		if !strings.Contains(line, params) {
			t.Errorf("%s: row %d missing bg params %q: %q",
				label, i, params, StripANSI(line))
		}
	}
}

// CheckEmojiColumnWidths verifies that a set of symbols, when rendered through
// lipgloss.Style.Width(colW), produce exactly colW display cells each. This
// catches column definitions that are too narrow for wide emoji.
func CheckEmojiColumnWidths(t *testing.T, symbols []string, colW int) {
	t.Helper()
	style := lipgloss.NewStyle().Width(colW).MaxWidth(colW)
	for _, sym := range symbols {
		rendered := style.Render(sym)
		got := lipgloss.Width(rendered)
		if got != colW {
			t.Errorf("symbol %q (display width %d) rendered in Width(%d) produced %d cells — column too narrow",
				sym, lipgloss.Width(sym), colW, got)
		}
	}
}

// ─── shared ANSI helpers (duplicated from status package to avoid import cycle) ──────

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

// StripANSI removes ANSI escape sequences, leaving only printable characters.
func StripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// NonBlankLines splits s on "\n", strips trailing blank lines, and returns
// only lines with non-whitespace content (after ANSI stripping).
func NonBlankLines(s string) []string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(StripANSI(l)) != "" {
			out = append(out, l)
		}
	}
	return out
}

// firstEscape returns the first ANSI escape sequence found in s.
func firstEscape(s string) string {
	i := strings.Index(s, "\x1b[")
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], "m")
	if j < 0 {
		return ""
	}
	return s[i : i+j+1]
}

// bgNumericParams extracts the numeric parameter string from the ANSI
// background escape code (e.g. "48;5;236" from "\x1b[48;5;236m").
func bgNumericParams(c color.Color) string {
	full := firstEscape(lipgloss.NewStyle().Background(c).Render("X"))
	if len(full) < 4 {
		return ""
	}
	return full[2 : len(full)-1]
}

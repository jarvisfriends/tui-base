package testutil

// conformance.go — model-agnostic conformance checks any tui-base-derived app can
// run against its router (or a single page) in unit tests. They drive the model
// purely through messages (WindowSizeMsg, theme messages, key/selection messages)
// and assert framework invariants that are easy to break in custom pages:
//
//	func TestMyAppConforms(t *testing.T) {
//	    m := router.NewWithRegisteredPages(myPages())
//	    testutil.CheckFitsViewport(t, m)
//	    testutil.CheckStatusBarVisible(t, m, testutil.PageAndOverlayStates(m))
//	    testutil.CheckThemeResponsive(t, m,
//	        settings.ThemeMsg{ID: "dracula", Mode: "dark", ApplyPreferences: true},
//	        settings.ThemeMsg{ID: "dracula", Mode: "light", ApplyPreferences: true})
//	}

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// CheckFitsViewport renders m at each standard (width,height) and asserts the
// frame never exceeds the terminal box: the content-line count is <= height (so
// over-tall content is clipped or scrolled, not spilling past the screen) and
// every line's display width is <= width. This catches the single most common
// TUI bug — content larger than the available space that is neither scrolled nor
// clipped, which corrupts the terminal.
func CheckFitsViewport(t *testing.T, m tea.Model) {
	t.Helper()
	for i, w := range StandardWidths {
		h := 24
		if i < len(StandardHeights) {
			h = StandardHeights[i]
		}
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		lines := strings.Split(m.View().Content, "\n")
		n := len(lines)
		for n > 0 && strings.TrimSpace(StripANSI(lines[n-1])) == "" {
			n-- // ignore trailing blank lines
		}
		if n > h {
			t.Errorf("width=%d height=%d: rendered %d content lines, exceeds height %d (content not clipped/scrolled)",
				w, h, n, h)
		}
		for li, line := range lines {
			if gw := lipgloss.Width(line); gw > w {
				t.Errorf("width=%d height=%d line %d width %d exceeds %d: %q",
					w, h, li, gw, w, StripANSI(line))
			}
		}
	}
}

// StatusProvider is implemented by models (the tui-base router) that can report
// their status bar's current text and visibility, so CheckStatusBarVisible can
// assert the status bar is present in every rendered frame.
type StatusProvider interface {
	StatusBarContent() (text string, visible bool)
}

// CheckStatusBarVisible asserts that — initially and after each state transition
// in states — the status bar's text (when visible) still appears in the fully
// rendered frame. Pass page switches and overlay/prompt toggles as states to
// cover pages, overlays, and prompts. Skips if the model does not implement
// StatusProvider.
func CheckStatusBarVisible(t *testing.T, m tea.Model, states []tea.Msg) {
	t.Helper()
	sp, ok := m.(StatusProvider)
	if !ok {
		t.Skip("model does not implement testutil.StatusProvider; cannot assert status bar")
		return
	}
	assert := func(label string) {
		text, vis := sp.StatusBarContent()
		if !vis {
			return // hidden by design in this state
		}
		sig := longestLine(StripANSI(text))
		if sig == "" {
			return
		}
		if !strings.Contains(StripANSI(m.View().Content), sig) {
			t.Errorf("status bar not present in frame at %s: missing %q", label, sig)
		}
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	assert("initial")
	for i, st := range states {
		m, _ = m.Update(st)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		// Re-resolve the provider: Update returns a fresh tea.Model each call.
		if next, ok := m.(StatusProvider); ok {
			sp = next
		}
		assert(fmt.Sprintf("state[%d] %T", i, st))
	}
}

// CheckThemeResponsive asserts that switching themes actually changes the colors
// the model renders — i.e. it draws through the shared theme rather than
// hard-coded colors. themeA and themeB must be two drastically different theme
// messages (e.g. the same tint in dark vs light mode, or two very different
// tints). Skips in no-color mode.
func CheckThemeResponsive(t *testing.T, m tea.Model, themeA, themeB tea.Msg) {
	t.Helper()
	colorsAfter := func(msg tea.Msg) map[string]bool {
		m, _ = m.Update(msg)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		return ansiColorSet(m.View().Content)
	}
	a := colorsAfter(themeA)
	b := colorsAfter(themeB)
	if len(a) == 0 && len(b) == 0 {
		t.Skip("no ANSI colors — running in no-color mode")
	}
	if sameSet(a, b) {
		t.Errorf("theme change did not alter any rendered colors (%d codes both ways) — model may use hard-coded colors instead of the base theme", len(a))
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func longestLine(s string) string {
	var best string
	bestW := -1
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.TrimSpace(l); lipgloss.Width(l) > bestW {
			best, bestW = l, lipgloss.Width(l)
		}
	}
	return best
}

// ansiColorSet returns the set of distinct ANSI SGR color sequences in s
// (excluding plain resets), using the package ansiRE from layout.go.
func ansiColorSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, code := range ansiRE.FindAllString(s, -1) {
		if strings.HasSuffix(code, "m") && code != "\x1b[m" && code != "\x1b[0m" {
			out[code] = true
		}
	}
	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

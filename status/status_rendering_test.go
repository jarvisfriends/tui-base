package status

// Tests that reproduce visual bugs in the status bar and notification overlay:
//
//  1. (ICONS POSITION) When full help is expanded the ⚙/🔔/ℹ icons snap to the
//     first row instead of the last row, because lipgloss.JoinHorizontal
//     top-aligns all columns.
//
//  2. (BACKGROUND BLEED) Rows in the expanded help bar and notification history
//     overlay revert to the terminal default background.  JoinHorizontal pads
//     shorter columns (gap, icons) to the height of the multi-line left column
//     with unstyled spaces, erasing StatusBg on those filler rows.
//
//  3. (SEPARATOR MISSING) The "•" separator between key name and description is
//     not visible in the short-help line.
//
//  4. (OVERLAY HEADER BACKGROUND) The notification overlay header does not fill
//     innerW, so JoinVertical pads it with plain (unstyled) spaces that show the
//     terminal-default background as a lighter band to the right of the text.
//
//  5. (OVERLAY ROW GAP) The notification row gap calculation subtracts the badge
//     width twice, making each row too narrow and causing the same JoinVertical
//     plain-space padding bug on the right side of notification rows.

import (
	"image/color"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jarvisfriends/tui-base/keys"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/lipgloss/v2"
)

// ─── ANSI helpers ────────────────────────────────────────────────────────────

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

// stripANSI removes ANSI escape sequences, leaving only printable characters.
func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// firstEscape returns the first ANSI escape sequence found in s,
// e.g. "\x1b[48;5;236m".
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

// bgEscape asks lipgloss to render a single character with the given
// background color and returns just the ANSI escape code prefix.
// Returns "" when lipgloss emits no escape (e.g. NO_COLOR mode).
func bgEscape(c color.Color) string {
	return firstEscape(lipgloss.NewStyle().Background(c).Render("X"))
}

// bgNumericParams extracts the numeric parameter string from the ANSI background
// escape code (e.g. "48;5;236" from "\x1b[48;5;236m").  Checking for this
// substring rather than the full standalone escape is necessary because lipgloss
// often emits combined fg+bg sequences such as "\x1b[38;5;238;48;5;236m".
func bgNumericParams(c color.Color) string {
	full := bgEscape(c) // e.g. "\x1b[48;5;236m"
	if len(full) < 4 {
		return ""
	}
	// strip leading "\x1b[" (2 bytes) and trailing "m" (1 byte)
	return full[2 : len(full)-1]
}

// nonBlankLines splits s on "\n" and strips trailing blank lines.
func nonBlankLines(s string) []string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(stripANSI(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// iconPresent reports whether any of the three status-bar icon glyphs appear in s.
func iconPresent(s string) bool {
	return strings.ContainsAny(s, "⚙🔔ℹ🔕")
}

// ─── Bug 1 – icon position ───────────────────────────────────────────────────

// TestFullHelpIconsOnLastRow asserts that the ⚙/🔔/ℹ icons land on the LAST
// row of the expanded full-help bar, not on the first row.
//
// Failing behavior: lipgloss.JoinHorizontal(Left, …) top-aligns all columns,
// so the single-row icons column aligns with row 0 of the multi-line help text.
func TestFullHelpIconsOnLastRow(t *testing.T) {
	b := New()
	b.SetKeys(keys.DefaultKeyMap())
	b.help.ShowAll = true
	b.SetWidth(120)

	lines := nonBlankLines(b.helpView.Content)
	if len(lines) < 2 {
		t.Skip("full help produced only one line — need multi-row rendering")
	}

	last := stripANSI(lines[len(lines)-1])
	first := stripANSI(lines[0])

	if !iconPresent(last) {
		t.Errorf("icons (⚙/🔔/ℹ) not found on last row (row %d)\n  last:  %q\n  first: %q",
			len(lines)-1, last, first)
	}
	if iconPresent(first) {
		t.Errorf("icons must NOT appear on first row but were found\n  first: %q", first)
	}
}

// ─── Bug 2 – background bleed ────────────────────────────────────────────────

// TestFullHelpAllRowsCarryStatusBg asserts that every row in the expanded
// help bar contains the StatusBg background escape sequence.
//
// Failing behavior: the gap/icon columns are single-row; JoinHorizontal pads
// them with unstyled whitespace to match the taller left column.  Those filler
// rows contain no background code and revert to the terminal default color.
func TestFullHelpAllRowsCarryStatusBg(t *testing.T) {
	b := New()
	b.SetKeys(keys.DefaultKeyMap())
	b.help.ShowAll = true
	b.SetWidth(120)

	c := theme.Active()
	wantParams := bgNumericParams(c.StatusBg)
	if wantParams == "" {
		t.Skip("no ANSI background code — running in no-color mode")
	}

	lines := nonBlankLines(b.helpView.Content)
	if len(lines) < 2 {
		t.Skip("full help produced only one line — need multi-row rendering")
	}

	for i, line := range lines {
		if !strings.Contains(line, wantParams) {
			t.Errorf("row %d missing StatusBg params %q\n  stripped: %q",
				i, wantParams, stripANSI(line))
		}
	}
}

// TestHistoryOverlayAllRowsCarryMainBg asserts that every row rendered by the
// notification history overlay carries the MAIN app background — the panel
// reads as part of the page, not the status bar (deliberate design choice,
// 2026-07-03). Inner join-filler cells must not revert to the terminal
// default, and the StatusBg escape code must not appear anywhere.
func TestHistoryOverlayAllRowsCarryMainBg(t *testing.T) {
	overlay := NewUserNotificationOverlay()
	nm := notifications.NewManager()
	overlay.SetNotifManager(nm)
	nm.Add("test notification", notifications.SeverityInfo, 5*time.Second)
	overlay.showHistory = true

	rendered := overlay.RenderHistoryOverlay(80, 20)
	if rendered == "" {
		t.Fatal("expected non-empty history overlay")
	}

	c := theme.Active()
	statusBgParams := bgNumericParams(c.StatusBg)
	mainBgParams := bgNumericParams(c.Bg)

	if mainBgParams == "" {
		t.Skip("no ANSI background code — running in no-color mode")
	}
	if mainBgParams == statusBgParams {
		t.Skip("main Bg and StatusBg are identical in this theme — cannot distinguish")
	}

	for i, line := range nonBlankLines(rendered) {
		if !strings.Contains(line, mainBgParams) {
			t.Errorf("overlay row %d missing main Bg params %q\n  stripped: %q",
				i, mainBgParams, stripANSI(line))
		}
		if strings.Contains(line, statusBgParams) {
			t.Errorf(
				"overlay row %d contains StatusBg params %q — should use the main Bg throughout\n  stripped: %q",
				i,
				statusBgParams,
				stripANSI(line),
			)
		}
	}
}

// ─── Bug 3 – separator dot ───────────────────────────────────────────────────

// TestHelpSeparatorPresentInShortHelp asserts that the "•" separator character
// is present (not missing) in the short-help rendered output after stripping
// ANSI codes.
//
// The separator may be present but invisible when its foreground color matches
// StatusBg; this test catches the case where the character is dropped entirely.
func TestHelpSeparatorPresentInShortHelp(t *testing.T) {
	b := New()
	b.SetKeys(keys.DefaultKeyMap())
	b.SetWidth(120)

	stripped := stripANSI(b.helpView.Content)
	if !strings.Contains(stripped, "•") {
		t.Error("separator '•' not found in short-help output — " +
			"check that the help widget separator style is wired correctly")
	}
}

// TestHelpSeparatorStyleUsesStatusBg asserts that the short-help separator
// style carries StatusBg, not the main page background.
//
// If the separator renders with main Bg it will appear as a visual hole in the
// status bar.
func TestHelpSeparatorStyleUsesStatusBg(t *testing.T) {
	c := theme.Active()

	mainBgParams := bgNumericParams(c.Bg)
	statusBgParams := bgNumericParams(c.StatusBg)
	if mainBgParams == "" || mainBgParams == statusBgParams {
		t.Skip("cannot distinguish main Bg from StatusBg in this environment")
	}

	sepRendered := c.Styles.Help.ShortSeparator.Render("•")

	if strings.Contains(sepRendered, mainBgParams) &&
		!strings.Contains(sepRendered, statusBgParams) {
		t.Errorf("separator uses main Bg params %q instead of StatusBg params %q\n  rendered: %q",
			mainBgParams, statusBgParams, sepRendered)
	}
}

// TestHelpSeparatorForegroundVisibleAgainstStatusBg asserts that the separator
// foreground color is visually distinguishable from the status bar background.
//
// Failing behavior: the separator style uses c.Border as foreground.  In many
// dark themes c.Border maps to the "black" terminal slot, which is the same
// dark shade as StatusBg — making the "•" effectively invisible.
func TestHelpSeparatorForegroundVisibleAgainstStatusBg(t *testing.T) {
	c := theme.Active()

	// Render "•" with the separator style and with "invisible" (fg == bg) style.
	// If both produce the same foreground ANSI code the dot is invisible.
	sepRendered := c.Styles.Help.ShortSeparator.Render("•")
	invisRendered := lipgloss.NewStyle().
		Background(c.StatusBg).
		Foreground(c.StatusBg).
		Render("•")

	sepFgCode := firstEscape(sepRendered)
	invFgCode := firstEscape(invisRendered)

	if sepFgCode != "" && invFgCode != "" && sepFgCode == invFgCode {
		t.Errorf("separator foreground ANSI code %q matches background — dot will be invisible",
			sepFgCode)
	}
}

// ─── Bug 4 – overlay header background ───────────────────────────────────────

// resetSpaceRE matches an ANSI style-reset escape (\x1b[m or \x1b[0m) immediately
// followed by a plain space character.  This pattern indicates that JoinVertical
// padded an element narrower than innerW with unstyled (terminal-default background)
// spaces.
var resetSpaceRE = regexp.MustCompile("\x1b\\[0?m ")

// TestHistoryOverlayHeaderNoTrailingUnstyled asserts that the notification history
// overlay header row does NOT have a plain-space cell immediately after a reset code.
//
// Failing behavior: headerStyle.Render(...) has no Width(innerW), so the rendered
// string is only as wide as the header text.  JoinVertical pads it with bare spaces
// that carry no background — visible as a lighter band to the right of the header.
func TestHistoryOverlayHeaderNoTrailingUnstyled(t *testing.T) {
	overlay := NewUserNotificationOverlay()
	nm := notifications.NewManager()
	overlay.SetNotifManager(nm)
	overlay.showHistory = true

	rendered := overlay.RenderHistoryOverlay(80, 20)
	lines := nonBlankLines(rendered)
	if len(lines) < 3 {
		t.Fatal("overlay too short")
	}

	// Row 0 = top border ╭───╮, row 1 = header.
	headerLine := lines[1]
	if resetSpaceRE.MatchString(headerLine) {
		t.Errorf(
			"header row has unstyled space after reset — right-side background will be terminal default\n"+
				"  stripped: %q",
			stripANSI(headerLine),
		)
	}
}

// ─── Bug 5 – overlay notification row gap ────────────────────────────────────

// TestHistoryOverlayNotifRowNoTrailingUnstyled asserts that a notification row
// does NOT have a plain-space cell immediately after a reset code.
//
// Failing behavior: gapW = maxContent - Width(badge) - Width(contentPart) subtracts
// badge twice (maxContent already excludes badge), so the gap is Width(badge) cells
// too small.  The row is too narrow and JoinVertical pads the right side with bare
// terminal-default spaces.
func TestHistoryOverlayNotifRowNoTrailingUnstyled(t *testing.T) {
	overlay := NewUserNotificationOverlay()
	nm := notifications.NewManager()
	overlay.SetNotifManager(nm)
	nm.Add("test notification", notifications.SeverityInfo, 5*time.Second)
	overlay.showHistory = true

	rendered := overlay.RenderHistoryOverlay(80, 20)
	lines := nonBlankLines(rendered)
	// row 0=border, row 1=header, row 2=notification row, row 3=footer, row 4=border
	if len(lines) < 4 {
		t.Fatal("overlay too short — need at least border+header+notif+footer")
	}

	notifLine := lines[2]
	if resetSpaceRE.MatchString(notifLine) {
		t.Errorf("notification row has unstyled space after reset — gap too small\n"+
			"  stripped: %q", stripANSI(notifLine))
	}
}

// ─── Bug 6 – inter-element background holes in help widget output ─────────────

// TestShortHelpNoInterElementUnstyled asserts that the short-help line rendered
// by the status bar has no plain (terminal-default background) space immediately
// after a reset code.
//
// Failing behavior: the bubbles help widget emits \x1b[m (reset) followed by a
// bare space between styled key/desc elements (e.g. "q\x1b[m quit").  That bare
// space carries no background escape so it punches a hole in the StatusBg bar.
func TestShortHelpNoInterElementUnstyled(t *testing.T) {
	b := New()
	b.SetKeys(keys.DefaultKeyMap())
	b.SetWidth(120)

	lines := nonBlankLines(b.helpView.Content)
	if len(lines) == 0 {
		t.Fatal("short help produced no output")
	}
	for i, line := range lines {
		if resetSpaceRE.MatchString(line) {
			t.Errorf("short-help row %d has unstyled space after reset — "+
				"inter-element background hole\n  stripped: %q", i, stripANSI(line))
		}
	}
}

// TestFullHelpRowsNoInterElementUnstyled asserts that every row of the expanded
// full-help bar has no plain space immediately after a reset code.
//
// Same root cause as Bug 6 above but for the multi-row full-help layout.
func TestFullHelpRowsNoInterElementUnstyled(t *testing.T) {
	b := New()
	b.SetKeys(keys.DefaultKeyMap())
	b.help.ShowAll = true
	b.SetWidth(120)

	lines := nonBlankLines(b.helpView.Content)
	if len(lines) < 2 {
		t.Skip("full help produced only one line — need multi-row rendering")
	}
	for i, line := range lines {
		if resetSpaceRE.MatchString(line) {
			t.Errorf("full-help row %d has unstyled space after reset — "+
				"inter-element background hole\n  stripped: %q", i, stripANSI(line))
		}
	}
}

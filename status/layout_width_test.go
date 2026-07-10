package status

// Layout width tests: verify that the status bar and related renders never
// produce a line wider than the requested terminal width, and that emoji
// columns rendered through lipgloss.Width(n) produce exactly n cells.

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/jarvisfriends/snap/keys"
	"github.com/jarvisfriends/snap/rendercheck"
)

// TestStatusBarNeverOverflows asserts that the rendered status bar content does
// not exceed the requested width at a variety of terminal widths. The status
// bar is a single row and must always fit in exactly `width` cells.
func TestStatusBarNeverOverflows(t *testing.T) {
	for _, w := range rendercheck.StandardWidths {
		b := New()
		b.SetKeys(keys.DefaultKeyMap())
		b.SetWidth(w)
		for i, line := range strings.Split(b.helpView.Content, "\n") {
			got := lipgloss.Width(line)
			if got > w {
				t.Errorf("status bar at width=%d line %d overflows by %d: %q",
					w, i, got-w, rendercheck.StripANSI(line))
			}
		}
	}
}

// TestFullHelpBarNeverOverflows asserts that even in full-help (multi-row)
// mode the status bar does not overflow the terminal width.
func TestFullHelpBarNeverOverflows(t *testing.T) {
	for _, w := range rendercheck.StandardWidths {
		b := New()
		b.SetKeys(keys.DefaultKeyMap())
		b.help.ShowAll = true
		b.SetWidth(w)
		for i, line := range strings.Split(b.helpView.Content, "\n") {
			got := lipgloss.Width(line)
			if got > w {
				t.Errorf("full-help bar at width=%d line %d overflows by %d: %q",
					w, i, got-w, rendercheck.StripANSI(line))
			}
		}
	}
}

// TestDNSColumnEmojiWidths verifies that the symbols used in DNS status columns
// render to exactly dnsColW=3 cells when passed through lipgloss.Style.Width().
// This prevents emoji wide-character misalignment:
//   - "✔" (U+2714) is 1 cell — should be padded to 3
//   - "⚠" (U+26A0) is 1 cell — should be padded to 3
//   - "❌" (U+274C) is 2 cells — should be padded to 3
//   - "-"   is 1 cell — should be padded to 3
//
// If any symbol exceeds 3 cells, columns shift right and the table becomes
// unreadable.
func TestDNSColumnEmojiWidths(t *testing.T) {
	symbols := []string{"✔", "⚠", "❌", "-"}
	rendercheck.CheckEmojiColumnWidths(t, symbols, 3)
}

// TestStatusColumnEmojiWidths verifies the HTTP status symbols used in the
// STATUS column render within a 22-cell wide column.
func TestStatusColumnEmojiWidths(t *testing.T) {
	symbols := []string{"✔️ OK", "⚠️ HTTP ERROR", "❌ HTTP FAILED", "❌ DNS INVALID"}
	rendercheck.CheckEmojiColumnWidths(t, symbols, 22)
}

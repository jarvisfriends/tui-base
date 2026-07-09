package status

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestRenderStyledUnicodeContentFitsWidth pushes CJK, ZWJ-emoji, and flag
// sequences through the status bar composer (TS-2): rows must never exceed
// the terminal width regardless of grapheme complexity.
func TestRenderStyledUnicodeContentFitsWidth(t *testing.T) {
	t.Parallel()

	tortureLeft := "日本語ヘルプ 👨‍👩‍👧‍👦 navigate • 🇺🇸 select • Ｗｉｄｅ toggle"
	tortureRight := "堆 42MiB 👍"

	for _, w := range []int{30, 60, 90, 120} {
		row, _ := RenderStyled(w, tortureLeft, tortureRight, -1, true, 3)
		for i, line := range strings.Split(row, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("width=%d: row %d is %d cells; overflows: %q", w, i, got, line)
			}
		}
	}
}

// TestHistoryOverlayUnicodeContentFitsWidth pushes torture content through
// the notification history panel.
func TestHistoryOverlayUnicodeContentFitsWidth(t *testing.T) {
	t.Parallel()

	overlay := newHistoryOverlay(
		t,
		"деплой завершён 🚀 в 環境 production",
		"👨‍👩‍👧‍👦👨‍👩‍👧‍👦👨‍👩‍👧‍👦 long ZWJ content that must truncate cleanly at panel width",
	)
	for _, w := range []int{40, 80} {
		rendered := overlay.RenderHistoryOverlay(w, 20)
		for i, line := range strings.Split(rendered, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("maxW=%d: line %d is %d cells; overflows: %q", w, i, got, line)
			}
		}
	}
}

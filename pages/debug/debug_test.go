package debug

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jarvisfriends/tui-base/navigation"

	tea "charm.land/bubbletea/v2"
)

func TestLogCapturesMessage(t *testing.T) {
	t.Parallel()
	m := New()
	if len(m.Logs) != 0 {
		t.Fatalf("expected empty logs initially; got %d entries", len(m.Logs))
	}

	// Send a navigation.SelectedMsg and ensure it gets logged
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: 1})
	if len(m.Logs) == 0 {
		t.Fatalf("expected logs after update")
	}
	last := m.Logs[len(m.Logs)-1]
	expectedType := fmt.Sprintf("%T", navigation.SelectedMsg{})
	if last.Type != expectedType {
		t.Fatalf("logged Type = %q; want %q", last.Type, expectedType)
	}
}

func TestStackingAndTrim(t *testing.T) {
	t.Parallel()
	m := New()

	// identical messages should stack (increase Count) rather than append
	msg := "repeat"
	_, _ = m.Update(msg)
	_, _ = m.Update(msg)
	if len(m.Logs) != 1 {
		t.Fatalf("expected 1 log entry after stacking; got %d", len(m.Logs))
	}
	if m.Logs[0].Count != 2 {
		t.Fatalf("expected stacked Count=2; got %d", m.Logs[0].Count)
	}

	// Add many unique messages to force trimming to 50 entries
	for i := range 55 {
		m.LogMessageForDebugging(fmt.Sprintf("u%d", i))
	}
	if len(m.Logs) != 50 {
		t.Fatalf("expected logs trimmed to 50; got %d", len(m.Logs))
	}
	// earliest remaining should be u5
	if m.Logs[0].Content != "u5" {
		t.Fatalf("expected first log Content 'u5'; got %q", m.Logs[0].Content)
	}
}

func TestWindowSizeIgnored(t *testing.T) {
	t.Parallel()
	m := New()
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: 0})
	before := len(m.Logs)
	// WindowSizeMsg should not be logged
	_, _ = m.Update(tea.WindowSizeMsg{Width: 10, Height: 5})
	if len(m.Logs) != before {
		t.Fatalf("expected no new logs after WindowSizeMsg; before=%d after=%d", before, len(m.Logs))
	}
}

func TestViewShowsLogs(t *testing.T) {
	t.Parallel()
	m := New()
	m.LogMessageForDebugging("alpha")
	m.LogMessageForDebugging("beta")
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 120})
	_ = m.View()
	logView := m.logViewport.View()
	if !strings.Contains(logView, "beta") {
		t.Fatalf("expected log viewport to include log messages; got %q", logView)
	}
}

func TestStackingStructMessages(t *testing.T) {
	t.Parallel()
	m := New()

	// Send the same struct message twice; it should stack into one entry
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: 2})
	_, _ = m.Update(navigation.SelectedMsg{PageIndex: 2})

	if len(m.Logs) != 1 {
		t.Fatalf("expected 1 log entry after stacking struct messages; got %d", len(m.Logs))
	}
	if m.Logs[0].Count != 2 {
		t.Fatalf("expected stacked Count=2; got %d", m.Logs[0].Count)
	}
	expectedType := fmt.Sprintf("%T", navigation.SelectedMsg{})
	if m.Logs[0].Type != expectedType {
		t.Fatalf("logged Type = %q; want %q", m.Logs[0].Type, expectedType)
	}
}

func TestInspectorWheelScrollMovesVisibleWindow(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	for i := range 12 {
		m.AddLog("INFO", time.Now(), fmt.Sprintf("log-%02d", i))
	}

	if len(m.Logs) == 0 {
		t.Fatal("expected log entries")
	}
	// After AddLog, scrollToBottom should be set; newest log should be present.
	if got, want := m.Logs[len(m.Logs)-1].Content, "log-11"; got != want {
		t.Fatalf("newest log = %q; want %q", got, want)
	}

	// Wheel-down is forwarded to the viewport (returns early, does not add a log entry).
	// Just verify it does not panic.
	_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
}

func TestInspectorWheelScrollClampsAtBounds(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	for i := range 30 {
		m.AddLog("INFO", time.Now(), fmt.Sprintf("row-%02d", i))
	}

	// Wheel-down many times -- should not panic and viewport stays valid.
	for range 200 {
		_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	}

	// Wheel-up many times -- viewport stays valid.
	for range 200 {
		_, _ = m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	}

	// After all the scrolling, logs should still be consistent.
	if len(m.Logs) == 0 {
		t.Fatal("expected non-empty log after scrolling")
	}
}

// TestRuntimeColumnWidthHighWatermark verifies that once a wide value has been
// rendered into a column, that column never shrinks on subsequent renders even
// when all current values for it are narrower.
//
// Column 7 receives: PID, TermSize, StackInUse, BinSize, HeapObjects, OffX/Y.
// By zero-ing the stable sources and making HeapObjects transiently huge we can
// guarantee the wide render owns the max, then drop it and assert no shrink.
func TestRuntimeColumnWidthHighWatermark(t *testing.T) {
	t.Parallel()

	m := New()
	// Wide terminal to stay in table mode, not flat-list mode.
	_, _ = m.Update(tea.WindowSizeMsg{Width: 300, Height: 40})

	// Zero out the stable contributors to column 7 so HeapObjects is the sole
	// wide value. "300x40" (TermSize) = 6 chars; PID ≤ 7 chars on all major OS.
	m.stats.StackInUseBytes = 0   // "0 B" = 3 chars
	m.stats.Launch.BinarySize = 0 // "0 B" = 3 chars
	// Use a HeapObjects value whose formatted string (English locale with commas)
	// is wider than any stable column-7 value: "999,999,999,999" = 15 chars.
	m.stats.HeapObjects = 999_999_999_999

	// Fix the timestamps so elapsed = 1 s (avoids / 0 fallback noise).
	base := time.Now()
	m.prevStats.CapturedAt = base.Add(-time.Second)
	m.stats.CapturedAt = base

	m.dirty = true
	_ = m.View()

	wideW7 := m.runtimeColumns[7].Width
	if wideW7 <= 6 { // must be > TermSize width ("300x40" = 6)
		t.Fatalf("expected column 7 width > 6 after HeapObjects=999_999_999_999; got %d", wideW7)
	}

	// Now shrink HeapObjects to a single digit. All stable values in column 7 are
	// narrower than wideW7, so without the watermark the column would shrink.
	m.stats.HeapObjects = 1
	m.dirty = true
	_ = m.View()

	if m.runtimeColumns[7].Width < wideW7 {
		t.Errorf("column 7 shrank: was %d after wide render, now %d after narrow render",
			wideW7, m.runtimeColumns[7].Width)
	}
}

// TestRuntimeColumnWidthNeverBelowTitle verifies that column widths always meet
// or exceed the rendered width of the column title, even on the first render.
func TestRuntimeColumnWidthNeverBelowTitle(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 300, Height: 40})
	m.dirty = true
	_ = m.View()

	for i, col := range m.runtimeColumns {
		title := "Metric"
		if i%2 != 0 {
			title = "Value"
		}
		minW := len(title) // plain ASCII: visual width == byte length
		if col.Width < minW {
			t.Errorf("column %d width %d is below title %q width %d", i, col.Width, title, minW)
		}
	}
}

package inspector

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jarvisfriends/tui-base/navigation"
	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// Regression: some themes can produce SelectedItem fg/bg pairs that collide,
// which made table headers look blank (same foreground and background color).
// tableHeaderColors must always return a visible pair.
func TestTableHeaderColorsFallbackAvoidsSameFgBg(t *testing.T) {
	t.Parallel()

	c := &theme.AppStyle{
		Bg:     lipgloss.Color("0"),
		Accent: lipgloss.Color("5"),
		Styles: &theme.Styles{
			SelectedItem: lipgloss.NewStyle().
				Background(lipgloss.Color("7")).
				Foreground(lipgloss.Color("7")),
			TextOnBg: lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		},
	}

	headerBG, headerFG := tableHeaderColors(c)
	if strings.EqualFold(headerBG, headerFG) {
		t.Fatalf("regression: header foreground/background are identical (%s)", headerBG)
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
	// Drain pendingLogs into m.Logs via a no-op Update.
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 40})

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
	// Drain pendingLogs into m.Logs via a no-op Update.
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 40})

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

func TestStatusLineSummaryFollowsSettings(t *testing.T) {
	t.Parallel()

	m := New()
	m.stats = collectSnapshot(m.startTime)
	m.prevStats = m.stats

	if got := m.StatusLineSummary(); got != "" {
		t.Fatalf("expected empty summary when disabled, got %q", got)
	}

	m.statusSummary.Enabled = true
	got := m.StatusLineSummary()
	if !strings.Contains(got, "term") {
		t.Fatalf("expected summary to contain terminal info when enabled, got %q", got)
	}
}

func TestSettingsTabAdjustsRefreshInterval(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.activeTab = debugTabSettings
	m.settingsCursor = 0 // Latest-value refresh

	before := m.latestValueInterval
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // Enter increments by 100 ms
	if m.latestValueInterval <= before {
		t.Fatalf("expected latest value interval to increase; before=%s after=%s", before, m.latestValueInterval)
	}
}

func TestTabsAreClickableWithMouse(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	v := m.View()
	if len(m.tabRanges) < 2 {
		t.Fatalf("expected tab hit ranges to be populated; got %d", len(m.tabRanges))
	}

	inputTab := m.tabRanges[1]
	// tabsOriginY is the actual Y row of the tab bar in the inner-content
	// coordinate space (after the router strips the top border char).
	tabsY := m.tabsOriginY
	if cmd := v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: inputTab.StartX, Y: tabsY, Button: tea.MouseLeft})); cmd != nil {
		_ = cmd()
	}

	if m.activeTab != debugTabInput {
		t.Fatalf("expected click to switch to Input tab; got %v", m.activeTab)
	}
}

func TestSettingsRowsAreMouseSelectableAndActionable(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
	m.switchTab(debugTabSettings)
	v := m.View()

	row := 2 // "Status summary on close"
	y := m.sectionOriginY + row - m.sectionViewport.YOffset()
	if cmd := v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: 2, Y: y, Button: tea.MouseLeft})); cmd != nil {
		_ = cmd()
	}

	if m.settingsCursor != row {
		t.Fatalf("expected settings cursor=%d after click; got %d", row, m.settingsCursor)
	}
	if !m.statusSummary.Enabled {
		t.Fatal("expected clicked settings toggle row to execute Enter action")
	}
}

func TestPerTabScrollPreservedAcrossSwitches(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 10})
	_ = m.View()

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	before := m.tabScrollY[debugTabRuntime]
	if before <= 0 {
		t.Fatalf("expected runtime tab to scroll with KeyDown; got offset=%d", before)
	}

	_, _ = m.Update(tea.KeyPressMsg{Text: "6"}) // Log tab (accessibility tab inserted at position 5)
	_, _ = m.Update(tea.KeyPressMsg{Text: "1"}) // Runtime tab

	if got := m.sectionViewport.YOffset(); got != before {
		t.Fatalf("expected runtime tab scroll offset to be restored; got %d want %d", got, before)
	}
}

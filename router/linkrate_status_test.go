package router

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/pages/settings"
)

// TestLinkMeterDemandUnion proves the meter runs while either consumer wants
// it and only resets when demand goes zero -> nonzero.
func TestLinkMeterDemandUnion(t *testing.T) {
	l := newLinkRateMeter()

	l.setDemand(demandStatusBar, true)
	l.ObserveMsg(tea.KeyPressMsg{Text: "a"})
	if got := l.snapshot().txTotal; got != 1 {
		t.Fatalf("status-bar demand alone should collect, tx = %d", got)
	}

	// Second consumer joins: no reset.
	l.setDemand(demandInspectorTab, true)
	if got := l.snapshot().txTotal; got != 1 {
		t.Fatalf("joining consumer reset the session, tx = %d", got)
	}

	// One consumer leaves: still collecting.
	l.setDemand(demandInspectorTab, false)
	l.ObserveMsg(tea.KeyPressMsg{Text: "b"})
	if got := l.snapshot().txTotal; got != 2 {
		t.Fatalf("meter stopped while status bar still wants it, tx = %d", got)
	}

	// Last consumer leaves: idle; re-demand starts a fresh session.
	l.setDemand(demandStatusBar, false)
	l.ObserveMsg(tea.KeyPressMsg{Text: "c"})
	if got := l.snapshot().txTotal; got != 2 {
		t.Fatalf("idle meter kept collecting, tx = %d", got)
	}
	l.setDemand(demandStatusBar, true)
	if got := l.snapshot().txTotal; got != 0 {
		t.Fatalf("fresh demand did not reset the session, tx = %d", got)
	}
}

func TestLinkMeterStatusLine(t *testing.T) {
	l := newLinkRateMeter()
	if got := l.statusLine(); got != "" {
		t.Fatalf("idle meter statusLine = %q, want empty", got)
	}
	l.setDemand(demandStatusBar, true)
	line := l.statusLine()
	if !strings.Contains(line, "tx ") || !strings.Contains(line, "rx ") {
		t.Fatalf("statusLine = %q, want tx/rx rates", line)
	}
}

// TestStatusSummaryIncludesLinkRate drives the full path: with the summary
// and link part enabled, the inspector's status-bar summary carries the
// meter's tx/rx text — inspector closed the whole time.
func TestStatusSummaryIncludesLinkRate(t *testing.T) {
	settings.SetConfigDir(t.TempDir())
	m := NewWithOptions(Options{AppName: "Link Status Test"})
	t.Cleanup(m.Close)

	m.inspector.SetStatusSummaryEnabled(true)
	if !m.inspector.StatusSummaryLinkEnabled() {
		t.Fatal("ShowLink should default on once the summary is enabled")
	}

	// The router syncs status-bar demand in View; simulate that sync.
	m.linkMeter.setDemand(demandStatusBar, m.inspector.StatusSummaryLinkEnabled())

	summary := m.inspector.StatusLineSummary()
	if !strings.Contains(summary, "tx ") || !strings.Contains(summary, "rx ") {
		t.Fatalf("status summary missing link rates: %q", summary)
	}
}

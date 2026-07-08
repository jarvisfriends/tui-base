package router

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/theme"
)

func TestLinkMeterDisabledCostsNothing(t *testing.T) {
	l := newLinkRateMeter()
	l.ObserveMsg(tea.KeyPressMsg{Text: "a"})
	l.ObserveFrame("hello\nworld")
	s := l.snapshot()
	if s.txTotal != 0 || s.rxTotal != 0 {
		t.Fatalf("disabled meter accumulated tx=%d rx=%d", s.txTotal, s.rxTotal)
	}
}

func TestLinkMeterFirstFrameCountsInFull(t *testing.T) {
	l := newLinkRateMeter()
	l.start()
	frame := "line one\nline two"
	l.ObserveFrame(frame)
	s := l.snapshot()
	want := uint64(len("line one") + len("line two") + 2*6) // + per-line overhead
	if s.rxTotal != want {
		t.Fatalf("first frame rx = %d, want %d", s.rxTotal, want)
	}
}

func TestLinkMeterDiffOnlyChargesChangedLines(t *testing.T) {
	l := newLinkRateMeter()
	l.start()
	l.ObserveFrame("aaaa\nbbbb\ncccc")
	base := l.snapshot().rxTotal

	// Identical frame: free.
	l.ObserveFrame("aaaa\nbbbb\ncccc")
	if got := l.snapshot().rxTotal; got != base {
		t.Fatalf("identical frame charged %d bytes", got-base)
	}

	// One changed line: charged for that line only.
	l.ObserveFrame("aaaa\nBBBB\ncccc")
	got := l.snapshot().rxTotal - base
	want := uint64(len("BBBB") + 6)
	if got != want {
		t.Fatalf("single-line change rx = %d, want %d", got, want)
	}
}

func TestLinkMeterTxPricesInput(t *testing.T) {
	l := newLinkRateMeter()
	l.start()

	l.ObserveMsg(tea.KeyPressMsg{Text: "a"}) // 1 byte
	if got := l.snapshot().txTotal; got != 1 {
		t.Fatalf("printable key tx = %d, want 1", got)
	}

	// Mouse drag at (10, 5): ESC[<32;11;6M = 6 fixed + 2 + 2 + 1 digits.
	l.ObserveMsg(tea.MouseMotionMsg{X: 10, Y: 5})
	want := uint64(1 + 6 + 2 + 2 + 1)
	if got := l.snapshot().txTotal; got != want {
		t.Fatalf("after drag tx = %d, want %d", got, want)
	}

	// Ticks and internal messages are free.
	l.ObserveMsg(struct{ tea.Msg }{})
	if got := l.snapshot().txTotal; got != want {
		t.Fatalf("internal msg charged %d bytes", got-want)
	}
}

func TestLinkProviderRowsRender(t *testing.T) {
	l := newLinkRateMeter()
	p := &linkRateProvider{meter: l}
	p.Start()
	defer p.Stop()

	l.ObserveMsg(tea.KeyPressMsg{Text: "x"})
	l.ObserveFrame("frame")

	rows := p.BuildRows(theme.Active())
	if len(rows) == 0 {
		t.Fatal("BuildRows returned no rows")
	}
	joined := strings.Join(rows, "\n")
	for _, needle := range []string{"Tx", "Rx", "peak", "required", "totals"} {
		if !strings.Contains(joined, needle) {
			t.Errorf("rows missing %q:\n%s", needle, joined)
		}
	}
	if p.TabName() != "Link" {
		t.Errorf("TabName = %q", p.TabName())
	}
}

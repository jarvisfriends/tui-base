package router

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/pages/inspector"
	"github.com/jarvisfriends/tui-base/theme"
)

// linkRateMeter estimates the network data rate a tui-base app would need if
// it ran on a remote machine over a byte link (SSH, serial, telnet):
//
//   - Tx (client → app): input events priced as the ANSI byte sequences a
//     terminal actually sends — key presses as their text/escape sequences,
//     mouse events (including drag storms) as SGR mouse reports, pastes as
//     bracketed-paste payloads.
//   - Rx (app → client): rendered output priced as a line-diffing renderer
//     would transmit — the first frame in full, then only lines that changed
//     since the previous frame, plus a small per-line cursor-move overhead.
//
// Collection is off until Start (the inspector's provider lifecycle), so the
// meter adds no per-frame cost while the tab is not in use. All methods are
// safe for concurrent use.
type linkRateMeter struct {
	mu sync.Mutex
	// Two independent consumers can demand collection: the inspector's Link
	// tab (provider lifecycle) and the status-bar summary ("Include link
	// rate", which works with the inspector closed). The meter runs while
	// either wants it; the session resets when demand goes zero -> nonzero.
	wantTab    bool
	wantStatus bool

	prevLines []string // previous frame, split into lines

	txTotal, rxTotal uint64
	txPeak, rxPeak   uint64 // highest single-second bucket seen

	// bucketSec is the unix second the current bucket accumulates into;
	// buckets holds up to 60 finished one-second samples, newest last.
	bucketSec    int64
	bucketTx     uint64
	bucketRx     uint64
	buckets      []rateSample
	sessionStart time.Time
}

type rateSample struct {
	sec    int64
	tx, rx uint64
}

const linkRateWindow = 60 // seconds of history kept

func newLinkRateMeter() *linkRateMeter { return &linkRateMeter{} }

// linkDemand names one consumer of the meter for setDemand.
type linkDemand int

const (
	demandInspectorTab linkDemand = iota
	demandStatusBar
)

// setDemand records that a consumer wants (or no longer wants) collection.
// The first active demand starts a fresh session; collection stops when the
// last demand is withdrawn.
func (l *linkRateMeter) setDemand(who linkDemand, want bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	before := l.wantTab || l.wantStatus
	switch who {
	case demandInspectorTab:
		l.wantTab = want
	case demandStatusBar:
		l.wantStatus = want
	}
	after := l.wantTab || l.wantStatus
	switch {
	case !before && after:
		l.resetLocked()
	case before && !after:
		l.prevLines = nil
	}
}

// resetLocked starts a fresh measuring session. Callers hold l.mu.
func (l *linkRateMeter) resetLocked() {
	l.prevLines = nil // next frame counts in full — a client just connected
	l.txTotal, l.rxTotal = 0, 0
	l.txPeak, l.rxPeak = 0, 0
	l.buckets = nil
	l.bucketSec, l.bucketTx, l.bucketRx = 0, 0, 0
	l.sessionStart = time.Now()
}

// enabledLocked reports whether any consumer wants collection. Callers hold l.mu.
func (l *linkRateMeter) enabledLocked() bool { return l.wantTab || l.wantStatus }

// ObserveMsg prices an input message in client→app wire bytes.
func (l *linkRateMeter) ObserveMsg(msg tea.Msg) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.enabledLocked() {
		return
	}
	n := estimateTxBytes(msg)
	if n <= 0 {
		return
	}
	nb := uint64(n)
	l.roll(time.Now().Unix())
	l.bucketTx += nb
	l.txTotal += nb
}

// ObserveFrame prices a rendered frame in app→client wire bytes using a
// line diff against the previous frame.
func (l *linkRateMeter) ObserveFrame(frame string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.enabledLocked() {
		return
	}
	lines := strings.Split(frame, "\n")
	n := diffBytes(l.prevLines, lines)
	l.prevLines = lines
	if n <= 0 {
		return
	}
	nb := uint64(n)
	l.roll(time.Now().Unix())
	l.bucketRx += nb
	l.rxTotal += nb
}

// roll finishes the current one-second bucket when the clock moved past it.
// Callers hold l.mu.
func (l *linkRateMeter) roll(nowSec int64) {
	if l.bucketSec == nowSec {
		return
	}
	if l.bucketSec != 0 {
		l.buckets = append(l.buckets, rateSample{sec: l.bucketSec, tx: l.bucketTx, rx: l.bucketRx})
		if l.bucketTx > l.txPeak {
			l.txPeak = l.bucketTx
		}
		if l.bucketRx > l.rxPeak {
			l.rxPeak = l.bucketRx
		}
		if len(l.buckets) > linkRateWindow {
			l.buckets = l.buckets[len(l.buckets)-linkRateWindow:]
		}
	}
	l.bucketSec = nowSec
	l.bucketTx, l.bucketRx = 0, 0
}

// linkRateStats is a consistent snapshot for rendering.
type linkRateStats struct {
	txLast, rxLast   uint64 // most recent finished second
	tx5, rx5         uint64 // bytes/sec averaged over the last 5 finished seconds
	tx60, rx60       uint64 // bytes/sec averaged over the whole kept window
	txPeak, rxPeak   uint64
	txTotal, rxTotal uint64
	elapsed          time.Duration
}

func (l *linkRateMeter) snapshot() linkRateStats {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.roll(time.Now().Unix())

	s := linkRateStats{
		txPeak: l.txPeak, rxPeak: l.rxPeak,
		txTotal: l.txTotal, rxTotal: l.rxTotal,
	}
	if !l.sessionStart.IsZero() {
		s.elapsed = time.Since(l.sessionStart)
	}
	if n := len(l.buckets); n > 0 {
		s.txLast = l.buckets[n-1].tx
		s.rxLast = l.buckets[n-1].rx
		s.tx5, s.rx5 = avgRates(l.buckets, 5)
		s.tx60, s.rx60 = avgRates(l.buckets, linkRateWindow)
	}
	return s
}

func avgRates(buckets []rateSample, lastN int) (tx, rx uint64) {
	if len(buckets) == 0 {
		return 0, 0
	}
	if lastN > len(buckets) {
		lastN = len(buckets)
	}
	// Average over the covered wall-clock span (idle seconds have no bucket
	// but still count as time), bounded below by the sample count.
	window := buckets[len(buckets)-lastN:]
	var sumTx, sumRx uint64
	for _, b := range window {
		sumTx += b.tx
		sumRx += b.rx
	}
	span := max(window[len(window)-1].sec-window[0].sec+1, int64(lastN))
	spanU := uint64(span) //nolint:gosec // span >= lastN >= 1
	return sumTx / spanU, sumRx / spanU
}

// statusLine returns the compact status-bar form (5-second averages), or ""
// while the meter is idle.
func (l *linkRateMeter) statusLine() string {
	l.mu.Lock()
	idle := !l.enabledLocked()
	l.mu.Unlock()
	if idle {
		return ""
	}
	s := l.snapshot()
	return fmt.Sprintf("tx %s rx %s", rate(s.tx5), rate(s.rx5))
}

// ── wire-cost estimation ─────────────────────────────────────────────────────

// estimateTxBytes prices one input message as terminal-to-app wire bytes.
// Non-input messages (ticks, command results) cost nothing — they exist only
// inside the process.
func estimateTxBytes(msg tea.Msg) int {
	switch ev := msg.(type) {
	case tea.KeyPressMsg:
		return keyWireBytes(tea.Key(ev))
	case tea.KeyReleaseMsg:
		// Only reported under enhanced keyboard protocols; price like a CSI-u
		// release sequence.
		return 8
	case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg, tea.MouseWheelMsg:
		mm, ok := msg.(tea.MouseMsg)
		if !ok {
			return 0
		}
		pos := mm.Mouse()
		// SGR report: ESC [ < Cb ; Cx ; Cy M  → 6 fixed bytes + the digits.
		return 6 + digits(32) + digits(pos.X+1) + digits(pos.Y+1)
	case tea.PasteMsg:
		// Bracketed paste: ESC[200~ payload ESC[201~.
		return len([]byte(ev.Content)) + 12 // wire bytes, not display width
	default:
		return 0
	}
}

// keyWireBytes prices a key press: printable text costs its UTF-8 length,
// special keys cost a typical CSI sequence, and modifiers extend it.
func keyWireBytes(k tea.Key) int {
	if k.Text != "" {
		return len([]byte(k.Text)) // wire bytes, not display width
	}
	n := 3 // ESC [ X — typical special-key sequence
	if k.Mod != 0 {
		n += 3 // ;1+m modifier parameter
	}
	return n
}

// diffBytes prices a frame as a line-diffing renderer would: changed lines
// are retransmitted in full plus a per-line cursor-move cost; unchanged lines
// are free. The first frame (prev == nil) counts in full.
func diffBytes(prev, cur []string) int {
	const perLineOverhead = 6 // CSI row;colH cursor move (typical)
	total := 0
	for i, line := range cur {
		if i < len(prev) && prev[i] == line {
			continue
		}
		// len([]byte(...)) — transfer pricing in wire bytes, deliberately NOT a
		// display width; the explicit conversion is free and states the intent.
		total += len([]byte(line)) + perLineOverhead
	}
	// Lines removed when the frame shrank must be cleared remotely.
	if len(prev) > len(cur) {
		total += (len(prev) - len(cur)) * perLineOverhead
	}
	return total
}

// ── inspector provider ───────────────────────────────────────────────────────

// linkRateProvider exposes the meter as a built-in inspector tab ("Link").
type linkRateProvider struct {
	meter *linkRateMeter
}

func (p *linkRateProvider) TabName() string                { return "Link" }
func (p *linkRateProvider) RefreshInterval() time.Duration { return time.Second }

func (p *linkRateProvider) Start() { p.meter.setDemand(demandInspectorTab, true) }

func (p *linkRateProvider) Stop() { p.meter.setDemand(demandInspectorTab, false) }
func (p *linkRateProvider) BuildRows(c *theme.AppStyle) []string {
	s := p.meter.snapshot()
	label := c.Styles.TextOnBg.Bold(true)
	value := c.Styles.TextOnBg

	row := func(name string, tx, rx uint64) string {
		return label.Render(fmt.Sprintf("%-12s", name)) +
			value.Render(fmt.Sprintf("Tx %12s   Rx %12s", rate(tx), rate(rx)))
	}
	// The link needs to sustain the peak of Tx+Rx (half-duplex worst case).
	required := s.txPeak + s.rxPeak

	return []string{
		label.Render("Estimated remote-link usage (as if running over SSH/serial)"),
		"",
		row("last second", s.txLast, s.rxLast),
		row("5s average", s.tx5, s.rx5),
		row("60s average", s.tx60, s.rx60),
		row("peak", s.txPeak, s.rxPeak),
		"",
		label.Render(fmt.Sprintf("%-12s", "required")) +
			value.Render(
				fmt.Sprintf(
					"≈ %s (%s) to keep up with the observed peak",
					rate(required),
					bitRate(required),
				),
			),
		"",
		value.Render(fmt.Sprintf(
			"totals   Tx %s   Rx %s   over %s",
			byteCount(s.txTotal), byteCount(s.rxTotal), s.elapsed.Truncate(time.Second),
		)),
		"",
		c.Styles.TextOnBg.Faint(true).Render(
			"Tx prices key/mouse/paste input as ANSI wire bytes (drags = SGR reports);",
		),
		c.Styles.TextOnBg.Faint(true).Render(
			"Rx prices rendered frames as a line-diff renderer would transmit them.",
		),
	}
}

func rate(bytesPerSec uint64) string { return byteCount(bytesPerSec) + "/s" }

func bitRate(bytesPerSec uint64) string {
	bits := float64(bytesPerSec) * 8
	switch {
	case bits >= 1e6:
		return fmt.Sprintf("%.1f Mbit/s", bits/1e6)
	case bits >= 1e3:
		return fmt.Sprintf("%.1f kbit/s", bits/1e3)
	default:
		return fmt.Sprintf("%.0f bit/s", bits)
	}
}

func byteCount(b uint64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

var _ inspector.MetricsProvider = (*linkRateProvider)(nil)

func digits(n int) int {
	if n < 0 {
		n = -n
	}
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}

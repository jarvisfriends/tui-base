package notifications

import (
	"testing"
	"time"
)

// TestOrderingDeterministicOnEqualTimestamps is the regression test for the
// cross-platform golden flake: on Windows two back-to-back Adds often share a
// wall-clock timestamp (coarse clock) while Linux resolves them apart, so a
// pure CreatedAt sort produced different row orders per OS. With the ID
// tie-break, newest wins everywhere.
func TestOrderingDeterministicOnEqualTimestamps(t *testing.T) {
	t.Parallel()

	m := NewManager()
	m.Add("first", SeverityInfo, time.Hour)
	m.Add("second", SeverityWarning, time.Hour)

	// Force the coarse-clock case explicitly: identical CreatedAt on both.
	m.mu.Lock()
	ts := time.Now()
	for i := range m.items {
		m.items[i].CreatedAt = ts
	}
	m.sortUnsafe()
	m.mu.Unlock()

	active := m.Active()
	if len(active) != 2 {
		t.Fatalf("active = %d items; want 2", len(active))
	}
	if active[0].Content != "second" || active[1].Content != "first" {
		t.Fatalf("equal-timestamp order = [%s, %s]; want newest (higher ID) first",
			active[0].Content, active[1].Content)
	}

	// And with distinct timestamps, newest-first still holds.
	m2 := NewManager()
	m2.Add("old", SeverityInfo, time.Hour)
	time.Sleep(2 * time.Millisecond)
	m2.Add("new", SeverityInfo, time.Hour)
	if got := m2.Active()[0].Content; got != "new" {
		t.Fatalf("distinct-timestamp order starts with %q; want \"new\"", got)
	}
}

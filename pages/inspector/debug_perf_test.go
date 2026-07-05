package inspector

import (
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

type perfMetrics struct {
	iterations   int
	seconds      float64
	allocsPerSec float64
	bytesPerSec  float64
	gcPerSec     float64
}

func runScenario(duration time.Duration, hz int, step func(i int)) perfMetrics {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	start := time.Now()
	deadline := start.Add(duration)
	period := time.Second / time.Duration(hz)
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	i := 0
	for now := range ticker.C {
		if now.After(deadline) {
			break
		}
		step(i)
		i++
	}
	elapsed := time.Since(start)
	elapsedSec := elapsed.Seconds()
	if elapsedSec <= 0 {
		elapsedSec = 1
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	mallocs := float64(after.Mallocs - before.Mallocs)
	bytes := float64(after.TotalAlloc - before.TotalAlloc)
	gc := float64(after.NumGC - before.NumGC)

	return perfMetrics{
		iterations:   i,
		seconds:      elapsedSec,
		allocsPerSec: mallocs / elapsedSec,
		bytesPerSec:  bytes / elapsedSec,
		gcPerSec:     gc / elapsedSec,
	}
}

func parseFloatEnv(name string) (float64, bool) {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func assertMaxMetric(t *testing.T, metricName string, got float64, envName string) {
	t.Helper()
	if limit, ok := parseFloatEnv(envName); ok && got > limit {
		t.Fatalf("%s exceeded: got %.2f, limit %.2f (from %s)", metricName, got, limit, envName)
	}
}

func TestInspectorPerfReport(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 48})

	idle := runScenario(3*time.Second, 120, func(int) {
		_ = m.View().Content
	})

	mouse := runScenario(3*time.Second, 120, func(i int) {
		x := i % 140
		y := (i / 140) % 48
		_, _ = m.Update(tea.MouseMotionMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
		_ = m.View().Content
	})

	t.Logf("inspector idle: iterations=%d elapsed=%.2fs allocs/sec=%.2f bytes/sec=%.2f gc/sec=%.2f",
		idle.iterations, idle.seconds, idle.allocsPerSec, idle.bytesPerSec, idle.gcPerSec)
	t.Logf(
		"inspector mouse: iterations=%d elapsed=%.2fs allocs/sec=%.2f bytes/sec=%.2f gc/sec=%.2f",
		mouse.iterations,
		mouse.seconds,
		mouse.allocsPerSec,
		mouse.bytesPerSec,
		mouse.gcPerSec,
	)

	assertMaxMetric(t, "idle allocs/sec", idle.allocsPerSec, "TUIBASE_MAX_IDLE_ALLOCS_PER_SEC")
	assertMaxMetric(t, "idle gc/sec", idle.gcPerSec, "TUIBASE_MAX_IDLE_GC_PER_SEC")
	assertMaxMetric(t, "mouse allocs/sec", mouse.allocsPerSec, "TUIBASE_MAX_MOUSE_ALLOCS_PER_SEC")
	assertMaxMetric(t, "mouse gc/sec", mouse.gcPerSec, "TUIBASE_MAX_MOUSE_GC_PER_SEC")
}

func BenchmarkInspectorIdle(b *testing.B) {
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 48})

	b.ReportAllocs()

	for b.Loop() {
		_ = m.View().Content
	}
}

func BenchmarkInspectorMouseMotion(b *testing.B) {
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 48})

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		x := i % 140
		y := (i / 140) % 48
		_, _ = m.Update(tea.MouseMotionMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
		_ = m.View().Content
	}
}

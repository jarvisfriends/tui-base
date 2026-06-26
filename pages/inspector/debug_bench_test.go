package inspector

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func BenchmarkDebugViewWithLogs(b *testing.B) {
	m := New()
	// Pre-populate logs to simulate heavy state
	for i := range 200 {
		m.LogMessageForDebugging(fmt.Sprintf("message %d", i))
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	b.ReportAllocs()

	for b.Loop() {
		_ = m.View().Content
	}
}

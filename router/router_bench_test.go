package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/keys"

	tea "charm.land/bubbletea/v2"
)

func BenchmarkRouterViewWithSidebar(b *testing.B) {
	m := New()
	m.navigationVisible = true
	m.keys = keys.DefaultKeyMap()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View().Content
	}
}

func BenchmarkRouterViewNoSidebar(b *testing.B) {
	m := New()
	m.navigationVisible = false
	m.keys = keys.DefaultKeyMap()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View().Content
	}
}

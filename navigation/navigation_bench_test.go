package navigation

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func BenchmarkView(b *testing.B) {
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
	b.ReportAllocs()

	for b.Loop() {
		_ = m.View().Content
	}
}

func BenchmarkUpdateKeys(b *testing.B) {
	m := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		if i%2 == 0 {
			_, _ = m.Update(tea.KeyPressMsg{Text: "j"})
		} else {
			_, _ = m.Update(tea.KeyPressMsg{Text: "k"})
		}
	}
}

func BenchmarkMouseMapping(b *testing.B) {
	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
	v := m.View()
	lines := len(strings.Split(v.Content, "\n"))
	if lines == 0 {
		b.Skip("no lines to test")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		y := i % lines
		cmd := v.OnMouse(tea.MouseReleaseMsg{X: 0, Y: y, Button: tea.MouseLeft})
		if cmd != nil {
			_ = cmd()
		}
	}
}

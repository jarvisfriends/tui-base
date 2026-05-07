package navigation

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func FuzzMouseY(f *testing.F) {
	f.Add(0)
	f.Add(2)
	f.Fuzz(func(t *testing.T, y int) {
		m := New()
		_, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
		v := m.View()
		cmd := v.OnMouse(tea.MouseReleaseMsg{X: 0, Y: y, Button: tea.MouseLeft})
		if cmd != nil {
			_ = cmd()
		}
	})
}

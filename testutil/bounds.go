package testutil

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// AssertBounds validates that a model's View() respects the requested terminal dimensions.
func AssertBounds(t *testing.T, m tea.Model, width, height int) {
	t.Helper()

	// 1. Send resize message
	m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: height})

	// 2. Render view
	v := m.View()
	content := v.Content

	// 3. Verify total height
	actualHeight := lipgloss.Height(content)
	if actualHeight > height {
		t.Errorf("View height overflow: got %d, max allowed %d\nContent:\n%s", actualHeight, height, content)
	}

	// 4. Verify width of every line
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		actualWidth := lipgloss.Width(line)
		if actualWidth > width {
			t.Errorf("Line %d width overflow: got %d, max allowed %d\nLine content: %q", i, actualWidth, width, line)
		}
	}
}

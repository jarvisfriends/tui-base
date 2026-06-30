package testutil

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type mockModel struct {
	content string
}

func (m mockModel) Init() tea.Cmd { return nil }
func (m mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		if sz.Width < 10 {
			m.content = "123456789012345"
		}
	}
	return m, nil
}

func (m mockModel) View() tea.View {
	return tea.NewView(m.content)
}

const testShortContent = "short"

func TestCheckNoLineOverflow(t *testing.T) {
	m := mockModel{content: testShortContent}
	// should pass
	CheckNoLineOverflow(t, m, []int{20, 30})
}

func TestCheckNoLineOverflowAtSizes(t *testing.T) {
	m := mockModel{content: testShortContent}
	CheckNoLineOverflowAtSizes(t, m)
}

func TestCheckNoBorderOverflow(t *testing.T) {
	m := mockModel{content: testShortContent}
	CheckNoBorderOverflow(t, m, 40, 24)
}

func TestCheckEmojiColumnWidths(t *testing.T) {
	CheckEmojiColumnWidths(t, []string{"😀", "👍"}, 2)
}

func TestCheckNoBackgroundHoles(t *testing.T) {
	bg := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	style := lipgloss.NewStyle().Background(lipgloss.Color("#ff0000"))
	rendered := style.Render("hello")

	// Should pass
	CheckNoBackgroundHoles(t, rendered, bg, "test-bg")
}

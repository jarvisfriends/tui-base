package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// countingPage counts every non-key/mouse message it receives through Update.
type countingPage struct {
	msgs *int
}

func (p countingPage) Init() tea.Cmd { return nil }

func (p countingPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg, tea.WindowSizeMsg:
		return p, nil
	}
	*p.msgs++
	return p, nil
}

func (p countingPage) View() tea.View { return tea.NewView("counting") }

// alphaTickMsg is a targeted background message addressed to the Alpha page.
type alphaTickMsg struct{}

func (alphaTickMsg) TargetPage() string { return "Alpha" }

// plainTickMsg is an ordinary broadcast background message.
type plainTickMsg struct{}

// TestTargetedMsgWakesOnlyItsPage verifies the fast path: a background
// message implementing TargetedMsg is delivered only to the page it names,
// while ordinary messages keep the broadcast behavior.
func TestTargetedMsgWakesOnlyItsPage(t *testing.T) {
	t.Parallel()

	alphaCount, betaCount := 0, 0
	m := NewWithRegisteredPages([]RegisteredPage{
		{Title: "Alpha", Model: countingPage{msgs: &alphaCount}},
		{Title: "Beta", Model: countingPage{msgs: &betaCount}},
	})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	alphaCount, betaCount = 0, 0 // ignore construction-time traffic

	// Targeted at Alpha (the active page): only Alpha sees it.
	_, _ = m.Update(alphaTickMsg{})
	if alphaCount != 1 || betaCount != 0 {
		t.Fatalf("targeted msg: alpha=%d beta=%d; want 1/0", alphaCount, betaCount)
	}

	// Ordinary background messages still broadcast to every page.
	_, _ = m.Update(plainTickMsg{})
	if alphaCount != 2 || betaCount != 1 {
		t.Fatalf("broadcast msg: alpha=%d beta=%d; want 2/1", alphaCount, betaCount)
	}

	// Targeted at an inactive page: it is delivered there and the active
	// page is not woken.
	_, _ = m.Update(betaTargetedMsg{})
	if alphaCount != 2 || betaCount != 2 {
		t.Fatalf("targeted-at-inactive: alpha=%d beta=%d; want 2/2", alphaCount, betaCount)
	}

	// A target that matches no page wakes nobody (typo safety).
	_, _ = m.Update(ghostTargetedMsg{})
	if alphaCount != 2 || betaCount != 2 {
		t.Fatalf("unknown target: alpha=%d beta=%d; want 2/2", alphaCount, betaCount)
	}
}

type betaTargetedMsg struct{}

func (betaTargetedMsg) TargetPage() string { return "Beta" }

type ghostTargetedMsg struct{}

func (ghostTargetedMsg) TargetPage() string { return "NoSuchPage" }

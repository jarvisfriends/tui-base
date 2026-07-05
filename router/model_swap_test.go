package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// swapMsg triggers swapModel to return a brand-new model value from Update.
type swapMsg struct{}

// swapModel is a value-semantics tea.Model: Update returns a *new* model
// rather than mutating a pointer receiver. The router must store the returned
// model or the swap is silently lost (regression test for B-1).
type swapModel struct {
	generation int
}

func (s swapModel) Init() tea.Cmd { return nil }

func (s swapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(swapMsg); ok {
		return swapModel{generation: s.generation + 1}, nil
	}
	return s, nil
}

func (s swapModel) View() tea.View { return tea.View{} }

// swapGeneration finds the swapModel in the router's page list and returns its
// generation counter.
func swapGeneration(t *testing.T, m *RouterModel) int {
	t.Helper()
	for _, p := range m.pages {
		if sm, ok := p.(swapModel); ok {
			return sm.generation
		}
	}
	t.Fatal("swapModel page not found in router pages")
	return -1
}

// TestRouterKeepsReturnedModel verifies the router stores the model returned
// by a page's Update instead of discarding it (B-1). Both the background
// fan-out path (inactive page) and the active-page path must preserve swaps.
func TestRouterKeepsReturnedModel(t *testing.T) {
	t.Parallel()
	m := NewWithOptions(Options{
		ExtraPages: []RegisteredPage{{Title: "Swap", Model: swapModel{}}},
	})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if got := swapGeneration(t, m); got != 0 {
		t.Fatalf("initial generation = %d; want 0", got)
	}

	// Inactive-page fan-out: swap page is not active (home is), but non-key,
	// non-mouse messages are forwarded to every page.
	_, _ = m.Update(swapMsg{})
	if got := swapGeneration(t, m); got != 1 {
		t.Fatalf("generation after fan-out swap = %d; want 1 (returned model was discarded)", got)
	}

	// Active-page path: activate the swap page, then send the message again.
	idx := -1
	for i, p := range m.pages {
		if _, ok := p.(swapModel); ok {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("swap page index not found")
	}
	m.nav.SetActiveIndex(idx)
	_, _ = m.Update(swapMsg{})
	if got := swapGeneration(t, m); got != 2 {
		t.Fatalf("generation after active swap = %d; want 2 (returned model was discarded)", got)
	}
}

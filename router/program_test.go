package router

import (
	"context"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// nullModel is a minimal tea.Model for program-lifecycle tests.
type nullModel struct{}

func (nullModel) Init() tea.Cmd                       { return nil }
func (nullModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return nullModel{}, nil }
func (nullModel) View() tea.View                      { return tea.NewView("") }

// TestNewProgramWithContextCancelsRun verifies the graceful-shutdown path:
// a canceled context terminates Run instead of blocking forever.
func TestNewProgramWithContextCancelsRun(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled: Run must return promptly

	p := NewProgramWithContext(
		ctx,
		nullModel{},
		ColorProfileEnvVar,
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error from a canceled context; got nil")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("program did not shut down after context cancellation")
	}
}

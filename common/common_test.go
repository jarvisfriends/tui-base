package common

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type stubComponent struct{}

func (stubComponent) Init() tea.Cmd                       { return nil }
func (stubComponent) Update(tea.Msg) (tea.Model, tea.Cmd) { return stubComponent{}, nil }
func (stubComponent) View() tea.View                      { return tea.NewView("component") }
func (stubComponent) ShortHelp() []key.Binding            { return nil }
func (stubComponent) FullHelp() [][]key.Binding           { return nil }
func (stubComponent) SetSize(width, height int)           {}

func TestInterfaceAssertions(t *testing.T) {
	t.Parallel()

	var component Component = stubComponent{}
	_ = component

	if AppVersion() == "" {
		t.Fatal("expected AppVersion to return a non-empty value")
	}
}

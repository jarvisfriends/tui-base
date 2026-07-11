package tuibase

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/gate"
)

// stubDebugModel is a minimal tea.Model for WithDebugOverlay tests.
type stubDebugModel struct{}

func (stubDebugModel) Init() tea.Cmd                         { return nil }
func (s stubDebugModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (stubDebugModel) View() tea.View                        { return tea.NewView("debug") }

// TestApplyOptionsLayersOnStruct asserts With* options apply on top of the
// struct (Q-21: both coexist), appends append, and nil options are skipped.
func TestApplyOptionsLayersOnStruct(t *testing.T) {
	g := gate.NewGateRegistry()
	base := Options{
		AppName:    "Struct Name",
		ExtraPages: []RegisteredPage{{Title: "From Struct"}},
	}
	got := applyOptions(base, []Option{
		WithAppName("Option Name"), // options win over the struct
		WithPages(RegisteredPage{Title: "From Option"}),
		WithGates(g),
		WithWatchSettingsFile(),
		WithoutTerminalRelaunch(),
		WithDebugOverlay(stubDebugModel{}),
		nil, // must be tolerated
	})

	if got.AppName != "Option Name" {
		t.Errorf("AppName = %q; want the option to win", got.AppName)
	}
	if len(got.ExtraPages) != 2 || got.ExtraPages[0].Title != "From Struct" || got.ExtraPages[1].Title != "From Option" {
		t.Errorf("ExtraPages = %+v; want struct page then appended option page", got.ExtraPages)
	}
	if got.Gates != g {
		t.Error("WithGates did not set the registry")
	}
	if !got.WatchSettingsFile {
		t.Error("WithWatchSettingsFile did not set the flag")
	}
	if !got.DisableTerminalRelaunch {
		t.Error("WithoutTerminalRelaunch did not set the flag")
	}
	if got.DebugOverlay == nil {
		t.Error("WithDebugOverlay did not store the model")
	}
}

// TestApplyOptionsDoesNotMutateCallerSlice guards the append semantics: two
// routers built from the same base Options must not share appended pages.
func TestApplyOptionsDoesNotMutateBase(t *testing.T) {
	base := Options{AppName: "Base"}
	_ = applyOptions(base, []Option{WithPages(RegisteredPage{Title: "A"})})
	if len(base.ExtraPages) != 0 {
		t.Fatalf("applyOptions mutated the caller's Options: %+v", base.ExtraPages)
	}
}

package common

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type stubFocusable struct {
	name       string
	focused    bool
	focusCalls int
	blurCalls  int
	updates    int
	view       tea.View
}

func (s *stubFocusable) Focus() tea.Cmd {
	s.focused = true
	s.focusCalls++
	return nil
}

func (s *stubFocusable) Blur() {
	s.focused = false
	s.blurCalls++
}

func (s *stubFocusable) Update(tea.Msg) (tea.Model, tea.Cmd) {
	s.updates++
	return s, nil
}

func (s *stubFocusable) Init() tea.Cmd { return nil }

func (s *stubFocusable) View() tea.View {
	if s.view.Content == "" {
		s.view = tea.NewView(s.name)
	}
	return s.view
}

type stubComponent struct{}

func (stubComponent) Init() tea.Cmd                       { return nil }
func (stubComponent) Update(tea.Msg) (tea.Model, tea.Cmd) { return stubComponent{}, nil }
func (stubComponent) View() tea.View                      { return tea.NewView("component") }
func (stubComponent) ShortHelp() []key.Binding            { return nil }
func (stubComponent) FullHelp() [][]key.Binding           { return nil }
func (stubComponent) SetSize(width, height int)           {}

func TestKnownFocusableCyclesFocus(t *testing.T) {
	t.Parallel()

	first := &stubFocusable{name: "first"}
	second := &stubFocusable{name: "second"}
	k := &KnownFocusable{}
	k.Add(first)
	k.Add(second)

	_ = k.Init()
	if !first.focused {
		t.Fatal("expected first focusable to be focused after Init")
	}

	_ = k.Next()
	if !second.focused {
		t.Fatal("expected second focusable to be focused after Next")
	}
	if first.blurCalls != 1 {
		t.Fatalf("first blur calls = %d; want 1", first.blurCalls)
	}

	_ = k.Prev()
	if !first.focused {
		t.Fatal("expected first focusable to be focused after Prev")
	}
}

func TestKnownFocusableRoutesUpdateToFocusedChild(t *testing.T) {
	t.Parallel()

	first := &stubFocusable{name: "first"}
	second := &stubFocusable{name: "second"}
	k := &KnownFocusable{}
	k.Add(first)
	k.Add(second)
	_ = k.Init()
	_ = k.Next()

	_, _ = k.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if second.updates != 1 {
		t.Fatalf("second updates = %d; want 1", second.updates)
	}
	if first.updates != 0 {
		t.Fatalf("first updates = %d; want 0", first.updates)
	}
	if got := k.View().Content; got != "second" {
		t.Fatalf("View content = %q; want %q", got, "second")
	}
}

func TestInterfaceAssertions(t *testing.T) {
	t.Parallel()

	var focusable Focusable = &stubFocusable{name: "focusable"}
	if focusable == nil {
		t.Fatal("expected Focusable assignment to succeed")
	}

	var component Component = stubComponent{}
	if component == nil {
		t.Fatal("expected Component assignment to succeed")
	}

	if AppVersion() == "" {
		t.Fatal("expected AppVersion to return a non-empty value")
	}
	if normalizeVersion("(devel)") != "development" {
		t.Fatal("expected normalizeVersion to normalize devel builds")
	}
}

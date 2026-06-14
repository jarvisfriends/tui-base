package navigation

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type Page struct {
	ID    string
	Title string
}

// Side indicates where a navigation component docks relative to the page content.
type Side int

const (
	// DockLeft reserves columns on the left (e.g. a sidebar); the page renders
	// to its right via JoinHorizontal.
	DockLeft Side = iota
	// DockTop reserves rows at the top (e.g. a tab bar); the page renders below
	// it via JoinVertical.
	DockTop
)

type Navigator interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() tea.View
	Width() int
	Height() int
	// Dock reports which edge the navigator occupies, letting the router lay it
	// out and route input without type-asserting concrete navigator types.
	Dock() Side
	GetPages() []Page
	SetPages([]Page)
	SetActiveIndex(int)
	GetActiveIndex() int
}

// Focusable is implemented by navigators that support keyboard focus (the
// sidebar). Navigators with no focus concept (tabs) omit it; the router uses a
// capability check rather than a concrete-type assertion to drive focus.
type Focusable interface {
	SetFocused(bool)
}

// NumberLabeled is implemented by navigators that can optionally show a leading
// per-item number prefix (the minimal top nav). The router applies the user's
// preference via this capability without asserting a concrete type.
type NumberLabeled interface {
	SetShowNumbers(bool)
}

// SelectedMsg is emitted when a navigation item is selected (via click or key).
type SelectedMsg struct {
	PageIndex int
}

// KeyCapturer can be implemented by page models that need exclusive keyboard
// focus. When CapturesKeys returns true the router will bypass its own global
// key shortcuts (quit, page-cycling) and will not forward key events to the
// navigation component, ensuring every keystroke reaches the active page.
type KeyCapturer interface {
	CapturesKeys() bool
}

// NavFocusMsg signals that the sidebar has gained or lost keyboard focus.
// The sidebar emits NavFocusMsg{Focused: true} when clicked and
// NavFocusMsg{Focused: false} when Esc is pressed inside it.
// The router emits NavFocusMsg{Focused: false} when the page-content area is
// clicked so the sidebar's visual focus indicator is updated.
type NavFocusMsg struct{ Focused bool }

// CollapseToggleMsg is emitted when the user clicks the collapse/expand handle
// at the top of the sidebar. The router forwards it to the nav's Update so the
// sidebar can toggle its own collapsed state, then triggers a layout resize.
type CollapseToggleMsg struct{}

// NavKeyMap defines key bindings used when the sidebar has keyboard focus.
type NavKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Dismiss key.Binding
}

// DefaultNavKeyMap returns the default key bindings for sidebar navigation.
func DefaultNavKeyMap() NavKeyMap {
	return NavKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "exit nav"),
		),
	}
}

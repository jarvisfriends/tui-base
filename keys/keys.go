package keys

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
)

type AppKeyMap struct {
	viewport.KeyMap
	Quit           key.Binding // Quit the application
	Tab            key.Binding // Next page
	ShiftTab       key.Binding // Previous page
	OpenSettings   key.Binding // Jump directly to the Settings page
	ToggleNav      key.Binding // Toggle Nav view
	ToggleStatus   key.Binding // Toggle Help view
	ToggleFullHelp key.Binding // Toggle Full Help view
	Select         key.Binding // Select the current choice (e.g. in a menu or list)
	Top            key.Binding // Scroll to the top of a list or page
	Bottom         key.Binding // Scroll to the bottom of a list or page
	Dismiss        key.Binding // Dismiss a modal or notification
	DismissAll     key.Binding // Dismiss all notifications in the history panel
	Debug          key.Binding
}

func DefaultKeyMap() *AppKeyMap {
	return &AppKeyMap{
		KeyMap: viewport.KeyMap{
			PageDown: key.NewBinding(
				key.WithKeys("pgdown"),
				key.WithHelp("pgdn", "page down"),
			),
			PageUp: key.NewBinding(
				key.WithKeys("pgup"),
				key.WithHelp("pgup", "page up"),
			),
			HalfPageUp: key.NewBinding(
				key.WithKeys("ctrl+up"),
				key.WithHelp("ctrl+up", "½ page up"),
			),
			HalfPageDown: key.NewBinding(
				key.WithKeys("ctrl+down"),
				key.WithHelp("ctrl+down", "½ page down"),
			),
			Up: key.NewBinding(
				key.WithKeys("up"),
				key.WithHelp("↑", "up"),
			),
			Down: key.NewBinding(
				key.WithKeys("down"),
				key.WithHelp("↓", "down"),
			),
			Left: key.NewBinding(
				key.WithKeys("left"),
				key.WithHelp("←", "move left"),
			),
			Right: key.NewBinding(
				key.WithKeys("right"),
				key.WithHelp("→", "move right"),
			),
		},
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next page"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev page"),
		),
		OpenSettings: key.NewBinding(
			key.WithKeys("ctrl+,"),
			key.WithHelp("ctrl+,", "settings"),
		),
		Top: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("home", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end"),
			key.WithHelp("end", "go to bottom"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "Select current choice"),
		),
		ToggleNav: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("ctrl+b", "toggle nav"),
		),
		ToggleFullHelp: key.NewBinding(
			key.WithKeys("ctrl+h"),
			key.WithHelp("ctrl+h", "detailed help"),
		),
		ToggleStatus: key.NewBinding(
			key.WithKeys("ctrl+j"),
			key.WithHelp("ctrl+j", "toggle status"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "dismiss modal/notification"),
			// Note that the "esc" key is often used for other things in TUI apps,
			// such as going back a page or closing a menu.
			// You can choose to use it for those things instead of dismissing modals, or not use it at all.
		),
		DismissAll: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "dismiss all notifications"),
		),
		Debug: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "quick debug"),
		),
	}
}

// FullHelp implements the bubbles/help KeyMap interface.
// It returns the key bindings arranged into rows for display.
func (km *AppKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Quit, km.Tab, km.ShiftTab, km.OpenSettings, km.ToggleFullHelp},
		{km.ToggleNav, km.ToggleStatus, km.Debug},
	}
}

// ShortHelp implements the bubbles/help KeyMap interface's ShortHelp method.
func (km *AppKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Quit, km.ToggleFullHelp}
}

var _ help.KeyMap = (*AppKeyMap)(nil)

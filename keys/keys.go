package keys

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
)

type AppKeyMap struct {
	viewport.KeyMap
	Quit           key.Binding // Quit the application
	NextPage       key.Binding // Next tab
	PreviousPage   key.Binding // Previous tab
	OpenSettings   key.Binding // Jump directly to the Settings tab
	ToggleNav      key.Binding // Toggle Nav view
	ToggleStatus   key.Binding // Toggle Help view
	ToggleFullHelp key.Binding // Toggle Full Help view
	Select         key.Binding // Select the current choice (e.g. in a menu or list)
	Top            key.Binding // Scroll to the top of a list or tab
	Bottom         key.Binding // Scroll to the bottom of a list or tab
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
		NextPage: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next page"),
		),
		PreviousPage: key.NewBinding(
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

// ApplyCustomizations updates the AppKeyMap fields from a map of string values.
func (km *AppKeyMap) ApplyCustomizations(custom map[string]string) {
	apply := func(id string, current key.Binding) key.Binding {
		val, ok := custom[id]
		if !ok || val == "" || val == "(none)" {
			return current
		}

		var keys []string
		for p := range strings.SplitSeq(val, ",") {
			if strings.TrimSpace(p) != "" {
				keys = append(keys, strings.TrimSpace(p))
			}
		}
		if len(keys) > 0 {
			current.SetKeys(keys...)
		}
		return current
	}

	km.Quit = apply("Quit", km.Quit)
	km.NextPage = apply("NextPage", km.NextPage)
	km.PreviousPage = apply("PreviousPage", km.PreviousPage)
	km.OpenSettings = apply("OpenSettings", km.OpenSettings)
	km.ToggleNav = apply("ToggleNav", km.ToggleNav)
	km.ToggleStatus = apply("ToggleStatus", km.ToggleStatus)
	km.ToggleFullHelp = apply("ToggleFullHelp", km.ToggleFullHelp)
	km.Select = apply("Select", km.Select)
	km.Top = apply("Top", km.Top)
	km.Bottom = apply("Bottom", km.Bottom)
	km.Dismiss = apply("Dismiss", km.Dismiss)
	km.DismissAll = apply("DismissAll", km.DismissAll)
	km.Debug = apply("Debug", km.Debug)
	km.PageDown = apply("PageDown", km.PageDown)
	km.PageUp = apply("PageUp", km.PageUp)
	km.HalfPageDown = apply("HalfPageDown", km.HalfPageDown)
	km.HalfPageUp = apply("HalfPageUp", km.HalfPageUp)
	km.Up = apply("Up", km.Up)
	km.Down = apply("Down", km.Down)
	km.Left = apply("Left", km.Left)
	km.Right = apply("Right", km.Right)
}

// FullHelp implements the bubbles/help KeyMap interface.
// It returns the key bindings arranged into rows for display.
func (km *AppKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Quit, km.NextPage, km.PreviousPage, km.OpenSettings, km.ToggleFullHelp},
		{km.ToggleNav, km.ToggleStatus, km.Debug},
	}
}

// ShortHelp implements the bubbles/help KeyMap interface's ShortHelp method.
func (km *AppKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Quit, km.ToggleFullHelp}
}

var _ help.KeyMap = (*AppKeyMap)(nil)

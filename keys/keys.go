package keys

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
)

const (
	bindingQuit           = "Quit"
	bindingNextPage       = "NextPage"
	bindingPreviousPage   = "PreviousPage"
	bindingOpenSettings   = "OpenSettings"
	bindingToggleNav      = "ToggleNav"
	bindingToggleStatus   = "ToggleStatus"
	bindingToggleFullHelp = "ToggleFullHelp"
	bindingSelect         = "Select"
	bindingTop            = "Top"
	bindingBottom         = "Bottom"
	bindingDismiss        = "Dismiss"
	bindingDismissAll     = "DismissAll"
	bindingDebug          = "Debug"
	bindingPageDown       = "PageDown"
	bindingPageUp         = "PageUp"
	bindingHalfPageDown   = "HalfPageDown"
	bindingHalfPageUp     = "HalfPageUp"
	bindingUp             = "Up"
	bindingDown           = "Down"
	bindingLeft           = "Left"
	bindingRight          = "Right"
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
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "settings"),
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

	km.Quit = apply(bindingQuit, km.Quit)
	km.NextPage = apply(bindingNextPage, km.NextPage)
	km.PreviousPage = apply(bindingPreviousPage, km.PreviousPage)
	km.OpenSettings = apply(bindingOpenSettings, km.OpenSettings)
	km.ToggleNav = apply(bindingToggleNav, km.ToggleNav)
	km.ToggleStatus = apply(bindingToggleStatus, km.ToggleStatus)
	km.ToggleFullHelp = apply(bindingToggleFullHelp, km.ToggleFullHelp)
	km.Select = apply(bindingSelect, km.Select)
	km.Top = apply(bindingTop, km.Top)
	km.Bottom = apply(bindingBottom, km.Bottom)
	km.Dismiss = apply(bindingDismiss, km.Dismiss)
	km.DismissAll = apply(bindingDismissAll, km.DismissAll)
	km.Debug = apply(bindingDebug, km.Debug)
	km.PageDown = apply(bindingPageDown, km.PageDown)
	km.PageUp = apply(bindingPageUp, km.PageUp)
	km.HalfPageDown = apply(bindingHalfPageDown, km.HalfPageDown)
	km.HalfPageUp = apply(bindingHalfPageUp, km.HalfPageUp)
	km.Up = apply(bindingUp, km.Up)
	km.Down = apply(bindingDown, km.Down)
	km.Left = apply(bindingLeft, km.Left)
	km.Right = apply(bindingRight, km.Right)
}

// BindingDef provides the ID, title, and default keys for an AppKeyMap binding.
type BindingDef struct {
	ID    string
	Title string
	Def   string
}

// BindingDefs returns the current bindings in a format suitable for generating settings UI.
func (km *AppKeyMap) BindingDefs() []BindingDef {
	return []BindingDef{
		{bindingQuit, "Quit Application", strings.Join(km.Quit.Keys(), ",")},
		{bindingNextPage, "Next Page", strings.Join(km.NextPage.Keys(), ",")},
		{bindingPreviousPage, "Previous Page", strings.Join(km.PreviousPage.Keys(), ",")},
		{bindingOpenSettings, "Open Settings", strings.Join(km.OpenSettings.Keys(), ",")},
		{bindingToggleNav, "Toggle Nav", strings.Join(km.ToggleNav.Keys(), ",")},
		{bindingToggleFullHelp, "Toggle Full Help", strings.Join(km.ToggleFullHelp.Keys(), ",")},
		{bindingToggleStatus, "Toggle Status", strings.Join(km.ToggleStatus.Keys(), ",")},
		{bindingSelect, "Select", strings.Join(km.Select.Keys(), ",")},
		{bindingTop, "Go to Top", strings.Join(km.Top.Keys(), ",")},
		{bindingBottom, "Go to Bottom", strings.Join(km.Bottom.Keys(), ",")},
		{bindingDismiss, "Dismiss Modal", strings.Join(km.Dismiss.Keys(), ",")},
		{bindingDismissAll, "Dismiss All Notifications", strings.Join(km.DismissAll.Keys(), ",")},
		{bindingDebug, "Quick Debug", strings.Join(km.Debug.Keys(), ",")},
		{bindingPageDown, "Page Down", strings.Join(km.PageDown.Keys(), ",")},
		{bindingPageUp, "Page Up", strings.Join(km.PageUp.Keys(), ",")},
		{bindingHalfPageDown, "Half Page Down", strings.Join(km.HalfPageDown.Keys(), ",")},
		{bindingHalfPageUp, "Half Page Up", strings.Join(km.HalfPageUp.Keys(), ",")},
		{bindingUp, "Up", strings.Join(km.Up.Keys(), ",")},
		{bindingDown, "Down", strings.Join(km.Down.Keys(), ",")},
		{bindingLeft, "Left", strings.Join(km.Left.Keys(), ",")},
		{bindingRight, "Right", strings.Join(km.Right.Keys(), ",")},
	}
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

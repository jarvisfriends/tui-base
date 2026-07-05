package settings

import (
	"charm.land/bubbles/v2/key"
	huh "charm.land/huh/v2"
)

// filePickerKeyMap returns the form keymap for file-picker fields.
//
// huh's defaults bind both "open folder" and "select entry" to enter, and the
// underlying bubbles filepicker resolves that overlap by selecting — so enter
// on a folder submitted the folder as the value and there was no way to browse
// into it. This map splits the two actions: Enter/→ descends into the
// highlighted folder, Space selects the highlighted entry (file or folder).
//
// Two bubbles constraints shape the bindings: Select only fires for keys that
// also match Open (so space appears in both), and any Select match submits the
// highlighted entry immediately (so enter cannot both descend and select).
// Navigation uses standardized keys only — no vim fallbacks (ADR-011).
func filePickerKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.FilePicker.Open = key.NewBinding(
		key.WithKeys("enter", "right", "space"),
		key.WithHelp("enter/→", "open folder"),
	)
	km.FilePicker.Select = key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "select"),
	)
	km.FilePicker.Back = key.NewBinding(
		key.WithKeys("left", "backspace"),
		key.WithHelp("←", "up folder"),
	)
	km.FilePicker.Up = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	)
	km.FilePicker.Down = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	)
	km.FilePicker.GotoTop = key.NewBinding(
		key.WithKeys("home"),
		key.WithHelp("home", "first"),
	)
	km.FilePicker.GotoBottom = key.NewBinding(
		key.WithKeys("end"),
		key.WithHelp("end", "last"),
	)
	return km
}

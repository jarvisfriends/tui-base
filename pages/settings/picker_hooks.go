package settings

import (
	huh "charm.land/huh/v2"

	"github.com/jarvisfriends/snap/pickers"
	"github.com/jarvisfriends/snap/styles"
	"github.com/jarvisfriends/tui-base/envpath"
)

// pickerStyles maps the active theme onto the pickers style hooks so the
// snap components render with the app palette (SP-7's host half).
func pickerStyles() pickers.Styles {
	c := styles.Active()
	return pickers.Styles{
		Title:    c.Styles.Title.Bold(true).Padding(0, 1),
		Path:     c.Styles.Subtitle,
		Selected: c.Styles.SelectedItem.Bold(true),
		Normal:   c.Styles.TextOnBg,
		Dim:      c.Styles.Dim,
	}
}

// newThemedDirPicker builds a DirPicker wired to the live theme and the
// env-var path shortening tui-base displays elsewhere.
func newThemedDirPicker(initial string) *pickers.DirPicker {
	dp := pickers.NewDirPicker(initial)
	dp.Styles = pickerStyles()
	dp.CollapsePath = envpath.Collapse
	return dp
}

// newThemedMultiFileEditor builds a MultiFileEditor wired the same way,
// including the huh theme for its embedded file-picker form.
func newThemedMultiFileEditor(value string) *pickers.MultiFileEditor {
	e := pickers.NewMultiFileEditor(value)
	e.Styles = pickerStyles()
	e.CollapsePath = envpath.Collapse
	e.HuhTheme = func() huh.Theme { return styles.HuhThemeFunc() }
	return e
}

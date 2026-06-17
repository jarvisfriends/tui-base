package theme

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// BuildHuhStyles produces huh form styles by combining three orthogonal axes:
//   - structure (borders, prefixes, indicators, padding, glyphs) from the chosen
//     StylePreset's huh built-in theme,
//   - colors from the application palette c (derived from the active tint),
//   - the light/dark variant via isDark.
//
// The preset theme is used as the structural starting point; overlayPaletteColors
// then re-tints every foreground/background/border so any of the 342 tints works
// with any preset. Because lipgloss styles are immutable values, re-applying a
// color preserves the preset's BorderStyle, padding, and SetString glyphs.
func BuildHuhStyles(c *AppStyle, preset StylePreset, isDark bool) *huh.Styles {
	fn := presetThemes[NormalizePreset(string(preset))]
	t := fn(isDark)
	overlayPaletteColors(t, c)
	return t
}

// overlayPaletteColors re-applies the application palette's colors onto a huh
// theme in place, mapping each semantic color to the matching form role while
// leaving the theme's structural attributes untouched.
func overlayPaletteColors(t *huh.Styles, c *AppStyle) {
	t.Focused.Base = t.Focused.Base.BorderForeground(c.Accent)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(c.Accent).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(c.Accent).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(c.Muted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(c.Error)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(c.Error)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(c.Warning)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(c.Warning)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(c.Warning)
	t.Focused.Option = t.Focused.Option.Foreground(c.Fg)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(c.SelectionFg).Background(c.SelectionBg)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(c.Success)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(c.Fg)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(c.Muted)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(c.Bg).Background(c.Accent).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(c.Fg).Background(c.Border)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(c.Warning)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(c.Muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(c.Warning)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = c.Styles.TextOnBg
	t.Blurred.PrevIndicator = c.Styles.TextOnBg
	t.Blurred.Title = t.Focused.Title.Foreground(c.Muted).Bold(false)
	t.Blurred.Description = c.Styles.Dim
	t.Blurred.SelectedOption = t.Focused.SelectedOption.Foreground(c.Muted)
	t.Blurred.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(c.Muted)

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
}

// HuhThemeFunc returns a ThemeFunc backed by the styles precomputed in the
// active palette (see fromTint), so there is no separate huh cache to maintain.
// It returns a shallow copy each call because huh forms mutate the styles they
// receive; huh.Styles holds only value-type fields, so a shallow copy is safe.
func HuhThemeFunc() huh.ThemeFunc {
	return func(_ bool) *huh.Styles {
		active := Active()
		if active.HuhStyles == nil {
			return BuildHuhStyles(active, DefaultStylePreset, true)
		}
		cp := *active.HuhStyles
		return &cp
	}
}

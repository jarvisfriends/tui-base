package theme

import (
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

// This file maps the active AppStyle onto the stock bubbles widgets (TC-1) so
// consumers get themed tables, lists, spinners, and progress bars with one
// call instead of hand-assembling styles (which historically left them on the
// un-themed bubbles defaults).

// TableStyles returns bubbles/table styles derived from c: theme item colors
// for cells, a bold high-contrast header, and the semantic selection colors
// for the cursor row — the same look as the inspector's data tables and every
// other selectable list in the framework.
func TableStyles(c *AppStyle) table.Styles {
	s := table.DefaultStyles()
	s.Cell = s.Cell.
		Background(c.Styles.Item.GetBackground()).
		Foreground(c.Styles.Item.GetForeground())
	s.Selected = s.Selected.
		Background(c.SelectionBg).
		Foreground(c.SelectionFg)
	headerBG, headerFG := tableHeaderColors(c)
	s.Header = s.Header.
		Background(lipgloss.Color(headerBG)).
		Foreground(lipgloss.Color(headerFG)).
		Bold(true)
	return s
}

// tableHeaderColors picks a readable header bg/fg pair from the theme,
// guarding against palettes where the selection colors collide.
func tableHeaderColors(c *AppStyle) (bg, fg string) {
	bg = ColorHex(c.Styles.SelectedItem.GetBackground())
	fg = ColorHex(c.Styles.SelectedItem.GetForeground())
	if bg == "" {
		bg = ColorHex(c.Accent)
	}
	if fg == "" {
		fg = ColorHex(c.Bg)
	}
	if strings.EqualFold(bg, fg) {
		fg = ColorHex(c.Bg)
		if strings.EqualFold(bg, fg) {
			fg = ColorHex(c.Styles.TextOnBg.GetForeground())
		}
	}
	return bg, fg
}

// ListDelegateStyles returns bubbles/list default-delegate item styles themed
// from c: normal rows in the standard text colors, the selected row carried
// by the accent (list's border-bar affordance) with selection colors, dimmed
// rows from the muted slot.
func ListDelegateStyles(c *AppStyle) (s list.DefaultItemStyles) {
	s = list.NewDefaultItemStyles(true)
	s.NormalTitle = s.NormalTitle.Foreground(c.Fg)
	s.NormalDesc = s.NormalDesc.Foreground(c.Muted)
	s.SelectedTitle = s.SelectedTitle.
		Foreground(c.Accent).
		BorderForeground(c.Accent)
	s.SelectedDesc = s.SelectedDesc.
		Foreground(c.SelectionFg).
		BorderForeground(c.Accent)
	s.DimmedTitle = s.DimmedTitle.Foreground(c.Muted)
	s.DimmedDesc = s.DimmedDesc.Foreground(c.Muted)
	return s
}

// SpinnerStyle returns the accent-colored style for a bubbles/spinner.
func SpinnerStyle(c *AppStyle) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c.Accent)
}

// ProgressGradient returns the hex color pair for a themed
// bubbles/progress gradient (accent → success), for use with
// progress.WithGradient(from, to). For a single-color bar use
// progress.WithSolidFill(from).
func ProgressGradient(c *AppStyle) (from, to string) {
	return ColorHex(c.Accent), ColorHex(c.Success)
}

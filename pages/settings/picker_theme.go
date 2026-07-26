// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings

import (
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/snap/datepicker"
	"github.com/jarvisfriends/snap/timepicker"
	"github.com/jarvisfriends/tui-base/theme"
)

// The datepicker and timepicker components ship theme-free (they are
// extraction candidates and must not depend on this repo's theme package), so
// the consumer maps the active theme onto their style hooks here.

// themedDatePickerStyles derives datepicker styles from the active theme: the
// focused cell uses the semantic selection colors and plain cells use the
// standard text-on-background style, replacing the component's hardcoded ANSI
// defaults.
func themedDatePickerStyles(c *theme.AppStyle) datepicker.Styles {
	s := datepicker.DefaultStyles()
	s.HeaderText = c.Styles.Title.Bold(true)
	s.Text = c.Styles.TextOnBg
	s.SelectedText = c.Styles.TextOnBg.Bold(true)
	s.FocusedText = lipgloss.NewStyle().
		Foreground(c.SelectionFg).
		Background(c.SelectionBg).
		Bold(true)
	return s
}

// applyTimePickerTheme derives timepicker styles from the active theme:
// accent for the focused segment, dim for inactive segments, subtitle for the
// help line.
func applyTimePickerTheme(tp *timepicker.TimePickerModel, c *theme.AppStyle) {
	tp.ActiveStyle = tp.ActiveStyle.
		Foreground(c.Accent).
		BorderForeground(c.Accent)
	tp.InactiveStyle = tp.InactiveStyle.
		Foreground(c.Styles.Dim.GetForeground()).
		BorderForeground(c.Styles.Dim.GetForeground())
	tp.HelpStyle = tp.HelpStyle.Foreground(c.Styles.Subtitle.GetForeground())
}

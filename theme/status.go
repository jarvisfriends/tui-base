package theme

import (
	"strconv"
	"strings"

	key "charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// BoxStyle returns a rounded-border box style using the current theme colors.
func BoxStyle() lipgloss.Style {
	c := Active()
	return c.Styles.OverlayBorder.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.Muted).
		Padding(1, 2)
}

// BoxTitleStyle returns a bold title style using the current accent color.
func BoxTitleStyle() lipgloss.Style {
	return Active().Styles.Title
}

// SubtleStyle returns a dimmed text style for secondary / hint content.
func SubtleStyle() lipgloss.Style {
	return Active().Styles.Subtitle
}

// RenderStatusBar composes a left-aligned help string and a right-aligned
// status string into a single styled bar of the given width. If width <= 0
// the function will return a simple un-padded rendering.
func RenderStatusBar(width int, left, right string) string {
	return RenderStatusBarStyled(width, left, right, -1)
}

// RenderStatusBarStyled renders the status bar. When colorIndex >= 0 it
// overrides the foreground with that ANSI index (0-255), which is useful for
// fade-in/out animations. Pass -1 to use the theme's StatusFg color.
func RenderStatusBarStyled(width int, left, right string, colorIndex int) string {
	c := Active()
	fg := c.StatusFg
	if colorIndex >= 0 {
		fg = lipgloss.Color(strconv.Itoa(colorIndex))
	}
	s := c.Styles.StatusBase.Foreground(fg)

	if width <= 0 {
		return s.Render(left + " " + right)
	}

	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)

	// Ensure at least one space between the two sides.
	gap := max(width-lw-rw, 1)
	filler := strings.Repeat(" ", gap)
	return s.Render(left + filler + right)
}

// CommonKeyMap provides a small set of common key bindings used across views.
type CommonKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Quit         key.Binding
	ToggleDetail key.Binding
}

func DefaultKeys() CommonKeyMap {
	return CommonKeyMap{
		Up:           key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		Down:         key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		Quit:         key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "quit")),
		ToggleDetail: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "toggle details")),
	}
}

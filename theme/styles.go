package theme

import (
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
)

// Styles holds pre-computed lipgloss styles derived from an AppStyle palette.
// Styles are rebuilt when the theme changes or the terminal background is detected.
type Styles struct {
	Name string

	Help help.Styles

	// Pre-computed styles — use these instead of calling lipgloss.NewStyle() inline.
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	RealHeader lipgloss.Style
	TextOnBg   lipgloss.Style
	Dim        lipgloss.Style

	BoarderActive   lipgloss.Style
	BoarderInactive lipgloss.Style

	Item         lipgloss.Style
	SelectedItem lipgloss.Style

	Send lipgloss.Style

	FilterDim lipgloss.Style

	StatusBase lipgloss.Style
	StatusKey  lipgloss.Style
	StatusDesc lipgloss.Style

	OverlayBorder lipgloss.Style

	NavTitle     lipgloss.Style
	NavActive    lipgloss.Style
	NavInactive  lipgloss.Style
	NavContainer lipgloss.Style

	TabInactive lipgloss.Style
	TabHover    lipgloss.Style

	SwatchDot lipgloss.Style
	Row       lipgloss.Style

	Success lipgloss.Style // tint.Green
	Error   lipgloss.Style // tint.Red("Error")
	Warning lipgloss.Style // tint.Yellow("Warning")
}

// hoverBackground derives a subtle row-highlight background from the base
// background: lighter on dark themes, darker on light themes, so the highlight
// is always visible regardless of which tint is active.
func hoverBackground(bg color.Color, isDark bool) color.Color {
	if isDark {
		return lipgloss.Lighten(bg, 0.08)
	}
	return lipgloss.Darken(bg, 0.08)
}

// BuildStyles pre-computes commonly used lipgloss styles from one palette.
func BuildStyles(c *AppStyle) *Styles {
	name := "active"
	isDark := true
	if c.OrigTint != nil {
		name = c.OrigTint.DisplayName
		isDark = c.OrigTint.Dark
	}

	base := lipgloss.NewStyle().Background(c.Bg).Foreground(c.Fg)
	statusBase := lipgloss.NewStyle().Background(c.StatusBg).Foreground(c.StatusFg)
	hoverBg := hoverBackground(c.Bg, isDark)

	return &Styles{
		Name: name,

		Title:      base.Bold(true).Foreground(c.Accent),
		Subtitle:   base.Foreground(c.Muted),
		RealHeader: base.Bold(true).Foreground(c.Accent),
		TextOnBg:   base,
		Dim:        base.Faint(true),

		BoarderActive:   base.BorderForeground(c.Accent),
		BoarderInactive: base.BorderForeground(c.Border),

		Item:         base.Foreground(c.Muted),
		SelectedItem: base.Background(c.SelectionBg).Foreground(c.SelectionFg).Bold(true),

		Send: base.Background(c.SelectionBg).Foreground(c.SelectionFg).Padding(0, 1),

		FilterDim: base.Foreground(c.Muted),

		StatusBase: statusBase,
		StatusKey:  statusBase.Foreground(c.Fg),
		StatusDesc: statusBase.Foreground(c.Muted),

		OverlayBorder: base.Border(lipgloss.RoundedBorder()).BorderForeground(c.Accent),

		NavTitle: base.Bold(true).Foreground(c.Accent),
		NavActive: base.Background(c.SelectionBg).
			Foreground(c.SelectionFg).
			Bold(true).
			Underline(true),
		NavInactive:  base.Foreground(c.Muted),
		NavContainer: base.BorderForeground(c.Border),

		TabInactive: base.BorderForeground(c.Accent),
		TabHover:    base.BorderForeground(c.Muted).Background(hoverBg),

		SwatchDot: base,
		Row:       base,

		Success: base.Foreground(c.Success),
		Error:   base.Foreground(c.Error),
		Warning: base.Foreground(c.Warning),

		Help: help.Styles{
			Ellipsis:       statusBase.Foreground(c.Muted),
			ShortKey:       statusBase.Foreground(c.Accent),
			ShortDesc:      statusBase.Foreground(c.Fg),
			ShortSeparator: statusBase.Foreground(c.Muted),
			FullKey:        statusBase.Foreground(c.Accent),
			FullDesc:       statusBase.Foreground(c.Fg),
			FullSeparator:  statusBase.Foreground(c.Muted),
		},
	}
}

// StyleCombo describes one concrete foreground/background pair used by the UI.
// It is primarily used for diagnostics and temporary accessibility tests.
type StyleCombo struct {
	Name string
	Fg   color.Color
	Bg   color.Color
}

// StyleCombosFromAppStyle returns concrete fg/bg combinations from named styles.
func StyleCombosFromAppStyle(c *AppStyle) []StyleCombo {
	if c == nil || c.Styles == nil {
		return nil
	}
	combos := []StyleCombo{
		{Name: "Title", Fg: c.Styles.Title.GetForeground(), Bg: c.Styles.Title.GetBackground()},
		{
			Name: "Subtitle",
			Fg:   c.Styles.Subtitle.GetForeground(),
			Bg:   c.Styles.Subtitle.GetBackground(),
		},
		{
			Name: "TextOnBg",
			Fg:   c.Styles.TextOnBg.GetForeground(),
			Bg:   c.Styles.TextOnBg.GetBackground(),
		},
		{Name: "Dim", Fg: c.Styles.Dim.GetForeground(), Bg: c.Styles.Dim.GetBackground()},
		{
			Name: "SelectedItem",
			Fg:   c.Styles.SelectedItem.GetForeground(),
			Bg:   c.Styles.SelectedItem.GetBackground(),
		},
		{
			Name: "StatusBase",
			Fg:   c.Styles.StatusBase.GetForeground(),
			Bg:   c.Styles.StatusBase.GetBackground(),
		},
		{
			Name: "StatusKey",
			Fg:   c.Styles.StatusKey.GetForeground(),
			Bg:   c.Styles.StatusKey.GetBackground(),
		},
		{
			Name: "StatusDesc",
			Fg:   c.Styles.StatusDesc.GetForeground(),
			Bg:   c.Styles.StatusDesc.GetBackground(),
		},
		{
			Name: "NavActive",
			Fg:   c.Styles.NavActive.GetForeground(),
			Bg:   c.Styles.NavActive.GetBackground(),
		},
		{
			Name: "NavInactive",
			Fg:   c.Styles.NavInactive.GetForeground(),
			Bg:   c.Styles.NavInactive.GetBackground(),
		},
		{Name: "Send", Fg: c.Styles.Send.GetForeground(), Bg: c.Styles.Send.GetBackground()},
		{
			Name: "Success",
			Fg:   c.Styles.Success.GetForeground(),
			Bg:   c.Styles.Success.GetBackground(),
		},
		{Name: "Error", Fg: c.Styles.Error.GetForeground(), Bg: c.Styles.Error.GetBackground()},
		{
			Name: "Warning",
			Fg:   c.Styles.Warning.GetForeground(),
			Bg:   c.Styles.Warning.GetBackground(),
		},
	}
	out := make([]StyleCombo, 0, len(combos))
	for _, combo := range combos {
		if combo.Fg != nil && combo.Bg != nil {
			out = append(out, combo)
		}
	}
	return out
}

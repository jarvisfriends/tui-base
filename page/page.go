// Package page provides a small embeddable base for tui-base pages, removing the
// colors/size boilerplate every page would otherwise repeat. A page model embeds
// [Base] and inherits a shared-palette pointer (satisfying theme.ColorAware) plus
// size tracking:
//
//	type Model struct {
//	    page.Base
//	    // page-specific fields…
//	}
//
//	func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
//	    if ws, ok := msg.(tea.WindowSizeMsg); ok {
//	        m.SetSize(ws.Width, ws.Height)
//	    }
//	    // …
//	}
//
//	func (m *Model) View() tea.View {
//	    c := m.Colors()           // shared palette, never nil
//	    _ = m.Width(); _ = m.Height()
//	    // …
//	}
package page

import "github.com/jarvisfriends/tui-base/theme"

// Base is embedded by page models to get standard color + size handling. Embed
// it as a value and use the pointer receiver methods via a *Model. It implements
// theme.ColorAware through SetColors.
type Base struct {
	colors        *theme.AppStyle
	width, height int
}

// SetColors stores the shared palette pointer (implements theme.ColorAware). The
// router calls this so a single in-place palette update propagates to every page
// without re-wiring.
func (b *Base) SetColors(c *theme.AppStyle) { b.colors = c }

// Colors returns the active palette, falling back to theme.Active() when no
// pointer has been wired yet (e.g. before the router calls SetColors, or in
// tests). Never returns nil.
func (b *Base) Colors() *theme.AppStyle {
	if b.colors != nil {
		return b.colors
	}
	return theme.Active()
}

// SetSize records the page's content area, supplied by the router via
// tea.WindowSizeMsg.
func (b *Base) SetSize(w, h int) { b.width, b.height = w, h }

// Width returns the last width set via SetSize.
func (b *Base) Width() int { return b.width }

// Height returns the last height set via SetSize.
func (b *Base) Height() int { return b.height }

var _ theme.ColorAware = (*Base)(nil)

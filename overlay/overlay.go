// Package overlay provides shared overlay primitives used by the router's
// built-in overlays and by page models (settings, dashboard) that manage their
// own centered form dialogs.
//
// The design splits into two levels:
//
//  1. Router-level overlays (toast, history, inspector, info modal) implement
//     the [Overlay] interface and are registered in the router's Z-ordered
//     stack. The router drives them through generic render/key/mouse loops.
//
//  2. Page-level overlays (settings edit form, dashboard wizard) are managed
//     entirely within the page model using [FormOverlayHost]. They do not
//     interact with the router overlay stack.
//
// Capability interfaces (KeyConsumer, MouseConsumer, OutsideCloser) are opt-in;
// a passive overlay (e.g. a toast notification) implements only Overlay.
package overlay

import (
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/geom"
)

// Context carries the parent-supplied dimensions an overlay needs to position
// itself. The router fills this from its own width/height/status-height/nav-width
// on every frame and passes it to Overlay.Render.
type Context struct {
	Width, Height int
	StatusHeight  int
	NavWidth      int
}

// Overlay is anything that floats above the page stack.
//
// Implement only the capability interfaces you need:
//   - [KeyConsumer] — to receive keyboard input (makes the overlay modal for keys)
//   - [MouseConsumer] — to receive mouse events inside Bounds
//   - [OutsideCloser] — to close when a mouse release lands outside Bounds
//
// A passive overlay (e.g. a toast) that implements none of these is transparent
// to all input: keys and mouse events fall through to the page beneath.
type Overlay interface {
	// Name is a stable identifier used for lookup and diagnostics.
	Name() string
	// Z is the stacking order; higher renders on top and takes input priority.
	Z() int
	// Visible reports whether the overlay should render and receive input.
	Visible() bool
	// Render returns the overlay content string for the given layout and
	// records the rectangle it occupies (accessible via Bounds). Return ""
	// to render nothing.
	Render(ctx Context) string
	// Bounds returns the rectangle from the most recent Render call, used for
	// mouse hit-testing. Must be valid after every Render call.
	Bounds() geom.Rect
}

// KeyConsumer overlays intercept key presses while visible. The topmost visible
// KeyConsumer consumes every key; pages beneath it receive none.
type KeyConsumer interface {
	OverlayKey(tea.KeyPressMsg) tea.Cmd
}

// MouseConsumer overlays receive mouse events that land inside their Bounds.
type MouseConsumer interface {
	OverlayMouse(tea.MouseMsg) tea.Cmd
}

// OutsideCloser overlays close when a mouse release lands outside their Bounds.
type OutsideCloser interface {
	CloseOnOutsideClick() tea.Cmd
}

// CenteredBase is an embeddable helper for overlays that render centered on
// screen. Embed it in your overlay struct, call Place inside Render after you
// have the rendered string, and the router receives correct hit-test bounds.
//
//	type myOverlay struct {
//	    overlay.CenteredBase
//	    // ...
//	}
//
//	func (o *myOverlay) Render(ctx overlay.Context) string {
//	    content := buildContent()
//	    return o.Place(content, ctx.Width, ctx.Height)
//	}
type CenteredBase struct {
	bounds geom.Rect
}

// Bounds returns the rectangle from the most-recent Place call.
func (b *CenteredBase) Bounds() geom.Rect { return b.bounds }

// Place computes the centered position for content within the given area,
// stores the bounds, and returns the content string unchanged. Call this at
// the end of your Render implementation.
func (b *CenteredBase) Place(content string, areaW, areaH int) string {
	w, h := lipgloss.Size(content)
	b.bounds = geom.Rect{W: w, H: h}.CenteredIn(areaW, areaH)
	return content
}

// ─── FormOverlayHost ─────────────────────────────────────────────────────────

// FormOverlayHost manages the common lifecycle of a single huh form rendered as
// a centered overlay inside a page model. It handles form width capping,
// terminal resize, outside-click bounds tracking, and lipgloss compositor calls.
//
// Typical usage — in Update:
//
//	case tea.WindowSizeMsg:
//	    m.fo.OnResize(msg.Width, msg.Height)
//
//	case tea.KeyPressMsg:
//	    if m.fo.IsOpen() {
//	        state, cmd := m.fo.Update(msg)
//	        switch state {
//	        case huh.StateCompleted:
//	            // read bound values, save, m.fo.Close()
//	        case huh.StateAborted:
//	            m.fo.Close()
//	        }
//	        return m, cmd
//	    }
//
// In View:
//
//	content = m.fo.Composite(overview, c.Styles.OverlayBorder)
//
// In OnMouse:
//
//	if click, ok := mm.(tea.MouseClickMsg); ok && m.fo.IsOpen() {
//	    if m.fo.IsOutsideClick(click.X, click.Y) { m.fo.Close() }
//	    return nil
//	}
type FormOverlayHost struct {
	form   *huh.Form
	open   bool
	bounds geom.Rect
	termW  int
	termH  int
}

// Open initializes and shows the form overlay, returning the form's Init Cmd.
func (h *FormOverlayHost) Open(f *huh.Form, termW, termH int) tea.Cmd {
	h.termW, h.termH = termW, termH
	h.form = f.WithWidth(FormWidth(termW))
	h.open = true
	return h.form.Init()
}

// Close hides the overlay and releases the form.
func (h *FormOverlayHost) Close() {
	h.form = nil
	h.open = false
	h.bounds = geom.Rect{}
}

// IsOpen reports whether the overlay is currently shown.
func (h *FormOverlayHost) IsOpen() bool { return h.open && h.form != nil }

// Bounds returns the rectangle from the most-recent Composite call, for
// outside-click hit-testing.
func (h *FormOverlayHost) Bounds() geom.Rect { return h.bounds }

// OnResize adapts the hosted form to the new terminal size.
func (h *FormOverlayHost) OnResize(termW, termH int) {
	h.termW, h.termH = termW, termH
	if h.form != nil {
		h.form.WithWidth(FormWidth(termW))
	}
}

// Update forwards msg to the form and returns its new state plus any Cmd.
func (h *FormOverlayHost) Update(msg tea.Msg) (huh.FormState, tea.Cmd) {
	if h.form == nil {
		return huh.StateNormal, nil
	}
	_, cmd := h.form.Update(msg)
	return h.form.State, cmd
}

// IsOutsideClick reports whether (x, y) falls outside the overlay bounds.
func (h *FormOverlayHost) IsOutsideClick(x, y int) bool {
	return h.IsOpen() && !h.bounds.Contains(x, y)
}

// Composite renders the form overlay centered over base using the supplied
// lipgloss border style. The bounds are updated each call so IsOutsideClick
// reflects the current frame.
func (h *FormOverlayHost) Composite(base string, borderStyle lipgloss.Style) string {
	if !h.IsOpen() {
		return base
	}
	box := borderStyle.
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(h.form.View())
	ow, oh := lipgloss.Size(box)
	h.bounds = geom.Rect{W: ow, H: oh}.CenteredIn(h.termW, h.termH)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(box).X(h.bounds.X).Y(h.bounds.Y).Z(1),
	).Render()
}

// FormWidth returns the standard responsive form width: at least 30 columns, at
// most 120, with a 14-column gutter inside the terminal for border, padding, and
// visual breathing room.
func FormWidth(termW int) int {
	return max(30, min(termW-14, 120))
}

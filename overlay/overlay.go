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

	"github.com/jarvisfriends/snap/geom"
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
	initCmd := h.form.Init()
	// Deliver the current size the way tea.Program would on open. Plain
	// input/select forms just take their natural height from this, but
	// file-picker fields depend on it: bubbles' filepicker starts with a
	// collapsed one-row browse window that only its WindowSizeMsg handler
	// unconditionally expands — builder-API heights alone leave the first
	// directory listing collapsed until a resize or directory change.
	model, _ := h.form.Update(tea.WindowSizeMsg{Width: FormWidth(termW), Height: FormHeight(termH)})
	if ff, ok := model.(*huh.Form); ok {
		h.form = ff
	}
	return initCmd
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
	model, cmd := h.form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		h.form = f
	}
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

// FormHeight returns the standard responsive form height for fields that
// expand vertically (e.g. file pickers): most of the available area, keeping a
// 6-row gutter for the overlay border, padding, and breathing room, with a
// 5-row floor so tiny terminals stay usable.
func FormHeight(termH int) int {
	return max(5, termH-6)
}

// ─── ModelOverlayHost ─────────────────────────────────────────────────────────

// ModelOverlayHost manages the common lifecycle of a generic tea.Model rendered as
// a centered overlay inside a page model.
type ModelOverlayHost struct {
	model  tea.Model
	open   bool
	bounds geom.Rect
	termW  int
	termH  int
}

// Open initializes and shows the overlay, returning the model's Init Cmd.
func (h *ModelOverlayHost) Open(m tea.Model, termW, termH int) tea.Cmd {
	h.termW, h.termH = termW, termH
	h.model = m
	h.open = true
	if h.model != nil {
		return h.model.Init()
	}
	return nil
}

// Close hides the overlay and releases the model.
func (h *ModelOverlayHost) Close() {
	h.model = nil
	h.open = false
	h.bounds = geom.Rect{}
}

// IsOpen reports whether the overlay is currently shown.
func (h *ModelOverlayHost) IsOpen() bool { return h.open && h.model != nil }

// Bounds returns the rectangle from the most-recent Composite call.
func (h *ModelOverlayHost) Bounds() geom.Rect { return h.bounds }

// OnResize adapts the hosted model to the new terminal size.
func (h *ModelOverlayHost) OnResize(termW, termH int) {
	h.termW, h.termH = termW, termH
	if h.model != nil {
		h.model, _ = h.model.Update(tea.WindowSizeMsg{Width: termW, Height: termH})
	}
}

// Update forwards msg to the model.
func (h *ModelOverlayHost) Update(msg tea.Msg) tea.Cmd {
	if h.model == nil {
		return nil
	}
	var cmd tea.Cmd
	h.model, cmd = h.model.Update(msg)
	return cmd
}

func (h *ModelOverlayHost) Model() tea.Model {
	return h.model
}

// IsOutsideClick reports whether (x, y) falls outside the overlay bounds.
func (h *ModelOverlayHost) IsOutsideClick(x, y int) bool {
	return h.IsOpen() && !h.bounds.Contains(x, y)
}

// Content insets applied by Composite's box: RoundedBorder (1 cell) plus
// Padding(1, 2). ForwardMouse subtracts these so hosted models receive
// content-relative coordinates. Keep in sync with Composite.
const (
	modelOverlayInsetX = 3 // border 1 + horizontal padding 2
	modelOverlayInsetY = 2 // border 1 + vertical padding 1
)

// ForwardMouse translates a page-relative mouse event into the hosted
// model's content coordinates (using the bounds from the last Composite) and
// forwards it. Components like snap's date/time pickers hit-test against
// their own rendered content, so translation is what makes their zones line
// up. No-op when the overlay is closed.
func (h *ModelOverlayHost) ForwardMouse(mm tea.MouseMsg) tea.Cmd {
	if !h.IsOpen() {
		return nil
	}
	me := mm.Mouse()
	nm := tea.Mouse{
		X:      me.X - h.bounds.X - modelOverlayInsetX,
		Y:      me.Y - h.bounds.Y - modelOverlayInsetY,
		Button: me.Button,
		Mod:    me.Mod,
	}
	switch mm.(type) {
	case tea.MouseClickMsg:
		return h.Update(tea.MouseClickMsg(nm))
	case tea.MouseReleaseMsg:
		return h.Update(tea.MouseReleaseMsg(nm))
	case tea.MouseMotionMsg:
		return h.Update(tea.MouseMotionMsg(nm))
	case tea.MouseWheelMsg:
		return h.Update(tea.MouseWheelMsg(nm))
	default:
		return nil
	}
}

// Composite renders the overlay centered over base.
func (h *ModelOverlayHost) Composite(base string, borderStyle lipgloss.Style) string {
	if !h.IsOpen() {
		return base
	}
	box := borderStyle.
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(h.model.View().Content)
	ow, oh := lipgloss.Size(box)
	h.bounds = geom.Rect{W: ow, H: oh}.CenteredIn(h.termW, h.termH)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(box).X(h.bounds.X).Y(h.bounds.Y).Z(1),
	).Render()
}

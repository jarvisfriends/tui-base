package router

import (
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/tui-base/geom"
	"github.com/jarvisfriends/tui-base/notifications"
	ov "github.com/jarvisfriends/tui-base/overlay"
	"github.com/jarvisfriends/tui-base/status"
)

// Re-export overlay package types so router consumers can use router.Overlay,
// router.Context, etc. without a separate import.
type (
	// Overlay re-exports [overlay.Overlay].
	Overlay = ov.Overlay
	// Context re-exports [overlay.Context].
	Context = ov.Context
	// KeyConsumer re-exports [overlay.KeyConsumer].
	KeyConsumer = ov.KeyConsumer
	// MouseConsumer re-exports [overlay.MouseConsumer].
	MouseConsumer = ov.MouseConsumer
	// OutsideCloser re-exports [overlay.OutsideCloser].
	OutsideCloser = ov.OutsideCloser
	// CenteredBase re-exports [overlay.CenteredBase].
	CenteredBase = ov.CenteredBase
	// FormOverlayHost re-exports [overlay.FormOverlayHost].
	FormOverlayHost = ov.FormOverlayHost
	// Rect aliases geom.Rect for backward-compat use in this file.
	Rect = geom.Rect
)

// layoutContext is an internal alias for overlay.Context.
type layoutContext = ov.Context

// FormOverlayWidth re-exports [overlay.FormWidth].
func FormOverlayWidth(termW int) int { return ov.FormWidth(termW) }

// RegisterOverlay inserts an external overlay into the router's Z-ordered stack.
func (m *RouterModel) RegisterOverlay(o Overlay) {
	for i, existing := range m.overlays {
		if o.Z() < existing.Z() {
			m.overlays = append(m.overlays[:i], append([]Overlay{o}, m.overlays[i:]...)...)
			return
		}
	}
	m.overlays = append(m.overlays, o)
}

const (
	zToast     = 10
	zHistory   = 20
	zInspector = 30
	zInfo      = 40
)

// buildOverlays constructs the router's Z-ordered overlay stack. Stored in
// ascending Z so View can composite bottom-up; Update/OnMouse iterate in reverse
// for top-down input priority.
func (m *RouterModel) buildOverlays() {
	m.overlays = []Overlay{
		&toastOverlay{m: m},
		&historyOverlay{m: m},
		&inspectorOverlay{m: m},
		&infoOverlay{m: m},
	}
}

// overlayHandleKey routes a key press to the topmost visible modal overlay.
// Returns ok=true (and the overlay's cmd) when an overlay consumed the key. Only
// KeyConsumer overlays are modal; passive overlays are skipped so keys fall
// through to the page.
func (m *RouterModel) overlayHandleKey(k tea.KeyPressMsg) (tea.Cmd, bool) {
	for _, o := range slices.Backward(m.overlays) {
		if !o.Visible() {
			continue
		}
		if kc, ok := o.(KeyConsumer); ok {
			return kc.OverlayKey(k), true
		}
	}
	return nil, false
}

// overlayHandleMouse routes a mouse event to the topmost visible interactive
// overlay. Returns ok=true when an overlay consumed the event (modal), so the
// caller skips base routing. Passive overlays (the toast) are transparent to the
// mouse and let the event reach the page beneath.
func (m *RouterModel) overlayHandleMouse(mm tea.MouseMsg) (tea.Cmd, bool) {
	for _, o := range slices.Backward(m.overlays) {
		if !o.Visible() {
			continue
		}
		mc, isMouse := o.(MouseConsumer)
		oc, isCloser := o.(OutsideCloser)
		if !isMouse && !isCloser {
			continue // passive overlay: not modal for mouse
		}
		inside := o.Bounds().Contains(mm.Mouse().X, mm.Mouse().Y)
		if inside && isMouse {
			return mc.OverlayMouse(mm), true
		}
		if !inside && isCloser {
			if _, ok := mm.(tea.MouseReleaseMsg); ok {
				return oc.CloseOnOutsideClick(), true
			}
		}
		return nil, true // modal: consume everything else
	}
	return nil, false
}

// renderOverlays composites every visible overlay onto base, bottom-up by Z.
func (m *RouterModel) renderOverlays(base string, statusHeight int) string {
	ctx := m.layoutCtx(statusHeight)
	for _, o := range m.overlays {
		if !o.Visible() {
			continue
		}
		s := o.Render(ctx)
		if s == "" {
			continue
		}
		at := o.Bounds()
		base = lipgloss.NewCompositor(
			lipgloss.NewLayer(base),
			lipgloss.NewLayer(s).X(at.X).Y(at.Y).Z(1),
		).Render()
	}
	return base
}

// overlayByName returns the overlay with the given Name, or nil.
func (m *RouterModel) overlayByName(name string) Overlay {
	for _, o := range m.overlays {
		if o.Name() == name {
			return o
		}
	}
	return nil
}

// layoutCtx assembles the parent-supplied dimensions overlays need. NavWidth is
// the sidebar width when a sidebar nav is visible (tabs reserve height, not
// width, and the history panel only needs to avoid a left sidebar).
func (m *RouterModel) layoutCtx(statusHeight int) layoutContext {
	return layoutContext{
		Width:        m.width,
		Height:       m.height,
		StatusHeight: statusHeight,
		NavWidth:     m.navReservedWidth(),
	}
}

// ---- toast: passive lower-right notification (no input) ----

type toastOverlay struct {
	m    *RouterModel
	rect Rect
}

func (o *toastOverlay) Name() string { return "toast" }
func (o *toastOverlay) Z() int       { return zToast }
func (o *toastOverlay) Bounds() Rect { return o.rect }

func (o *toastOverlay) Visible() bool {
	m := o.m
	return m.notifMgr != nil && !m.status.IsHistoryVisible() && len(m.notifMgr.Active()) > 0
}

func (o *toastOverlay) Render(ctx layoutContext) string {
	m := o.m
	active := m.notifMgr.Active()
	if len(active) == 0 {
		o.rect = Rect{}
		return ""
	}
	toast := active[0]
	borderColor := lipgloss.Color(notifications.ColorForSeverity(toast.Severity))
	toastStyle := m.colors.Styles.OverlayBorder.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(m.colors.Styles.TextOnBg.GetBackground()).
		Foreground(m.colors.Styles.TextOnBg.GetForeground()).
		Padding(0, 1)
	msg := toast.Content
	if len([]rune(msg)) > 40 {
		msg = string([]rune(msg)[:39]) + "…"
	}
	toastStr := toastStyle.Render(msg)
	tw, th := lipgloss.Size(toastStr)
	o.rect = Rect{X: max(ctx.Width-tw, 0), Y: max(ctx.Height-ctx.StatusHeight-th, 0), W: tw, H: th}
	return toastStr
}

// ---- notification history panel: lower-right, modal ----

type historyOverlay struct {
	m    *RouterModel
	rect Rect
}

func (o *historyOverlay) Name() string  { return "history" }
func (o *historyOverlay) Z() int        { return zHistory }
func (o *historyOverlay) Bounds() Rect  { return o.rect }
func (o *historyOverlay) Visible() bool { return o.m.status.IsHistoryVisible() }

func (o *historyOverlay) Render(ctx layoutContext) string {
	// Cap height to ~1/3 of the content area (max 12 rows) so nav and content
	// remain visible; limit width to the space right of any sidebar.
	contentH := ctx.Height - ctx.StatusHeight
	maxPanelH := max(min(contentH/3, 12), 4)
	maxPanelW := ctx.Width - ctx.NavWidth
	panelStr := o.m.status.RenderHistoryOverlay(maxPanelW, maxPanelH)
	if panelStr == "" {
		o.rect = Rect{}
		return ""
	}
	pw, ph := lipgloss.Size(panelStr)
	o.rect = Rect{
		X: max(ctx.Width-pw, ctx.NavWidth),
		Y: max(ctx.Height-ctx.StatusHeight-ph, 0),
		W: pw,
		H: ph,
	}
	return panelStr
}

func (o *historyOverlay) OverlayKey(k tea.KeyPressMsg) tea.Cmd {
	m := o.m
	notifCount := 0
	if m.notifMgr != nil {
		notifCount = len(m.notifMgr.Active())
	}
	switch {
	case key.Matches(k, m.keys.Quit):
		m.status.CloseHistory()
	case key.Matches(k, m.keys.Up):
		m.status.NotifHistoryCursorUp()
	case key.Matches(k, m.keys.Down):
		m.status.NotifHistoryCursorDown(notifCount)
	case key.Matches(k, m.keys.Select):
		cursor := m.status.HistoryCursor()
		if m.notifMgr != nil {
			active := m.notifMgr.Active()
			if cursor >= 0 && cursor < len(active) {
				m.notifMgr.Dismiss(active[cursor].ID)
			}
		}
	case key.Matches(k, m.keys.DismissAll), key.Matches(k, m.keys.Dismiss):
		if m.notifMgr != nil {
			m.notifMgr.DismissAll(nil)
		}
	}
	// Consume every key while open (modal), matching the historical behavior.
	return m.handleResizeCmd()
}

func (o *historyOverlay) OverlayMouse(mm tea.MouseMsg) tea.Cmd {
	if ev, ok := mm.(tea.MouseWheelMsg); ok {
		if ev.Mouse().Button == tea.MouseWheelUp {
			o.m.status.NotifHistoryCursorUp()
		} else {
			count := 0
			if o.m.notifMgr != nil {
				count = len(o.m.notifMgr.Active())
			}
			o.m.status.NotifHistoryCursorDown(count)
		}
		return o.m.handleResizeCmd()
	}
	return nil // clicks inside the panel are no-ops
}

func (o *historyOverlay) CloseOnOutsideClick() tea.Cmd {
	return tea.Batch(o.m.status.ToggleNotifications(), o.m.handleResizeCmd())
}

// ---- inspector: centered debug overlay, modal ----

type inspectorOverlay struct {
	m    *RouterModel
	rect Rect
}

func (o *inspectorOverlay) Name() string  { return "inspector" }
func (o *inspectorOverlay) Z() int        { return zInspector }
func (o *inspectorOverlay) Bounds() Rect  { return o.rect }
func (o *inspectorOverlay) Visible() bool { return o.m.inspector != nil && o.m.inspector.IsVisible() }

func (o *inspectorOverlay) Render(ctx layoutContext) string {
	m := o.m
	ow, oh := m.inspectorOverlayOuterSize()
	ox := max((ctx.Width-ow)/2, 0)
	oy := max((ctx.Height-oh)/2, 0)
	iw, ih := m.inspectorOverlayInnerSize()
	_, _ = m.inspector.Update(tea.WindowSizeMsg{Width: iw, Height: ih})
	o.rect = Rect{X: ox, Y: oy, W: ow, H: oh}
	return m.inspector.View().Content
}

func (o *inspectorOverlay) OverlayKey(k tea.KeyPressMsg) tea.Cmd {
	m := o.m
	switch {
	case key.Matches(k, m.keys.Debug), key.Matches(k, m.keys.Dismiss):
		m.inspector.ToggleVisible()
		return m.handleResizeCmd()
	default:
		_, cmd := m.inspector.Update(k)
		return tea.Batch(cmd, m.handleResizeCmd())
	}
}

func (o *inspectorOverlay) OverlayMouse(mm tea.MouseMsg) tea.Cmd {
	iv := o.m.inspector.View()
	if iv.OnMouse == nil {
		return nil
	}
	me := mm.Mouse()
	// Inspector content sits one cell inside its border.
	offX, offY := o.rect.X+1, o.rect.Y+1
	nm := tea.Mouse{X: me.X - offX, Y: me.Y - offY, Button: me.Button, Mod: me.Mod}
	switch mm.(type) {
	case tea.MouseClickMsg:
		return iv.OnMouse(tea.MouseClickMsg(nm))
	case tea.MouseReleaseMsg:
		return iv.OnMouse(tea.MouseReleaseMsg(nm))
	case tea.MouseMotionMsg:
		return iv.OnMouse(tea.MouseMotionMsg(nm))
	case tea.MouseWheelMsg:
		return iv.OnMouse(tea.MouseWheelMsg(nm))
	}
	return nil
}

func (o *inspectorOverlay) CloseOnOutsideClick() tea.Cmd {
	o.m.inspector.ToggleVisible()
	return o.m.handleResizeCmd()
}

// ---- info modal: centered version/dependency overlay, modal ----

type infoOverlay struct {
	m    *RouterModel
	rect Rect
}

func (o *infoOverlay) Name() string  { return "info" }
func (o *infoOverlay) Z() int        { return zInfo }
func (o *infoOverlay) Visible() bool { return o.m.infoModal.IsVisible() }

func (o *infoOverlay) Bounds() Rect {
	bx, by, bw, bh := o.m.infoModal.Bounds()
	return Rect{X: max(0, bx), Y: max(0, by), W: bw, H: bh}
}

func (o *infoOverlay) Render(ctx layoutContext) string {
	modalStr := o.m.infoModal.View()
	if modalStr.Content == "" {
		o.rect = Rect{}
		return ""
	}
	o.rect = o.Bounds()
	return modalStr.Content
}

func (o *infoOverlay) OverlayKey(k tea.KeyPressMsg) tea.Cmd {
	if _, cmd := o.m.infoModal.Update(k); cmd != nil {
		return cmd
	}
	return o.m.handleResizeCmd() // consume all other keys
}

func (o *infoOverlay) OverlayMouse(mm tea.MouseMsg) tea.Cmd {
	if ev, ok := mm.(tea.MouseWheelMsg); ok {
		up := ev.Mouse().Button == tea.MouseWheelUp
		return func() tea.Msg { return status.InfoModalScrollMsg{Up: up} }
	}
	return nil
}

func (o *infoOverlay) CloseOnOutsideClick() tea.Cmd {
	return func() tea.Msg { return status.CloseInfoModalMsg{} }
}

package router

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// customDebugOverlay presents an app-injected tea.Model as the Ctrl+D debug
// pop-up in place of the built-in inspector (SP-11/Q-22: Options.DebugOverlay /
// tuibase.WithDebugOverlay). It reuses the inspector's box geometry so the two
// experiences are visually interchangeable; tui-base owns the Ctrl+D toggle
// whenever the injected model is non-nil.
type customDebugOverlay struct {
	m    *RouterModel
	rect Rect
}

func (o *customDebugOverlay) Name() string { return "debug-overlay" }

// Z matches the inspector layer: only one of the two debug overlays is ever
// reachable (Ctrl+D routes to the injected model when it is set).
func (o *customDebugOverlay) Z() int       { return zInspector }
func (o *customDebugOverlay) Bounds() Rect { return o.rect }

func (o *customDebugOverlay) Visible() bool {
	return o.m.debugOverlay != nil && o.m.debugOverlayVisible
}

func (o *customDebugOverlay) Render(ctx layoutContext) string {
	m := o.m
	ow, oh := m.inspectorOverlayOuterSize()
	ox := max((ctx.Width-ow)/2, 0)
	oy := max((ctx.Height-oh)/2, 0)
	iw, ih := m.inspectorOverlayInnerSize()
	m.updateDebugOverlay(tea.WindowSizeMsg{Width: iw, Height: ih})
	o.rect = Rect{X: ox, Y: oy, W: ow, H: oh}
	return m.debugOverlay.View().Content
}

// OverlayKey makes the overlay key-modal while visible: Ctrl+D (or Esc)
// closes it, everything else is forwarded to the injected model.
func (o *customDebugOverlay) OverlayKey(k tea.KeyPressMsg) tea.Cmd {
	m := o.m
	switch {
	case key.Matches(k, m.keys.Debug), key.Matches(k, m.keys.Dismiss):
		m.debugOverlayVisible = false
		m.updatePageKeys()
		return m.handleResizeCmd()
	default:
		return m.updateDebugOverlay(k)
	}
}

// OverlayMouse forwards mouse events (overlay-relative, inside the border)
// when the injected model's view opts into mouse handling.
func (o *customDebugOverlay) OverlayMouse(mm tea.MouseMsg) tea.Cmd {
	dv := o.m.debugOverlay.View()
	if dv.OnMouse == nil {
		return nil
	}
	me := mm.Mouse()
	offX, offY := o.rect.X+1, o.rect.Y+1
	nm := tea.Mouse{X: me.X - offX, Y: me.Y - offY, Button: me.Button, Mod: me.Mod}
	switch mm.(type) {
	case tea.MouseClickMsg:
		return dv.OnMouse(tea.MouseClickMsg(nm))
	case tea.MouseReleaseMsg:
		return dv.OnMouse(tea.MouseReleaseMsg(nm))
	case tea.MouseMotionMsg:
		return dv.OnMouse(tea.MouseMotionMsg(nm))
	case tea.MouseWheelMsg:
		return dv.OnMouse(tea.MouseWheelMsg(nm))
	}
	return nil
}

func (o *customDebugOverlay) CloseOnOutsideClick() tea.Cmd {
	o.m.debugOverlayVisible = false
	o.m.updatePageKeys()
	return o.m.handleResizeCmd()
}

// updateDebugOverlay dispatches msg to the injected debug model, storing the
// returned model (model-swap pattern, B-1).
func (m *RouterModel) updateDebugOverlay(msg tea.Msg) tea.Cmd {
	if m.debugOverlay == nil {
		return nil
	}
	updated, cmd := m.debugOverlay.Update(msg)
	m.debugOverlay = updated
	return cmd
}

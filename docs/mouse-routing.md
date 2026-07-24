# Mouse Routing & Click Mapping

Summary
- The top-level `tea.View` receives all mouse messages in Bubble Tea.
  Nested `tea.View.OnMouse` handlers are not called automatically by the runtime.
  Dispatch mouse events from the top-level view into each child's `OnMouse` manually.
- Bubble Tea delivers every mouse event **twice**: once to the top-level view's
  `OnMouse` callback and once to the model's `Update` as a `tea.MouseMsg`. Any
  routing or modality rule must be enforced on **both** paths, or events leak
  through the one you forgot (see "Overlay modality" below).

Key points
- Always compute child-relative coordinates by subtracting the child's origin
  (offset) from global mouse coordinates. Example:

  childX = globalX - childOffsetX
  childY = globalY - childOffsetY

- Use the same sizing logic at render-time and click-time.
  If child width/height is computed in `Update` via `WindowSizeMsg`, verify those
  `Update` calls have run before relying on `.Width()` or `.Height()` in routing.

- Use `lipgloss.Width` and `lipgloss.Height` on rendered strings to compute
  accurate column/row positions for click hit testing (including styled text and wide runes).

- When top-level `OnMouse` returns a `tea.Cmd` created via `tea.Batch`, tests and
  callers need to execute every sub-cmd inside `tea.BatchMsg` to see produced
  `tea.Msg` values. The runtime does this automatically; tests must call each sub-cmd.

Overlay modality (router `overlayHandleMouse` + `mouseModalOverlayVisible`)
- While a modal overlay (notification history, inspector, info modal — any
  visible `MouseConsumer`/`OutsideCloser`) is open, it owns the mouse, exactly
  as the topmost `KeyConsumer` owns the keyboard:
  - **Positional events** (click, motion): inside the overlay's `Bounds()` they
    go to `OverlayMouse`; a release outside closes `OutsideCloser` overlays;
    everything else is consumed.
  - **Wheel events** are positionless scrolling intent: they always route to
    the overlay regardless of where the pointer sits, matching keyboard
    scrolling. Overlays therefore must handle `tea.MouseWheelMsg` in
    `OverlayMouse` without hit-testing the coordinates.
  - The **Update path** is gated too: `RouterModel.Update` drops mouse messages
    to the nav and pages while a mouse-modal overlay is visible
    (`mouseModalOverlayVisible`). Without this, the same wheel event that
    scrolled the overlay via `OnMouse` would also scroll the page behind it via
    `Update` — the double-delivery pitfall above.
- Passive overlays (the toast) implement neither interface and are fully
  transparent to the mouse.

Design choices we used in this repo
- Router-level dispatch: the router computes main layout sizes and either calls
  `child.OnMouse(tea.MouseXxxMsg(...))` or, for special cases like status bar
  icons, synthesizes a `ClickRegionMsg` when it detects a click in a known region.

- Inspector messages: every routed mouse event emits
  `debug.MouseHighlightMsg{GlobalX,GlobalY,Child,OffX,OffY}` so the inspector can
  display where the click landed and offsets used when invoking the child handler.

Common pitfalls
- Calling the child's `View().Content` and searching for a rune in raw text is
  fragile. Prefer computing hit regions with the same renderer that produced
  content (`RenderStyled` or `lipgloss` helpers) so offsets stay consistent.

- Not accounting for overlays: status overlays may add extra lines above the
  status row. Compute overlay height (`lipgloss.Height(overlay)`) and account for
  it when mapping clicks to the status line.

Tests
- Use a helper to execute `tea.Cmd` values and flatten `tea.BatchMsg`
  (see `execCmd` in `router/mouse_routing_test.go`).
  This verifies tests see the same messages the runtime would dispatch.

If something is still misrouted
1. Log `mEvent.X/mEvent.Y`, `nav.Width()`, `nav.Height()`, `status.Height()`, and
  computed `mainHeight`. Compare these to `Lipgloss.Width`/`Height` of rendered views.
2. Verify all `Update(tea.WindowSizeMsg{...})` calls complete before calling
  `View().OnMouse` in tests or code paths that depend on sizes.
3. Check how borders, padding, and built-in widths affect `lipgloss.Width`
  versus the nav model's `Width()` value.

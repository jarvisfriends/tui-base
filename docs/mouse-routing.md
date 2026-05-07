# Mouse Routing & Click Mapping

Summary
- The top-level `tea.View` receives all mouse messages in Bubble Tea. Nested `tea.View.OnMouse` handlers are not called automatically by the runtime — you must dispatch mouse events from the top-level view into each child's `OnMouse` manually.

Key points
- Always compute child-relative coordinates by subtracting the child's origin (offset) from the global mouse coordinates. Example:

  childX = globalX - childOffsetX
  childY = globalY - childOffsetY

- Use the same sizing logic at render-time and at click-time. If the child width/height is computed in `Update` via `WindowSizeMsg`, ensure those `Update` calls have run before relying on `.Width()` or `.Height()` during `View().OnMouse` routing.

- Use `lipgloss.Width`/`lipgloss.Height` on rendered strings to compute accurate column/row positions for click hit testing (accounts for styling and wide runes).

- When your top-level `OnMouse` returns a `tea.Cmd` created via `tea.Batch`, tests and callers need to execute every sub-cmd inside the `tea.BatchMsg` to see the produced `tea.Msg` values. The runtime does this for you, but tests must explicitly call each sub-cmd.

Design choices we used in this repo
- Router-level dispatch: the router computes the main layout sizes and either calls `child.OnMouse(tea.MouseXxxMsg(...))` or, for special cases like the status bar icons, synthesizes a `ClickRegionMsg` directly when it detects a click within a known region.

- Inspector messages: every routed mouse event also emits a `debug.MouseHighlightMsg{GlobalX,GlobalY,Child,OffX,OffY}` so the inspector can display where the click landed and the offsets used when invoking the child handler.

Common pitfalls
- Calling the child's `View().Content` and then searching for a rune in the raw string is fragile; prefer computing hit regions with the same renderer that produced the content: use `RenderStyled` or `lipgloss` helpers to compute offsets consistently.

- Not accounting for overlays: status overlays may add extra lines above the status row; compute overlay height (`lipgloss.Height(overlay)`) and account for it when mapping clicks to the status line.

Tests
- Use a helper to execute `tea.Cmd` values and flatten `tea.BatchMsg` (see `execCmd` in `router/mouse_routing_test.go`). This ensures tests see the same messages the runtime would have dispatched.

If something is still misrouted
1. Log `mEvent.X/mEvent.Y`, `nav.Width()`, `nav.Height()`, `status.Height()` and the computed `mainHeight`. Compare those numbers to `Lipgloss.Width`/`Height` of the corresponding rendered views.
2. Verify all `Update(tea.WindowSizeMsg{...})` calls have completed before calling `View().OnMouse` in tests or code paths that depend on the sizes.
3. Check how borders/padding/built-in widths affect `lipgloss.Width` vs the nav model's `Width()` value.

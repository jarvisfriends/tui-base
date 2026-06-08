# Mouse Debugging Checklist

This short checklist captures quick diagnostics and the exact steps we found useful when hunting down click routing issues.

1) Does top-level `View.OnMouse` run?
- Ensure the `tea.View` returned by your `Model.View()` has `OnMouse` set.
	Bubble Tea calls only the top-level `OnMouse`.
	Nested views are not routed automatically.

2) Are child sizes up-to-date?
- Send `tea.WindowSizeMsg` to the parent so it can forward the size to children.
	In tests, call `Update(WindowSizeMsg{Width,Height})` and run returned `tea.Cmd`s
	so children can recompute `.Width()` and `.Height()`.

3) Are you using the same renderer for click regions?
- If you compute click regions using rendered output (searching runes or substrings),
  generate that output with the same `lipgloss` style used at render-time.
  Otherwise you risk mismatches from padding, border, or ANSI sequences.

4) Are coordinates global vs local?
- For a child positioned at (offX, offY) inside the top-level view,
  subtract those offsets before calling the child's `OnMouse` handler:
  `local := tea.Mouse{X: globalX - offX, Y: globalY - offY}`.

5) Is the status overlay shifting rows?
- Status overlays (notifications panel) can insert extra lines above the status line.
	Use `lipgloss.Height(overlay)` to compute `overlayHeight` and map clicks.

6) Test helper: flatten `tea.BatchMsg`.
- In tests, the `tea.Cmd` returned by a view may be a batch.
	Execute it and run each contained `tea.Cmd` so you can inspect resulting
	`tea.Msg`s (e.g., `debug.MouseHighlightMsg`, `status.ClickRegionMsg`).

7) When in doubt, log everything:
- Log: `globalX/globalY`, `offX/offY`, `nav.Width()`, `nav.Height()`, `statusHeight`, `rendered status line`.

Example commands used during debugging
```bash
go test ./router -run TestMouseRoutingBoundaries -v
go test ./status -run TestBarModel_ClickRegions -v
```

# Architecture Decisions

This document captures stable decisions that were previously mixed into completed roadmap items.

## ADR-001: Router-Owned Composition

Decision:
- The router is the root model that owns navigation, active page selection,
  status bar integration, theme pointer wiring, and message forwarding.

Why:
- Centralizes cross-cutting concerns and keeps page models focused on local behavior.

## ADR-002: Shared Mutable App Style Pointer

Decision:
- Use a single shared `*theme.AppStyle` pointer across router, navigation, pages, and status components.

Why:
- Theme changes can be applied in place and become visible everywhere immediately.
- Avoids deep rebuild/rebind of model trees on theme switch.

## ADR-003: Theme-Driven Rendering Contract

Decision:
- UI rendering should use style helpers (`c.Styles.*`) rather than ad hoc colors.

Why:
- Provides visual consistency and supports centralized theme evolution.

## ADR-004: Non-Blocking Update Loop

Decision:
- Runtime I/O belongs in `tea.Cmd` paths, not directly in `Update`.

Why:
- Keeps the Bubble Tea event loop responsive and predictable.

## ADR-005: Navigation Input Parity

Decision:
- Navigation primitives support both keyboard and mouse inputs with equivalent semantics.

Why:
- Reduces accessibility and usability regressions as features grow.

## ADR-006: Built-In Runtime Observability

Decision:
- Debug inspector and logger fan-out are first-class parts of the architecture.

Why:
- Large TUIs are easier to evolve when message flow and logs are inspectable in-app.

## ADR-007: Shared Notification Manager

Decision:
- Use a shared `*notifications.Manager` pointer pattern similar to colors.

Why:
- Gives one source of truth for toast/history rendering, persistence, and runtime control.

## ADR-008: Compositor-Based Overlay Strategy

Decision:
- Use Lip Gloss compositor patterns for overlays (notification/history and future inspector overlay).

Why:
- Overlay positioning and layering remain stable without each page implementing custom z-order logic.

## ADR-009: Engineering Workflow Baseline

Decision:
- Keep tests and static checks as part of normal development (`go test`, race tests, vet; modernize when available).

Why:
- Maintains confidence while iterating quickly on interaction-heavy code.

## ADR-010: Public Reusability Bias

Decision:
- Types in `common/` may be consumer-facing even if not yet heavily used internally.

Why:
- `tui-base` is intended as a reusable foundation, not only an app-local code dump.
	- These types should be periodically reviewed for real external value.

## ADR-011: Standardized Key Mapping & Help System

Decision:
- All custom components must define their bindings directly in a `KeyMap` struct.
- `help.KeyMap` methods (`ShortHelp()`, `FullHelp()`) must **never** instantiate `key.NewBinding` inline. They must 
  only return bindings from the predefined struct.
- Vim fallback navigation keys (`j`, `k`, `h`, `l`) are prohibited in UI components to prevent mapping conflicts.
  Always use standardized `PreviousPage`/`NextPage` and `Up`/`Down` fields.
- Legacy pop-up string-based help menus are prohibited. Components must expose their options natively via the 
  standard Bubbletea `help.KeyMap` integration in the status bar.

Why:
- Verifies consistent keyboard navigation across all applications.
- Prevents memory allocations on every frame render when generating help menus.
- Centralizes help rendering into a single predictable UI element.

## ADR-012: Four-Axis Theme System

Decision:
- Styling is four independent, user-controllable axes: **color tint** (any bubbletint
  palette) × **style preset** (huh built-in structure) × **mode** (light/dark) ×
  **accessibility** (CVD engine).
- One build pipeline and one cache: `fromTint` builds the palette, chrome `Styles`,
  and `HuhStyles` together, keyed by `tintID|preset|access`. `BuildHuhStyles(c, preset, isDark)`
  takes *structure* from a huh preset and overlays *colors* from the active tint.
- Rejected dependencies: glamour (not needed) and `compat.AdaptiveColor` (blocks on a
  terminal query at init and ignores the explicit Mode setting).

Why:
- Users can change any axis independently with live preview; a single cache avoids
  divergent palette/huh state, and the preset overlay fixed a latent bug where
  `huh.ThemeBase(false)` ignored mode.

## ADR-013: Unified Z-Ordered Overlay Stack

Decision:
- All floating overlays (toast, notification history, inspector, info modal) share one
  Z-ordered `[]Overlay` in the router (`router/overlay.go`), driven by three generic
  loops (key intercept, view compositing, mouse hit-test).
- `Overlay` is a minimal interface (`Name/Z/Visible/Render/Bounds`) with opt-in
  capability interfaces: `KeyConsumer`, `MouseConsumer`, `OutsideCloser`. A passive
  overlay (toast) implements only `Overlay` and is never modal.
- Modality covers both input paths and both devices: the topmost `KeyConsumer`
  takes every key; a visible `MouseConsumer`/`OutsideCloser` takes every mouse
  event — positional events by `Bounds()` hit-test, wheel events unconditionally
  (scrolling intent follows the active view, like the keyboard). Because Bubble
  Tea delivers mouse events to `View.OnMouse` *and* `Update`, the router gates
  the `Update` path as well (`mouseModalOverlayVisible`) so events never leak to
  the nav or the page behind an open overlay.
- `Navigator.Dock() Side` and the `Focusable` capability replaced concrete
  `*navigation.Sidebar` / `*navigation.Tabs` type switches in the router.

Why:
- Adding an overlay is now "implement + append" with no edits to Update/View/OnMouse;
  the previous design hardcoded each overlay in three separate methods.

## ADR-014: In-House `geom.Rect` for Mouse Hit-Testing (Not bubblezone)

Decision:
- All rectangle geometry (router overlays, settings overlay, consumer widget grids)
  uses `tui-base/geom` (`Rect{X,Y,W,H}` + `Contains`/`Empty`/`CenteredIn`) with the
  v2 `tea.View` OnMouse callback. Status `ClickRegion` (a 1-D horizontal span)
  intentionally stays separate.
- bubblezone was evaluated and rejected: its `v2.0.0` tag misbehaves with the lipgloss
  canvas/compositor used for overlays, and the v2 idiom is the OnMouse callback with
  explicit rects anyway.

Why:
- One tiny well-tested type instead of a dependency that fights the compositor.

## ADR-015: Shared Page Scaffolding via `page.Base`

Decision:
- Pages embed `page.Base`, which provides `Colors()` (nil-safe), `Width()`, `Height()`,
  `SetColors` (satisfies `theme.ColorAware`), and `SetSize`. Pages needing side effects
  override `SetColors`/`SetSize` and call `m.Base.*` first.

Why:
- Deleted byte-identical boilerplate across every page in this repo and its consumers
  (dash, aSettings, verify_setup).

## ADR-016: Linting Posture — Broad Set, Explicit Exclusions

Decision:
- `.golangci.yml` uses `default: none` with a deliberately broad explicit set (~45
  linters across correctness, style, performance, security, testing; see the config).
- gosec runs with only two exclusions: G204 and G104-in-tests (intentional).
- Linters evaluated and intentionally **not** enabled, with reasons:
  - `err113` — forces package-level sentinels for display-only errors; reconsider if
    library packages are extracted.
  - `noctx` — wrong for the pprof browser-launcher and fire-and-forget external tool
    calls in the inspector; revisit if the inspector gains a lifecycle context.
  - `godox` — TODO/FIXME flags are too noisy during active development.
  - `gochecknoglobals` — TUI app state requires globals.
  - `exhaustruct` — too noisy for large TUI models.
  - `lll` / `funlen` / `varnamelen` — style-enforcing size limits.
  - `testpackage` — large refactor, little benefit.

Why:
- `default: none` makes the linter set reproducible; recording the rejected linters
  prevents re-litigating them. Review new golangci-lint releases periodically for
  useful additions.

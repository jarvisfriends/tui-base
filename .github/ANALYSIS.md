# Deep Analysis — tui-base

> Updated: 2026-06-07. Keep updated as the project evolves.

---

## Goal recap

Build a bubbletea v2 framework that any downstream app (dash, aSettings,
verify_setup, …) can import and get navigation, status bar, help, settings
(per-app-name scoped), layouts, color themes, notifications, and a debug
inspector — for free. Consumers only write `tea.Model` pages.

---

## Recent work (2026-06)

### Theme system — four orthogonal axes
The `theme` package was redesigned so styling is four independent axes the end
user controls: **color tint** (any bubbletint palette) × **style preset** (huh's
built-in structure) × **mode** (light/dark) × **accessibility** (CVD engine).

- `BuildHuhStyles(c, preset, isDark)` takes *structure* from a huh built-in theme
  (`presets.go`) and overlays *colors* from the active tint — fixing a latent bug
  where `huh.ThemeBase(false)` ignored mode.
- New user-selectable **Form Style** setting (`style_preset`), live-previewed.
- Single build pipeline + single cache: `fromTint` builds palette + chrome
  `Styles` + `HuhStyles` together, cached by `tintID|preset|access`. The separate
  huh cache was removed; `AppStyle.HuhStyles` holds the precomputed styles.
- `theme.go` (834 → ~310 lines) split into `presets.go`, `huh.go`, `styles.go`,
  `accessibility.go` (CVD engine isolated), `status.go`. Dropped 13 dead `Styles`
  fields; used lipgloss `Darken`/`Lighten` for derived border/hover colors.
- **No new dependencies** (glamour and `compat.AdaptiveColor` were evaluated and
  rejected — the latter blocks on a terminal query at init and ignores the
  explicit Mode setting).

### Router — unified overlay system
The four floating overlays (toast, notification history, inspector, info modal)
were each hardcoded in three methods (Update key intercept, View compositing,
OnMouse hit-test). They now share one Z-ordered `[]Overlay` driven by three
generic loops (`router/overlay.go`):

- `Overlay` interface (`Name/Z/Visible/Render(layoutContext)/Bounds`) + opt-in
  capability interfaces `KeyConsumer`, `MouseConsumer`, `OutsideCloser`. A passive
  overlay (toast) implements only `Overlay` and is never modal.
- `Rect{X,Y,W,H}` + `Contains` replaced the two `[4]int` bounds caches; each
  overlay owns its bounds. Adding an overlay = implement + append (no edits to
  Update/View/OnMouse).
- `Navigator.Dock() Side` + `Focusable` capability removed all
  `*navigation.Sidebar` / `*navigation.Tabs` type switches; `cyclePage(delta)` and
  `debugEnabled()` helpers de-duplicated Tab/Shift+Tab and the env checks.
- `router.go`: 1392 → ~1150 lines.

### Status bar — inspector summary fix
`InspectorModel.StatusLineSummary()` (compact runtime stats) was computed but
never plumbed to the status bar (right segment was hardcoded `""`). Added
`BarModel.SetSummaryProvider`, wired in the router to show the summary only while
the inspector is closed, with a regression test that fails without the fix.

---

## Where we are today

### What's solid

| Area | Notes |
|---|---|
| Charm v2 imports | All `charm.land/…/v2`; no v1 transitive deps |
| Navigation (sidebar + tabs) | Keyboard + mouse; `Dock()`-driven layout; `NavStyleMsg` live |
| Settings page | Overview + single-field huh overlay; `CapturesKeys` gates globals; async save |
| Settings persistence | JSON to per-app config dir; round-trips cleanly |
| Theming | tint × huh preset × mode × accessibility; live preview; `ThemeMsg` propagates |
| Inspector | Message log (dedup+count), runtime stats, status-bar summary; Ctrl+D overlay |
| Notifications | Goroutine-safe; severity + TTL + JSON persistence; runtime-applied settings |
| Overlays | Unified `Overlay` stack (toast/history/inspector/info) via compositor |
| Dark/light auto-detect | `tea.BackgroundColorMsg` handler flips mode live |
| Logger | File-backed; subscriber fan-out to inspector; runtime level |
| Page registration | `RegisteredPage` + `ExtraPages` + `ReplaceAppPagesMsg`; no router edits |
| Per-page keymaps → status bar | `updatePageKeys` + `bubbles/help` |
| Test coverage (core) | Router, nav, inspector, notifications, theme (incl. preset tests) |

### Still open

| Ref | Item | Impact |
|---|---|---|
| Module name | `github.com/jarvisfriends/tui-base` (local-only, no remote) | Intentional today; rename for public import |
| Maintainability | Duplicated page boilerplate, hand-rolled scroll/hit-test (see below) | Tracked as Tiers 1–5 |

---

## Maintainability path forward (Tiers 1–5)

The framework is feature-complete enough; the next investment is **removing
self-maintained code by leaning on the charm v2 libraries already in the tree**
and de-duplicating cross-repo patterns. Prioritized:

### Tier 1 — Shared page scaffolding ✅ DONE (2026-06)
Added embeddable `page.Base` (`tui-base/page`) exposing `Colors()` (nil-safe),
`Width()`, `Height()`, `SetColors` (satisfies `theme.ColorAware`), `SetSize`.
Migrated home, inspector/debug, dash dashboard, and aSettings paths/aliases/
spelling, verify_setup health — deleting the byte-identical boilerplate. Pages
that need side effects override `SetColors`/`SetSize` and call `m.Base.*` first.
`page/page_test.go` covers the fallback/size/ColorAware behavior.

### Tier 2 — Replace hand-rolled list scrolling with `bubbles` (already a dep)
`pages/settings/settings.go` has ~80 lines of manual `scrollTop` /
`ensureCursorVisible` / `visibleItemRange`; inspector has manual `tabScrollY`.
`charm.land/bubbles/v2/list` / `viewport` (already imported) own cursor + scroll +
clipping. Add tests asserting the same selection/scroll behavior.

### Tier 3 — Unify mouse hit-testing geometry ✅ DONE (2026-06)
Added `tui-base/geom` with `Rect{X,Y,W,H}` + `Contains`/`Empty`/`CenteredIn`.
Router `Rect` now aliases `geom.Rect`; settings `overlayX/Y/W/H` and dash
`CellGeometry` (embeds `geom.Rect`) + dash overlay rect all use it with
`Contains`/`CenteredIn` via the v2 `tea.View` OnMouse callback. Status
`ClickRegion` (a 1-D named horizontal span, not a rectangle) intentionally
stays separate. `geom/geom_test.go` covers edges/empty/centering.
> **bubblezone** has a `v2.0.0` tag but misbehaves with the lipgloss
> canvas/compositor we use for overlays — and v2's idiom is the `tea.View`
> OnMouse callback with explicit rects anyway — so the in-house `geom.Rect` is
> the right path, not the library.

### Tier 4 — `lipgloss/v2/table` & `/list` for static rendering (no new dep)
Inspector section/dependency views and dash `display.go` hand-align columns with
width math (fragile with emoji, per layout-guidelines). `lipgloss/v2/table`
renders aligned/bordered tables declaratively. Add golden/width tests.

### Tier 5 — Decompose dash `dashboard.go` god-object (927 lines)
Mixes a 4-mode state machine (View/Edit/Preset/Detail), overlay forms, widget
grid, mouse routing, and focus movement. Split into mode handlers + an overlay
manager (mirroring the router's `Overlay` pattern). Pure structure; add tests for
mode transitions and focus movement to lock behavior before/after.

---

## Architecture notes for multi-repo use

```
my-dashboard/
    main.go     ← router.NewWithRegisteredPages(...); router.NewProgram(m).Run()
    pages/
        inventory/  ← tea.Model page; embeds tui-base BasePage (Tier 1)
        metrics/    ← same pattern
```

Consumers never touch `router/router.go` — only the public API. Settings,
navigation, status bar, theme, overlays, and notifications all "just work". The
only thing consumers write is their page `tea.Model` and optionally a
`config.Section` for custom settings rows.

---

## Comparable projects (ecosystem check)

| Project | Base | Gap vs tui-base |
|---|---|---|
| Orvyn (halsten-dev/orvyn) | BubbleTea v1 | v1 only; no theme registry, settings, notifications, logging, inspector |
| teacup (mistakenelf/teacup) | BubbleTea v1 | v1 only; component lib, no router/settings/themes |
| Charm soft-serve | BubbleTea v2 | App-specific; not a reusable framework |

**Conclusion:** No public bubbletea v2 framework exists with this feature set;
tui-base remains the right thing to build.

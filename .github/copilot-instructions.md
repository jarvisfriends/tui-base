

We are using Go 1.26.5+ (never lower the `go.mod` directive below 1.26.5 — 1.26.4 has a known CVE) with all the latest language features. Ensure that any code you suggest is compatible with this version. We are also using the latest versions of all Charm v2 libraries (`charm.land/…/v2` vanity imports). Verify any new dependency uses v2 vanity paths and does not pull in `github.com/charmbracelet/bubbletea` (the v1 path) as a transitive dependency.

**Any time a new library could replace a homegrown solution, say so first** — describe what the library provides and why it is better before touching code. The human is new to Charm v2; always prefer the simplest, most idiomatic Charm solution.

---

## Session Checklist

1. Build baseline: `go build ./...`
2. Run tests: `go test ./... -v`
3. Run race tests: `go test -race ./... -v`
4. Run lint: `golangci-lint run ./...`
5. Run local verify script: `bash tools/local_verify.sh`
6. TUI code lives under `navigation/`, `pages/`, `router/`, `status/`, `theme/`, `logging/`, `keys/`; the root `tuibase` package is the thin consumer API and `cmd/tui-base/` is the reference app.
7. Check [`.github/ROADMAP.md`](ROADMAP.md) for current tasks and status.
8. Check [`.github/CHARM_ECOSYSTEM.md`](CHARM_ECOSYSTEM.md) for established patterns before writing any UI code.

---

## Design Principles

1. **Air-gap safe.** No telemetry, no cloud, no calls home. Local YAML + on-premise Postgres.
2. **Minimal footprint.** Runs alongside mission-critical software. CPU/RAM must stay flat.
3. **Consistent navigation.** Every element: keyboard-only, mouse-only, and mixed. Arrows navigate, Enter/Space confirm, Esc cancel.
4. **Themed everything.** All colors from the active theme via `theme/theme.go` → `AppColors`. No hardcoded colors anywhere.
5. **Observable internals.** Inspector page always receives all messages; Ctrl+D overlay is planned to make it available from any page.
6. **TDD.** Every feature gets tests. Every bug fix gets a regression test. Run `go test ./... -v`, `go test -race ./... -v`, and `golangci-lint run ./...` before and after every change.
7. **Less code = easier to maintain.** Always prefer an existing Charm v2 library over a custom implementation. Flag any component that is growing toward something Charm already ships.

---

## Package Layout

```
tuibase.go           — root consumer package: Options/RegisteredPage aliases + Run()
cmd/tui-base/        — reference app entry point; creates router, runs tea.Program
router/              — root model; owns nav, pages, status, colors; dispatches all messages
navigation/          — Navigator interface + Sidebar and Tabs implementations
pages/
  home/              — placeholder home page
  inspector/         — inspector: message log + mouse highlight + debug overlay (Ctrl+D)
  settings/          — three-pane settings: Layout | Log | Theme
(keys, geom, datepicker, timepicker, gate, winterm — moved to github.com/jarvisfriends/snap)
status/              — themed status bar with click regions (candidate: bubblestatus)
  statusbar/         — low-level render helpers and region detection
theme/               — AppColors struct + Active() + style helpers
logging/             — file-backed runtime logger with subscriber fan-out
common/              — shared types (currently minimal)
```

---

## Debug Overlay (Ctrl+D)

The **inspector** is a built-in debug overlay accessible via **Ctrl+D** from any page. It displays 7 tabs:

1. **Runtime** — Goroutines, memory (heap/stack/GC), CPU%, GC frequency, uptime, allocation rate
2. **Input** — Last mouse click/release/motion, last key press/release with modifiers
3. **Disks** — Drive usage, free space per mount point
4. **Terminal** — TERM, SSH status, color profile, background color, binary size, uptime
5. **Accessibility** — Screen reader detection (WinScreen, NVDA, JAWS, etc.)
6. **Log** — Application log messages (INFO/WARN/ERROR), searchable, filterable to WARN+
7. **Settings** — pprof server control (enable/disable, set address), CPU/heap profile capture, output directory

### Inspector Architecture

**Key files:**
- `pages/inspector/debug.go` — Main inspector model, tab rendering, refresh loop (statsTickMsg)
- `pages/inspector/render_sections.go` — Row builders for each tab (buildRuntimeRows, buildInputRows, etc.)
- `pages/inspector/disk_windows.go` / `disk_unix.go` — Platform-specific disk stats
- `router/overlay.go` — Overlay lifecycle: ToggleVisible, hit-testing, key/mouse routing

**Metrics collection:**
- `runtimeStatsSnapshot` struct (line ~924 in debug.go) captures all Go runtime metrics
- `collectSnapshot(startTime)` reads runtime.MemStats, goroutines, GC stats, disks
- Refresh loop runs on ~1 Hz cadence via `statsTickMsg`; color-codes thresholds (warn/crit)

**Custom metrics extension:**
Consuming apps implement `inspector.MetricsProvider` (TabName/BuildRows/
RefreshInterval/Start/Stop) and call `router.RegisterInspectorTab(p)`; the tab
appears after the built-ins and the provider runs only while the inspector is
open. See `docs/inspector-extensions.md`.

---

---

## Architecture Notes

Each **page** is a `tea.Model` composed of smaller `tea.Model` sub-components. The
main loop lives in `router/router.go` and routes messages to the active page.

`View()` always returns `tea.View` (not a string). Set `BackgroundColor`,
`ForegroundColor`, and `WindowTitle` on every view. The router sets them for the
terminal canvas; child pages set them for their own content area.

All I/O (file writes, network, DB) must happen inside `tea.Cmd` functions that
return a message on completion. `Update()` must be non-blocking.

Background (non-key/mouse) messages broadcast to every page so inactive pages
receive their command results. High-frequency messages (tickers, progress
streams) should implement `router.TargetedMsg` (`TargetPage() string`, returning
the page's nav ID or title) so only their page is woken.

Mouse events are dispatched manually: the router's `OnMouse` converts global
coordinates to child-relative before calling the child view's `OnMouse`. Every
child that has clickable regions must register an `OnMouse` handler.

Bubble Tea delivers each mouse event to BOTH `View.OnMouse` and `Update`.
While a modal overlay is open it owns the mouse on both paths: wheel events
always go to the overlay (regardless of pointer position, matching keyboard
scrolling), and `RouterModel.Update` drops mouse messages to nav/pages
(`mouseModalOverlayVisible`) so nothing scrolls behind the overlay.

### Shared Color Pointer

The router holds `colors *theme.AppColors`. All children receive this pointer via
`SetColors(c *theme.AppColors)`. On `settings.ThemeMsg` the router does:

```go
tint.SetTintID(msg.ID)
*m.colors = theme.Active()    // mutate in place — all children see it immediately
return m, m.handleResizeCmd() // re-render children at correct size
```

### Logging

Use `github.com/jarvisfriends/tui-base/logging` everywhere in UI code. Never use `fmt.Printf` or `log.Println`
inside models. The logging package fans out to a file (configurable path) and to the
Inspector via registered subscribers. Runtime level is configurable from the Settings
Log pane.

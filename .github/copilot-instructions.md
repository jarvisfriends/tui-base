

We are using Go 1.26+ with all the latest language features. Ensure that any code you suggest is compatible with this version. We are also using the latest versions of all Charm v2 libraries (`charm.land/…/v2` vanity imports). Verify any new dependency uses v2 vanity paths and does not pull in `github.com/charmbracelet/bubbletea` (the v1 path) as a transitive dependency.

**Any time a new library could replace a homegrown solution, say so first** — describe what the library provides and why it is better before touching code. The human is new to Charm v2; always prefer the simplest, most idiomatic Charm solution.

---

## Session Checklist

1. Build baseline: `go build -o tui_base_test_build.exe . && rm tui_base_test_build.exe`
2. Run tests: `go test ./... -v`
3. All TUI code lives in the root package and under `navigation/`, `pages/`, `router/`, `status/`, `theme/`, `logging/`, `keys/`.
4. Check [`.github/ROADMAP.md`](ROADMAP.md) for current tasks and status.
5. Check [`.github/CHARM_ECOSYSTEM.md`](CHARM_ECOSYSTEM.md) for established patterns before writing any UI code.

---

## Design Principles

1. **Air-gap safe.** No telemetry, no cloud, no calls home. Local YAML + on-premise Postgres.
2. **Minimal footprint.** Runs alongside mission-critical software. CPU/RAM must stay flat.
3. **Consistent navigation.** Every element: keyboard-only, mouse-only, and mixed. Arrows navigate, Enter/Space confirm, Esc cancel.
4. **Themed everything.** All colors from the active theme via `theme/theme.go` → `AppColors`. No hardcoded colors anywhere.
5. **Observable internals.** Inspector (debug page) always receives all messages; Ctrl+D overlay is planned to make it available from any page.
6. **TDD.** Every feature gets tests. Every bug fix gets a regression test. Run `go test ./... -v` before and after every change.
7. **Less code = easier to maintain.** Always prefer an existing Charm v2 library over a custom implementation. Flag any component that is growing toward something Charm already ships.

---

## Package Layout

```
main.go              — entry point; creates router, runs tea.Program
router/              — root model; owns nav, pages, status, colors; dispatches all messages
navigation/          — Navigator interface + Sidebar and Tabs implementations
pages/
  home/              — placeholder home page
  debug/             — inspector: message log + mouse highlight (candidate: bubbleinspector)
  settings/          — three-pane settings: Layout | Log | Theme
keys/                — AppKeyMap with all global key bindings
status/              — themed status bar with click regions (candidate: bubblestatus)
  statusbar/         — low-level render helpers and region detection
theme/               — AppColors struct + Active() + style helpers
logging/             — file-backed runtime logger with subscriber fan-out
common/              — shared types (currently minimal)
```

---

## Architecture Notes

Each **page** is a `tea.Model` composed of smaller `tea.Model` sub-components. The
main loop lives in `router/router.go` and routes messages to the active page.

`View()` always returns `tea.View` (not a string). Set `BackgroundColor`,
`ForegroundColor`, and `WindowTitle` on every view. The router sets them for the
terminal canvas; child pages set them for their own content area.

All I/O (file writes, network, DB) must happen inside `tea.Cmd` functions that
return a message on completion. `Update()` must be non-blocking.

Mouse events are dispatched manually: the router's `OnMouse` converts global
coordinates to child-relative before calling the child view's `OnMouse`. Every
child that has clickable regions must register an `OnMouse` handler.

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

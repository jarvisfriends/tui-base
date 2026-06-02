# Deep Analysis — tui-base

> Generated: 2026-05-13. Keep updated as the project evolves.

---

## Goal recap

Build a bubbletea v2 framework that any downstream app can import and get:
navigation, status bar, help, settings (per-app-name scoped), layouts, color themes,
notifications, and a debug view — for free. Consumer only writes `tea.Model` pages.

---

## Where we are today

### What's solid

| Area | Status | Notes |
|---|---|---|
| Charm v2 imports | ✅ | No v1 transitive deps; all `charm.land/…/v2` |
| Shared color pointer | ✅ | Mutated in-place on `ThemeMsg`; zero re-wiring |
| Navigation (sidebar + tabs) | ✅ | Keyboard + mouse; `NavStyleMsg` toggles live |
| Settings page | ✅ | Overview + single-field huh overlay; `CapturesKeys` gates globals |
| Settings persistence | ✅ | JSON to `tui_settings.json`; round-trips cleanly |
| Color theming (bubbletint) | ✅ | Live preview; `ThemeMsg` propagates everywhere |
| Debug / Inspector page | ✅ | Message log, dedup+count, mouse highlight, runtime stats |
| Notification manager | ✅ | Goroutine-safe; severity + TTL + JSON persistence |
| Toast overlay | ✅ | Canvas compositing; auto-dismiss via `expireMsg` |
| Logger | ✅ | File-backed; subscriber fan-out to inspector; runtime level |
| Router | ✅ | Resize, mouse routing, key capture gating, `tea.View` |
| Test coverage (core) | ✅ | Router (9 files), nav (5), debug (4), notifications (7 tests) |

### What's stubbed / incomplete

| Ref | Item | Impact |
|---|---|---|
| SB-5 / NF-4 | `UserNotificationOverlay.View()` returns `"TODO"` | History panel visually broken |
| NF-5 | `NotificationsEnabled`/`NotificationsPersist` saved but not applied at runtime | Settings rows are cosmetic |
| I-4 | Inspector uses `MaxHeight` clipping instead of `bubbles/viewport` | No scroll; old entries lost |
| I-5 / O-2 | Inspector only accessible as a nav page; Ctrl+D overlay not built | Requires leaving current page |
| A-1 | No page registration API | Consumers must hand-edit router internals |
| A-2 | No message bus | Pages can't talk without growing the router switch |
| A-3 | No per-page `help.KeyMap` → status bar wiring | Key hints are global-only |
| T-3 | `tea.BackgroundColorMsg` not handled | Dark/light auto-detect missing |
| S-13 | `SaveToFile` is synchronous | Risk of blocking `Update` when file writes slow |
| S-14 | No keyboard shortcut to open Settings | Users must navigate manually |
| N-5 | Sidebar not migrated to `bubbles/list` | ~200 lines of custom scroll code |
| SB-6 | `bubbles/help` not wired to status bar | Manual key hint string construction |
| keyboard.log | Opened (not written) on every keypress in router | Dead artifact; fix now |
| `keys.Debug` | Defined, exported, shown in help — never matched in router | Dead binding |
| `common.KnownFocusable` | No imports anywhere | Dead code; delete or wire |

### Test gaps

| Package | Tests | Priority |
|---|---|---|
| `logging/` | ❌ | High — goroutine-safe globals with multiple code paths |
| `pages/settings/` | Thin (1 smoke test) | High — most complex page |
| `theme/` | ❌ | Medium — `Active()` fallback paths |
| `keys/` | ❌ | Low |
| `config/` | ❌ | Low — pure data types |
| `common/` | ❌ | Low (but delete it if unused) |

---

## Framework gap analysis

These are the things that must change before this can be imported as a framework by another repo.

### 1. Module name

`module github.com/jarvisfriends/tui-base` in `go.mod` is not importable by other repos. Needs a real path like
`github.com/amarcum/tui-base` (or similar vanity path).

### 2. Page registration API (A-1)

Consumers cannot add pages without modifying `router/router.go`. A simple API:

```go
// router.RegisterPage registers a page before the program starts.
func (m *RouterModel) RegisterPage(id, title string, page tea.Model)
```

Pages then self-register in `init()` or at `main()` time. The router's `Init`
builds nav items from the registry instead of hard-coded slices.

### 3. Per-app settings namespace

`tui_settings.json` is hardcoded. Framework users need:

```go
// Each app calls this before router.New().
router.SetAppName("my-dashboard")   // → ~/.config/my-dashboard/settings.json
```

The settings file path and notification persistence dir should both derive from this.

### 4. Per-page keymaps → status bar (A-3)

Pages should implement an optional interface:

```go
type KeyMapper interface {
    KeyMap() help.KeyMap
}
```

The router checks the active page for this interface and forwards it to the status
bar's `bubbles/help` model (SB-6). This gives free short/long help toggle for each page.

### 5. Layout abstraction

The current layout is hardcoded in `router.go`: `nav | content + statusbar`.
A framework should let the consumer choose a layout preset or inject one. Even a
simple enum (`LayoutSidebar`, `LayoutTabs`, `LayoutFull`) passed to `router.New()`
would cover 90% of use cases.

### 6. Message bus (A-2)

As more pages are added, the router's `Update` switch will accumulate cases for
every cross-page message. A typed pub-sub bus scoped to the router would decouple
pages. Simplest form: a topic-keyed map of subscriber functions, all called inside
the router's `Update`. No external dependency needed.

### 7. Ctrl+D inspector overlay (I-5)

The inspector should be available from **any** page via Ctrl+D. This is the most
important DX feature for framework consumers debugging their own pages.

---

## Comparable projects (ecosystem check)

| Project | Lang / Base | What it covers | Gap vs tui-base |
|---|---|---|---|
| **Orvyn** ([halsten-dev/orvyn](https://github.com/halsten-dev/orvyn)) | Go / BubbleTea v1 | Layout engine, widget system, focus manager, dialog | v1 only; no theme registry, settings, notifications, logging, inspector |
| **teacup** ([mistakenelf/teacup](https://github.com/mistakenelf/teacup)) | Go / BubbleTea v1 | Status bar, file tree, markdown/code bubbles, help | v1 only; component lib, no router/settings/themes/notifications |
| **Charm soft-serve** (in this workspace) | Go / BubbleTea v2 | SSH git server with full TUI | App-specific; not a reusable framework |

**Conclusion:** No public bubbletea v2 framework exists with this feature set.
tui-base is the right thing to build. The two closest projects target v1 and cover
only parts of what we need.

---

## Recommended priority order

### P0 — Must fix before sharing with anyone

1. **Remove `keyboard.log` artifact** in `router.go` (vestigial; opens file on every keypress).
2. **Fix dead `keys.Debug` binding** — either handle it or remove it.
3. **Delete `common.KnownFocusable`** — unused dead code.
4. **Implement `UserNotificationOverlay.View()`** — currently returns a `"TODO"` string visible in the UI.

### P1 — Framework readiness

5. **Rename module** to a real import path.
6. **Page registration API** (`router.RegisterPage`).
7. **Per-app settings namespace** (`router.SetAppName`).
8. **Ctrl+D inspector overlay** (I-5) — the single biggest DX win.
9. **Wire `bubbles/help` to status bar** (SB-6) + per-page `KeyMapper` interface (A-3).

### P2 — Polish and robustness

10. **`bubbles/viewport` in inspector** (I-4) — replaces `MaxHeight` clipping.
11. **Notification keyboard navigation** (NF-4) — history panel is visible but not navigable.
12. **Apply notification settings at runtime** (NF-5).
13. **Dark/light auto-detect** via `tea.BackgroundColorMsg` (T-3).
14. **Async `SaveToFile`** (S-13).
15. **Sidebar → `bubbles/list`** (N-5).

### P3 — Nice-to-have

16. Message bus (A-2).
17. Log rotation / slog migration (L-5, L-6).
18. Custom theme YAML (T-4).
19. Command palette (O-5).
20. Extract packages: `bubbleinspector`, `bubblestatus`, `bubblelog` (see ROADMAP Extraction Candidates).

---

## Architecture notes for multi-repo use

```
my-dashboard/
    main.go          ← router.SetAppName("my-dashboard"); router.New(); tea.NewProgram
    pages/
        inventory/   ← registers itself with router.RegisterPage("inventory", "Inventory", inventory.New())
        metrics/     ← same pattern
```

The consumer never touches `router/router.go` — only calls the public API.
Settings, navigation, status bar, theme, and notifications all "just work".
The only thing consumers write is their page `tea.Model` and optionally a
`config.Section` for custom settings rows.

---

## Quick wins you can ship today

- Remove the `keyboard.log` open-on-keypress line in `router.go`  (5 min, zero risk)
- Delete `common/focusable.go` (5 min, zero risk — no imports)
- Wire `keys.Debug` to open the Inspector page (15 min)
- Add `logging/` tests (1 hr — highest risk untested package)

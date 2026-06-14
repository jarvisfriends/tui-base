# ROADMAP

> Forward-looking backlog only.
> Completed milestones are now documented in README and docs.
> Legend: 🔄 In Progress · 🧪 Needs Tests · ⬜ Planned · 💡 Idea · 📋 Noted

---

## Code Quality Status (2026-05-31)

**Duplication Analysis:** Minimal duplicates detected across 60 Go files (2.73% in `pages/settings/settings.go` with 27 lines; negligible in other files). No refactoring needed at this time.

**Completed Tasks (Current Sprint):**
- ✅ N-6: Default to tabs navigation if config missing
- ✅ S-13: Async SaveToFile via `saveCmd()` + SettingsSavedMsg
- ✅ S-14: OS-aware config paths (UserConfigDir + per-app subdirs)
- ✅ I-4: Replace MaxHeight with bubbles/viewport (keyboard nav + mouse wheel scrolling)
- ✅ Added test coverage for first-run defaults

**Priority Q2 Focus (Remaining):**
- ✅ N-5: Migrate Sidebar to `bubbles/list` with custom delegate
- ✅ Consumer-friendliness: env vars derived from app name (MY_APP_COLOR_PROFILE, MY_APP_DEBUG)
- ✅ DebugKeyMap: rebindable key bindings for debug page actions
- ✅ A-5: Removed `common.KnownFocusable` dead code
- ✅ CI/CD: golangci-lint, Go 1.26.x, actions v5/v6, goreleaser fixes
- ✅ `NewProgramWithEnvVar()`: consumers get branded color profile control
- ✅ `TestHistoryOverlayAllRowsCarryStatusBg`: fixed StatusBg throughout overlay

**Priority Q3 Focus (Remaining):**
1. 🔄 **Inspector improvements** (I-5, I-6, I-7) — Ctrl+D overlay + filtering + export
2. 💡 **Architecture scalability** (A-1, A-2) — Page registry + pub-sub message bus

---

## Navigation

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | N-5 | Migrate Sidebar to `bubbles/list` with custom delegate | Custom `navDelegate` + pinned Settings item |
| ✅ Done | N-6 | Default to tabs navigation if no config found | Fallback when `tui_settings.json` missing or unreadable |

---

## Settings Page

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | S-13 | Settings `SaveToFile` run as `tea.Cmd` (async) | Uses `saveCmd()` + `SettingsSavedMsg`; file write runs in background |
| ✅ Done | S-14 | OS-aware config path for `tui_settings.json` | Router uses `os.UserConfigDir()` on all platforms; falls back to CWD |

---

## Inspector / Debug Page

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | I-4 | Replace `MaxHeight` clipping with `bubbles/viewport` | ScrollUp/Down/PageUp/PageDown; mouse wheel; all tests passing |
| ⬜ Planned | I-5 | Move Inspector to a Ctrl+D overlay (compositor layer) | Available from any page, not just the Inspector tab |
| ⬜ Planned | I-6 | Level-filtered view in inspector (show only WARN+) | Toggle with key while inspector is visible |
| ⬜ Planned | I-7 | Export inspector log to file on demand | e.g., `X` key writes snapshot, then send info user message on where it was successfully written out to |
| ⬜ Planned | I-8 | Show/Hide the Snap packages from the drive view | Filter option in drive view |

---

## Status Bar

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | SB-6 | Add rebindable `DebugKeyMap` for debug page actions | `DebugKeyMap` type + `DefaultDebugKeys()` constructor; eliminates magic key strings |

---

## Overlays (Compositor Layer)

| Status | # | Task | Notes |
|---|---|---|---|
| ⬜ Planned | O-2 | Inspector as Ctrl+D overlay | See I-5 |
| ⬜ Planned | O-3 | Confirmation dialog overlay | For destructive actions |
| 💡 Idea | O-5 | Command palette overlay (Ctrl+P) | Fuzzy find over pages + actions |

---

## Theme & Color

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | T-3 | Handle `tea.BackgroundColorMsg` for dark/light auto-detect | Implemented in `router.Init()` + `Update`; result forwarded to inspector via `TermDiagMsg` |
| 💡 Idea | T-4 | Custom theme authoring (user-defined YAML tints) | Load from `~/.config/tui-base/themes/` |

---

## Logging

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | L-5 | Log rotation / max size cap | Rotates to `<file>.1` past `maxLogBytes` (default 10 MiB); `SetMaxLogBytes(0)` disables; deduped write path; tested |
| 💡 Idea | L-6 | Evaluate migration to `log/slog` | `slog` handlers compose cleanly; keeps dep graph minimal |

---

## Scalability & Architecture

| Status | # | Task | Notes |
|---|---|---|---|
| ⬜ Planned | A-1 | Page registry / dynamic page registration | `router.RegisterPage(pageTitle, model)` so new pages self-register |
| ⬜ Planned | A-2 | Message bus / pub-sub between pages | Decouples pages; avoids growing router switch cases |
| ✅ Done | A-3 | Per-page keymaps auto-appearing in status help | `router.updatePageKeys()` detects a page's `help.KeyMap` and pushes it to the status bar via `SetPageBindings` on every page switch |
| 💡 Idea | A-4 | Plugin / extension point for external pages | Go plugin API or embedded Lua for extensibility |
| ✅ Done | A-5 | Evaluate `common.KnownFocusable` and `common.Component` | Removed `KnownFocusable` as dead code; `Focusable` interface preserved |

---

## Tooling & CI

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | TL-4 | Add `.golangci.yml` lint config | Enables `govet`(+shadow), `staticcheck`, `unused`, `gosimple`, `errcheck`, `ineffassign`; config kept aligned with current golangci-lint schema |

## aSettings Integration Follow-ups

| Status | # | Task | Notes |
|---|---|---|---|
| ⬜ Planned | AS-8 | Add deeper interaction tests for aSettings pages | Basic selection and `pages/ui` badge/helper tests are in place; remaining scope is rendering snapshots, key-flow edge cases, and rescan behavior |
| 📋 Noted | AS-9 | `pages/ui/badges.go` hardcoded Catppuccin palette is intentional | Keep as categorical badges, not semantic theme colors |

---

## Salvaged from the `charming` playground (2026-06)

> `charming` was the original charm-library playground that this repo + `dash` were
> extracted from; it is being archived (git tag, read-only). These are the non-OBE
> ideas/decisions pulled out of `charming/Next_comments_questions.md` and its code so
> nothing is lost when it's archived. Items whose code still lives only in `charming`
> point at the archive tag to port from. (Source: consolidation-plan/05-charming-salvage.md.)

| Status | # | Task | Notes |
|---|---|---|---|
| ⬜ Planned | KB-1 | Runtime **keyboard-paradigm swap** setting (arrow-centric ↔ vim) | Swap the entire keymap set without code changes; **default arrow-centric** (decided). Ties into the focus-model work (consolidation-plan/06) |
| ⬜ Planned | HB-1 | **Expose the help bar's rendered height** so layouts query it instead of guessing | Directly supports the no-magic-numbers goal (D6); blocks clean layout math |
| 💡 Idea | PP-1 | **Preview Picker** component: `bubbles/filepicker` (left ⅓) + `bubbles/viewport` (right, file contents) | Reusable input/output selector; showcases two bubbles components for ~no custom code. Verify no equivalent exists before building |
| 💡 Idea | FW-1 | **Filewatch helper** (`fsnotify` → `tea.Cmd`) for live-reloading views | Clean 65-line helper in `charming/internal/ui/filewatch/filewatch.go` (archive); port if/when a consumer needs live reload |
| ⬜ Planned | L-7 | Change **default log level to ERROR** (currently INFO / `minLevel=1`) + expose as a setting | `charming` decided ERROR-by-default unless changed in settings/config |

**Confirmed OBE (already done here — discarded from `charming`):** full-screen pages +
help + title + mouse/keyboard nav; page-per-folder model with child message routing;
`bubbles/key` keymaps; OS-aware config path (UserConfigDir + app name); SetWindowTitle;
**default-terminal selector (now in the Settings page)**; log rotation (L-5);
alt-screen (v2 supports it natively).

---

## Conformance harness (2026-06)

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | CF-1 | Shared conformance checks in `testutil` | `CheckFitsViewport`, `CheckStatusBarVisible` (+ `StatusProvider`/`RouterModel.StatusBarContent`), `CheckThemeResponsive`; reference test `router/conformance_test.go`; doc `docs/conformance.md`. Derived apps run them against their own router (proven in `media/tui`). |
| ⬜ Planned | CF-2 | **Inspector "Conformance" tab** — runtime pass/fail | Run the same checks against the live `RouterModel` and show a per-check pass/fail list, so engineers see conformance in-app (complement to the unit tests). |
| ⬜ Planned | CF-3 | **Border-count invariant** check | Make a page's section-border symbol configurable, then assert the per-line border-char count is stable across resizes (a sharper signal for line-wrapping bugs than overflow alone). |

## Themed charm components (2026-06)

> Goal: every fancy element draws its colors from the active theme, and we use the
> animated charm widgets rather than hand-rolled ones (most features, least code).

| Status | # | Task | Notes |
|---|---|---|---|
| ⬜ Planned | TC-1 | Theme helpers for `bubbles` styles | Add `theme` accessors returning themed `table.Styles`, `list` delegate styles, `spinner.Style`, and a `progress` gradient/solid from the active `AppStyle` (Accent/Success/Subtle). Today `bubbles/table` in consumers uses default (un-themed) styles. |
| ⬜ Planned | TC-2 | **Animated progress** via `bubbles/progress` | Provide a themed, animated progress bar; replace hand-rolled block bars (e.g. `media/tui` Overview) with it. |
| ⬜ Planned | TC-3 | Apply TC-1/TC-2 across consumers | Wire themed table/list/spinner/progress into `media/tui`, `dash`, and migrations so they all match the theme and recolor on `ThemeMsg` (guarded by `CheckThemeResponsive`). |
| 📋 Noted | TC-4 | Huh forms already themed | `theme.HuhThemeFunc()` themes forms; keep using it (no work — listed for completeness). |


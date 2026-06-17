# ROADMAP

> Forward-looking backlog only.
> Completed milestones are now documented in README and docs.
> Legend: 🔄 In Progress · 🧪 Needs Tests · ⬜ Planned · 💡 Idea · 📋 Noted

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

## Theme & Color

| Status | # | Task | Notes |
|---|---|---|---|
| 💡 Idea | T-4 | Custom theme authoring (user-defined YAML tints) | Load from `~/.config/tui-base/themes/` |

## Logging

| Status | # | Task | Notes |
|---|---|---|---|
| 💡 Idea | L-6 | Evaluate migration to Uber `zap` logging lib | fast and has structured logging capabilities |

## Scalability & Architecture

| Status | # | Task | Notes |
|---|---|---|---|
| 💡 Idea | FW-1 | **Filewatch helper** (`fsnotify` → `tea.Cmd`) for live-reloading views | Clean 65-line helper in `charming/internal/ui/filewatch/filewatch.go` (archive); port if/when a consumer needs live reload |

## Conformance harness (2026-06)

| Status | # | Task | Notes |
|---|---|---|---|
| ⬜ Planned | CF-2 | **Inspector "Conformance" tab** — runtime pass/fail | Run the same checks against the live `RouterModel` and show a per-check pass/fail list, so engineers see conformance in-app (complement to the unit tests). |
| ⬜ Planned | CF-3 | **Border-count invariant** check | Make a page's section-border symbol configurable, then assert the per-line border-char count is stable across resizes (a sharper signal for line-wrapping bugs than overflow alone). |

## Themed charm components (2026-06)

> Goal: every fancy element draws its colors from the active theme, and we use the
> animated charm widgets rather than hand-rolled ones (most features, least code).

| Status | # | Task | Notes |
|---|---|---|---|
| ⬜ Planned | TC-1 | Theme helpers for `bubbles` styles | Add `theme` accessors returning themed `table.Styles`, `list` delegate styles, `spinner.Style`, and a `progress` gradient/solid from the active `AppStyle` (Accent/Success/Subtle). Today `bubbles/table` in consumers uses default (un-themed) styles. |


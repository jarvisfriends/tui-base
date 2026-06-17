# ROADMAP

> Forward-looking backlog only.
> Completed milestones are now documented in README and docs.
> Legend: 🔄 In Progress · 🧪 Needs Tests · ⬜ Planned · 💡 Idea · 📋 Noted

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


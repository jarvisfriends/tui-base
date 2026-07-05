# ROADMAP

> Forward-looking backlog only. Completed milestones are documented in the README,
> `docs/architecture-decisions.md`, and `docs/`.
>
> Consolidated 2026-07-01 from the AI code reviews (`.agent/code_review/`),
> `.github/ANALYSIS.md`, and `.agent/QUALITY_CHECKLIST.md` — all since removed.
> Every item below was re-verified against the codebase on that date.
>
> Legend: 🔄 In Progress · 🧪 Needs Tests · ⬜ Planned · 💡 Idea · 📋 Noted · ❓ Blocked on a human answer

---

## ❓ Questions for the human

Answers here unblock the tagged tasks below. Recommendations included where one exists.

| # | Question | Options / Recommendation | Unblocks |
|---|---|---|---|
| Q-10 | **Component extraction shape: one shared "extended bubbles" repo, or one repo per capability?** (Your open debate from the extraction section below.) | Repo-per-capability gives independent update cycles; a single repo is less overhead. Either way each component gets a VHS `.tape` demo. | X-1…X-4 |
- HUMAN: one jarvis-bubbles repo with all the unique components, Create folders to organize each category over time. For instance, the navigation component folder should have 3 additional folders, one for each type that we could swap between (Tabs, Sidebar, Minimal Top).
| Q-11 | **One-time manual pass: does the VS Code workspace setup behave as intended?** (format-on-save via gofumpt, 2-space JSON/YAML, "go: full verify" task, Error Lens inline lint, Delve race debug config, "go: api compat check" task.) | Run through once and confirm; this is the remainder of the old `.agent/TODO.md`. | — |
- HUMAN: Yes, I believe it does
| Q-12 | **A-4 scroll refactor — keep or replace the settings overview scroller?** The inspector already scrolls via `bubbles/viewport` (`tabScrollY` is only per-tab scroll *memory*), so the settings overview is the last hand-rolled scroller — but it is a multi-**column**, category-grouped, entry-windowed layout. `bubbles/list` is single-column with its own chrome; a raw viewport trades entry-snapping for line scrolling. | Recommend closing A-4 as "won't do for settings; inspector already compliant". | A-4 |
| Q-13 | **CF-2 conformance tab: confirm the snapshot design.** The inspector renders *inside* `router.View`, so a tab checking the live frame would recurse. Sketch: the router records frame metrics (size fit, status-bar presence, border counts) at the end of `View`; a built-in `MetricsProvider` tab reports pass/fail from that snapshot. | Confirm that design — or drop CF-2 now that the unit-level conformance checks cover the same invariants. | CF-2 |
| Q-14 | **FW-1 filewatch: OK to add the `fsnotify` dependency?** The archived charming reference isn't in this repo, so it's a fresh ~80-line `fsnotify → tea.Cmd` helper plus one new direct dep. | Yes/no on the dep; implementation is straightforward once approved. | FW-1 |
- HUMAN: Yes on the dependency and the settings change notifications
| Q-15 | **T-4 YAML themes: pick the YAML dep and schema.** Needs `goccy/go-yaml` vs `gopkg.in/yaml.v3`, and a schema decision: full 16-slot terminal tint vs the smaller semantic `AppStyle` slots. | Recommend goccy/go-yaml + full tint schema (drops straight into bubbletint's registry). | T-4 |
| Q-16 | **Priority order for the remaining feature ideas?** Effort estimates: I-1 lifecycle hooks (S), I-3 confirm-modal service (M), I-4 persistent layout state (M), I-5 capability detection (M), I-2 command palette (L), I-6 panic overlay (M — riskiest: `recover()` inside the tea loop). | Pick the order (or "none for now") and I'll work down the list. | I-1…I-6 |
| Q-17 | **I-8 release hardening: cosign keyless (OIDC) or a managed signing key?** Keyless is zero-secret and recommended; either way goreleaser + workflow permissions need a decision. | Confirm keyless and I'll wire SBOM + signing into the release workflow. | I-8 |
| Q-18 | **I-9 i18n: recommend rejecting.** Message catalogs bloat every consumer, and a framework's user-facing strings are mostly consumer-owned anyway. | Confirm to close as rejected. | I-9 |
| Q-19 | **A-8 cyclomatic-complexity linter: enable now or after the big files shrink?** Enabling `cyclop`/`gocyclo` today flags `router.go` and inspector `debug.go`; a high threshold (~25) would pass but adds little until those files are split. | Recommend enabling at threshold 25 now and ratcheting down later. | A-8 |

### ✅ Answered (2026-07-01)

| # | Question | Decision |
|---|---|---|
| Q-1 | Public release goal? | Already public at **v0.2.1**. v1.0 = "no more changes wanted"; until then breaking improvements are fine. Library-readiness items stay a priority band. |
| Q-2 | Module import path? | `github.com/jarvisfriends/tui-base` is final. Consider a top-level wrapper around `router`, or moving router to the root under a new name (see LR-1). |
| Q-3 | `testutil` export? | **Keep exported** — consumers run `CheckCodeStandards`; the `golang.org/x/tools` dep is accepted. LR-2 resolved. |
| Q-4 | `go.mod` directive? | **`go 1.26.4` (or above) is required** — 1.26.3 has a CVE. Never suggest downgrading to bare `go 1.26`. LR-3 resolved. |
| Q-5 | Page registry pattern? | **No `init()` functions anywhere in the library.** Keep globals as small as possible; prefer passing values. Shared pointers must be thread-safe before being handed out. See A-7. |
| Q-6 | Component interface shape? | **Stay monolithic.** Goal is ease of getting a full app running — excitement over terminal UIs, not minimal interfaces. Closed. |
| Q-7 | Coverage strategy? | Keep the 55% floor **and** add Codecov/Coveralls trend tracking (CI-7; needs the human to create the account/token — reminder lives there). |
| Q-8 | Inspector extensibility API? | **Yes** — registerable inspector tabs + pluggable metrics providers. Requirements: a scalable API; every provider must be startable **and** stoppable; developer-facing docs making clear it is a developers-only extension surface. Drives E-5. |
| Q-9 | `lipgloss.JoinHorizontal/Vertical` deprecated? | **No** — not deprecated; recommended throughout the Charm v2 libraries. The Gemini claim was wrong. No migration. Closed. |

---

## Now — bugs & correctness (do first, independent of any answer)

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | B-1 | **Router discards returned `tea.Model`** from `Update()` on children/inactive pages | Fixed 2026-07-01: `updatePage`/`updateNav`/`updateInspector`/`updateStatus` helpers store returned models at every site (router, overlays, huh forms, inspector panel). Regression test: `router/model_swap_test.go`. |
| ✅ Done | B-2 | **Atomic config writes** | Fixed 2026-07-01: new `common.WriteFileAtomic` (temp file + rename, fsync'd) used by settings save (async + sync) and `notifications.writeFileLocked`. Tests in `common/fileutil_test.go`. |
| ✅ Done | B-3 | **`logging.notify` runs subscribers synchronously under `subsMu.RLock()`** | Fixed 2026-07-01: subscriber slice is snapshotted under the lock, invoked outside it. Regression test: `TestNotifySubscriberCanRegisterSubscriber`. |

## Library readiness

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | LR-1 | Move `main.go` to `cmd/tui-base/` | Done 2026-07-03: entry point moved to `cmd/tui-base/`; new root `tuibase` package (per Q-2) exposes `Run(Options)`, `New`, `NewWithOptions`, and the `Options`/`RegisteredPage` aliases so consumers need one import. goreleaser/CI/tasks/docs updated. |
| ✅ Done | LR-2 | Resolve `testutil` export vs `internal/` | Per Q-3: stays exported; `x/tools` dep accepted. |
| ✅ Done | LR-3 | Settle `go.mod` version directive | Per Q-4: `go 1.26.4`+ is mandatory (CVE in 1.26.3). Guard against downgrades. |
| ✅ Done | LR-4 | `doc.go` for every public package | Done 2026-07-03: doc.go added for router, navigation, status, logging, keys, gate, common, envpath, timepicker, pages/{home,inspector,settings}; the rest (theme, notifications, page, testutil, overlay, geom, config, table, datepicker, tuibase) already carried package comments on their main file. |
| ✅ Done | LR-5 | `Example*` test functions | Done 2026-07-03: `ExampleRun`, `ExampleNewWithOptions`, `ExampleRouterModel_RegisterPage` (compile-checked — running needs a TTY), `ExampleBase_Colors`, `ExampleActive`, `ExampleManager_Add` (executed with Output). |
| 🔄 In Progress | LR-6 | Godoc comments on exported symbols | Package docs now exist everywhere (LR-4) and new APIs ship documented; the backlog is older exported symbols. Plan: enable revive's `exported` rule per-package and burn down, starting with `navigation`, `router`, `theme`, `page`, `keys`; flip the global rule on (`.golangci.yml` note) when clean. |
| ✅ Done | LR-7 | Community files: `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `CHANGELOG.md`, `CODEOWNERS`, issue + PR templates | Done 2026-07-03: all created (CHANGELOG in Keep-a-Changelog format with the Unreleased section populated; issue forms ask for terminal size + inspector diagnostics). Keep CHANGELOG updated per release. |
| ✅ Done | LR-8 | `examples/` directory with 2–3 small consumer apps | Done 2026-07-03: `examples/minimal` (one call), `examples/dashboard` (custom page, themed bubbles table, status segment, notification), `examples/multipage` (pages + custom settings section + RunContext graceful shutdown). All build/vet in CI via `./...`. |
| ✅ Done | LR-9 | Consumer docs: compatibility matrix (Go/OS/terminals), theme cookbook, `go.work` dev setup, migration-from-plain-bubbletea guide | Done 2026-07-03: `docs/compatibility.md`, `docs/theme-cookbook.md`, `docs/migration-from-bubbletea.md` (includes the go.work recipe). |

## CI/CD

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | CI-1 | **Pin tool versions** | Done 2026-07-03: golangci-lint v2.12.2 (lint action + coverage job), stringer v0.47.0 (ci + release), govulncheck v1.5.0, goreleaser v2.16.0. `uses:` actions stay dependabot-managed; the `go install` pins need manual bumps (comments mark them). |
| ✅ Done | CI-2 | Add macOS to CI matrix | Done 2026-07-03: `macos-14` added to the verify matrix (build + race tests + vet). |
| ✅ Done | CI-3 | Wire fuzz tests into CI | Done 2026-07-03: `fuzz-smoke` job runs both targets 30 s each via `tools/fuzz.sh` (FUZZTIME env; script's stale `pages/debug` path fixed). Committing a `testdata/fuzz/` corpus stays open as a nice-to-have. |
| ✅ Done | CI-4 | Wire benchmarks into CI | Done 2026-07-03: `bench` job runs `-bench . -benchmem` and uploads `bench.txt` artifacts; informational only (shared runners too noisy for hard gates). |
| ✅ Done | CI-5 | Add CodeQL scanning | Updated 2026-07-05: GitHub CodeQL **default setup** is the active scan path for this repo; the checked-in advanced workflow was removed because GitHub rejects advanced-setup SARIF uploads while default setup is enabled. |
| ✅ Done | CI-6 | Fix lychee link-check flakiness | Done 2026-07-03: GITHUB_TOKEN env + `--max-retries 3 --retry-wait-time 5`. Add a `.lycheeignore` later only if specific hosts keep flaking. |
| 🔄 In Progress | CI-7 | Coverage: reduce job redundancy + add trend/badge | Wired 2026-07-03: Coveralls + Codecov upload steps in the coverage job, Coveralls badge in the README, redundant test run + lint dropped (`SKIP_LINT=1` for the script). 55% floor kept. **🔔 Human action required:** add the `COVERALLS_REPO_TOKEN` repo secret (value provided out-of-band; consider rotating it) and a `CODECOV_TOKEN` secret if Codecov should authenticate. |
| ✅ Done | CI-8 | `goreleaser release --snapshot` on PRs/CI | Done 2026-07-03: `release-snapshot` job builds the full matrix with `--snapshot --skip=publish` on every push/PR. |
| ✅ Done | CI-9 | Add Windows-GOOS lint to `tools/local_verify.sh` | Done 2026-07-03: the script lints GOOS=windows and GOOS=linux in subshells, matching CI. |
| ✅ Done | CI-10 | One-time local `govulncheck ./...` clean-pass verification | Verified 2026-07-03: "No vulnerabilities found." (govulncheck v1.5.0, go 1.26.4). |
| 💡 Idea | CI-11 | PR size labels (XS–XL); conventional-commit title enforcement | Nice-to-haves for review flow / changelog. |

## Architecture & code quality

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | A-1 | **Bound background message fanning** | Done 2026-07-03: `router.TargetedMsg{ TargetPage() string }` fast path — targeted background messages wake only the page whose nav ID/title matches; everything else keeps broadcasting. Test: `TestTargetedMsgWakesOnlyItsPage`. |
| ✅ Done | A-2 | `tea.WithContext` support for graceful shutdown | Done 2026-07-03: `router.NewProgramWithContext(ctx, m, envVar, opts...)` and `tuibase.RunContext(ctx, opts)` — cancel the context (e.g. `signal.NotifyContext`) for a SIGTERM-clean exit. Test: `TestNewProgramWithContextCancelsRun`. |
| ✅ Done | A-3 | Theme the datepicker/timepicker | Done 2026-07-03: components stay theme-free (extraction candidates) but expose style hooks (`datepicker.Styles`, timepicker `Active/Inactive/HelpStyle`); settings maps the active theme onto them (`picker_theme.go` — selection colors for focus, accent for the active segment). The small editor overlays (DirPicker/MultiFileEditor/KeyRecorder) now use theme styles too. |
| ❓ | A-4 | **Tier 2: replace hand-rolled scrolling with `bubbles` list/viewport** | Re-analyzed 2026-07-03: the inspector already scrolls via `bubbles/viewport`; only the settings overview scroller is hand-rolled, and its multi-column entry-windowed layout doesn't map onto `bubbles/list`. Decision per Q-12 (recommend: close as won't-do). |
| ✅ Done | A-5 | **Tier 4: display-width column rendering** | Done 2026-07-03: the info modal's dependency table was the last byte-padded (`%-50s`) column layout — now display-cell padded (lipgloss `Width` cells + ANSI-aware tail truncation). `renderRuntimeFlat` audited: already width-based. Full `lipgloss/table` adoption judged unnecessary for these two-column views. |
| ✅ Done | A-6 | Small audits: all key handling via `key.Binding` (no `.String()==` compares); `tea.Batch` for multi-cmd returns; `ReapplyBg` on every `View()` that needs it | Done 2026-07-03: last two `.String()==` compares removed (redundant shift+tab clause; KeyRecorder save now Code+Mod, tests use the real key form). Multi-cmd sites all batch via `cmds` slices + `tea.Batch` (verified during the sweep). `ReapplyBg` covers both composition roots (router layout, status help); other surfaces are protected by the themed `View.BackgroundColor`. |
| ✅ Done | A-7 | **Eliminate `init()` functions; shrink globals** | Per Q-5. Verified zero `init()` functions in the library. Removed the global `RegisterPage()`/`RegisteredPages()`/`ClearRegisteredPages()` registry (2026-07-01) — construction-time pages go through `Options.ExtraPages`/`NewWithRegisteredPages`; runtime adds through the `(*RouterModel).RegisterPage` method. Remaining globals (theme cache, logging state) are mutex-guarded. |
| ❓ | A-8 | Add `gocyclo`/`cyclop` linter; periodically review new golangci-lint linters (`default: none` hides new ones) | Threshold decision via Q-19 (recommend enable at 25, ratchet down). |
| 💡 Idea | A-9 | `context.Context` in notification TTL, config I/O, logging fan-out | Partially covered 2026-07-03: `RunContext`/`NewProgramWithContext` bound the program lifetime (A-2). Remaining idea is plumbing ctx into TTL timers and config I/O — still gated on Bubble Tea's loop not passing contexts. |
| 📋 Noted | A-10 | Tier 5 (consumer repo): decompose dash `dashboard.go` god-object (927 lines) into mode handlers + overlay manager mirroring the router `Overlay` pattern | Lives in the dash repo, tracked here so the tier list stays complete. |

## Testing

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | TS-1 | **Golden-file / snapshot rendering tests** | Done 2026-07-03: `testutil.Golden` (UPDATE_GOLDEN=1 to regenerate) + goldens for the three navigators (incl. overflow window), status bar, and history panel. Add more surfaces as they stabilize. |
| ✅ Done | TS-2 | Unicode torture tests | Done 2026-07-03: CJK/ZWJ/flag/fullwidth titles through tabs, top nav (contiguous click ranges), sidebar, status bar, and history panel. |
| ✅ Done | TS-3 | Tiny-terminal tests | Done 2026-07-03: router driven at 40×10, 60×20, 80×24 across page switch + inspector overlay (`TestTinyTerminals`). |
| ✅ Done | TS-4 | Architectural dependency enforcement | Done 2026-07-03: `testutil.CheckNoImports` + root `TestArchitectureLayering` (foundations import nothing internal; mid-layer never imports router/pages/status/navigation; pages/status never import router). |
| ⬜ Planned | TS-5 | `teatest` integration tests | Drive the full app through `tea.Program`, assert terminal output. |
| ❓ | CF-2 | **Inspector "Conformance" tab** — runtime pass/fail | Blocked on the Q-13 design confirmation (frame-snapshot provider; naive live checks would recurse through `router.View`). |
| ✅ Done | CF-3 | **Border-count invariant** check | Done 2026-07-03: `testutil.CheckBorderIntegrity` (model, all standard sizes) + `CheckBorderIntegrityString` (prerendered overlays) — every bordered line must carry exactly two edge glyphs; the glyph is a parameter for content that legitimately contains `│`. Applied to the inspector box and history panel. |
| 💡 Idea | TS-6 | Theme fuzzing (randomized fg/bg/border/accent — never panic); overlay-stacking + mouse-routing integration tests | |

## Extensibility (consumers customize without forking)

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | TC-1 | Theme helpers for `bubbles` styles | Done 2026-07-03: `theme.TableStyles` (inspector now delegates to it — one visual source of truth), `theme.ListDelegateStyles`, `theme.SpinnerStyle`, `theme.ProgressGradient` (`theme/bubbles.go`). |
| ✅ Done | E-5 | `RegisterInspectorTab` / `MetricsProvider` interface | Done 2026-07-03 per Q-8: `inspector.MetricsProvider` (TabName/BuildRows/RefreshInterval/Start/Stop, idempotent lifecycle tied to inspector visibility) + `router.RegisterInspectorTab`/`RemoveInspectorTab`; custom tabs join every switching affordance. Developer docs: `docs/inspector-extensions.md`. |
| ✅ Done | E-1 | `StatusSegmentProvider` — consumer status-bar widgets (git branch, connection state) | Done 2026-07-03: `status.BarModel.SetSegment(name, fn)` + `router.SetStatusSegment` passthrough — named right-aligned segments in registration order, re-evaluated per render, "" skipped, nil removes. |
| ✅ Done | E-2 | `RegisterOverlay(Overlay)` — expose the router overlay stack | Verified 2026-07-03: `router.RegisterOverlay` was already exported; `TestRegisterOverlayExternal` now proves the full consumer contract (composited, key-modal while open, releases input on close). |
| ✅ Done | E-3 | Runtime key remapping (`KeyConfig` layer) | Verified 2026-07-03: already shipped — Settings → Keybindings (KeyRecorder) → `keys.ApplyCustomizations` → `KeybindingsChangedMsg` re-syncs status/info modal/inspector at runtime, persisted via `custom_keys`. Covered by `keys_customization_test.go`. |
| ✅ Done | E-4 | Notification action handlers — `OnAction(key, handler)` | Done 2026-07-03: `notifications.Manager.OnAction(key, func(Notification) tea.Cmd)` — fires on the selected history row; built-in panel keys take precedence; nil removes. |
| 💡 Idea | E-6 | Pluggable `Navigator` implementations (breadcrumbs, command palette) | |
| ✅ Done | E-7 | Message middleware — `Use(func(tea.Msg) tea.Msg)` for analytics/logging/gating | Done 2026-07-03: `router.Use` — runs in order at the top of Update; return nil to gate, a different msg to translate. Documented caveat: don't swallow infrastructure messages. |

## Component extraction to other repos (gated on Q-10)

> These can have independent update cycles and could benefit other projects.
> Whichever shape wins Q-10: set up a VHS `.tape` config per component to show off
> why someone should use it vs. the alternatives.

| Status | # | Candidate | Notes |
|---|---|---|---|
| ❓ | X-1 | `navigation` — Tabs, Sidebar, Slim/topnav | Started from other examples and expanded. Sidebar is close to `bubbles/list` with a custom delegate — consider migrating before extraction to thin the key/mouse logic. |
| ❓ | X-2 | `pages/inspector` → `bubbleinspector` | Prime candidate; deps are only `bubbletea/v2` + `lipgloss/v2`. |
| ❓ | X-3 | `logging` → `bubblelog` | Could grow a ring buffer, level histogram, export interface. |
| ❓ | X-4 | `status` bar | Remove the hard-coded notification seed before extracting. |
| 🔄 In Progress | X-5 | VHS gif creator with `.tape` config for the main tui-base app | Tape written 2026-07-03 (`tools/demo.tape`: pages, settings, inspector tour). **🔔 Human action:** install `vhs` and run `vhs tools/demo.tape` once to render/verify the gif (not installed in the dev environment). |

## Ideas (unranked)

| Status | # | Task | Notes |
|---|---|---|---|
| ❓ | T-4 | Custom theme authoring (user-defined YAML tints) | Load from `~/.config/tui-base/themes/`. Blocked on Q-15 (YAML dep + schema). |
| ✅ Done | L-6 | Evaluate migration to Uber `zap` logging lib | Evaluated 2026-07-03 — **recommend rejecting**: zap optimizes high-throughput structured logging; this logger is low-volume and UI-bound, already rotates files, and its framework value is the subscriber fan-out. If structured logging becomes a consumer ask, stdlib `log/slog` (zero dep, expose a `slog.Handler`) is the path. Veto welcome. |
| ❓ | FW-1 | **Filewatch helper** (`fsnotify` → `tea.Cmd`) for live-reloading views | The archived charming reference isn't in this repo — it's a fresh helper plus a new dep. Blocked on Q-14. |
| ❓ | I-1 | Page lifecycle hooks (`OnEnter`, `OnLeave`, `OnResize`, `OnThemeChange`) | Prioritize via Q-16 (effort: S). |
| ❓ | I-2 | Command palette (Ctrl+P) | Prioritize via Q-16 (effort: L). |
| ❓ | I-3 | Modal confirmation service (router-owned, `ConfirmMsg`) | Prioritize via Q-16 (effort: M). |
| ❓ | I-4 | Persistent layout state (sidebar width, tab order, last page) | Prioritize via Q-16 (effort: M; nav style already persists). |
| ❓ | I-5 | Terminal capability detection beyond background color (truecolor, kitty graphics) | Prioritize via Q-16 (effort: M). |
| ❓ | I-6 | Global error overlay — panic → inspector with stack | Prioritize via Q-16 (effort: M; riskiest — `recover()` inside the tea loop). |
| ✅ Done | I-7 | Configurable ring-buffer size for inspector message dedup | Done 2026-07-03: `inspector.SetLogCapacity(n)` (floor 10, 0 restores the 50 default); both dedup paths trim through one helper. |
| ❓ | I-8 | Supply-chain hardening at release: SBOM, cosign, provenance attestations | Blocked on Q-17 (keyless vs managed key). |
| ❓ | I-9 | i18n keys in user-facing strings | Recommend rejecting — confirm via Q-18. |

The main Settings page is great, the Inspector settings page should look like the main one

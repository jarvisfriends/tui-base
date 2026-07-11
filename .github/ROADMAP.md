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


### ✅ Answered (2026-07-01, updated 2026-07-09)

| # | Question | Decision |
|---|---|---|
| Q-25 | rendercheck name? | **Answered 2026-07-09 (human): `rendercheck` confirmed.** Closed. |
| Q-16 | Feature-idea priority? | **Answered 2026-07-09 (human): do all of them**, lower number = higher priority (I-1 → I-6). |
| Q-17 | Release signing? | **Answered 2026-07-09 (human): cosign keyless (OIDC)** — wire SBOM + signing into the release workflow (I-8). |
| Q-18 | i18n? | **Answered 2026-07-09 (human): rejected.** I-9 closed. |
| Q-19 | Cyclomatic-complexity linter? | **Answered 2026-07-09 (human): enable at a high threshold and gradually reduce.** A-8 unblocked. |
| Q-12 | Settings overview scroller? | **Answered 2026-07-09 (human): keep the existing scroller** — it works and has nice features. A-4 closed as won't-do (inspector already viewport-based). |
| Q-13 | CF-2 conformance tab design? | **Answered 2026-07-09 (human): drop CF-2** — the unit-level conformance checks are the better approach. Closed. |
| Q-15 | T-4 YAML dep + schema? | **Answered 2026-07-09 (human): `gopkg.in/yaml.v3` + the full tint schema.** T-4 unblocked. |
| Q-21 | Program-options API shape (SP-11)? | **Answered 2026-07-09 (human): both** — keep the `Options` struct *and* add variadic `WithXXX` functional options. Implemented as `Run(opts, options...)` / `RunContext` / `NewWithOptions` gaining a source-compatible variadic tail plus `tuibase.New(options ...Option)`; options apply on top of the struct, defaults fill anything unset. |
| Q-22 | Inspector standalone API (SP-12)? | **Answered 2026-07-09 (human):** the independent inspector ships a `cmd/` main that runs it under snap's minimal-top nav; as a library it is delivered as a plain `tea.Model`. tui-base adds `WithDebugOverlay(tea.Model)` — the stored model becomes the Ctrl+D overlay, and tui-base controls Ctrl+D whenever the pointer is non-nil. Depends on SP-5 (minimal-top must live in snap first). |
| Q-23 | Pickers move timing (SP-7)? | **Answered 2026-07-09 (human):** the dir/file-picker WIP is committed — SP-7 unblocked. |
| Q-24 | Timepicker redesign model (SP-8)? | **Answered 2026-07-09 (human):** two columns with a highlight color on the colon; clicking either side opens a scrollable dropdown; Enter or click commits mouse-set numbers; validate when leaving the field. |
| Q-20 | snap dependency direction? | **Answered 2026-07-09 (human):** `snap` and `inspector` are both public on GitHub; tui-base imports them back — one source of truth. The old `jarvis-bubbles` repo was renamed to `snap` ("Jarvis Friends Snap"). Sequencing constraint: tui-base's `go.mod` can only reference *pushed, tagged* snap releases (a public library must not carry `replace` directives), so each move lands in snap first, gets tagged, then tui-base flips imports. |
| Q-10 | Component extraction shape? | **One `snap` repo** holding all the unique components, organized into category folders that grow over time — e.g. `navigation/` with one folder per swappable style (`tabs/`, `sidebar/`, `minimal-top/`). Each component gets a VHS `.tape` demo. Scaffolded 2026-07-07; dependency direction answered 2026-07-09 (Q-20). The inspector now goes to its **own repo** (`jarvisfriends/inspector`), not a snap subfolder — see SP-12. |
| Q-11 | VS Code workspace manual pass? | **Confirmed working** (2026-07-07). Closed. |
| Q-14 | FW-1 `fsnotify` dep? | **Yes** — dep accepted, and include settings-change notifications: external edits to `tui_settings.json` reload live and surface a notification. Shipped 2026-07-07 (see FW-1). |
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
| ✅ Won't do | A-4 | **Tier 2: replace hand-rolled scrolling with `bubbles` list/viewport** | Closed 2026-07-09 per Q-12: keep the existing settings scroller (works, feature-rich); the inspector was already viewport-based. |
| ✅ Done | A-5 | **Tier 4: display-width column rendering** | Done 2026-07-03: the info modal's dependency table was the last byte-padded (`%-50s`) column layout — now display-cell padded (lipgloss `Width` cells + ANSI-aware tail truncation). `renderRuntimeFlat` audited: already width-based. Full `lipgloss/table` adoption judged unnecessary for these two-column views. |
| ✅ Done | A-6 | Small audits: all key handling via `key.Binding` (no `.String()==` compares); `tea.Batch` for multi-cmd returns; `ReapplyBg` on every `View()` that needs it | Done 2026-07-03: last two `.String()==` compares removed (redundant shift+tab clause; KeyRecorder save now Code+Mod, tests use the real key form). Multi-cmd sites all batch via `cmds` slices + `tea.Batch` (verified during the sweep). `ReapplyBg` covers both composition roots (router layout, status help); other surfaces are protected by the themed `View.BackgroundColor`. |
| ✅ Done | A-7 | **Eliminate `init()` functions; shrink globals** | Per Q-5. Verified zero `init()` functions in the library. Removed the global `RegisterPage()`/`RegisteredPages()`/`ClearRegisteredPages()` registry (2026-07-01) — construction-time pages go through `Options.ExtraPages`/`NewWithRegisteredPages`; runtime adds through the `(*RouterModel).RegisterPage` method. Remaining globals (theme cache, logging state) are mutex-guarded. |
| ✅ Done | A-8 | `cyclop` linter enabled | Done 2026-07-09 per Q-19: ceiling 54 (today's worst passing offender is inspector handleSettingsKey at 53); router.Update (116) carries a documented exemption and is the ratchet's first split target. Reduce toward ~25 as the big files shrink. |
| 💡 Idea | A-9 | `context.Context` in notification TTL, config I/O, logging fan-out | Partially covered 2026-07-03: `RunContext`/`NewProgramWithContext` bound the program lifetime (A-2). Remaining idea is plumbing ctx into TTL timers and config I/O — still gated on Bubble Tea's loop not passing contexts. |
| 📋 Noted | A-10 | Tier 5 (consumer repo): decompose dash `dashboard.go` god-object (927 lines) into mode handlers + overlay manager mirroring the router `Overlay` pattern | Lives in the dash repo, tracked here so the tier list stays complete. |
| ⬜ Planned | A-11 | **Restyle the inspector's settings surface to match the main Settings page** | Human feedback (2026-07-07): "The main Settings page is great, the Inspector settings page should look like the main one." Reuse the settings page's section/field rendering (or its styles) inside the inspector so the two read as one design. |

## Testing

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | TS-1 | **Golden-file / snapshot rendering tests** | Done 2026-07-03: `testutil.Golden` (UPDATE_GOLDEN=1 to regenerate) + goldens for the three navigators (incl. overflow window), status bar, and history panel. Add more surfaces as they stabilize. |
| ✅ Done | TS-2 | Unicode torture tests | Done 2026-07-03: CJK/ZWJ/flag/fullwidth titles through tabs, top nav (contiguous click ranges), sidebar, status bar, and history panel. |
| ✅ Done | TS-3 | Tiny-terminal tests | Done 2026-07-03: router driven at 40×10, 60×20, 80×24 across page switch + inspector overlay (`TestTinyTerminals`). |
| ✅ Done | TS-4 | Architectural dependency enforcement | Done 2026-07-03: `testutil.CheckNoImports` + root `TestArchitectureLayering` (foundations import nothing internal; mid-layer never imports router/pages/status/navigation; pages/status never import router). |
| ⬜ Planned | TS-5 | `teatest` integration tests | Drive the full app through `tea.Program`, assert terminal output. |
| ✅ Won't do | CF-2 | **Inspector "Conformance" tab** — runtime pass/fail | Dropped 2026-07-09 per Q-13: the unit-level conformance checks cover the same invariants better. |
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
| 🔄 In Progress | X-0 | `snap` repo scaffold (per Q-10) | Scaffolded 2026-07-07; **first five packages moved in 2026-07-09** (SP-1/SP-3). Remaining moves tracked as SP-items in the "Snap & Inspector split" section below. |
| ⬜ Planned | X-1 | `navigation` — Tabs, Sidebar, Slim/topnav → `snap/navigation/{tabs,sidebar,minimal-top}` | Superseded by **SP-5** (adds the Navigator-interface requirements). Sidebar is close to `bubbles/list` with a custom delegate — consider migrating before extraction to thin the key/mouse logic. |
| ⬜ Planned | X-2 | `pages/inspector` → own repo | **Superseded by SP-12** (2026-07-09): the inspector goes to `github.com/jarvisfriends/inspector` (its own public repo, "debug any charm based app"), not a snap subfolder. |
| ⬜ Planned | X-3 | `logging` → `snap/logging` | Shape now depends on **SP-10** (zap for dev logs); what remains framework-specific is the subscriber fan-out feeding the inspector. |
| ✅ Done | X-4 | `status` bar → `snap/status` | Done 2026-07-10 as part of the wholesale move (snap v0.1.5): `status/`, `navigation/`, `page/`, `table/`, `notifications/`, and most of `theme/` now import back from snap. |
| 🔄 In Progress | X-5 | VHS gif creator with `.tape` config for the main tui-base app | 2026-07-10: adopted snap's container pipeline — `tools/rendertapes` (own module, ported from snap) cross-compiles `cmd/*` and `examples/*` to `demo-bin` and renders every `*.tape` in the official vhs container. Tapes: `cmd/tui-base/demo.tape` (the app tour, moved from `tools/demo.tape` and re-pointed at the prebuilt binary), `cmd/tui-base/notifications.tape` (toasts + status bar), `examples/multipage/demo.tape` (navigation) — the latter two close the snap ROADMAP's "navigation/status/notifications need a host-shaped app" item. README gained a Demos section. **🔔 Human action:** no Docker/Podman on this machine — run `go -C tools/rendertapes run .` where one exists and commit the three gifs. |

## Ideas (unranked)

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | T-4 | Custom theme authoring (user-defined YAML tints) | Done 2026-07-09 per Q-15: `theme.LoadYAMLTints`/`RegisterYAMLTints` parse the full 16-slot schema (`gopkg.in/yaml.v3`, `#rgb`/`#rrggbb`, dark derived from bg luminance) from `<config-dir>/themes/`; the settings page registers them at startup so they appear in the Theme selector. Bad files are skipped with a logged warning. Recipe in docs/theme-cookbook.md. |
| 🔄 Reopened | L-6 | Migrate developer logging to Uber `zap` | 2026-07-03 evaluation recommended rejecting (slog if anything) — **vetoed by the human 2026-07-09**: use zap for actual dev logs instead of maintaining wrappers around what zap provides. Now tracked as SP-10 (zap core + a custom sink preserving the subscriber fan-out the inspector Log tab consumes). |
| ✅ Done | FW-1 | **Filewatch helper** (`fsnotify` → `tea.Cmd`) for live-reloading views | Done 2026-07-07 per Q-14: new `filewatch` package (parent-dir watch so atomic renames are seen, debounced bursts, `Next()`/`Stop()` lifecycle) + `Options.WatchSettingsFile` — external edits to `tui_settings.json` reload live, re-apply the theme, and raise a notification; the app's own saves stay silent (JSON no-op detection). `RouterModel.Close` releases the OS watch; `tuibase.Run/RunContext` call it automatically. Tests: `filewatch/filewatch_test.go`, `router/settings_watch_test.go`. |
| ✅ Done | I-1 | Page lifecycle hooks (`OnEnter`, `OnLeave`, `OnResize`, `OnThemeChange`) | Done 2026-07-10: `common.PageEnterer`/`common.PageLeaver` (optional interfaces; `OnEnter() tea.Cmd` / `OnLeave() tea.Cmd`). The router fires them through one choke point (`switchActivePage`) covering Tab cycling, number keys, sidebar/tab selection, Ctrl+G, and status-bar page clicks; the startup page enters from `Init` after page Inits; re-selecting the active page fires nothing; quit fires no OnLeave. **OnResize and OnThemeChange are deliberately not new interfaces** — resize is already delivered to every page (WindowSizeMsg + `Component.SetSize`) and theme changes via `styles.ColorAware` + the shared palette pointer; documented on the interfaces. Tests: `router/lifecycle_test.go`. |
| ⬜ Planned | I-2 | Command palette (Ctrl+P) | Approved per Q-16, priority 2 of 6 (effort: L). |
| ⬜ Planned | I-3 | Modal confirmation service (router-owned, `ConfirmMsg`) | Approved per Q-16, do in number order (effort: M). |
| ⬜ Planned | I-4 | Persistent layout state (sidebar width, tab order, last page) | Approved per Q-16, do in number order (effort: M; nav style already persists). |
| ⬜ Planned | I-5 | Terminal capability detection beyond background color (truecolor, kitty graphics) | Approved per Q-16, do in number order (effort: M). |
| ⬜ Planned | I-6 | Global error overlay — panic → inspector with stack | Approved per Q-16, priority 6 of 6 (effort: M; riskiest — `recover()` inside the tea loop). |
| ✅ Done | I-7 | Configurable ring-buffer size for inspector message dedup | Done 2026-07-03: `inspector.SetLogCapacity(n)` (floor 10, 0 restores the 50 default); both dedup paths trim through one helper. |
| ✅ Done | I-8 | Supply-chain hardening at release: SBOM + keyless cosign | Done 2026-07-09 per Q-17: goreleaser emits SPDX SBOMs per archive and cosign keyless-signs the checksum file (verifying it transitively verifies every archive); release workflow grants `id-token: write` and installs cosign+syft; the CI snapshot job skips sign/sbom. First real validation happens on the next tag. |
| ✅ Rejected | I-9 | i18n keys in user-facing strings | Rejected 2026-07-09 per Q-18. |
| ✅ Done | I-10 | **Inspector "Link" tab: estimated remote data rate** (Tx = input as ANSI wire bytes — key/mouse/drag/paste; Rx = frames as a line-diff renderer would transmit) | Done 2026-07-07 (human request): built-in `MetricsProvider` tab via the E-5 API. Shows last-second / 5 s / 60 s averages, peaks, session totals, and the required link rate in B/s + bit/s for a remote (SSH/serial/embedded) deployment. Collection runs on demand: the Link tab or the status summary. 2026-07-07 follow-up: "Include link rate" toggle in the inspector settings puts compact `tx … rx …` 5 s averages in the status bar with the inspector closed (like the GC part). `router/linkrate.go` + tests. |
| ⬜ Planned | I-11 | **Data-rate limiter modes** — degrade visuals to fit a constrained link, guided by the I-10 meter | Human intent (2026-07-07): options like capping the update/refresh rate, disabling or reducing transitions/animations, and similar "looks less good, costs fewer bytes" behaviors, so the app can run within a target link budget on an embedded device. Design sketch: a `LinkBudget` setting (target bit/s) that gates spinner/progress tick rates, coalesces re-renders (min frame interval), and prefers full-cell updates over per-frame gradients. Build on the I-10 meter for feedback. |
| ✅ Done | I-12 | **Keyboard binding to open the notification history panel** | Done 2026-07-10 against snap v0.1.7: `keys.AppKeyMap.ToggleHistory` (default `ctrl+n`, rebindable via the settings keybindings section). The router's global switch opens the panel (same path as the status-bar click) and, since the open panel is a modal KeyConsumer, its OverlayKey handler closes on the same binding — a true toggle. The notifications tape now demos the panel (open, cursor, dismiss all, close). Found while scripting that tape: the panel was mouse-only, violating snap's keyboard-only rule. Test: `router/history_key_test.go`. |


## Snap & Inspector split (directive 2026-07-09)

> Human directive: `jarvis-bubbles` is renamed to **`snap`** ("Jarvis Friends
> Snap") — production-ready components with first-class keyboard **and** mouse
> support. `snap` and `inspector` are both public on GitHub; tui-base imports
> them back (Q-20). Components that have multiple implementations expose an
> interface so the engineer can decide which options users may switch between
> at runtime in the settings area.
>
> **Sequencing rule for every move:** land in snap → push + tag → flip
> tui-base imports in a dedicated changeset (a public library must not carry
> `replace` directives, so unpushed snap code is unusable from tui-base).

| Status | # | Task | Notes |
|---|---|---|---|
| ✅ Done | SP-1 | `keys` → `snap/keys` (whole folder) | Moved 2026-07-09 (builds/tests green in snap) so every snap can share the common bindings without copy-paste. tui-base import flip is SP-2. |
| ✅ Done | SP-2 | **snap `v0.1.0` published + wholesale import flip** | Done 2026-07-09: snap pushed to GitHub (master + `adding_fun_stuff` branch; `v0.1.0` tagged, including gate/winterm/timefield). tui-base imports `keys`, `geom`, `datepicker`, `timepicker`, `gate`, `winterm`, and `dependencies` from `github.com/jarvisfriends/snap@v0.1.0` — local packages deleted, no compat aliases, go.sum recorded, full gate green without any workspace. Minor version bump lands at the next tui-base tag. New rule: snap work goes through non-master branches. |
| ✅ Done | SP-3 | `geom`, `datepicker` (renamed from `bubble-datepicker`), `timepicker`, `dependencies` (from `common/dependencies.go`) → snap | Moved 2026-07-09; all had zero internal tui-base imports. Datepicker file + doc renamed to drop "bubble". Timepicker moved as-is; its UX redesign is SP-8. |
| 🔄 In Progress | SP-5 | `navigation` → `snap/navigation/{tabs,sidebar,minimal-top}` + **Navigator interface** | Interface speaks intent, not geometry: `NextPage`/`PreviousPage` (never up/down/left/right). Width contract: a navigator reports how much horizontal space it consumes — top nav returns 0, sidebar returns its width. Audit says tui-base mostly does this already (`navigation.Navigator`); verify and codify in the interface docs during the move. |
| ✅ Done | SP-6 | `table` → `snap/table` after style-hook decoupling | Only 4 theme touchpoints (`c.Border`, `c.Filter`, `c.SelectionBg`, `c.Styles`); replace with an injected `table.Styles` (the A-3 datepicker pattern), tui-base maps its theme onto it. |
| ✅ Done | SP-7 | Directory/File pickers + Multi-file/Multi-directory pickers → `snap/pickers` | Completed 2026-07-10: settings imports snap/pickers via themed adapters (picker_hooks.go maps theme→Styles/HuhTheme/envpath.Collapse); component tests live with the component. |
| ✅ Done | SP-8 | **Timepicker UX redesign** (spec per Q-24) | Shipped 2026-07-09 in `snap/timepicker` as `TimeFieldModel`: two `HH:MM` columns, highlighted colon, click/space opens a scrollable per-side dropdown (wheel scrolls, click/Enter commits), digit type-ahead, and validation (range clamping) whenever the field is left. Legacy `TimePickerModel` (duration editing) kept until tui-base migrates its settings usage. |
| ✅ Done | SP-9 | Settings view: **collapse TUI Base categories by default; hide zero-choice fields** | Done 2026-07-10. Framework categories start collapsed (`▸ Title (n)` headers), app `ExtraSections` categories stay expanded; collapse state survives rebuilds (runtime-only, not persisted); `ExpandAllCategories()` restores the old look. The overview cursor now walks *stops* — category headers plus editable visible items — so Enter on a header toggles collapse from the keyboard and clicking a header does the same; the cursor snaps to the header when its category collapses beneath it. Zero-choice: `settingItem.choices` records the effective option count per select (Color Theme counts registered tints dynamically); `choices == 1` rows render dimmed, take no cursor stop, and refuse to open an editor. Text fields/pickers/key recorder are unrestricted (0); gates are always 2, never hidden. Tests: `sp9_collapse_test.go`. |
| ⬜ Planned | SP-10 | **Dev logs on `zap`; sharpen User Notifications vs Developer Logs; rename the inspector's tea.Msg tab** | Human veto of the L-6 rejection: use `uber-go/zap` for dev logs instead of homegrown wrappers; keep the subscriber fan-out as a custom zap sink (the inspector Log tab consumes it). Naming cleanup: **User Notifications** = `notifications` (user-visible toasts/history); **Developer Logs** = zap (files + inspector Log tab); the inspector tab that shows `tea.Msg` traffic drops the word "log" — rename to **Messages** (candidate: "Message Passing Reader"; pick during implementation, charm-style naming). |
| ✅ Done | SP-11 | **Program-options API** (shape per Q-21: struct + `WithXXX`) | Shipped 2026-07-09: `tuibase.Option` + `With*` funcs for every Options field; `Run/RunContext/NewWithOptions` gained source-compatible variadic tails; `tuibase.New(options ...Option)`. Includes `WithDebugOverlay(tea.Model)` (Q-22's tui-base half): the injected model owns Ctrl+D, rendered in the inspector's overlay box with keys/mouse/resize forwarded. Available-set options (navigation styles etc.) grow as snap fills out. |
| ⬜ Planned | SP-12 | **Inspector → `github.com/jarvisfriends/inspector`** (own repo, "debug any charm based app") | Design per Q-22: library form is a plain `tea.Model`; `cmd/inspector` main runs it under snap's **minimal-top nav** (⇒ depends on SP-5). tui-base consumes it via `WithDebugOverlay`. Untangling needed first: theme pointer, logging fan-out, shared table styles, router overlay stack. |
| ✅ Done | SP-14 | **Render/layout checks moved to `snap/rendercheck`** | Done 2026-07-09 (shipped in snap v0.1.1): golden files, border integrity, viewport fit, layout-math + key-binding standards now live in snap; every tui-base call site re-pointed. tui-base testutil keeps only `CheckNoImports` and `CheckDescriptiveStructNames`. Name `rendercheck` stands unless Q-25 is vetoed. |
| 🔄 In Progress | SP-15 | **VHS `.tape` per snap component + run vhs** | Tapes + example mains + `snap/tools/render_tapes.sh` committed. **Root cause of the Windows hang (diagnosed 2026-07-09):** vhs.exe finds Chrome and connects DevTools fine; it spawns ttyd correctly (`--port … --once --writable cmd.exe /k …`) and ttyd binds — but ttyd exits right after the first browser connection (`--once` + an early disconnect), leaving vhs waiting forever for the `canvas.xterm-text-layer` element on a dead page. Same ttyd command works standalone, and **the same tapes render fine under WSL** (human-confirmed) — so this is an upstream vhs/ttyd/Windows bug, not our tapes. Render via `bash tools/render_tapes.sh` in WSL; also delete the stray Linux ELF `vhs` at `E:/code/home/go/bin/vhs` that can shadow vhs.exe. Remaining: run the render in WSL and commit the gifs. |
| ⬜ Planned | SP-13 | **Test-file policy: one `<file>_test.go` per source file** (+ optional `<file>_regressions_test.go` when it grows) | Consolidate the current per-feature test files (e.g. router has 20+ suffixed test files) during each package's move; consider a `testutil.CheckCodeStandards` rule to hold the line afterward. |

### Gaps found while planning (the "did I forget anything?" answer)

- **Versioning/CI sequencing** — confirmed (human 2026-07-09): push and tag
  snap before rebuilding tui-base.
- **Compat shims** — decided (human 2026-07-09): **none.** No deprecation
  aliases; push and tag in order, wholesale flip, minor-version bump pre-1.0,
  CHANGELOG entries in both repos.
- **Theme contract for snap** — agreed (human 2026-07-09): theme-free with
  injected style hooks (A-3 pattern); tui-base keeps the theme→styles mapping.
- **`testutil` sharing** — decided (human 2026-07-09), tracked as **SP-14**:
  move the render/layout "string building" checks (layout math, borders,
  viewport fit, goldens, key-mapping standards) to snap under a better name
  than "conformance" (proposal: `rendercheck`, Q-25), and re-point every
  current call site. The **type-name checker stays in tui-base** testutil
  (it is a tui-base house rule), as does `CheckNoImports`.
- **`gate` and `winterm`** — approved (human 2026-07-09): both move to snap,
  riding the SP-2 wave so `v0.1.0` ships them.
- **Arch tests + docs** — `TestArchitectureLayering`, `.github/`
  copilot-instructions, and `docs/` all reference the moved packages; each
  flip must update them (the arch test shrinks as foundations leave).
- **VHS tapes** — directed (human 2026-07-09): add a `.tape` per snap and run
  vhs on it (SP-15). **Determinism answer:** re-rendering the same tape on the
  same code is *not* guaranteed byte-identical — GIF output depends on the
  render environment (fonts, vhs/ttyd versions) and encoder timing, so treat
  gifs as build artifacts (regenerate on change, don't diff-gate them); with a
  pinned vhs version and environment the output is *visually* identical and
  usually byte-stable, but there is no tool-level guarantee.
- **go.work dev recipe** — the human keeps a `_go.work` file and moves it in
  when a change spans repos; `go.work` stays untracked.

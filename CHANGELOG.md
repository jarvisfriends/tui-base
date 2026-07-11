# Changelog

All notable changes to this project are documented in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
project adheres to semantic versioning (breaking changes allowed before v1.0).

## [Unreleased]

### Added

- Page lifecycle hooks (I-1): page models may implement
  `common.PageEnterer` (`OnEnter() tea.Cmd`) and/or `common.PageLeaver`
  (`OnLeave() tea.Cmd`); the router calls them on every page switch (and
  OnEnter for the startup page). Resize and theme changes intentionally
  stay on the existing mechanisms (size forwarding, `styles.ColorAware`).

- VHS demo tapes rendered through the container pipeline:
  `tools/rendertapes` (ported from snap) cross-compiles `cmd/*` and
  `examples/*` and renders every `*.tape` in the official vhs image. New
  tapes: `cmd/tui-base/demo.tape` (app tour, relocated from
  `tools/demo.tape`), `cmd/tui-base/notifications.tape` (toasts, TTL expiry,
  status bar), and `examples/multipage/demo.tape` (navigation) — the
  host-shaped demos the snap ROADMAP called for. README gained a Demos
  section (gifs pending a Docker-equipped render).

- Custom YAML themes (T-4): drop full 16-slot tint files into
  `<config-dir>/themes/` and they join the Theme selector at startup
  (`theme.LoadYAMLTints`/`RegisterYAMLTints`; `gopkg.in/yaml.v3`; bad files
  are skipped with a logged warning). Authoring recipe in
  docs/theme-cookbook.md.
- **BREAKING (wholesale wave, 2026-07-10):** `navigation`, `page`, `status`,
  `table`, `notifications`, and the picker components moved to
  `github.com/jarvisfriends/snap`; the entire `theme` implementation moved to
  `snap/styles`, with tui-base's `theme` package remaining as aliases so
  existing imports keep compiling. dash's chart primitives joined snap as
  `snap/charts`. tui-base now depends on the tagged `snap v0.1.5` (the
  wholesale-move release); the interim `go.work` replace is gone.
- **BREAKING (SP-14):** the render/layout test helpers (goldens, border
  integrity, viewport fit, layout-math + key-binding standards) moved from
  `testutil` to `github.com/jarvisfriends/snap/rendercheck` (snap v0.1.1).
  tui-base's `testutil` keeps `CheckNoImports` and
  `CheckDescriptiveStructNames` only.
- **BREAKING (SP-2, minor bump):** the `keys`, `geom`, `datepicker`,
  `timepicker`, `gate`, and `winterm` packages and the dependency/build-info
  reader (`common.Dependencies`/`ExpandedBuildInfo`) moved to
  [github.com/jarvisfriends/snap](https://github.com/jarvisfriends/snap) —
  tui-base imports them back; no compat aliases (per the 2026-07-09 decision).
  Update imports from `github.com/jarvisfriends/tui-base/<pkg>` to
  `github.com/jarvisfriends/snap/<pkg>` (build-info: `snap/dependencies`).

- App icon: a brand mark embedded in the Windows binary (Explorer/taskbar/
  shortcuts). `assets/icon.svg` is the master vector; the `tools/genicon`
  standalone module rasterizes it into `assets/icon.ico` and the committed
  `cmd/tui-base/resource_windows_<arch>.syso` resources (arch-suffixed, so
  non-Windows builds ignore them). Apps brand their own binary without
  vendoring: `go run github.com/jarvisfriends/tui-base/tools/genicon@latest
  -svg app.svg -syso ./cmd/app -name "My App"`. Regenerate tui-base's own icon
  with `go -C tools/genicon generate .`; it is kept out of `go generate ./...`
  so the CI drift check and release build never depend on the SVG toolchain.
  See [docs/branding.md](docs/branding.md).
- Windows Terminal tab icon: the tab glyph is a profile setting (the binary
  icon covers Explorer/taskbar; the tab is separate — `wt new-tab` has no icon
  flag). `tuibase.InstallWindowsTerminalProfile` /
  `UninstallWindowsTerminalProfile` register a Windows Terminal profile
  fragment so the app shows in the new-tab dropdown with its own name + icon;
  `genicon -png` emits the icon PNG; and `TerminalRelaunchConfig.ProfileName`
  makes the auto-relaunch open under that profile (when installed) so relaunched
  tabs carry the icon too. The reference app wires all of it
  (`tui-base -install-terminal-profile`). See
  [docs/branding.md](docs/branding.md#windows-terminal-tab-icon).
- App icon small-size quality: `genicon` now renders each icon size supersampled
  (`-supersample`, default 4×) and downsamples with a Catmull-Rom filter, so the
  16–48 px variants Windows shows in Explorer and the taskbar are crisp instead
  of aliased.
- Windows Terminal auto-relaunch: on Windows, when started under the legacy
  console (conhost) in an interactive session with `wt.exe` available and no
  modern terminal already in use, tui-base relaunches itself inside Windows
  Terminal so the Charm v2 truecolor/mouse/styling features work — a guard
  against the default-terminal registry setting being reset. Detection is
  console-window based (`ConsoleWindowClass` vs ConPTY's
  `PseudoConsoleWindow`), because a double-clicked app hosted by WT through the
  default-terminal delegation inherits Explorer's environment and carries no
  `WT_SESSION` — env markers alone would relaunch it into a duplicate window.
  `tuibase.Run`/`RunContext` do it automatically; `tuibase.
  EnsureWindowsTerminal` relaunches from the very top of `main`;
  `router.MaybeRelaunchInWindowsTerminal` is the primitive. Opt out with
  `Options.DisableTerminalRelaunch` or the `TUI_BASE_NO_WT_RELAUNCH` env var.
  No-op on non-Windows platforms.
- Program-options API (SP-11, shape per Q-21): `tuibase.Option` functional
  options (`WithAppName`, `WithPages`, `WithGates`, `WithKeyMap`,
  `WithWatchSettingsFile`, `WithoutTerminalRelaunch`, …) coexist with the
  `Options` struct — `Run`/`RunContext`/`NewWithOptions` gained
  source-compatible variadic tails and `tuibase.New(options ...Option)` builds
  from options alone. Options apply on top of the struct; defaults fill the
  rest.
- `WithDebugOverlay(tea.Model)` / `Options.DebugOverlay` (Q-22): an injected
  model replaces the built-in inspector as the Ctrl+D debug pop-up — tui-base
  owns the toggle whenever the model is non-nil, rendering it in the
  inspector's overlay box with keys, mouse, and sizing forwarded. Pairs with
  the standalone `jarvisfriends/inspector` (delivered as a plain `tea.Model`).
- Live feature gates ([docs/feature-gates.md](docs/feature-gates.md)): flipping
  a gate in the settings Feature Flags section now takes effect immediately —
  the commit broadcasts `settings.GatesChangedMsg` and the router re-derives
  gate-dependent UI on the spot. The inspector's Accessibility tab is the first
  gated feature (`inspector.AccessibilityTabGate`, default **hidden**): enable
  it at runtime via Feature Flags or at startup via
  `<APPNAME>_GATE_INSPECTOR_ACCESSIBILITY_TAB=1`. The router now always has a
  gate registry (creating one when the app passes none), registers built-in
  gates the app hasn't defined (`gate.Has`), and applies env overrides. Gate
  values are runtime-only — never persisted to the settings file. Hidden tabs
  drop out of the tab bar, digit keys, cycling, and click targets; disabling
  the gate while its tab is active snaps back to Runtime.
- `winterm` package: read/write the Windows default-terminal delegation (the
  `DelegationConsole`/`DelegationTerminal` values under
  `HKCU\Console\%%Startup`) via `winterm.Detect` and `winterm.Set` with a typed
  `Delegation` enum — the same mechanism the Windows Terminal settings UI uses
  (there is no supported OS API). The settings page's "Default Terminal" item
  now delegates to it, replacing per-platform registry code and GUID literals
  in the UI layer; consumer apps can offer the same repair programmatically.
  Off-Windows calls return `errors.ErrUnsupported`.
- `filewatch` package (FW-1): fsnotify -> `tea.Cmd` bridge with rename-safe
  parent-directory watching (atomic writes are seen), debounced event bursts,
  and a `Next()`/`Stop()` lifecycle.
- `Options.WatchSettingsFile`: live reload of `tui_settings.json` — external
  edits re-apply at runtime (including the theme) and raise a notification;
  the app's own saves stay silent. `RouterModel.Close` releases the watch;
  `tuibase.Run`/`RunContext` call it automatically.
- Inspector "Link" tab (I-10): estimated remote-link data rates — Tx prices
  key/mouse/paste input as ANSI wire bytes (mouse drags as SGR reports), Rx
  prices rendered frames as a line-diff renderer would transmit them. Shows
  1 s / 5 s / 60 s averages, peaks, totals, and the required link rate for a
  remote (SSH/serial/embedded) deployment. Collects only while the inspector
  is open.
- Status-bar link rate: the inspector's status summary (the same one that
  shows GC/heap when the inspector is closed) gained an "Include link rate"
  toggle — compact `tx … rx …` 5-second averages, with the meter collecting
  on demand while the summary displays it.

### Added

- Root `tuibase` package: `Run`/`RunContext`, `New`, `NewWithOptions` — one
  import bootstraps a full app; entry point moved to `cmd/tui-base`.
- `router.TargetedMsg` fast path so high-frequency background messages wake
  only their page.
- `router.NewProgramWithContext` for SIGTERM-clean shutdown.
- Directory-only picker overlay (files hidden, Ctrl+S selects the browsed
  folder); file pickers open in browse mode sized to the page with
  Enter-opens / Space-selects keys.
- Inspector: row selection and internal scrolling in the data tables,
  categorized Terminal tab, app-synced tab-cycling keys, horizontal-wheel tab
  switching (also on the Tabs bar and minimal top nav).
- Conformance: `CheckFitsViewport` replays overlay states at every standard
  size (including 90x76); package docs and `Example*` functions.

### Changed

- Notification history panel restyled to the info-modal look, sizes to fit
  its content, and highlights the cursor row with the selection colors.
- Date/time pickers and editor overlays draw from the active theme.
- OpenSettings default rebound from `ctrl+,` to `ctrl+g`.

### Fixed

- Mouse clicks on date/time pickers hosted in the settings model overlay now
  work: the page's OnMouse used to swallow inside clicks while the Update
  path forwarded them with untranslated page coordinates that missed every
  hit zone (and wheel events were converted to arrow keys before reaching
  the picker). Hosted models now receive mouse exclusively via
  `ModelOverlayHost.ForwardMouse`, translated into their content space;
  regression test drives a real click through the composed overlay. The
  standalone snap demos additionally needed `MouseMode` on the root view
  (Bubble Tea v2 only reports mouse when the root view requests it).

- Feature Flags edits on the settings page now actually commit: the select
  bound its value to a variable that went out of scope before the form
  completed, so toggling a gate silently did nothing. The binding now outlives
  the form and the commit is covered by a regression test.
- `router.NewProgram` now actually honors `TUI_BASE_COLOR_PROFILE` as its doc
  comment (and consumer apps) always claimed — it previously forwarded straight
  to `tea.NewProgram` without applying the override. Apps with a branded env
  var should keep using `NewProgramWithEnvVar`.
- Router stored the `tea.Model` returned by child `Update` calls (model-swap
  pattern) at every dispatch site.
- Config writes are atomic (temp file + rename); crash mid-write can no
  longer truncate settings or notification history.
- Logging subscriber fan-out no longer holds its lock during callbacks.
- Mouse-wheel events no longer leak to the page behind a modal overlay;
  status bar no longer overflows narrow terminals.

## [0.2.1]

Baseline public release: router-owned composition, four-axis theme system,
Z-ordered overlay stack, notification manager, Ctrl+D inspector, conformance
test helpers.

[Unreleased]: https://github.com/jarvisfriends/tui-base/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/jarvisfriends/tui-base/releases/tag/v0.2.1

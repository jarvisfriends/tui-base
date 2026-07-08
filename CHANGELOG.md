# Changelog

All notable changes to this project are documented in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
project adheres to semantic versioning (breaking changes allowed before v1.0).

## [Unreleased]

### Added

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

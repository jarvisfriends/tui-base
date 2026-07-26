# Architecture

tui-base is an application framework for Bubble Tea v2 (routing, theming, settings, logging, overlays). This document
describes the system at the level the
OpenSSF Baseline asks for: the actors involved, the actions they can take, and every external interface of
the released software.

## Actors

- **Host application** — a Go program built on `tuibase.Run`/`RunContext`, pages, and the router.
- **Terminal user** — navigates pages, edits settings, and toggles debug overlays.
- **Filesystem** — config dir (settings JSON), optional log destination, watched paths.

## Actions and data flow

The host application drives a Bubble Tea event loop: terminal input arrives as messages, the model updates,
and a new frame is rendered to the terminal. This library sits inside that loop — it does not spawn its own
event sources beyond those documented below, and it holds no global mutable state that outlives the model.

## External interfaces

- The public Go API (root package, `router`, `pages`, `theme`, `logging`, `filewatch`, `envpath`,
  `overlay`, `config`, `common`). See the [Go reference](https://pkg.go.dev/github.com/jarvisfriends/tui-base).
- Settings JSON file under the user config directory (documented in `docs/`).
- Optional log file/directory chosen by the user.
- No network listeners; the framework opens no sockets.

## Security-relevant surfaces

- **Settings persistence.** Settings are stored as JSON under the user's config directory and written
  atomically. Values are validated on load; a corrupt file falls back to defaults instead of failing open.
- **File watching.** `filewatch/` watches paths the host application chooses (e.g. the settings file) and
  debounces events; it never acts on file contents itself.
- **Logging.** `logging/` writes to a user-chosen destination with rotation; log lines are plain text and
  never interpreted.
- **Rendered content.** Framework chrome renders host-provided strings; untrusted content rendering is the
  host application's responsibility and is tracked in the threat model.

See [threat-model.md](threat-model.md) for the corresponding threat analysis and mitigations.

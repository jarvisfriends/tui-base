# Compatibility

## Go

| Requirement | Version |
|---|---|
| Go toolchain | **1.26.4+** (1.26.3 and below contain a known CVE) |
| Charm stack | v2 vanity imports only (`charm.land/bubbletea/v2`, `bubbles/v2`, `lipgloss/v2`, `huh/v2`) — the v1 `github.com/charmbracelet/bubbletea` path must never appear in the module graph |

## Operating systems

CI builds and race-tests every commit on Linux (ubuntu-24.04), Windows
(windows-2022), and macOS (macos-14); releases ship linux/darwin/windows for
amd64 and arm64. Platform-specific code is limited to disk statistics
(`pages/inspector/disk_*.go`) and the Windows default-terminal setting.

## Terminals

| Terminal | Notes |
|---|---|
| Windows Terminal | Full support: truecolor, mouse (incl. modifiers), enhanced keyboard |
| iTerm2 / kitty / WezTerm / Ghostty | Full support |
| conhost (classic) | Works; degraded keyboard (e.g. some ctrl chords alias), 256-color |
| SSH sessions | Colors quantize to ANSI256 unless `COLORTERM` is forwarded — force with `<APP>_COLOR_PROFILE=truecolor` (see the inspector's Terminal tab for a live diagnosis and the exact fix hint) |
| Screen readers | The inspector's Accessibility tab detects common readers; CVD-safe palettes via the accessibility theme axis |

The inspector (Ctrl+D → Terminal) reports the negotiated color profile, TERM,
SSH state, and OSC-11 background detection for any environment you need to
debug.

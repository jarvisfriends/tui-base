# Threat model

Scope: tui-base (an application framework for Bubble Tea v2 (routing, theming, settings, logging, overlays)) and its
released artifacts (the `tui-base` binary and per-example demo binaries).
Companion document: [architecture.md](architecture.md) (actors, interfaces, surfaces).

## Trust boundaries

1. **Host application ↔ library.** The host is fully trusted: it links the code into its own process. The
   library makes no attempt to defend against a malicious host — that is outside the model.
2. **Terminal user ↔ application.** The user owns the process and the terminal. Input is untrusted in the
   parsing sense (must never crash or corrupt state) but not in the privilege sense (the user cannot gain
   anything they do not already have).
3. **Displayed content ↔ terminal emulator.** Strings rendered to the terminal can carry ANSI/OSC escape
   sequences. Content that originates outside the process (file names, network data supplied by the host)
   is the main injection surface.
4. **Supply chain.** Dependencies, CI, and the release pipeline.

## Threats and mitigations

| # | Threat | Mitigation |
| - | ------ | ---------- |
| 1 | Escape-sequence injection via displayed content (terminal state corruption, clipboard writes, spoofed UI) | Rendering goes through lipgloss/bubbletea width-aware printing; fuzz tests cover parsers/formatters; content-bearing surfaces are tracked in architecture.md and reviewed on change |
| 2 | Malformed/hostile input crashing the event loop (DoS) | Race-enabled tests, fuzz smoke tests in CI on every PR, defensive bounds checks in hit-testing and layout code |
| 3 | Filesystem misuse through file/dir features | Read-only listing; paths are chosen by the host or the local user; no symlink following into unrequested trees; writes (exports, settings, logs) go to user-owned locations with conservative permissions |
| 4 | Dependency vulnerability or malicious update | Minimal dependency set; `go.sum` pinning; Dependabot weekly; `govulncheck` + `dependency-review` in CI; SBOM published per release |
| 5 | CI/release pipeline compromise (tampered artifacts) | All workflow actions pinned by commit SHA; least-privilege `GITHUB_TOKEN`; releases built by goreleaser from a tag, checksummed, SBOM'd, and signed with keyless cosign (OIDC identity = the release workflow); reproducible build flags (`-trimpath`, commit timestamp) |
| 6 | Secret leakage in the repository | No long-lived secrets exist; gitleaks runs in CI; GitHub push protection enabled |

## Out of scope

- A malicious host application or a compromised developer machine.
- Terminal emulator bugs (we emit only well-formed sequences).
- Privilege escalation: the software runs entirely with the invoking user's privileges and opens no
  network listeners.

## Review cadence

This document is reviewed with every change that adds an external interface (new file/network/terminal
surface) and at least once per year. Last review: 2026-07-25.

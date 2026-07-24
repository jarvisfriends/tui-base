# Contributing to tui-base

Thanks for helping improve tui-base. This document is the short version of how
changes get in; the architecture background lives in
[docs/architecture-decisions.md](docs/architecture-decisions.md) and the
patterns catalog in [.github/CHARM_ECOSYSTEM.md](.github/CHARM_ECOSYSTEM.md).

## Requirements

- Go **1.26.5 or newer** (1.26.4 and below contain a known CVE).
- `golangci-lint` v2, `gofumpt`, `shellcheck` for the full local gate.

## Workflow

1. Branch from `main`.
2. Make the change **with tests** — every feature gets tests, every bug fix
   gets a regression test (see the conformance helpers in `testutil/`).
3. Run the full local gate before pushing:

   ```bash
   bash tools/local_verify.sh
   ```

   which covers formatting, both-GOOS linting, docs/link lint, `go vet`, and
   the race-enabled test suite. The pre-commit hook runs the same gate.
4. Open a PR against `main`. CI must pass (verify matrix on Linux, Windows,
   and macOS; lint; CodeQL; coverage floor of 55%; goreleaser snapshot).

## Code conventions

- Charm v2 (`charm.land/…/v2`) imports only; never pull in the v1
  `github.com/charmbracelet/bubbletea` path.
- All colors come from the active theme (`theme.Active()` / shared
  `*theme.AppStyle`); no hardcoded colors in UI code (ADR-003).
- Key handling goes through `key.Binding` structs — no inline
  `msg.String() == "x"` comparisons, no vim fallback keys (ADR-011).
- Runtime I/O belongs in `tea.Cmd` functions, never directly in `Update`
  (ADR-004).
- Prefer an existing Charm library over a custom implementation; flag any
  component growing toward something Charm already ships.

## Roadmap and decisions

Open work is tracked in [.github/ROADMAP.md](.github/ROADMAP.md). Stable
decisions (and rejected alternatives) are recorded as ADRs in
[docs/architecture-decisions.md](docs/architecture-decisions.md) — if your
change reverses one, update the ADR in the same PR.

# tui-base

A structured, production-minded foundation for building large Charm v2 terminal applications in Go.

This project is designed for engineers who want more than a demo app: predictable architecture,
consistent theme behavior, keyboard and mouse interaction patterns, and debug tooling that helps
you understand message flow while you build.

## Why This Exists

Building a large TUI gets hard when app state, routing, styling, and diagnostics are scattered.
`tui-base` provides an opinionated structure that keeps those concerns coherent:

- Router-first composition for multi-page apps.
- Shared live theme pointer propagated across components.
- Status and notification primitives for global UX.
- Inspector workflows for observing runtime behavior.
- Test- and benchmark-friendly package boundaries.

## Current Capabilities

The following milestone capabilities are already implemented:

- Shared style propagation via one mutable app style pointer.
- Window title + terminal canvas foreground/background managed at router and page level.
- Keyboard + mouse navigation via Sidebar and Tabs.
- Global shortcuts for settings, navigation visibility, detailed help, and status visibility.
- Runtime navigation style switching and settings persistence.
- Settings UX with overview rows + overlay editor forms.
- Live theme preview while editing settings.
- Runtime logging configuration and level updates.
- Inspector message log with deduplication and runtime log streaming.
- Notification manager with severity, TTL, persistence, keyed action items, and history panel.
- Status bar integrations for settings and notification controls, including pending-action counts.
- Compositor-based overlays for toast/history rendering plus router-registered action prompts.
- Modern Go cleanup workflow using `modernize -fix`.

## Project Layout

- `main.go`: entrypoint and app bootstrap.
- `router/`: root model and message routing.
- `navigation/`: Sidebar and Tabs components.
- `pages/`: page models (`home`, `settings`, `inspector`).
- `status/`: status bar and notification overlays.
- `theme/`: app style model and style helpers.
- `notifications/`: notification manager and persistence.
- `logging/`: runtime logger + subscriber fanout.
- `keys/`: global key map model.
- `common/`: shared public interfaces/types.

## Quick Start

For a practical walkthrough, see [docs/getting-started.md](docs/getting-started.md).

## Architectural Notes

For durable design rationale and decisions, see [docs/architecture-decisions.md](docs/architecture-decisions.md).

## Roadmap

The roadmap now tracks only open work. See [.github/ROADMAP.md](.github/ROADMAP.md).

## Local Development

### Prerequisites

Install these once to match the full CI gate locally:

```bash
# Go (1.26+) - https://go.dev/dl/

# golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# shellcheck
brew install shellcheck          # macOS/Linux via Homebrew
# or: apt-get install shellcheck

# markdownlint-cli2
npm install -g markdownlint-cli2

# actionlint
brew install actionlint          # macOS/Linux
# or: go install github.com/rhysd/actionlint/cmd/actionlint@latest

# govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
```

- Build baseline:
  - `go build -o tui_base_test_build . && rm -f tui_base_test_build tui_base_test_build.exe`
- Run tests:
  - `go test ./... -v`
- Run race tests:
  - `go test -race ./... -v`
- Run lint:
  - `GOOS=windows GOARCH=amd64 golangci-lint run ./...`
  - `GOOS=linux GOARCH=amd64 golangci-lint run ./...`
- Run full local verification (hook-equivalent):
  - `bash tools/local_verify.sh`

# Getting Started with tui-base

This tutorial is for engineers building a multi-page Charm v2 application that needs structure, observability, and maintainability from day one.

## 1. Prerequisites

- Go 1.26+
- A terminal that supports Bubble Tea interaction
- Optional: VS Code tasks from this workspace

## 2. Build and Test First

Run these before making changes:

```bash
go build -o tui_base_test_build.exe . && rm tui_base_test_build.exe
go test ./... -v
```

## 3. Understand the Runtime Model

High-level flow:

1. `main.go` starts the router model.
2. `router/` owns global wiring (pages, nav, status, colors, notifications).
3. Each page is a `tea.Model` with `Init`, `Update`, `View`.
4. Shared app style pointer is passed into children with `SetColors`.

This lets theme changes propagate immediately without replacing every child model.

## 4. Key Engineering Patterns to Follow

- Keep I/O in `tea.Cmd`; avoid blocking `Update`.
- Always render through style helpers from `theme`.
- Maintain keyboard and mouse parity for interactive elements.
- Prefer composing smaller models rather than expanding router switch logic.

## 5. Debug While You Build

Use the debug page and status bar to inspect behavior:

- Message log deduplicates repeated events.
- Runtime logs are streamed from the shared logger.
- Mouse routing highlight helps verify coordinate forwarding.
- Notification test actions validate toast/history paths.

Useful global shortcuts during development:

- `ctrl+,` opens Settings from any normal page.
- `ctrl+b` toggles navigation visibility.
- `ctrl+h` toggles expanded help.
- `ctrl+j` toggles status bar visibility.

## 6. Add a New Page Safely

Checklist:

1. Implement page model (`Init`, `Update`, `View`).
2. Support `SetColors` and use shared styles.
3. Add keyboard bindings and mouse handling.
4. Register page in router startup path.
5. Add tests for update/view behavior and event handling.

## 7. Verify Before Commit

```bash
go test ./... -v
go test -race ./... -v
```

If available in your environment, also run:

```bash
modernize -fix ./...
```

## 8. What to Read Next

- [docs/architecture-decisions.md](architecture-decisions.md)
- [docs/mouse-routing.md](mouse-routing.md)
- [docs/mouse-debugging.md](mouse-debugging.md)
- [.github/CHARM_ECOSYSTEM.md](../.github/CHARM_ECOSYSTEM.md)
- [.github/ROADMAP.md](../.github/ROADMAP.md)

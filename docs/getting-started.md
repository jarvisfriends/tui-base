# Getting Started with tui-base

This tutorial is for engineers building a multi-page Charm v2 application that needs structure,
observability, and maintainability from day one.

## 1. Prerequisites

- Go 1.26.4+ (1.26.3 and below contain a known CVE)
- A terminal that supports Bubble Tea interaction
- Optional: VS Code tasks from this workspace

## 2. Build and Test First

Run these before making changes:

```bash
go build ./... && go run ./cmd/tui-base
go test ./... -v
```

## 3. Understand the Runtime Model

High-level flow:

1. `cmd/tui-base/main.go` starts the router model (or call `tuibase.Run` from
   the root package in your own app).
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

Use the inspector page and status bar to inspect behavior:

- Message log deduplicates repeated events.
- Runtime logs are streamed from the shared logger.
- Mouse routing highlight helps verify coordinate forwarding.
- Notification test actions validate toast/history paths.

Useful global shortcuts during development:

- `ctrl+,` opens Settings from any normal page.
- `ctrl+b` toggles navigation visibility.
- `ctrl+h` toggles expanded help.
- `ctrl+j` toggles status bar visibility.

## 6. Build Pending Action Prompts

Use the shared notification manager for anything that should briefly toast but stay pending until resolved.

Pattern:

1. Emit `notifications.AddMsg{Key, Pending: true, TTL: ..., Content: ...}`.
2. Register a router overlay for the actual prompt UI.
3. Handle `notifications.ActivateMsg` to open that overlay when the user selects the pending item from
   notification history.
4. On resolve, emit `notifications.DismissMsg` or `notifications.DismissKeyMsg` and persist whatever decision
   hash/state prevents repeated prompts.

The status bar bell automatically shows the pending count, while the toast can expire independently of the item
remaining selectable in history.

## 7. Add a New Page Safely

Checklist:

1. Implement page model (`Init() tea.Cmd`, `Update(tea.Msg) (tea.Model, tea.Cmd)`, `View() tea.View`).
   - **Note on Bubble Tea v2**: Ensure `Init` returns just `tea.Cmd` (not the model)
   - and `View` returns `tea.View` (e.g. `tea.NewView("...")`).
2. Support `SetColors` and use shared styles.
3. Add keyboard bindings and mouse handling.
4. Register page in router startup path.
5. Add tests for update/view behavior and event handling.

## 8. Verify Before Commit

```bash
go test ./... -v
go test -race ./... -v
GOOS=windows GOARCH=amd64 golangci-lint run ./...
GOOS=linux GOARCH=amd64 golangci-lint run ./...
```

Preferred local gate (matches pre-commit expectations):

```bash
bash tools/local_verify.sh
```

If available in your environment, also run:

```bash
modernize -fix ./...
```

## 9. What to Read Next

- [docs/architecture-decisions.md](architecture-decisions.md)
- [docs/mouse-routing.md](mouse-routing.md)
- [docs/mouse-debugging.md](mouse-debugging.md)
- [.github/CHARM_ECOSYSTEM.md](../.github/CHARM_ECOSYSTEM.md)
- [.github/ROADMAP.md](../.github/ROADMAP.md)

# Architecture Decisions

This document captures stable decisions that were previously mixed into completed roadmap items.

## ADR-001: Router-Owned Composition

Decision:
- The router is the root model that owns navigation, active page selection, status bar integration, theme pointer wiring, and message forwarding.

Why:
- Centralizes cross-cutting concerns and keeps page models focused on local behavior.

## ADR-002: Shared Mutable App Style Pointer

Decision:
- Use a single shared `*theme.AppStyle` pointer across router, navigation, pages, and status components.

Why:
- Theme changes can be applied in place and become visible everywhere immediately.
- Avoids deep rebuild/rebind of model trees on theme switch.

## ADR-003: Theme-Driven Rendering Contract

Decision:
- UI rendering should use style helpers (`c.Styles.*`) rather than ad hoc colors.

Why:
- Provides visual consistency and supports centralized theme evolution.

## ADR-004: Non-Blocking Update Loop

Decision:
- Runtime I/O belongs in `tea.Cmd` paths, not directly in `Update`.

Why:
- Keeps the Bubble Tea event loop responsive and predictable.

## ADR-005: Navigation Input Parity

Decision:
- Navigation primitives support both keyboard and mouse inputs with equivalent semantics.

Why:
- Reduces accessibility and usability regressions as features grow.

## ADR-006: Built-In Runtime Observability

Decision:
- Debug inspector and logger fan-out are first-class parts of the architecture.

Why:
- Large TUIs are easier to evolve when message flow and logs are inspectable in-app.

## ADR-007: Shared Notification Manager

Decision:
- Use a shared `*notifications.Manager` pointer pattern similar to colors.

Why:
- Gives one source of truth for toast/history rendering, persistence, and runtime control.

## ADR-008: Compositor-Based Overlay Strategy

Decision:
- Use Lip Gloss compositor patterns for overlays (notification/history and future inspector overlay).

Why:
- Overlay positioning and layering remain stable without each page implementing custom z-order logic.

## ADR-009: Engineering Workflow Baseline

Decision:
- Keep tests and static checks as part of normal development (`go test`, race tests, vet; modernize when available).

Why:
- Maintains confidence while iterating quickly on interaction-heavy code.

## ADR-010: Public Reusability Bias

Decision:
- Types in `common/` may be consumer-facing even if not yet heavily used internally.

Why:
- `tui-base` is intended as a reusable foundation, not only an app-local code dump.
- These types should be periodically reviewed for real external value.

// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

// Package common holds small shared building blocks with no UI dependencies:
// the Component interface contract for pages, build/version metadata baked in
// via -ldflags (AppVersion, dependency info for the info modal), and
// WriteFileAtomic for crash-safe config persistence.
//
// Types here may be consumer-facing even when lightly used internally
// (ADR-010) — tui-base is a reusable foundation, not an app-local dump.
package common

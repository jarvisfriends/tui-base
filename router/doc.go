// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

// Package router provides the root tea.Model for tui-base applications: it
// owns navigation, active-page selection, the status bar, the shared theme
// pointer, the notification manager, and the Z-ordered overlay stack (toast,
// notification history, Ctrl+D inspector, info modal).
//
// Applications construct a router with NewWithOptions (or through the root
// tuibase package), register pages via Options.ExtraPages or
// (*RouterModel).RegisterPage, and run it with NewProgramWithEnvVar or
// NewProgramWithContext. Messages are dispatched to the active page; non-key,
// non-mouse messages broadcast to every page unless they implement
// TargetedMsg. Overlays own all input — keyboard and mouse — while visible
// (see docs/mouse-routing.md).
package router

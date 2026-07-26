// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

// Package envpath shortens absolute paths for display by collapsing
// well-known environment-variable prefixes (e.g.
// C:\Users\alice\AppData\Roaming\app becomes $APPDATA/app), keeping long
// paths readable in narrow TUI layouts.
package envpath

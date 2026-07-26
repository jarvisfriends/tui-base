// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

// Package settings is the built-in three-pane settings page: a compact
// overview of every setting with an edit overlay per item (huh forms for
// selects/text/file pickers; custom overlays for directories, multi-file
// lists, durations, dates, and key recording).
//
// Applications contribute their own sections via config.Section passed
// through router.Options.SettingsSections; values persist to
// tui_settings.json (atomically) in the app's config directory. Theme,
// navigation, logging, notification, and keybinding changes are applied live
// through messages the router consumes.
package settings

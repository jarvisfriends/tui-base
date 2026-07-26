// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings

import (
	"bytes"
	"encoding/json"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/theme"
)

// FilePath returns the absolute (or CWD-relative) path of the persisted
// settings file (tui_settings.json under the configured directory). Exposed
// so the router can watch it for external edits (FW-1).
func FilePath() string { return settingsFilePath() }

// ReloadFromDisk re-reads the persisted settings file and applies it to the
// model. It reports whether anything actually changed — the app's own saves
// also trigger file-watch events, and those must not produce "reloaded"
// noise. When changed, the returned command emits the ThemeMsg that
// re-applies theme preferences router-wide (same contract as abortEdit).
func (m *SettingsModel) ReloadFromDisk() (changed bool, cmd tea.Cmd) {
	before, err := json.Marshal(m)
	if err != nil {
		return false, nil
	}
	if loadErr := m.LoadFromFile(settingsFilePath()); loadErr != nil {
		return false, nil
	}
	after, err := json.Marshal(m)
	if err != nil || bytes.Equal(before, after) {
		return false, nil
	}

	m.buildItems()
	m.ThemeMode = theme.NormalizeMode(m.ThemeMode)
	m.StylePreset = string(theme.NormalizePreset(m.StylePreset))
	m.ColorThemeID = theme.ResolveTintIDForMode(m.ColorThemeID, m.ThemeMode)
	id := m.ColorThemeID
	mode := m.ThemeMode
	style := m.StylePreset
	accessibility := m.AccessibilityColors
	return true, func() tea.Msg {
		return ThemeMsg{
			ID:               id,
			Mode:             mode,
			Style:            style,
			Accessibility:    accessibility,
			ApplyPreferences: true,
		}
	}
}

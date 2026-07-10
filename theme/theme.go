// Package theme is tui-base's compatibility surface over the shared style
// contract, which moved to github.com/jarvisfriends/snap/styles in the
// wholesale component wave (ROADMAP SP-2 follow-up, 2026-07-10). Every
// symbol here aliases its snap/styles counterpart so existing imports keep
// working; new code can import snap/styles directly.
package theme

import (
	styles "github.com/jarvisfriends/snap/styles"
)

// Type aliases.
type (
	// AppStyle re-exports [styles.AppStyle].
	AppStyle = styles.AppStyle
	// ColorAware re-exports [styles.ColorAware].
	ColorAware = styles.ColorAware
	// Styles re-exports [styles.Styles].
	Styles = styles.Styles
	// StylePreset re-exports [styles.StylePreset].
	StylePreset = styles.StylePreset
	// ColorPair re-exports [styles.ColorPair].
	ColorPair = styles.ColorPair
	// ThemePreferences re-exports [styles.ThemePreferences].
	ThemePreferences = styles.ThemePreferences
)

// Constants.
const (
	// ThemeModeDark re-exports [styles.ThemeModeDark].
	ThemeModeDark = styles.ThemeModeDark
	// ThemeModeLight re-exports [styles.ThemeModeLight].
	ThemeModeLight = styles.ThemeModeLight
	// DefaultStylePreset re-exports [styles.DefaultStylePreset].
	DefaultStylePreset = styles.DefaultStylePreset
)

// Function aliases.
var (
	// Active re-exports [styles.Active].
	Active = styles.Active
	// SetCurrentTint re-exports [styles.SetCurrentTint].
	SetCurrentTint = styles.SetCurrentTint
	// FromTint re-exports [styles.FromTint].
	FromTint = styles.FromTint
	// SetThemePreferences re-exports [styles.SetThemePreferences].
	SetThemePreferences = styles.SetThemePreferences
	// ThemePreferencesSnapshot re-exports [styles.ThemePreferencesSnapshot].
	ThemePreferencesSnapshot = styles.ThemePreferencesSnapshot
	// NormalizeMode re-exports [styles.NormalizeMode].
	NormalizeMode = styles.NormalizeMode
	// NormalizePreset re-exports [styles.NormalizePreset].
	NormalizePreset = styles.NormalizePreset
	// StylePresets re-exports [styles.StylePresets].
	StylePresets = styles.StylePresets
	// ResolveTintIDForMode re-exports [styles.ResolveTintIDForMode].
	ResolveTintIDForMode = styles.ResolveTintIDForMode
	// HuhThemeFunc re-exports [styles.HuhThemeFunc].
	HuhThemeFunc = styles.HuhThemeFunc
	// TableStyles re-exports [styles.TableStyles].
	TableStyles = styles.TableStyles
	// ReapplyBg re-exports [styles.ReapplyBg].
	ReapplyBg = styles.ReapplyBg
	// ColorHex re-exports [styles.ColorHex].
	ColorHex = styles.ColorHex
	// AccessiblePairsFromTint re-exports [styles.AccessiblePairsFromTint].
	AccessiblePairsFromTint = styles.AccessiblePairsFromTint
	// RegisterYAMLTints re-exports [styles.RegisterYAMLTints].
	RegisterYAMLTints = styles.RegisterYAMLTints
	// LoadYAMLTints re-exports [styles.LoadYAMLTints].
	LoadYAMLTints = styles.LoadYAMLTints
)

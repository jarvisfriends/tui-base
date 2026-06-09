// Package theme builds the application's visual styling by combining three
// orthogonal axes that the end user can choose independently:
//
//   - color theme — any of the bubbletint palettes (the color source),
//   - style preset — huh's built-in form structure (borders, prefixes,
//     indicators); see [StylePreset] and BuildHuhStyles,
//   - mode — light or dark,
//
// plus an optional accessibility pass that adjusts foreground colors for
// contrast and color-vision deficiencies (see accessibility.go).
//
// Most consumers only need [Active] (the current palette), [HuhThemeFunc] (for
// huh forms), and the [ColorAware] interface. The package keeps a small amount
// of custom code: the semantic [AppStyle]/[Styles] mapping that lipgloss and huh
// do not provide, and the CVD accessibility engine that has no library
// equivalent. Everything structural is delegated to huh and lipgloss.
package theme

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"sync"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

// ColorAware is implemented by any component that accepts a shared color palette pointer.
type ColorAware interface {
	SetColors(c *AppStyle)
}

const (
	ThemeModeDark  = "dark"
	ThemeModeLight = "light"
)

// ThemePreferences controls global theme behavior applied by Active and
// HuhThemeFunc. Mode selects light/dark, Style selects the huh structural
// preset, and Accessibility toggles the CVD foreground adjustments.
type ThemePreferences struct {
	Mode          string
	Accessibility bool
	Style         StylePreset
}

var (
	appStyleCacheMu sync.RWMutex
	appStyleCache   = map[string]*AppStyle{}

	themePrefsMu sync.RWMutex
	themePrefs   = ThemePreferences{Mode: ThemeModeDark, Style: DefaultStylePreset}

	// tintMu serialises writes to the bubbletint global registry. The library
	// is not goroutine-safe; concurrent calls to tint.SetTintID or
	// tint.NewDefaultRegistry produce data races. All callers in this module
	// should use SetCurrentTint instead of calling tint.SetTintID directly.
	tintMu sync.Mutex
)

// SetCurrentTint sets the active tint ID on the bubbletint global registry.
// It is safe to call from multiple goroutines.
func SetCurrentTint(id string) error {
	if id == "" {
		return nil
	}
	tintMu.Lock()
	defer tintMu.Unlock()
	if ok := tint.SetTintID(id); !ok {
		return fmt.Errorf("unknown tint ID: %s", id)
	}
	return nil
}

// AppStyle holds the semantic color palette for the application, derived from
// the active bubbletint theme. Each field maps a UI role to a color.Color.
// Call [Active] on every render to pick up live theme changes.
type AppStyle struct {
	// Fg is the primary foreground / body text color.
	Fg color.Color
	// Bg is the primary background color.
	Bg color.Color
	// Muted is used for secondary / dimmed text and inactive navigation items
	// (maps to the "comment" or "bright_black" slot in most terminal themes).
	Muted color.Color
	// Border is used for borders and dividers (typically slightly darker than Muted).
	Border color.Color
	// Accent is the primary accent color: navigation titles, box titles, and
	// tab / form highlights (maps to the purple/violet slot).
	Accent color.Color
	// SelectionBg is the background of the active / selected navigation item.
	SelectionBg color.Color
	// SelectionFg is the foreground of the selected navigation item.
	SelectionFg color.Color
	// StatusBg is the status bar background color.
	StatusBg color.Color
	// StatusFg is the status bar foreground color.
	StatusFg color.Color
	// Success is used for affirmative / selected-option states (green slot).
	Success color.Color
	// Error is used for error indicators (red slot).
	Error color.Color
	// Warning is used for warning / indicator cues (yellow slot).
	Warning color.Color

	Styles          *Styles     // pre-computed lipgloss styles (app chrome) for this palette
	HuhStyles       *huh.Styles // pre-computed huh form styles for this palette + current style preset
	OrigTint        *tint.Tint  // the original tint this palette was derived from; used for debugging and testing
	AccessibleTint  *tint.Tint  // a suggested tint with improved accessibility, if the original fails; used for debugging and testing
	OrigPairs       []ColorPair // all color combinations from the original tint
	AccessiblePairs []ColorPair // the same pairs but with colors adjusted for accessibility where needed
}

// col returns a color.Color from a *tint.Color, using fallback ANSI/hex string when nil.
func col(c *tint.Color, fallback string) color.Color {
	if c != nil {
		return lipgloss.Color(c.Hex())
	}
	return lipgloss.Color(fallback)
}

// borderColor resolves the divider/border color for a tint. It prefers the
// theme's "black" slot and otherwise derives a visible border from the
// background (lighter on dark themes, darker on light themes) using lipgloss.
func borderColor(t *tint.Tint) color.Color {
	if t.Black != nil {
		return lipgloss.Color(t.Black.Hex())
	}
	bg := col(t.Bg, "235")
	if t.Dark {
		return lipgloss.Lighten(bg, 0.12)
	}
	return lipgloss.Darken(bg, 0.12)
}

// NormalizeMode returns the normalized theme mode value.
func NormalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ThemeModeLight:
		return ThemeModeLight
	default:
		return ThemeModeDark
	}
}

// SetThemePreferences updates global preferences used by Active and HuhThemeFunc.
func SetThemePreferences(mode string, accessibility bool, style StylePreset) {
	themePrefsMu.Lock()
	themePrefs.Mode = NormalizeMode(mode)
	themePrefs.Accessibility = accessibility
	themePrefs.Style = NormalizePreset(string(style))
	themePrefsMu.Unlock()
}

// ThemePreferencesSnapshot returns a copy of the current global preferences.
func ThemePreferencesSnapshot() ThemePreferences {
	themePrefsMu.RLock()
	defer themePrefsMu.RUnlock()
	return themePrefs
}

// ResolveTintIDForMode returns a tint ID matching the requested mode.
// If requestedID already matches, it is returned unchanged.
func ResolveTintIDForMode(requestedID string, mode string) string {
	requestedModeDark := NormalizeMode(mode) == ThemeModeDark
	tints := tint.Tints()
	if len(tints) == 0 {
		return requestedID
	}

	if requestedID != "" {
		for _, t := range tints {
			if t.ID == requestedID && t.Dark == requestedModeDark {
				return requestedID
			}
		}
	}

	for _, t := range tints {
		if t.Dark == requestedModeDark {
			return t.ID
		}
	}

	if requestedID != "" {
		return requestedID
	}
	return tints[0].ID
}

func tintForMode(current *tint.Tint, mode string) *tint.Tint {
	if current == nil {
		return nil
	}
	requestedModeDark := NormalizeMode(mode) == ThemeModeDark
	if current.Dark == requestedModeDark {
		return current
	}
	resolved := ResolveTintIDForMode(current.ID, mode)
	for _, candidate := range tint.Tints() {
		if candidate.ID == resolved {
			return candidate
		}
	}
	return current
}

// Active returns the current AppStyle palette derived from the active bubbletint.
// It is safe to call before the registry has been initialised; a built-in
// fallback palette (matching the Dracula aesthetic) is returned in that case.
func Active() *AppStyle {
	prefs := ThemePreferencesSnapshot()
	var t *tint.Tint
	func() {
		defer func() { recover() }() //nolint:errcheck
		t = tint.Current()
	}()
	t = tintForMode(t, prefs.Mode)
	return fromTint(t, prefs.Accessibility, prefs.Style)
}

// FromTint maps a *tint.Tint onto the application's semantic AppStyle.
// Every field has a hardcoded fallback that works in any 256-color terminal.
func FromTint(t *tint.Tint) *AppStyle {
	return fromTint(t, false, DefaultStylePreset)
}

// FromTintWithOptions maps a tint into AppStyle with optional accessibility
// adjustments for semantic foreground/background pairs.
func FromTintWithOptions(t *tint.Tint, accessibility bool) *AppStyle {
	return fromTint(t, accessibility, DefaultStylePreset)
}

// fromTint builds (and caches) the full styling artifact for a tint: the
// semantic palette, the app-chrome Styles (palette-derived, independent of the
// form style preset), and the huh form Styles (which DO depend on the preset).
// Caching by tint ID, preset, and accessibility means HuhThemeFunc can reuse the
// precomputed huh styles instead of maintaining a second cache.
func fromTint(t *tint.Tint, accessibility bool, preset StylePreset) *AppStyle {
	preset = NormalizePreset(string(preset))
	cacheKey := "fallback"
	if t != nil && t.ID != "" {
		cacheKey = t.ID
	}
	cacheKey += "|" + string(preset)
	if accessibility {
		cacheKey += "|access"
	}

	appStyleCacheMu.RLock()
	cached, ok := appStyleCache[cacheKey]
	appStyleCacheMu.RUnlock()
	if ok {
		return cached
	}

	if t == nil {
		pairs := colorPairsFromSimple("250", "235")
		colors := &AppStyle{
			Fg:              lipgloss.Color("250"),
			Bg:              lipgloss.Color("235"),
			Muted:           lipgloss.Color("240"),
			Border:          lipgloss.Color("238"),
			Accent:          lipgloss.Color("205"),
			SelectionBg:     lipgloss.Color("62"),
			SelectionFg:     lipgloss.Color("255"),
			StatusBg:        lipgloss.Color("236"),
			StatusFg:        lipgloss.Color("250"),
			Success:         lipgloss.Color("35"),
			Error:           lipgloss.Color("9"),
			Warning:         lipgloss.Color("11"),
			OrigPairs:       pairs,
			AccessiblePairs: colorPairsFromSimple("250", "235"),
		}
		if accessibility {
			applyAccessibilityAdjustments(colors)
			colors.AccessiblePairs = colorPairsFromSimple("250", "235")
		}
		colors.Styles = BuildStyles(colors)
		colors.HuhStyles = BuildHuhStyles(colors, preset, true)

		appStyleCacheMu.Lock()
		appStyleCache[cacheKey] = colors
		appStyleCacheMu.Unlock()
		return colors
	}

	// Selection background: prefer the theme's explicit selection color, then
	// fall back to its blue slot, then a reasonable default.
	var sel color.Color
	if t.SelectionBg != nil {
		sel = lipgloss.Color(t.SelectionBg.Hex())
	} else if t.Blue != nil {
		sel = lipgloss.Color(t.Blue.Hex())
	} else {
		sel = lipgloss.Color("62")
	}

	o := colorPairsFromTint(t, false)
	colors := &AppStyle{
		Fg:          col(t.Fg, "250"),
		Bg:          col(t.Bg, "235"),
		Muted:       col(t.BrightBlack, "240"),
		Border:      borderColor(t),
		Accent:      col(t.Purple, "205"),
		SelectionBg: sel,
		SelectionFg: col(t.BrightWhite, "255"),
		StatusBg:    col(t.Black, "236"),
		StatusFg:    col(t.Fg, "250"),
		Success:     col(t.Green, "35"),
		Error:       col(t.Red, "9"),
		Warning:     col(t.Yellow, "11"),
		OrigTint:    t,
		OrigPairs:   o,
		// Accessibility-adjusted pairs are expensive to compute; defer to
		// explicit callers (for example, the debug accessibility panel).
		AccessiblePairs: o,
	}

	// Dynamic selection contrast adjustment
	bgL := colorLuminance(colors.Bg)
	selL := colorLuminance(colors.SelectionBg)
	if math.Abs(bgL-selL) < 25.0 {
		if t.Dark {
			colors.SelectionBg = lipgloss.Lighten(colors.Bg, 0.15)
		} else {
			colors.SelectionBg = lipgloss.Darken(colors.Bg, 0.15)
		}
	}

	if accessibility {
		applyAccessibilityAdjustments(colors)
		colors.AccessiblePairs = colorPairsFromTint(t, true)
	}
	colors.Styles = BuildStyles(colors)
	colors.HuhStyles = BuildHuhStyles(colors, preset, t.Dark)

	appStyleCacheMu.Lock()
	appStyleCache[cacheKey] = colors
	appStyleCacheMu.Unlock()
	return colors
}

func colorLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
}

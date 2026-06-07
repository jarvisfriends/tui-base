package theme

import "charm.land/huh/v2"

// StylePreset selects the structural source for huh form styling: borders,
// prefixes, indicators, padding, and glyphs. It is orthogonal to the color
// theme (supplied by the active bubbletint) and to the light/dark mode. Any
// preset can be combined with any of the registered tints.
type StylePreset string

const (
	PresetBase       StylePreset = "base"
	PresetCharm      StylePreset = "charm"
	PresetDracula    StylePreset = "dracula"
	PresetBase16     StylePreset = "base16"
	PresetCatppuccin StylePreset = "catppuccin"
)

// DefaultStylePreset is used when no preset has been chosen or an unknown value
// is supplied. Charm has the most refined prefixes/indicators of the built-ins.
const DefaultStylePreset = PresetCharm

// presetThemes maps each preset to the huh built-in theme function that defines
// its structure. Colors from these themes are overwritten by the active tint in
// overlayPaletteColors; only the structural decisions are kept.
var presetThemes = map[StylePreset]func(bool) *huh.Styles{
	PresetBase:       huh.ThemeBase,
	PresetCharm:      huh.ThemeCharm,
	PresetDracula:    huh.ThemeDracula,
	PresetBase16:     huh.ThemeBase16,
	PresetCatppuccin: huh.ThemeCatppuccin,
}

// orderedPresets is the stable display order for pickers.
var orderedPresets = []StylePreset{
	PresetCharm, PresetBase, PresetDracula, PresetBase16, PresetCatppuccin,
}

// NormalizePreset validates s and returns a known StylePreset, defaulting to
// DefaultStylePreset for empty or unrecognized values.
func NormalizePreset(s string) StylePreset {
	p := StylePreset(s)
	if _, ok := presetThemes[p]; ok {
		return p
	}
	return DefaultStylePreset
}

// StylePresets returns the selectable style presets in display order. Use this
// to build a settings picker.
func StylePresets() []StylePreset {
	out := make([]StylePreset, len(orderedPresets))
	copy(out, orderedPresets)
	return out
}

// DisplayName returns a human-friendly label for the preset.
func (p StylePreset) DisplayName() string {
	switch p {
	case PresetBase:
		return "Base"
	case PresetCharm:
		return "Charm"
	case PresetDracula:
		return "Dracula"
	case PresetBase16:
		return "Base16"
	case PresetCatppuccin:
		return "Catppuccin"
	default:
		return string(p)
	}
}

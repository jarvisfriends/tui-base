package theme

import (
	"testing"

	tint "github.com/lrstanley/bubbletint/v2"
)

// TestStylePresetsPreserveStructureAndApplyColor verifies the core redesign
// invariant: BuildHuhStyles takes structure (prefixes/glyphs/borders) from the
// chosen StylePreset's huh theme while overlaying colors from the active tint.
func TestStylePresetsPreserveStructureAndApplyColor(t *testing.T) {
	tint.NewDefaultRegistry()
	tints := tint.DefaultTints()
	if len(tints) == 0 {
		t.Skip("no tints registered")
	}
	app := FromTint(tints[0])

	for _, p := range StylePresets() {
		for _, isDark := range []bool{true, false} {
			s := BuildHuhStyles(app, p, isDark)
			if s == nil {
				t.Errorf("preset %s isDark=%v: nil styles", p, isDark)
				continue
			}

			// Structure survives the overlay: the raw glyph (set-string, which is
			// independent of color) must match the preset's own huh theme.
			want := presetThemes[p](isDark).Focused.SelectedPrefix.Value()
			if got := s.Focused.SelectedPrefix.Value(); got != want {
				t.Errorf("preset %s isDark=%v: SelectedPrefix glyph = %q, want %q (structure not preserved)", p, isDark, got, want)
			}

			// Color is applied from the tint: focused title fg == palette Accent.
			if fg := s.Focused.Title.GetForeground(); fg != app.Accent {
				t.Errorf("preset %s isDark=%v: focused title fg = %v, want accent %v", p, isDark, fg, app.Accent)
			}
		}
	}

	// Distinct presets should produce distinct structure. Base uses bracketed
	// checkbox prefixes; Charm uses check/dot glyphs.
	base := BuildHuhStyles(app, PresetBase, true)
	charm := BuildHuhStyles(app, PresetCharm, true)
	if base.Focused.SelectedPrefix.Value() == charm.Focused.SelectedPrefix.Value() {
		t.Errorf("expected Base and Charm presets to differ in SelectedPrefix glyph; both = %q", base.Focused.SelectedPrefix.Value())
	}
}

// TestNormalizePreset checks defaulting and round-tripping of preset values.
func TestNormalizePreset(t *testing.T) {
	cases := map[string]StylePreset{
		"":           DefaultStylePreset,
		"nonsense":   DefaultStylePreset,
		"charm":      PresetCharm,
		"base":       PresetBase,
		"dracula":    PresetDracula,
		"base16":     PresetBase16,
		"catppuccin": PresetCatppuccin,
	}
	for in, want := range cases {
		if got := NormalizePreset(in); got != want {
			t.Errorf("NormalizePreset(%q) = %q, want %q", in, got, want)
		}
	}
}

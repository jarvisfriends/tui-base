package keys

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

func TestDefaultKeyMapProvidesExpectedBindings(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	if km == nil {
		t.Fatal("DefaultKeyMap() returned nil")
		return
	}
	if len(km.ShortHelp()) == 0 {
		t.Fatal("ShortHelp() returned no bindings")
	}
	if len(km.FullHelp()) == 0 {
		t.Fatal("FullHelp() returned no rows")
	}
	if km.OpenSettings.Help().Desc != "settings" {
		t.Fatalf("OpenSettings help desc = %q; want %q", km.OpenSettings.Help().Desc, "settings")
	}
	if keys := km.OpenSettings.Help().Key; keys != "ctrl+g" {
		t.Fatalf("OpenSettings help key = %q; want %q", keys, "ctrl+g")
	}
}

func TestDefaultKeyMapImplementsHelpKeyMap(t *testing.T) {
	t.Parallel()

	var bindings help.KeyMap = DefaultKeyMap()
	if len(bindings.ShortHelp()) == 0 {
		t.Fatal("help.KeyMap.ShortHelp() returned no bindings")
	}
}

func TestNoDuplicateDefaultKeys(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	bindings := []struct {
		name    string
		binding key.Binding
	}{
		{"Quit", km.Quit},
		{"NextPage", km.NextPage},
		{"PreviousPage", km.PreviousPage},
		{"OpenSettings", km.OpenSettings},
		{"ToggleNav", km.ToggleNav},
		{"ToggleStatus", km.ToggleStatus},
		{"ToggleFullHelp", km.ToggleFullHelp},
		{"Select", km.Select},
		{"Top", km.Top},
		{"Bottom", km.Bottom},
		{"Dismiss", km.Dismiss},
		{"DismissAll", km.DismissAll},
		{"Debug", km.Debug},
		{"PageDown", km.PageDown},
		{"PageUp", km.PageUp},
		{"HalfPageUp", km.HalfPageUp},
		{"HalfPageDown", km.HalfPageDown},
		{"Up", km.Up},
		{"Down", km.Down},
		{"Left", km.Left},
		{"Right", km.Right},
	}

	seen := make(map[string]string)
	for _, b := range bindings {
		for _, k := range b.binding.Keys() {
			kNorm := strings.ToLower(strings.TrimSpace(k))
			if kNorm == "" {
				continue
			}
			if existing, found := seen[kNorm]; found {
				t.Errorf("Duplicate key %q: found in %q and %q", kNorm, existing, b.name)
			}
			seen[kNorm] = b.name
		}
	}
}

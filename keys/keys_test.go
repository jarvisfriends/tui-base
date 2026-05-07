package keys

import (
	"testing"

	"charm.land/bubbles/v2/help"
)

func TestDefaultKeyMapProvidesExpectedBindings(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	if km == nil {
		t.Fatal("DefaultKeyMap() returned nil")
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
	if keys := km.OpenSettings.Help().Key; keys != "ctrl+," {
		t.Fatalf("OpenSettings help key = %q; want %q", keys, "ctrl+,")
	}
}

func TestDefaultKeyMapImplementsHelpKeyMap(t *testing.T) {
	t.Parallel()

	var bindings help.KeyMap = DefaultKeyMap()
	if bindings == nil {
		t.Fatal("expected DefaultKeyMap to satisfy help.KeyMap")
	}
	if len(bindings.ShortHelp()) == 0 {
		t.Fatal("help.KeyMap.ShortHelp() returned no bindings")
	}
}

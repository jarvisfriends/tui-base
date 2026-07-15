package settings

import (
	"testing"

	"github.com/jarvisfriends/tui-base/theme"
)

// TestColorThemePickerLeadsWithBuiltins verifies the Color Theme picker lists
// snap's built-in themes first (in display order) and that each is actually
// registered, so selecting it applies rather than silently failing.
func TestColorThemePickerLeadsWithBuiltins(t *testing.T) {
	theme.EnsureRegistry()

	want := theme.BuiltinTintIDs()
	if len(want) != 7 {
		t.Fatalf("expected 7 built-in themes, got %d", len(want))
	}

	opts := buildThemeOptions("dark")
	if len(opts) < len(want) {
		t.Fatalf("picker has %d options, want at least %d", len(opts), len(want))
	}

	for i, id := range want {
		if opts[i].Value != id {
			t.Errorf("picker option %d = %q, want built-in %q first", i, opts[i].Value, id)
		}
		// A listed theme must be selectable (present in the registry).
		if err := theme.SetCurrentTint(id); err != nil {
			t.Errorf("built-in theme %q not selectable: %v", id, err)
		}
	}
}

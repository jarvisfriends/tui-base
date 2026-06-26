package keys

import (
	"strings"
	"testing"
)

const testKeyCtrlQ = "ctrl+q"

func TestApplyCustomizationsUpdatesBinding(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	km.ApplyCustomizations(map[string]string{
		bindingQuit: testKeyCtrlQ,
	})
	keys := km.Quit.Keys()
	if len(keys) == 0 || keys[0] != testKeyCtrlQ {
		t.Errorf("Quit keys after customization = %v; want [ctrl+q]", keys)
	}
}

func TestApplyCustomizationsMultipleKeys(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	km.ApplyCustomizations(map[string]string{
		bindingNextPage: "n,ctrl+n",
	})
	keys := km.NextPage.Keys()
	if len(keys) != 2 {
		t.Errorf("NextPage keys after customization = %v; want [n ctrl+n]", keys)
	}
}

func TestApplyCustomizationsIgnoresEmpty(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	original := km.Quit.Keys()
	km.ApplyCustomizations(map[string]string{
		bindingQuit: "",
	})
	if strings.Join(km.Quit.Keys(), ",") != strings.Join(original, ",") {
		t.Errorf("empty custom value should not change binding; got %v", km.Quit.Keys())
	}
}

func TestApplyCustomizationsIgnoresNoneValue(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	original := km.Dismiss.Keys()
	km.ApplyCustomizations(map[string]string{
		bindingDismiss: "(none)",
	})
	if strings.Join(km.Dismiss.Keys(), ",") != strings.Join(original, ",") {
		t.Errorf("(none) value should not change binding; got %v", km.Dismiss.Keys())
	}
}

func TestApplyCustomizationsIgnoresUnknownKeys(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	// Unknown IDs should not panic and should not change anything.
	km.ApplyCustomizations(map[string]string{
		"NonExistent": "x",
	})
}

func TestApplyCustomizationsAllViewportBindings(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	km.ApplyCustomizations(map[string]string{
		bindingPageDown:     "ctrl+f",
		bindingPageUp:       "ctrl+b",
		bindingHalfPageDown: "ctrl+d",
		bindingHalfPageUp:   "ctrl+u",
		bindingUp:           "k",
		bindingDown:         "j",
		bindingLeft:         "h",
		bindingRight:        "l",
	})
	checks := []struct {
		name string
		got  []string
		want string
	}{
		{bindingPageDown, km.PageDown.Keys(), "ctrl+f"},
		{bindingPageUp, km.PageUp.Keys(), "ctrl+b"},
		{bindingHalfPageDown, km.HalfPageDown.Keys(), "ctrl+d"},
		{bindingHalfPageUp, km.HalfPageUp.Keys(), "ctrl+u"},
		{bindingUp, km.Up.Keys(), "k"},
		{bindingDown, km.Down.Keys(), "j"},
		{bindingLeft, km.Left.Keys(), "h"},
		{bindingRight, km.Right.Keys(), "l"},
	}
	for _, c := range checks {
		if len(c.got) == 0 || c.got[0] != c.want {
			t.Errorf("%s: keys = %v; want [%s]", c.name, c.got, c.want)
		}
	}
}

func TestBindingDefsCoversAllBindings(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	defs := km.BindingDefs()

	if len(defs) == 0 {
		t.Fatal("BindingDefs() returned no entries")
		return
	}

	seen := make(map[string]bool, len(defs))
	for _, d := range defs {
		if d.ID == "" {
			t.Errorf("BindingDef with empty ID: %+v", d)
		}
		if d.Title == "" {
			t.Errorf("BindingDef %q has empty Title", d.ID)
		}
		if seen[d.ID] {
			t.Errorf("duplicate BindingDef ID %q", d.ID)
		}
		seen[d.ID] = true
	}

	// Spot-check a few required IDs.
	required := []string{
		bindingQuit, bindingNextPage, bindingPreviousPage, bindingOpenSettings,
		bindingToggleNav, bindingToggleStatus, bindingDebug, bindingPageDown, bindingPageUp,
	}
	for _, id := range required {
		if !seen[id] {
			t.Errorf("BindingDefs missing required ID %q", id)
		}
	}
}

func TestBindingDefsDefaultKeysNonEmpty(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	for _, d := range km.BindingDefs() {
		if d.Def == "" {
			t.Errorf("BindingDef %q has empty default key string", d.ID)
		}
	}
}

func TestFullHelpReturnsRows(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	rows := km.FullHelp()
	if len(rows) == 0 {
		t.Fatal("FullHelp() returned no rows")
		return
	}
	for i, row := range rows {
		if len(row) == 0 {
			t.Errorf("FullHelp() row %d is empty", i)
		}
	}
}

func TestShortHelpReturnsBindings(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	bindings := km.ShortHelp()
	if len(bindings) < 2 {
		t.Errorf("ShortHelp() returned %d bindings; want at least 2", len(bindings))
	}
}

func TestApplyCustomizationsAndBindingDefsRoundTrip(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	km.ApplyCustomizations(map[string]string{
		bindingQuit:     testKeyCtrlQ,
		bindingNextPage: "n",
	})

	defs := km.BindingDefs()
	found := map[string]string{}
	for _, d := range defs {
		found[d.ID] = d.Def
	}

	if found[bindingQuit] != testKeyCtrlQ {
		t.Errorf("BindingDefs after customization: Quit = %q; want ctrl+q", found[bindingQuit])
	}
	if found[bindingNextPage] != "n" {
		t.Errorf("BindingDefs after customization: NextPage = %q; want n", found[bindingNextPage])
	}
}

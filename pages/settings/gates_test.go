package settings

import (
	"testing"

	"github.com/jarvisfriends/tui-base/gate"
)

// findItem returns the settings item with the given title, or nil.
func findItem(m *SettingsModel, title string) *settingItem {
	for i := range m.items {
		if m.items[i].title == title {
			return &m.items[i]
		}
	}
	return nil
}

// TestGateEditCommitsToRegistry is the regression test for the Feature Flags
// commit bug: the huh select used to bind to a local variable inside buildForm,
// so completing the form never reached the gate registry. The edit binding now
// lives on the model (gateEditVals) and apply commits it.
func TestGateEditCommitsToRegistry(t *testing.T) {
	const name = "Test Gate"
	g := gate.NewGateRegistry()
	g.Register(gate.FeatureGate{Name: name, Default: false})
	m := NewWithOptions(Options{Gates: g})

	item := findItem(m, name)
	if item == nil {
		t.Fatal("Feature Flags item for the registered gate was not built")
	}

	// Open the editor: buildForm seeds the binding from the registry.
	_ = item.buildForm()
	if got := *m.gateEditVals[name]; got != boolStrFalse {
		t.Fatalf("edit binding seeded to %q; want %q", got, boolStrFalse)
	}

	// The user selects Enabled (the huh select writes through the pointer),
	// then the completed form runs apply.
	*m.gateEditVals[name] = boolStrTrue
	if err := item.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if !g.Value(name) {
		t.Fatal("apply did not commit the new value to the gate registry")
	}
	if !m.gatesChanged {
		t.Fatal("gate change was not flagged for the GatesChangedMsg broadcast")
	}
}

// TestGateApplyNoChangeDoesNotBroadcast asserts committing the form without
// changing the value does not flag a broadcast.
func TestGateApplyNoChangeDoesNotBroadcast(t *testing.T) {
	const name = "Steady Gate"
	g := gate.NewGateRegistry()
	g.Register(gate.FeatureGate{Name: name, Default: true})
	m := NewWithOptions(Options{Gates: g})

	item := findItem(m, name)
	if item == nil {
		t.Fatal("Feature Flags item for the registered gate was not built")
	}
	_ = item.buildForm()
	if err := item.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if m.gatesChanged {
		t.Fatal("no-op apply flagged a gate change")
	}
	if !g.Value(name) {
		t.Fatal("no-op apply mutated the gate value")
	}
}

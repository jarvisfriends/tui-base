package tuibase_test

import (
	"testing"

	"github.com/jarvisfriends/tui-base/testutil"
)

const mod = "github.com/jarvisfriends/tui-base"

// TestArchitectureLayering enforces the dependency direction of the framework
// (TS-4): foundation packages must never import the composition layers above
// them. A violation here usually means logic landed in the wrong package.
func TestArchitectureLayering(t *testing.T) {
	ui := []string{mod + "/router", mod + "/pages"}

	// Pure foundations: no tui-base dependencies at all beyond themselves.
	testutil.CheckNoImports(t, mod+"/envpath", mod)

	// Mid-layer packages: independent of the UI composition layers.
	for _, pkg := range []string{
		mod + "/theme",
		mod + "/logging",
		mod + "/common",
		mod + "/config",
		mod + "/overlay",
	} {
		testutil.CheckNoImports(t, pkg, ui...)
	}

	// Pages must not import the router (the router composes pages, never the
	// reverse). Navigation, status, page, table, and notifications moved to
	// the snap repo in the wholesale wave and are layered there.
	testutil.CheckNoImports(t, mod+"/pages/...", mod+"/router")
}

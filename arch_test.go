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
	ui := []string{mod + "/router", mod + "/pages", mod + "/status", mod + "/navigation"}

	// Pure foundations: no tui-base dependencies at all beyond themselves.
	testutil.CheckNoImports(t, mod+"/geom", mod)
	testutil.CheckNoImports(t, mod+"/envpath", mod)
	testutil.CheckNoImports(t, mod+"/gate", mod)

	// Mid-layer packages: independent of the UI composition layers.
	for _, pkg := range []string{
		mod + "/theme",
		mod + "/keys",
		mod + "/logging",
		mod + "/common",
		mod + "/config",
		mod + "/notifications",
		mod + "/overlay",
		mod + "/page",
		mod + "/datepicker",
		mod + "/timepicker",
		mod + "/table",
	} {
		testutil.CheckNoImports(t, pkg, ui...)
	}

	// Navigation sits below the router and pages.
	testutil.CheckNoImports(t, mod+"/navigation", mod+"/router", mod+"/pages")

	// Pages must not import the router (the router composes pages, never the
	// reverse), and status must not import the router either.
	testutil.CheckNoImports(t, mod+"/pages/...", mod+"/router")
	testutil.CheckNoImports(t, mod+"/status", mod+"/router")
}

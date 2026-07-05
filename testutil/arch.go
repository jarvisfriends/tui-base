package testutil

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// CheckNoImports asserts that no package matched by pkgPattern directly
// imports a package whose path starts with any of the forbidden prefixes.
// Use it to enforce architectural layering in a plain unit test, e.g. the
// theme package must never grow a dependency on the router:
//
//	testutil.CheckNoImports(t,
//	    "github.com/jarvisfriends/tui-base/theme",
//	    "github.com/jarvisfriends/tui-base/router")
func CheckNoImports(t *testing.T, pkgPattern string, forbidden ...string) {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports}
	pkgs, err := packages.Load(cfg, pkgPattern)
	if err != nil {
		t.Fatalf("loading %s: %v", pkgPattern, err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("pattern %s matched no packages", pkgPattern)
	}
	for _, p := range pkgs {
		for imp := range p.Imports {
			for _, f := range forbidden {
				if imp == f || strings.HasPrefix(imp, f+"/") {
					t.Errorf(
						"%s imports %s — forbidden by architecture rule (%s must stay independent of %s)",
						p.PkgPath,
						imp,
						p.PkgPath,
						f,
					)
				}
			}
		}
	}
}

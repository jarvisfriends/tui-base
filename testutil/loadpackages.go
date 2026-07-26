// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package testutil

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// loadConformancePackages loads the patterns for a conformance check and
// fails the test — instead of letting the check pass vacuously — when the
// patterns match no loadable packages. A conformance test whose pattern no
// longer matches the module (a renamed module path, a typo, a copy-pasted
// pattern from another repo) otherwise checks nothing and stays green
// forever. Packages that match but fail to load are also test failures:
// an unloadable package is unchecked code, not a skippable detail.
func loadConformancePackages(
	t *testing.T,
	cfg *packages.Config,
	patterns []string,
) []*packages.Package {
	t.Helper()

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatalf("Failed to load packages for %q: %v", patterns, err)
	}

	clean, loadErrors := partitionLoadedPackages(pkgs)
	if len(clean) == 0 {
		t.Fatal(noPackagesMessage(patterns, loadErrors))
	}
	for _, le := range loadErrors {
		t.Errorf(
			"conformance check cannot load %s — this code is UNCHECKED until the pattern or build is fixed",
			le,
		)
	}
	return clean
}

// partitionLoadedPackages splits a packages.Load result into cleanly loaded
// packages and human-readable load-error descriptions. Split out from
// loadConformancePackages so the vacuous-pattern detection can be unit
// tested without a *testing.T whose Fatal would end the calling test.
func partitionLoadedPackages(pkgs []*packages.Package) (clean []*packages.Package, loadErrors []string) {
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", pkg.PkgPath, e))
			}
			continue
		}
		clean = append(clean, pkg)
	}
	return clean, loadErrors
}

// noPackagesMessage explains a vacuous conformance pattern and how to fix it.
func noPackagesMessage(patterns, loadErrors []string) string {
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"conformance check matched no loadable packages for %q — NOTHING was checked and the test would have passed vacuously.\n",
		patterns,
	)
	b.WriteString("How to fix:\n")
	b.WriteString("  - Use this module's own path from go.mod: module example.com/mod → pattern \"example.com/mod/...\"\n")
	b.WriteString("  - Or use the directory form \"./...\" (relative to the test file's package), which follows module renames automatically\n")
	if len(loadErrors) > 0 {
		b.WriteString("Load errors:\n")
		for _, le := range loadErrors {
			b.WriteString("  " + le + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

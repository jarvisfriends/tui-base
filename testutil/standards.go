// Package testutil holds tui-base's house-rule test helpers: architecture
// layering (CheckNoImports) and descriptive type naming
// (CheckDescriptiveStructNames). The render/layout checks moved to
// github.com/jarvisfriends/snap/rendercheck (tui-base ROADMAP SP-14).
package testutil

import (
	"go/ast"
	"testing"

	"golang.org/x/tools/go/packages"
)

// CheckDescriptiveStructNames verifies that structs are not generically named "Model" or "model".
func CheckDescriptiveStructNames(t *testing.T, patterns ...string) {
	t.Helper()

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Tests: false,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				t.Logf("skipping package %s due to load error: %v", pkg.PkgPath, e)
			}
			continue
		}

		for _, file := range pkg.Syntax {
			filename := pkg.Fset.Position(file.Pos()).Filename
			ast.Inspect(file, func(n ast.Node) bool {
				if x, ok := n.(*ast.TypeSpec); ok {
					if x.Name.Name == "Model" || x.Name.Name == "model" {
						if _, isStruct := x.Type.(*ast.StructType); isStruct {
							t.Errorf(
								"%s:%d: Struct must be given a more descriptive name than '%s'",
								filename,
								pkg.Fset.Position(x.Pos()).Line,
								x.Name.Name,
							)
						}
					}
				}
				return true
			})
		}
	}
}

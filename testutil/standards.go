package testutil

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// CheckCodeStandards runs all AST-based code standard checks on the given module patterns.
// It verifies key mapping conventions (no inline bindings, no vim fallbacks) and
// layout calculation safety (no len() on strings in UI code).
func CheckCodeStandards(t *testing.T, patterns ...string) {
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
			continue
		}

		uiPkg := isUIPackage(pkg.PkgPath)

		for _, file := range pkg.Syntax {
			filename := pkg.Fset.Position(file.Pos()).Filename
			ast.Inspect(file, func(n ast.Node) bool {
				checkKeyMappings(t, pkg.Fset, filename, n)
				if uiPkg {
					checkLayoutCalculations(t, pkg, filename, n)
				}
				return true
			})
		}
	}
}

func isUIPackage(pkgPath string) bool {
	uiDirs := []string{"/pages", "/cui", "/theme", "/navigation", "/overlay", "/status", "/table", "/router", "/creator"}
	for _, d := range uiDirs {
		if strings.Contains(pkgPath, d) {
			return true
		}
	}
	if strings.Contains(pkgPath, "tui-base/page") || strings.Contains(pkgPath, "tui-base/notifications") {
		return true
	}
	return false
}

func checkKeyMappings(t *testing.T, fset *token.FileSet, path string, n ast.Node) {
	switch x := n.(type) {
	case *ast.FuncDecl:
		name := x.Name.Name
		if name == "ShortHelp" || name == "FullHelp" {
			ast.Inspect(x.Body, func(bodyNode ast.Node) bool {
				if call, ok := bodyNode.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if pkg, ok := sel.X.(*ast.Ident); ok {
							if pkg.Name == "key" && sel.Sel.Name == "NewBinding" {
								t.Errorf("%s:%d: %s() must not call key.NewBinding inline",
									path, fset.Position(call.Pos()).Line, name)
							}
						}
					}
				}
				return true
			})
		}

	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "key" && sel.Sel.Name == "WithKeys" {
				hasDirection := false
				hasVim := ""
				for _, arg := range x.Args {
					if basicLit, ok := arg.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
						val := strings.Trim(basicLit.Value, "\"")
						if val == "up" || val == "down" || val == "left" || val == "right" {
							hasDirection = true
						}
						if val == "j" || val == "k" || val == "h" || val == "l" {
							hasVim = val
						}
					}
				}
				if hasDirection && hasVim != "" {
					t.Errorf("%s:%d: key.WithKeys contains prohibited vim fallback '%s' alongside directional key",
						path, fset.Position(x.Pos()).Line, hasVim)
				}
			}
		}

	case *ast.StructType:
		for _, field := range x.Fields.List {
			for _, name := range field.Names {
				if name.Name == "showHelpForm" || name.Name == "helpFormText" {
					t.Errorf("%s:%d: Struct contains prohibited legacy help field '%s'",
						path, fset.Position(name.Pos()).Line, name.Name)
				}
			}
		}
	}
}

func checkLayoutCalculations(t *testing.T, pkg *packages.Package, path string, n ast.Node) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "strings" && sel.Sel.Name == "Count" {
			if len(call.Args) == 2 {
				if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Value == `"\n"` {
					pos := pkg.Fset.Position(call.Pos())
					t.Errorf("%s:%d: Use lipgloss.Height() instead of strings.Count(x, \"\\n\") for visual height", path, pos.Line)
				}
			}
		}
	}

	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "len" && len(call.Args) == 1 {
		arg := call.Args[0]
		if typeInfo := pkg.TypesInfo.Types[arg]; typeInfo.Type != nil {
			typeString := typeInfo.Type.Underlying().String()
			if typeString == "string" || typeString == "untyped string" {
				if binaryExpr, isBinary := n.(*ast.BinaryExpr); isBinary {
					if lit, isLit := binaryExpr.Y.(*ast.BasicLit); isLit {
						if lit.Value == "0" || lit.Value == "1" {
							return
						}
					}
				}
				pos := pkg.Fset.Position(call.Pos())
				t.Errorf("%s:%d: Use lipgloss.Width() or ansi.StringWidth() instead of len() for string visual width", path, pos.Line)
			}
		}
	}
}

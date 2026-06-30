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
			for _, e := range pkg.Errors {
				t.Logf("skipping package %s due to load error: %v", pkg.PkgPath, e)
			}
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
	uiDirs := []string{"/pages", "/cui", "/tui/", "/theme", "/navigation", "/overlay", "/status", "/table", "/router", "/creator"}
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
	t.Helper()
	switch x := n.(type) {
	case *ast.FuncDecl:
		name := x.Name.Name
		if name == "ShortHelp" || name == "FullHelp" || name == "Update" {
			checkFuncBodyForInlineBindings(t, fset, path, name, x.Body)
		}
	case *ast.CallExpr:
		checkWithKeysVimFallback(t, fset, path, x)
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

func checkFuncBodyForInlineBindings(t *testing.T, fset *token.FileSet, path, funcName string, body *ast.BlockStmt) {
	t.Helper()
	ast.Inspect(body, func(bodyNode ast.Node) bool {
		call, ok := bodyNode.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkg.Name == "key" && sel.Sel.Name == "NewBinding" {
			t.Errorf("%s:%d: %s() must not call key.NewBinding inline",
				path, fset.Position(call.Pos()).Line, funcName)
		}
		if pkg.Name == "shared" && sel.Sel.Name == "HelpBinding" {
			t.Errorf("%s:%d: %s() must not call shared.HelpBinding inline",
				path, fset.Position(call.Pos()).Line, funcName)
		}
		return true
	})
}

func checkWithKeysVimFallback(t *testing.T, fset *token.FileSet, path string, x *ast.CallExpr) {
	t.Helper()
	sel, ok := x.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "key" || sel.Sel.Name != "WithKeys" {
		return
	}
	hasDirection := false
	hasVim := ""
	for _, arg := range x.Args {
		basicLit, ok := arg.(*ast.BasicLit)
		if !ok || basicLit.Kind != token.STRING {
			continue
		}
		val := strings.Trim(basicLit.Value, "\"")
		if val == "up" || val == "down" || val == "left" || val == "right" {
			hasDirection = true
		}
		if val == "j" || val == "k" || val == "h" || val == "l" {
			hasVim = val
		}
	}
	if hasDirection && hasVim != "" {
		t.Errorf("%s:%d: key.WithKeys contains prohibited vim fallback '%s' alongside directional key",
			path, fset.Position(x.Pos()).Line, hasVim)
	}
}

func checkLayoutCalculations(t *testing.T, pkg *packages.Package, path string, n ast.Node) {
	t.Helper()
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}
	checkStringsCountNewline(t, pkg, path, call)
	checkLenOnString(t, pkg, path, n, call)
}

func checkStringsCountNewline(t *testing.T, pkg *packages.Package, path string, call *ast.CallExpr) {
	t.Helper()
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != "strings" || sel.Sel.Name != "Count" {
		return
	}
	if len(call.Args) == 2 {
		if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Value == `"\n"` {
			pos := pkg.Fset.Position(call.Pos())
			t.Errorf("%s:%d: Use lipgloss.Height() instead of strings.Count(x, \"\\n\") for visual height", path, pos.Line)
		}
	}
}

func checkLenOnString(t *testing.T, pkg *packages.Package, path string, n ast.Node, call *ast.CallExpr) {
	t.Helper()
	id, ok := call.Fun.(*ast.Ident)
	if !ok || id.Name != "len" || len(call.Args) != 1 {
		return
	}
	typeInfo := pkg.TypesInfo.Types[call.Args[0]]
	if typeInfo.Type == nil {
		return
	}
	typeString := typeInfo.Type.Underlying().String()
	if typeString != "string" && typeString != "untyped string" {
		return
	}
	if binaryExpr, isBinary := n.(*ast.BinaryExpr); isBinary {
		if lit, isLit := binaryExpr.Y.(*ast.BasicLit); isLit && (lit.Value == "0" || lit.Value == "1") {
			return
		}
	}
	pos := pkg.Fset.Position(call.Pos())
	t.Errorf("%s:%d: Use lipgloss.Width() or ansi.StringWidth() instead of len() for string visual width", path, pos.Line)
}

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
							t.Errorf("%s:%d: Struct must be given a more descriptive name than '%s'",
								filename, pkg.Fset.Position(x.Pos()).Line, x.Name.Name)
						}
					}
				}
				return true
			})
		}
	}
}

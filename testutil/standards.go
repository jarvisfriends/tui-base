package testutil

import (
	"go/ast"
	"go/token"
	"slices"
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
			packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
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

		uiPkg := isUIPackage(pkg)

		for _, file := range pkg.Syntax {
			filename := pkg.Fset.Position(file.Pos()).Filename
			// ancestors holds the chain of enclosing AST nodes (outermost first,
			// immediate parent last) for the node currently being visited. The
			// layout checks use it to inspect how a len() result is consumed.
			var ancestors []ast.Node
			ast.Inspect(file, func(n ast.Node) bool {
				if n == nil {
					ancestors = ancestors[:len(ancestors)-1]
					return true
				}
				checkKeyMappings(t, pkg.Fset, filename, n)
				if uiPkg {
					checkLayoutCalculations(t, pkg, filename, ancestors, n)
				}
				ancestors = append(ancestors, n)
				return true
			})
		}
	}
}

// isUIPackage reports whether a package participates in terminal rendering, by
// checking whether it imports one of the Charm rendering libraries. This is a
// self-maintaining replacement for a hardcoded list of directory names: any
// package that lays out or measures text for the screen necessarily imports
// lipgloss or bubbletea, and packages that do neither are exempt from the
// layout-width checks.
func isUIPackage(pkg *packages.Package) bool {
	for path := range pkg.Imports {
		if strings.Contains(path, "charm.land/lipgloss") || strings.Contains(path, "charm.land/bubbletea") {
			return true
		}
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

func checkLayoutCalculations(t *testing.T, pkg *packages.Package, path string, ancestors []ast.Node, n ast.Node) {
	t.Helper()
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}
	checkStringsCountNewline(t, pkg, path, call)
	checkLenOnString(t, pkg, path, ancestors, call)
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

func checkLenOnString(t *testing.T, pkg *packages.Package, path string, ancestors []ast.Node, call *ast.CallExpr) {
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
	// len() on a string returns a byte count, which only differs from the
	// terminal cell width for multi-byte / wide / zero-width runes. That
	// distinction is harmless when the result is used for byte-level work
	// (indexing, slicing, allocation, emptiness/loop bounds) but wrong when it
	// is used as a display dimension that feeds rendered output. Only flag the
	// latter so legitimate ASCII/byte uses don't need to be rewritten.
	if lenUsedForByteSemantics(ancestors, call) {
		return
	}
	pos := pkg.Fset.Position(call.Pos())
	t.Errorf("%s:%d: Use lipgloss.Width() or ansi.StringWidth() instead of len() for string visual width", path, pos.Line)
}

// lenUsedForByteSemantics reports whether the result of a len() call is consumed
// in a way that is content-independent (and therefore safe), as opposed to being
// used as a visual width. It walks outward from the call through its enclosing
// expressions and decides based on the first meaningful consumer:
//
//   - index / slice bound (s[len(x)-1], s[:len(x)])      -> safe (byte offset)
//   - argument to make / cap                             -> safe (allocation)
//   - comparison against an integer literal (len(s) > 0) -> safe (emptiness/size)
//   - the condition of an enclosing for loop             -> safe (iteration bound)
//   - anything else (width arithmetic, comparison to a   -> flagged
//     non-literal dimension, padding, struct fields, …)
//
// Arithmetic (len(s)-1, len(a)+len(b)) is transparent: the classifier keeps
// climbing and decides on whatever ultimately consumes the computed value.
func lenUsedForByteSemantics(ancestors []ast.Node, call *ast.CallExpr) bool {
	cur := ast.Node(call)
	for _, a := range slices.Backward(ancestors) {
		switch p := a.(type) {
		case *ast.ParenExpr:
			cur = p
		case *ast.BinaryExpr:
			if isComparisonOp(p.Op) {
				other := p.X
				if sameNode(cur, p.X) {
					other = p.Y
				}
				if isIntLiteral(other) {
					return true
				}
				// for i := 0; i < len(s); i++ { … s[i] … } — iterating bytes.
				return inForCond(ancestors, call)
			}
			// Arithmetic operand: the len result is being combined into a larger
			// expression; defer the decision to its eventual consumer.
			cur = p
		case *ast.IndexExpr:
			return sameNode(cur, p.Index)
		case *ast.SliceExpr:
			return sameNode(cur, p.Low) || sameNode(cur, p.High) || sameNode(cur, p.Max)
		case *ast.CallExpr:
			if fn, ok := p.Fun.(*ast.Ident); ok && (fn.Name == "make" || fn.Name == "cap") {
				return true
			}
			return false
		default:
			return false
		}
	}
	return false
}

func isComparisonOp(op token.Token) bool {
	switch op { //nolint:exhaustive // only comparison operators are relevant; default handles every other token
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	default:
		return false
	}
}

func isIntLiteral(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.INT
	case *ast.ParenExpr:
		return isIntLiteral(e.X)
	case *ast.UnaryExpr:
		// -1, +1
		return (e.Op == token.SUB || e.Op == token.ADD) && isIntLiteral(e.X)
	default:
		return false
	}
}

// inForCond reports whether call sits within the condition expression of an
// enclosing for loop, where a byte-count bound is the idiomatic choice.
func inForCond(ancestors []ast.Node, call ast.Node) bool {
	for _, a := range ancestors {
		if f, ok := a.(*ast.ForStmt); ok && f.Cond != nil && nodeContains(f.Cond, call) {
			return true
		}
	}
	return false
}

func nodeContains(root, target ast.Node) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if found {
			return false
		}
		if n == target {
			found = true
			return false
		}
		return true
	})
	return found
}

// sameNode compares an ast.Node against an ast.Expr by identity. The conversion
// is needed because == between the two interface types is a compile error even
// though they hold the same concrete pointer.
func sameNode(n ast.Node, e ast.Expr) bool {
	return e != nil && n == ast.Node(e)
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

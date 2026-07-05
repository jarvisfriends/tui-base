package testutil

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
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
		if uiPkg {
			checkFrameSizeGuesses(t, pkg)
		}

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
		if strings.Contains(path, "charm.land/lipgloss") ||
			strings.Contains(path, "charm.land/bubbletea") {
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

func checkFuncBodyForInlineBindings(
	t *testing.T,
	fset *token.FileSet,
	path, funcName string,
	body *ast.BlockStmt,
) {
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
		t.Errorf(
			"%s:%d: key.WithKeys contains prohibited vim fallback '%s' alongside directional key",
			path,
			fset.Position(x.Pos()).Line,
			hasVim,
		)
	}
}

func checkLayoutCalculations(
	t *testing.T,
	pkg *packages.Package,
	path string,
	ancestors []ast.Node,
	n ast.Node,
) {
	t.Helper()
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}
	checkStringsCountNewline(t, pkg, path, call)
	checkLenOnString(t, pkg, path, ancestors, call)
}

// checkFrameSizeGuesses flags hardcoded integer arithmetic against a
// width/height-shaped expression (m.width-2, boxW-4, innerW+1, …) in any
// method of a type that elsewhere draws a lipgloss border (an X.Border(...)
// call somewhere in the type's method set, possibly in a different method
// than the arithmetic). That combination is exactly the bug class found in
// tui-base's own table/status/navigation packages: a component computes its
// content size by subtracting a hand-counted border+padding literal instead
// of calling the border style's own GetHorizontalFrameSize() /
// GetVerticalFrameSize(), which silently mis-sizes content (or breaks hit
// tests) the moment the actual border/padding configuration changes.
//
// The check is intentionally scoped to types that already draw a border:
// unrelated line/row-budget arithmetic (reserving N lines for a title or
// footer with no border involved) is a different, unrelated pattern and is
// not flagged here.
func checkFrameSizeGuesses(t *testing.T, pkg *packages.Package) {
	t.Helper()
	for _, msg := range findFrameSizeGuesses(pkg) {
		t.Error(msg)
	}
}

// findFrameSizeGuesses is the pure decision logic behind checkFrameSizeGuesses,
// separated out so it can be unit tested directly (assert on the returned
// messages) without needing a *testing.T whose failures would otherwise
// propagate into the calling test.
func findFrameSizeGuesses(pkg *packages.Package) []string {
	borderTypes := borderReceiverTypes(pkg)
	if len(borderTypes) == 0 {
		return nil
	}
	var msgs []string
	for _, file := range pkg.Syntax {
		filename := pkg.Fset.Position(file.Pos()).Filename
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			if !borderTypes[receiverTypeName(fn.Recv.List[0].Type)] {
				continue
			}
			msgs = append(msgs, findSizeGuessesInBody(pkg, filename, fn.Body)...)
		}
	}
	return msgs
}

// borderReceiverTypes returns the set of method receiver type names in pkg
// that have at least one method calling a lipgloss-style Border(...) method
// (any X.Border(...) call — the receiver of Border itself doesn't matter,
// only that the enclosing method draws one).
func borderReceiverTypes(pkg *packages.Package) map[string]bool {
	found := map[string]bool{}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			if containsBorderCall(fn.Body) {
				if rt := receiverTypeName(fn.Recv.List[0].Type); rt != "" {
					found[rt] = true
				}
			}
		}
	}
	return found
}

// receiverTypeName extracts the bare type name from a method receiver's type
// expression (T or *T), or "" if it isn't a simple named type.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

// containsBorderCall reports whether body calls any method literally named
// Border (X.Border(...)), regardless of X's type.
func containsBorderCall(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Border" {
			found = true
			return false
		}
		return true
	})
	return found
}

// sizeGuessRE matches identifier names that read as a width/height value:
// an exact "w"/"h", a name ending in Width/Height, or a lowercase-to-W/H
// camelCase boundary (boxW, innerW, vpH, colW, …). The case-insensitive
// flag is scoped to the first two alternatives only — the camelCase
// alternative must stay case-sensitive (only a literal uppercase W/H),
// otherwise it also matches ordinary words ending in a lowercase w
// ("now", "raw", "new", …) once (?i) is left unscoped.
var sizeGuessRE = regexp.MustCompile(`(?i:^[wh]$)|(?i:(width|height)$)|[a-z][WH]$`)

// looksLikeSizeExpr reports whether expr is a bare identifier or selector
// whose name matches sizeGuessRE. Call expressions (including
// GetHorizontalFrameSize()/GetVerticalFrameSize()) never match, so code that
// already derives its frame size from the style is naturally exempt.
func looksLikeSizeExpr(expr ast.Expr) bool {
	var name string
	switch e := expr.(type) {
	case *ast.Ident:
		name = e.Name
	case *ast.SelectorExpr:
		name = e.Sel.Name
	default:
		return false
	}
	return sizeGuessRE.MatchString(name)
}

func findSizeGuessesInBody(pkg *packages.Package, filename string, body *ast.BlockStmt) []string {
	var msgs []string
	var ancestors []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			ancestors = ancestors[:len(ancestors)-1]
			return true
		}
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || (bin.Op != token.SUB && bin.Op != token.ADD) {
			ancestors = append(ancestors, n)
			return true
		}
		other, hasLit := nonLiteralOperand(bin)
		if hasLit && looksLikeSizeExpr(other) && !inForCond(ancestors, bin) {
			pos := pkg.Fset.Position(bin.Pos())
			msgs = append(
				msgs,
				fmt.Sprintf(
					"%s:%d: %s looks like a hardcoded border/frame-size guess in a type that draws a border — "+
						"call style.GetHorizontalFrameSize()/GetVerticalFrameSize() instead of subtracting a literal",
					filename,
					pos.Line,
					types.ExprString(bin),
				),
			)
		}
		ancestors = append(ancestors, n)
		return true
	})
	return msgs
}

// nonLiteralOperand reports the non-literal side of a binary expression with
// exactly one integer-literal operand, e.g. "boxW - 4" -> (boxW, true).
func nonLiteralOperand(bin *ast.BinaryExpr) (other ast.Expr, ok bool) {
	switch {
	case isIntLiteral(bin.Y) && !isIntLiteral(bin.X):
		return bin.X, true
	case isIntLiteral(bin.X) && !isIntLiteral(bin.Y):
		return bin.Y, true
	default:
		return nil, false
	}
}

func checkStringsCountNewline(
	t *testing.T,
	pkg *packages.Package,
	path string,
	call *ast.CallExpr,
) {
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
			t.Errorf(
				"%s:%d: Use lipgloss.Height() instead of strings.Count(x, \"\\n\") for visual height",
				path,
				pos.Line,
			)
		}
	}
}

// builtinLen is the identifier name of Go's builtin len(), used to spot
// len() calls by AST inspection rather than by type-checked identity.
const builtinLen = "len"

func checkLenOnString(
	t *testing.T,
	pkg *packages.Package,
	path string,
	ancestors []ast.Node,
	call *ast.CallExpr,
) {
	t.Helper()
	id, ok := call.Fun.(*ast.Ident)
	if !ok || id.Name != builtinLen || len(call.Args) != 1 {
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
	t.Errorf(
		"%s:%d: Use lipgloss.Width() or ansi.StringWidth() instead of len() for string visual width",
		path,
		pos.Line,
	)
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
				// len(a) <op> len(b): comparing two strings' byte lengths
				// against each other (e.g. a prefix/subset relationship
				// checked before a same-length byte slice) — not a display
				// dimension, since neither side is a fixed layout width.
				if isLenCall(other) {
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
			// x.Grow(len(s)) pre-allocates a strings.Builder/bytes.Buffer by
			// byte count — an allocation hint, not a display dimension.
			if sel, ok := p.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Grow" {
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

// isLenCall reports whether expr is (a possibly parenthesized or arithmetic
// combination of) a call to the builtin len(). Used to recognize
// "len(a) <op> len(b)" comparisons as byte-length comparisons — e.g. a
// prefix/subset length check before slicing — rather than a display-width
// check against a fixed layout dimension.
func isLenCall(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.CallExpr:
		id, ok := e.Fun.(*ast.Ident)
		return ok && id.Name == builtinLen && len(e.Args) == 1
	case *ast.ParenExpr:
		return isLenCall(e.X)
	case *ast.BinaryExpr:
		return isLenCall(e.X) || isLenCall(e.Y)
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

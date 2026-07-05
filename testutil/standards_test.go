package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/packages"
)

// classifyLenCalls parses a function body snippet and returns the byte-semantics
// classification for every len() call it contains, in source order. It mirrors
// the ancestor tracking that CheckCodeStandards performs so the classifier is
// exercised exactly as it is in production.
func classifyLenCalls(t *testing.T, body string) []bool {
	t.Helper()
	src := "package p\nimport \"strings\"\nvar _ = strings.Repeat\nfunc f() {\n" + body + "\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "snippet.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse snippet %q: %v", body, err)
	}

	var results []bool
	var ancestors []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			ancestors = ancestors[:len(ancestors)-1]
			return true
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == builtinLen {
				anc := append([]ast.Node(nil), ancestors...)
				results = append(results, lenUsedForByteSemantics(anc, call))
			}
		}
		ancestors = append(ancestors, n)
		return true
	})
	return results
}

func TestLenUsedForByteSemantics(t *testing.T) {
	// Each snippet declares the names it uses; go/parser does not enforce
	// unused-variable rules, so type checking is unnecessary here.
	prelude := "var s, suffix string\nvar width, maxWidth, i int\n_ = i\n"

	safe := map[string]string{
		"index last byte":        "_ = s[len(s)-1]",
		"slice high bound":       "_ = s[:len(s)]",
		"slice low bound":        "_ = s[len(s):]",
		"slice high arithmetic":  "_ = s[:len(s)-1]",
		"slice high two lens":    "_ = s[:len(s)-len(suffix)]",
		"make allocation":        "b := make([]byte, len(s)); _ = b",
		"emptiness eq zero":      "if len(s) == 0 {}",
		"emptiness gt zero":      "if len(s) > 0 {}",
		"size ge literal":        "if len(s) >= 1 {}",
		"compare to int literal": "if len(s) > 80 {}",
		"for loop bound":         "for i = 0; i < len(s); i++ {}",
		"compare two lens":       "if len(s) < len(suffix) {}",
		"builder grow":           "var b strings.Builder\nb.Grow(len(s))",
	}
	for name, body := range safe {
		t.Run("safe/"+name, func(t *testing.T) {
			got := classifyLenCalls(t, prelude+body)
			if len(got) == 0 {
				t.Fatalf("no len() calls found in %q", body)
			}
			for j, ok := range got {
				if !ok {
					t.Errorf(
						"len call #%d in %q classified as display width; want byte-safe",
						j,
						body,
					)
				}
			}
		})
	}

	flagged := map[string]string{
		"width minus len":         "pad := width - len(s); _ = pad",
		"width plus len":          "_ = width + len(s)",
		"compare to dimension":    "if len(s) > maxWidth {}",
		"padding via repeat":      "x := strings.Repeat(\" \", len(s)); _ = x",
		"bare assignment":         "n := len(s); _ = n",
		"len in loop body":        "for i = 0; i < width; i++ { if len(s) > maxWidth {} }",
		"struct field assignment": "type box struct{ W int }; var b box; b.W = len(s); _ = b",
	}
	for name, body := range flagged {
		t.Run("flagged/"+name, func(t *testing.T) {
			got := classifyLenCalls(t, prelude+body)
			if len(got) == 0 {
				t.Fatalf("no len() calls found in %q", body)
			}
			for j, ok := range got {
				if ok {
					t.Errorf(
						"len call #%d in %q classified as byte-safe; want flagged as display width",
						j,
						body,
					)
				}
			}
		})
	}
}

// ─── checkFrameSizeGuesses ──────────────────────────────────────────────────

func firstFuncBody(t *testing.T, file *ast.File) *ast.BlockStmt {
	t.Helper()
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("first decl is %T, want *ast.FuncDecl", file.Decls[0])
	}
	return fn.Body
}

func TestContainsBorderCall(t *testing.T) {
	yes := parseSnippet(t, "package p\nfunc f() { x.Border(y) }\n")
	if !containsBorderCall(firstFuncBody(t, yes)) {
		t.Error("expected X.Border(...) call to be detected")
	}

	no := parseSnippet(t, "package p\nfunc f() { x.Padding(1) }\n")
	if containsBorderCall(firstFuncBody(t, no)) {
		t.Error("did not expect a non-Border call to be detected")
	}
}

func TestLooksLikeSizeExpr(t *testing.T) {
	idents := map[string]bool{
		"w": true, "h": true, "W": true,
		"width": true, "Height": true,
		"boxWidth": true, "boxW": true, "innerW": true, "vpH": true, "colWidth": true,
		"raw": false, "now": false, "count": false, "retries": false, "new": false,
	}
	for name, want := range idents {
		t.Run("ident/"+name, func(t *testing.T) {
			expr, err := parser.ParseExpr(name)
			if err != nil {
				t.Fatalf("parse expr %q: %v", name, err)
			}
			if got := looksLikeSizeExpr(expr); got != want {
				t.Errorf("looksLikeSizeExpr(%q) = %v, want %v", name, got, want)
			}
		})
	}

	t.Run("selector", func(t *testing.T) {
		sel, err := parser.ParseExpr("m.width")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if !looksLikeSizeExpr(sel) {
			t.Error("expected m.width to match as a size expression")
		}
	})

	t.Run("call expressions never match", func(t *testing.T) {
		// GetHorizontalFrameSize()-derived code must stay exempt: a call
		// result can't be a "bare literal guess" regardless of its name.
		call, err := parser.ParseExpr("style.GetHorizontalFrameSize()")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if looksLikeSizeExpr(call) {
			t.Error("call expressions must never match looksLikeSizeExpr")
		}
	})
}

func TestNonLiteralOperand(t *testing.T) {
	cases := map[string]bool{
		"boxW - 4":      true, // classic guess: flagged
		"4 - boxW":      true,
		"boxW - otherW": false, // len(a) vs len(b)-style: no literal operand
		"boxW - width":  false, // both sides are size-shaped identifiers
	}
	for exprSrc, wantOK := range cases {
		t.Run(exprSrc, func(t *testing.T) {
			expr, err := parser.ParseExpr(exprSrc)
			if err != nil {
				t.Fatalf("parse %q: %v", exprSrc, err)
			}
			bin, ok := expr.(*ast.BinaryExpr)
			if !ok {
				t.Fatalf("parsed %q as %T, want *ast.BinaryExpr", exprSrc, expr)
			}
			_, ok = nonLiteralOperand(bin)
			if ok != wantOK {
				t.Errorf("nonLiteralOperand(%q) ok=%v, want %v", exprSrc, ok, wantOK)
			}
		})
	}
}

// parseSnippet parses a complete Go source file (with "package p" and any
// decls) for tests that need real *ast.File / *ast.FuncDecl nodes rather
// than a bare expression.
func parseSnippet(t *testing.T, src string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "snippet.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse snippet: %v\n%s", err, src)
	}
	return file
}

// findFrameSizeGuessesForSrc parses a standalone source file and runs the
// pure findFrameSizeGuesses logic against it, wrapped in the minimal
// *packages.Package shape the function needs (it only reads Fset and Syntax).
func findFrameSizeGuessesForSrc(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "box.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return findFrameSizeGuesses(&packages.Package{Fset: fset, Syntax: []*ast.File{file}})
}

func TestFindFrameSizeGuesses(t *testing.T) {
	t.Run("flags a guess co-located with a Border call", func(t *testing.T) {
		msgs := findFrameSizeGuessesForSrc(t, `package p

type Box struct{ width int }

func (b *Box) Render() string {
	return b.style().Border(RoundedBorder{}).Render("x")
}

func (b *Box) innerWidth() int {
	return b.width - 2
}
`)
		if len(msgs) == 0 {
			t.Error(
				"expected b.width - 2 to be flagged: Box draws a border elsewhere in its method set",
			)
		}
	})

	t.Run("does not flag the same arithmetic without a border", func(t *testing.T) {
		msgs := findFrameSizeGuessesForSrc(t, `package p

type Box struct{ width int }

func (b *Box) innerWidth() int {
	return b.width - 2
}
`)
		if len(msgs) != 0 {
			t.Errorf(
				"did not expect b.width - 2 to be flagged: Box never draws a border; got %v",
				msgs,
			)
		}
	})

	t.Run("does not flag GetFrameSize()-derived code", func(t *testing.T) {
		msgs := findFrameSizeGuessesForSrc(t, `package p

type Box struct{ width int }

func (b *Box) Render() string {
	return b.style().Border(RoundedBorder{}).Render("x")
}

func (b *Box) innerWidth() int {
	return b.width - b.style().GetHorizontalFrameSize()
}
`)
		if len(msgs) != 0 {
			t.Errorf(
				"did not expect GetHorizontalFrameSize()-derived arithmetic to be flagged; got %v",
				msgs,
			)
		}
	})

	t.Run("does not flag a named constant", func(t *testing.T) {
		msgs := findFrameSizeGuessesForSrc(t, `package p

const prefixWidth = 2

type Box struct{ width int }

func (b *Box) Render() string {
	return b.style().Border(RoundedBorder{}).Render("x")
}

func (b *Box) innerWidth() int {
	return b.width - prefixWidth
}
`)
		if len(msgs) != 0 {
			t.Errorf(
				"did not expect arithmetic against a named constant to be flagged; got %v",
				msgs,
			)
		}
	})

	t.Run("does not flag a for-loop bound", func(t *testing.T) {
		msgs := findFrameSizeGuessesForSrc(t, `package p

type Box struct{ width int }

func (b *Box) Render() string {
	return b.style().Border(RoundedBorder{}).Render("x")
}

func (b *Box) fill() string {
	out := ""
	for i := 0; i < b.width-1; i++ {
		out += "x"
	}
	return out
}
`)
		if len(msgs) != 0 {
			t.Errorf("did not expect a for-loop bound to be flagged; got %v", msgs)
		}
	})
}

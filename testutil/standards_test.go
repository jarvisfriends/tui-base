package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
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
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "len" {
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
	}
	for name, body := range safe {
		t.Run("safe/"+name, func(t *testing.T) {
			got := classifyLenCalls(t, prelude+body)
			if len(got) == 0 {
				t.Fatalf("no len() calls found in %q", body)
			}
			for j, ok := range got {
				if !ok {
					t.Errorf("len call #%d in %q classified as display width; want byte-safe", j, body)
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
					t.Errorf("len call #%d in %q classified as byte-safe; want flagged as display width", j, body)
				}
			}
		})
	}
}

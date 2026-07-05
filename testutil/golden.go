package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// Golden compares got against testdata/golden/<name>.golden in the calling
// package's directory. Set UPDATE_GOLDEN=1 to (re)write the files instead:
//
//	UPDATE_GOLDEN=1 go test ./...   # bash
//	$env:UPDATE_GOLDEN="1"; go test ./...   # PowerShell
//
// Golden files capture the exact rendered output — ANSI styling included —
// so unintended changes to layout, alignment, borders, or theme wiring fail
// loudly with a diffable artifact (TS-1). Only render deterministic content
// into them (no timestamps, PIDs, or terminal-size-dependent values).
func Golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".golden")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
		return
	}

	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("reading golden %s (run with UPDATE_GOLDEN=1 to create it): %v", path, err)
	}
	if string(want) != got {
		t.Errorf(
			"rendered output differs from %s\n--- want (stripped):\n%s\n--- got (stripped):\n%s\n"+
				"(set UPDATE_GOLDEN=1 to accept the new output)",
			path, StripANSI(string(want)), StripANSI(got),
		)
	}
}

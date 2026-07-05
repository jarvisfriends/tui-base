package common

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomicCreatesFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "out.json")
	want := []byte(`{"a":1}`)
	if err := WriteFileAtomic(path, want, 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content = %q; want %q", got, want)
	}
}

func TestWriteFileAtomicOverwritesExisting(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("content = %q; want %q", got, "new")
	}
}

func TestWriteFileAtomicLeavesNoTempFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := WriteFileAtomic(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Fatalf("dir entries = %d; want 1", len(entries))
	}
}

func TestWriteFileAtomicMissingDirFails(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "no-such-dir", "out.json")
	if err := WriteFileAtomic(path, []byte("data"), 0o600); err == nil {
		t.Fatal("expected error for missing parent directory, got nil")
	}
}

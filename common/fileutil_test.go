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

// TestWriteFileAtomicRenameFails covers the os.Rename error branch: when the
// destination path is an existing directory the final rename cannot succeed,
// and the temp file must be cleaned up rather than left behind.
func TestWriteFileAtomicRenameFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Target a path that is itself a directory; renaming a file onto it fails.
	target := filepath.Join(dir, "iam-a-dir")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := WriteFileAtomic(target, []byte("data"), 0o600); err == nil {
		t.Fatal("expected error renaming onto an existing directory, got nil")
	}
	// The temp file (<base>.tmp-*) must not be left behind in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp file left behind after failed rename: %s", e.Name())
		}
	}
}

// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package logging

import (
	"os"
	"path/filepath"
	"testing"
)

// closeActiveLog closes the global log file so Windows can remove the temp dir.
// It must be registered AFTER t.TempDir() so it runs BEFORE the dir's RemoveAll
// (t.Cleanup is LIFO).
func closeActiveLog() {
	writeMu.Lock()
	if outFile != nil {
		_ = outFile.Close()
		outFile = nil
	}
	writeMu.Unlock()
}

// TestLogRotation verifies that the active log file is rotated to "<file>.1"
// once it exceeds the configured size cap, and that logging continues into a
// fresh active file afterwards.
//
// Not parallel: it mutates package-global logging state, so it must run in the
// sequential phase before any t.Parallel() tests start.
func TestLogRotation(t *testing.T) {
	resetLoggerState(t)
	target := filepath.Join(t.TempDir(), "rot.log")
	t.Cleanup(func() {
		closeActiveLog()
		SetMaxLogBytes(10 << 20)
		_ = SetLevel("INFO")
	})

	if _, err := InitFromSettings("file", target); err != nil {
		t.Fatalf("InitFromSettings: %v", err)
	}
	_ = SetLevel("DEBUG")
	SetMaxLogBytes(200) // tiny cap forces rotation within a few lines

	for i := range 50 {
		Infof("rotation filler line number %d aaaaaaaaaaaaaaaaaaaa", i)
	}

	rotated := target + ".1"
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("expected rotated file %q to exist: %v", rotated, err)
	}

	fi, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat active log: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("active log is empty after rotation; expected continued writes")
	}
}

// TestLogRotationDisabled verifies that a non-positive cap disables rotation.
func TestLogRotationDisabled(t *testing.T) {
	resetLoggerState(t)
	target := filepath.Join(t.TempDir(), "norot.log")
	t.Cleanup(func() {
		closeActiveLog()
		SetMaxLogBytes(10 << 20)
		_ = SetLevel("INFO")
	})

	if _, err := InitFromSettings("file", target); err != nil {
		t.Fatalf("InitFromSettings: %v", err)
	}
	_ = SetLevel("DEBUG")
	SetMaxLogBytes(0) // disabled

	for i := range 50 {
		Infof("no-rotation filler line number %d aaaaaaaaaaaaaaaaaaaa", i)
	}

	if _, err := os.Stat(target + ".1"); err == nil {
		t.Fatal("rotation file exists but rotation was disabled (maxLogBytes=0)")
	}
}

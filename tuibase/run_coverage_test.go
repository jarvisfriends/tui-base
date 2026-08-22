// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package tuibase

import (
	"context"
	"io"
	"os"
	"runtime"
	"testing"
	"time"
)

// TestWindowsTerminalHelpersOffWindows pins the non-Windows behavior of the
// re-exported Windows Terminal helpers: profile install/uninstall report a
// clear error, and EnsureWindowsTerminal is a no-op (it must NOT exit the
// process, or this test binary would vanish).
func TestWindowsTerminalHelpersOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exercises the non-Windows paths")
	}

	if _, err := InstallWindowsTerminalProfile(WindowsTerminalProfile{AppName: "Cover App"}); err == nil {
		t.Error("InstallWindowsTerminalProfile should fail off Windows")
	}
	if err := UninstallWindowsTerminalProfile("Cover App"); err == nil {
		t.Error("UninstallWindowsTerminalProfile should fail off Windows")
	}

	EnsureWindowsTerminal() // must return without relaunching or exiting
}

// TestRunReturnsWhenContextCanceled drives the batteries-included Run path
// end to end with an already-canceled context: the program must build, start,
// and shut down cleanly instead of blocking on a live terminal.
func TestRunReturnsWhenContextCanceled(t *testing.T) {
	// Capture stdout for the duration: the Bubble Tea renderer writes terminal
	// control sequences that would otherwise pollute the test output.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, r)
		close(drained)
	}()
	defer func() {
		os.Stdout = origStdout
		_ = w.Close()
		<-drained
		_ = r.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled: RunContext must return promptly

	done := make(chan error, 1)
	go func() {
		done <- RunContext(ctx, Options{
			AppName:   "Cover Run",
			ConfigDir: t.TempDir(),
		})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("RunContext with a canceled context should surface the context error")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("RunContext did not return after context cancellation")
	}
}

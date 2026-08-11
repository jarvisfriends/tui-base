// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package logging

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Local aliases for the log-output modes so the literals stay in one place.
const (
	modeDir  = "dir"
	modeFile = "file"
)

// setTempDir redirects os.TempDir to dir on every OS: Unix reads TMPDIR,
// Windows reads TMP and then TEMP.
func setTempDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
}

func TestSetAppNameChangesPrefixAndIgnoresEmpty(t *testing.T) {
	resetLoggerState(t)

	SetAppName("cover-app")
	appNameMu.RLock()
	got := logAppName
	appNameMu.RUnlock()
	if got != "cover-app" {
		t.Fatalf("logAppName = %q; want %q", got, "cover-app")
	}

	SetAppName("")
	appNameMu.RLock()
	got = logAppName
	appNameMu.RUnlock()
	if got != "cover-app" {
		t.Fatalf("empty SetAppName must be ignored; logAppName = %q", got)
	}
}

func TestInitFromSettingsDirMode(t *testing.T) {
	// TempDir first: cleanups run LIFO, so its RemoveAll must be registered
	// before the ones that close the log handle. Windows refuses to delete a
	// directory that still holds an open file.
	dir := t.TempDir()
	resetLoggerState(t)
	t.Cleanup(closeActiveLog)

	SetAppName("dirmode")
	path, err := InitFromSettings(modeDir, dir)
	if err != nil {
		t.Fatalf("InitFromSettings(dir): %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("log file %q not under %q", path, dir)
	}
	if base := filepath.Base(path); !strings.HasPrefix(base, "dirmode-") {
		t.Fatalf("log file name %q missing app prefix", base)
	}

	// Re-initializing closes the previous handle before replacing it.
	if _, err := InitFromSettings(modeDir, dir); err != nil {
		t.Fatalf("second InitFromSettings(dir): %v", err)
	}
}

func TestInitFromSettingsDefaultTempMode(t *testing.T) {
	tmp := t.TempDir() // registered before the log-closing cleanups; see DirMode
	resetLoggerState(t)
	t.Cleanup(closeActiveLog)

	setTempDir(t, tmp)
	SetAppName("tempmode")
	path, err := InitFromSettings("", "")
	if err != nil {
		t.Fatalf("InitFromSettings(default): %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(tmp, "tempmode-logs")) {
		t.Fatalf("default log file %q not under %q", path, filepath.Join(tmp, "tempmode-logs"))
	}
}

func TestInitFromSettingsErrorPaths(t *testing.T) {
	resetLoggerState(t)
	t.Cleanup(closeActiveLog)

	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		mode    string
		logPath string
	}{
		{"dir mode requires a path", modeDir, ""},
		{"dir mode fails when the dir cannot be created", modeDir, filepath.Join(blocker, "sub")},
		{"file mode requires a path", modeFile, ""},
		{"file mode fails when the parent cannot be created", modeFile, filepath.Join(blocker, "sub", "x.log")},
		{"file mode fails when the target is a directory", modeFile, dir},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := InitFromSettings(tc.mode, tc.logPath); err == nil {
				t.Fatalf("InitFromSettings(%q, %q) should fail", tc.mode, tc.logPath)
			}
		})
	}

	// Default mode fails when the temp dir cannot host the log subdirectory.
	setTempDir(t, blocker)
	if _, err := InitFromSettings("", ""); err == nil {
		t.Fatal("default mode should fail when the temp dir is unusable")
	}
}

// TestRotationDegradesWhenReopenFails: when rotation cannot reopen the log
// target (its directory vanished), the logger drops to subscribers-only
// instead of crashing.
func TestRotationDegradesWhenReopenFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The setup needs the log directory deleted while the logger still
		// holds the file open. Windows opens files without FILE_SHARE_DELETE,
		// so the OS refuses that removal and the scenario cannot be staged.
		t.Skip("cannot delete a directory holding an open file on Windows")
	}
	root := t.TempDir() // registered before the log-closing cleanups; see DirMode
	resetLoggerState(t)
	t.Cleanup(closeActiveLog)

	logDir := filepath.Join(root, "logs")
	target := filepath.Join(logDir, "app.log")
	if _, err := InitFromSettings(modeFile, target); err != nil {
		t.Fatalf("InitFromSettings: %v", err)
	}
	SetMaxLogBytes(1)
	t.Cleanup(func() { SetMaxLogBytes(10 << 20) })

	if err := os.RemoveAll(logDir); err != nil {
		t.Fatal(err)
	}
	Errorf("forces a rotation whose reopen fails")

	writeMu.Lock()
	degraded := outFile == nil
	writeMu.Unlock()
	if !degraded {
		t.Fatal("rotation with an unreachable target should degrade to subscribers-only")
	}
	// Further writes must be safe in the degraded state.
	Errorf("still safe without a file")
}

func TestGetLevelFallsBackToInfoOnUnknownLevel(t *testing.T) {
	resetLoggerState(t)

	levelMu.Lock()
	minLevel = 42
	levelMu.Unlock()
	if got := GetLevel(); got != "INFO" {
		t.Fatalf("GetLevel() with unknown minLevel = %q; want INFO", got)
	}
}

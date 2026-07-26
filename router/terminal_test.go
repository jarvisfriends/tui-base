// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"strings"
	"testing"
)

const (
	// reasonModern is the substring every "already good terminal" decision reports.
	reasonModern = "modern terminal"
	// wtExe is a stand-in wt.exe path for the decision tests.
	wtExe = `C:\wt.exe`
)

// envFunc builds an os.Getenv-style lookup from a map so the relaunch decision
// can be exercised without touching the real process environment.
func envFunc(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestShouldRelaunchToWindowsTerminal(t *testing.T) {
	// base is the happy path: a legacy Windows console with wt.exe installed.
	// Each case starts from it and overrides just the field under test, which
	// keeps the shared literals in one place.
	base := func(mut func(*terminalEnv)) terminalEnv {
		e := terminalEnv{
			goos:          goosWindows,
			lookup:        envFunc(nil),
			interactive:   true,
			legacyConsole: true,
			wtPath:        wtExe,
		}
		if mut != nil {
			mut(&e)
		}
		return e
	}
	withEnv := func(m map[string]string) func(*terminalEnv) {
		return func(e *terminalEnv) { e.lookup = envFunc(m) }
	}

	tests := []struct {
		name    string
		env     terminalEnv
		want    bool
		wantSub string // substring the reason must contain
	}{
		{"legacy console relaunches", base(nil), true, "legacy console"},
		{
			"non-windows never relaunches",
			base(func(e *terminalEnv) { e.goos = "linux" }),
			false,
			"not windows",
		},
		{
			"disable flag stops relaunch",
			base(func(e *terminalEnv) { e.disable = true }),
			false,
			"disabled by option",
		},
		{
			"disabled by env",
			base(withEnv(map[string]string{NoTerminalRelaunchEnv: "1"})),
			false,
			NoTerminalRelaunchEnv,
		},
		{
			"guard env stops loop",
			base(withEnv(map[string]string{terminalRelaunchGuardEnv: "1"})),
			false,
			"already relaunched",
		},
		{
			"already in windows terminal",
			base(withEnv(map[string]string{"WT_SESSION": "abc"})),
			false,
			reasonModern,
		},
		{
			"vscode terminal is modern",
			base(withEnv(map[string]string{"TERM_PROGRAM": "vscode"})),
			false,
			reasonModern,
		},
		{
			"ssh session is left in place",
			base(withEnv(map[string]string{"SSH_TTY": "/dev/pts/0"})),
			false,
			reasonModern,
		},
		{
			"git bash TERM is modern",
			base(withEnv(map[string]string{"TERM": "xterm-256color"})),
			false,
			reasonModern,
		},
		{
			"non-interactive stays put",
			base(func(e *terminalEnv) { e.interactive = false }),
			false,
			"not an interactive terminal",
		},
		{
			// The default-terminal delegation case: Windows Terminal already
			// hosts the session via ConPTY but sets no env markers, so only the
			// console-window check can tell. Regression for the duplicate-window
			// bug where every double-click opened a second WT window.
			"conpty-hosted delegation stays put",
			base(func(e *terminalEnv) { e.legacyConsole = false }),
			false,
			"not a legacy conhost window",
		},
		{
			"no wt.exe to relaunch into",
			base(func(e *terminalEnv) { e.wtPath = "" }),
			false,
			"wt.exe not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := shouldRelaunchToWindowsTerminal(tc.env)
			if got != tc.want {
				t.Fatalf("relaunch = %v, want %v (reason %q)", got, tc.want, reason)
			}
			if !strings.Contains(reason, tc.wantSub) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.wantSub)
			}
		})
	}
}

func TestModernHostMarker(t *testing.T) {
	if got := modernHostMarker(envFunc(nil)); got != "" {
		t.Fatalf("bare console should have no marker, got %q", got)
	}
	if got := modernHostMarker(envFunc(map[string]string{"TERM": "dumb"})); got != "" {
		t.Fatalf("TERM=dumb is not a modern host, got %q", got)
	}
	const conEmu = "ConEmuPID"
	if got := modernHostMarker(envFunc(map[string]string{conEmu: "123"})); got != conEmu {
		t.Fatalf("ConEmu should be detected, got %q", got)
	}
}

func TestTruthy(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on", " y "} {
		if !truthy(v) {
			t.Errorf("truthy(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "off", "maybe"} {
		if truthy(v) {
			t.Errorf("truthy(%q) = true, want false", v)
		}
	}
}

// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/x/term"
)

// Environment variables that govern the Windows Terminal auto-relaunch.
const (
	// NoTerminalRelaunchEnv disables the automatic relaunch into Windows
	// Terminal when set to a truthy value (1/true/yes/on). It is the runtime
	// escape hatch that mirrors [Options.DisableTerminalRelaunch].
	NoTerminalRelaunchEnv = "TUI_BASE_NO_WT_RELAUNCH"

	// terminalRelaunchGuardEnv is set on the relaunched child process so a
	// second relaunch is never attempted, even in the unlikely case that the
	// child's terminal is not yet reporting WT_SESSION when it starts.
	terminalRelaunchGuardEnv = "TUI_BASE_WT_RELAUNCHED"
)

// goosWindows is runtime.GOOS on Windows; named so the relaunch gate and its
// tests share one literal.
const goosWindows = "windows"

// TerminalRelaunchConfig controls [MaybeRelaunchInWindowsTerminal].
type TerminalRelaunchConfig struct {
	// AppName titles the new Windows Terminal tab. It may be empty.
	AppName string
	// ProfileName, when set and a matching Windows Terminal profile is
	// installed (e.g. via [InstallWindowsTerminalProfile]), makes the relaunch
	// open that profile with `wt --profile`, so the tab shows the profile's
	// icon and colors. It is ignored when no such profile is installed, so the
	// relaunch still works. (Windows Terminal has no per-tab icon flag; the
	// icon can only come from a profile.)
	ProfileName string
	// Disable turns the relaunch off regardless of the environment. It is the
	// programmatic equivalent of the TUI_BASE_NO_WT_RELAUNCH env var.
	Disable bool
}

// MaybeRelaunchInWindowsTerminal relaunches the current process inside Windows
// Terminal when it detects that it was started under the legacy Windows console
// (conhost) instead. Windows Terminal is a full VT/xterm host, so moving into
// it unlocks the truecolor, mouse, and styling features the Charm v2 stack
// relies on — features conhost silently drops. It exists because the per-user
// default-terminal setting (the DelegationConsole/DelegationTerminal registry
// values) is known to be reset on some machines, which drops apps back into
// conhost. This relaunch is the per-session guard; the
// github.com/jarvisfriends/tui-base/winterm package reads and writes the
// setting itself (surfaced as "Default Terminal" on the settings page).
//
// It returns relaunched=true only when a new Windows Terminal process was
// started; the caller must then stop and let this process exit so the two do
// not run against the same console concurrently:
//
//	if relaunched, _ := router.MaybeRelaunchInWindowsTerminal(cfg); relaunched {
//	    return
//	}
//
// It is a deliberate no-op (returns false, nil) when a relaunch is unnecessary
// or unsafe: on non-Windows platforms, when already inside Windows Terminal or
// another known terminal emulator, over SSH, in a non-interactive (piped or
// redirected) session, when wt.exe is not installed, or when disabled via cfg
// or TUI_BASE_NO_WT_RELAUNCH. Relaunch failures are returned as err with
// relaunched=false so callers can simply continue in the current console.
func MaybeRelaunchInWindowsTerminal(cfg TerminalRelaunchConfig) (relaunched bool, err error) {
	env := terminalEnv{
		goos:          runtime.GOOS,
		lookup:        os.Getenv,
		interactive:   interactiveStdio(),
		legacyConsole: legacyConsoleHost(),
		wtPath:        lookupWindowsTerminal(),
		disable:       cfg.Disable,
	}
	if ok, _ := shouldRelaunchToWindowsTerminal(env); !ok {
		return false, nil
	}
	if err := launchWindowsTerminal(env.wtPath, cfg.AppName, cfg.ProfileName); err != nil {
		return false, err
	}
	return true, nil
}

// terminalEnv is the fully resolved view of the runtime that the relaunch
// decision depends on. Collecting it into a struct keeps
// [shouldRelaunchToWindowsTerminal] pure and table-testable on every platform.
type terminalEnv struct {
	goos        string
	lookup      func(string) string
	interactive bool
	// legacyConsole is true only when the process's console is a classic
	// conhost window (see legacyConsoleHost). ConPTY-hosted sessions — including
	// Windows Terminal reached via the default-terminal delegation, which sets
	// no env markers — report false.
	legacyConsole bool
	wtPath        string
	disable       bool
}

// shouldRelaunchToWindowsTerminal reports whether the process looks like a
// legacy Windows console session that should be moved into Windows Terminal,
// plus a short human-readable reason for the decision (used in tests and
// available for diagnostics).
func shouldRelaunchToWindowsTerminal(e terminalEnv) (relaunch bool, reason string) {
	switch {
	case e.goos != goosWindows:
		return false, "not windows"
	case e.disable:
		return false, "disabled by option"
	case truthy(e.lookup(NoTerminalRelaunchEnv)):
		return false, "disabled by " + NoTerminalRelaunchEnv
	case e.lookup(terminalRelaunchGuardEnv) != "":
		return false, "already relaunched"
	}
	if host := modernHostMarker(e.lookup); host != "" {
		return false, "modern terminal detected (" + host + ")"
	}
	switch {
	case !e.interactive:
		return false, "not an interactive terminal"
	case !e.legacyConsole:
		return false, "not a legacy conhost window (ConPTY-hosted or no console)"
	case e.wtPath == "":
		return false, "wt.exe not found"
	}
	return true, "legacy console; wt.exe available"
}

// modernHostMarker returns the environment key that identifies an already-good
// terminal host, or "" when none is found. A bare conhost/cmd.exe session sets
// none of these, so their absence is the signal that a relaunch is warranted.
// SSH markers are included so a remote session is never yanked into a local GUI
// window.
func modernHostMarker(get func(string) string) string {
	for _, key := range []string{
		"WT_SESSION", "WT_PROFILE_ID", // Windows Terminal
		"TERM_PROGRAM",                       // VS Code, WezTerm, Tabby, Hyper, ...
		"WEZTERM_PANE", "WEZTERM_EXECUTABLE", // WezTerm
		"ConEmuPID", "ConEmuBuild", // ConEmu / Cmder
		"ALACRITTY_SOCKET", "ALACRITTY_WINDOW_ID", "ALACRITTY_LOG", // Alacritty
		"SSH_TTY", "SSH_CONNECTION", // remote session
	} {
		if get(key) != "" {
			return key
		}
	}
	// mintty/Git Bash/Cygwin/MSYS and VT terminals export a real TERM; conhost
	// leaves it unset. "dumb" is treated as no terminal.
	if t := get("TERM"); t != "" && t != "dumb" {
		return "TERM=" + t
	}
	return ""
}

// interactiveStdio reports whether both stdin and stdout are attached to a
// terminal. When either is redirected (a pipe, a file, a CI runner) a relaunch
// would break the caller's expectations, so the app stays in place.
func interactiveStdio() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

// truthy reports whether an env-var value means "on".
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "y":
		return true
	}
	return false
}

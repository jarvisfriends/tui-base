// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

//go:build windows

package router

import (
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetConsoleWindow = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")
	procGetClassNameW    = windows.NewLazySystemDLL("user32.dll").NewProc("GetClassNameW")
)

// legacyConsoleHost reports whether this process's console is hosted by the
// classic conhost (window class "ConsoleWindowClass"). Every ConPTY-backed host
// — Windows Terminal, the default-terminal delegation path, VS Code — exposes a
// "PseudoConsoleWindow" instead, and a process with no console has no window at
// all; both mean the session is already modern (or not a console) and must not
// be relaunched.
//
// This check exists because environment markers cannot detect the delegation
// case: when Windows Terminal is the user's default terminal, a double-clicked
// app is hosted by WT but inherits Explorer's environment, so WT_SESSION is
// absent and env-based detection would relaunch into a duplicate window.
func legacyConsoleHost() bool {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return false
	}
	var class [64]uint16
	n, _, _ := procGetClassNameW.Call(
		hwnd,
		uintptr(unsafe.Pointer(&class[0])),
		uintptr(len(class)),
	)
	return windows.UTF16ToString(class[:n]) == "ConsoleWindowClass"
}

// lookupWindowsTerminal resolves wt.exe on PATH (Windows Terminal installs a
// launcher alias under %LOCALAPPDATA%\Microsoft\WindowsApps, which is on PATH),
// returning "" when it is not installed.
func lookupWindowsTerminal() string {
	p, err := exec.LookPath("wt.exe")
	if err != nil {
		return ""
	}
	return p
}

// windowsTerminalProfileInstalled reports whether a profile fragment for
// profileName was installed by InstallWindowsTerminalProfile. It guards the
// --profile flag so the relaunch never names a profile Windows Terminal does
// not know about (which would make wt error instead of launching).
func windowsTerminalProfileInstalled(profileName string) bool {
	dir, err := windowsTerminalFragmentDir(profileName)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, fragmentFileName(profileName)))
	return err == nil
}

// launchWindowsTerminal starts wt.exe running this same executable — with the
// same arguments and working directory — in a fresh Windows Terminal window,
// then returns without waiting. The guard env var prevents the child from
// relaunching itself again.
//
// `-w new` forces a brand-new window rather than adding a tab to an unrelated
// existing one; `--profile` opens the app's installed profile (for its icon and
// colors) when one exists; everything from the executable onward is passed
// through to the new tab's command line.
//
// Note: Windows Terminal has no per-tab icon argument — the tab icon comes from
// the profile, which is why branding the tab means launching under a profile
// rather than passing an icon path here.
func launchWindowsTerminal(wtPath, appName, profileName string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{"-w", "new", "new-tab"}
	if profileName != "" && windowsTerminalProfileInstalled(profileName) {
		args = append(args, "--profile", profileName)
	}
	if appName != "" {
		args = append(args, "--title", appName)
	}
	args = append(args, exe)
	args = append(args, os.Args[1:]...)

	cmd := exec.Command(wtPath, args...)
	cmd.Env = append(os.Environ(), terminalRelaunchGuardEnv+"=1")
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}
	return cmd.Start()
}

//go:build !windows

package router

import "errors"

// lookupWindowsTerminal always reports "not found" off Windows, which short-
// circuits [shouldRelaunchToWindowsTerminal] before any launch is attempted.
func lookupWindowsTerminal() string { return "" }

// legacyConsoleHost is Windows-only; there is no conhost elsewhere.
func legacyConsoleHost() bool { return false }

// launchWindowsTerminal is never reached off Windows (the decision gate returns
// early), but it exists so the package compiles on every platform.
func launchWindowsTerminal(_ /*wtPath*/, _ /*appName*/, _ /*profileName*/ string) error {
	return errors.New("windows terminal relaunch is only supported on windows")
}

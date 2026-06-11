//go:build !windows

package settings

// detectDefaultTerminal is a no-op on non-Windows platforms.
func detectDefaultTerminal() (string, error) { return "", nil }

// applyTerminalSetting is a no-op on non-Windows platforms.
func applyTerminalSetting(consoleGUID, terminalGUID string) error { return nil }

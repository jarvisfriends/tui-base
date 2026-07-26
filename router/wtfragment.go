// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// WindowsTerminalProfile describes a Windows Terminal profile fragment: a small
// JSON file that makes an app appear in Windows Terminal's new-tab dropdown with
// its own name and icon, and gives tabs launched from it that icon regardless of
// how the app was started. Unlike the relaunch --icon (which only brands tabs
// this process spawns), a fragment is persistent and user-visible — it is the
// documented way to make a terminal app easy to find and consistently branded.
//
// Install it once (e.g. from an installer, a first-run step, or an
// `--install-terminal-profile` flag), not on every launch.
type WindowsTerminalProfile struct {
	// AppName is the profile name shown in Windows Terminal and the fragment's
	// source folder. Required.
	AppName string
	// Commandline is the command Windows Terminal runs for the profile.
	// Defaults to the current executable (quoted) when empty.
	Commandline string
	// IconPath is an existing image file (PNG recommended) used as the profile
	// icon. When empty and IconData is set, the bytes are written next to the
	// fragment and referenced automatically.
	IconPath string
	// IconData, used only when IconPath is empty, is written to a durable file
	// alongside the fragment and referenced as the icon.
	IconData []byte
	// StartingDirectory for the profile (optional, e.g. "%USERPROFILE%").
	StartingDirectory string
}

// wtFragment/wtProfile mirror the subset of the Windows Terminal fragment schema
// tui-base emits. Omitting the profile GUID lets Windows Terminal derive a
// stable one from the fragment source and profile name, so reinstalling does not
// create duplicates.
type wtFragment struct {
	Profiles []wtProfile `json:"profiles"`
}

type wtProfile struct {
	Name              string `json:"name"`
	Commandline       string `json:"commandline"`
	Icon              string `json:"icon,omitempty"`
	StartingDirectory string `json:"startingDirectory,omitempty"`
}

// InstallWindowsTerminalProfile writes p as a Windows Terminal fragment under
// %LOCALAPPDATA%\Microsoft\Windows Terminal\Fragments\<AppName>\ and returns the
// path of the fragment file. When p.IconPath is empty and p.IconData is set, the
// icon bytes are written into that folder and referenced by the profile.
//
// It errors on non-Windows platforms and when AppName is empty.
func InstallWindowsTerminalProfile(p WindowsTerminalProfile) (string, error) {
	if runtime.GOOS != goosWindows {
		return "", errors.New("windows terminal fragments are only supported on windows")
	}
	if strings.TrimSpace(p.AppName) == "" {
		return "", errors.New("windows terminal fragment: AppName is required")
	}
	dir, dirErr := windowsTerminalFragmentDir(p.AppName)
	if dirErr != nil {
		return "", dirErr
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}

	if p.Commandline == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		p.Commandline = quoteCommand(exe)
	}
	if p.IconPath == "" && len(p.IconData) > 0 {
		iconFile := filepath.Join(dir, "icon.png")
		if err := os.WriteFile(iconFile, p.IconData, 0o600); err != nil {
			return "", err
		}
		p.IconPath = iconFile
	}

	data, jsonErr := fragmentJSON(p)
	if jsonErr != nil {
		return "", jsonErr
	}
	out := filepath.Join(dir, fragmentFileName(p.AppName))
	if err := os.WriteFile(out, data, 0o600); err != nil {
		return "", err
	}
	return out, nil
}

// UninstallWindowsTerminalProfile removes the fragment folder previously written
// by InstallWindowsTerminalProfile for appName. It is a no-op if the folder does
// not exist.
func UninstallWindowsTerminalProfile(appName string) error {
	if runtime.GOOS != goosWindows {
		return errors.New("windows terminal fragments are only supported on windows")
	}
	dir, err := windowsTerminalFragmentDir(appName)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// windowsTerminalFragmentDir returns the per-app fragment directory under
// LOCALAPPDATA. It is separated out so the path logic is unit-testable via the
// LOCALAPPDATA environment variable.
func windowsTerminalFragmentDir(appName string) (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(base, "Microsoft", "Windows Terminal", "Fragments", appName), nil
}

// fragmentJSON renders p as an indented Windows Terminal fragment document.
func fragmentJSON(p WindowsTerminalProfile) ([]byte, error) {
	frag := wtFragment{Profiles: []wtProfile{{
		Name:              p.AppName,
		Commandline:       p.Commandline,
		Icon:              p.IconPath,
		StartingDirectory: p.StartingDirectory,
	}}}
	return json.MarshalIndent(frag, "", "  ")
}

// fragmentFileName derives a safe fragment filename from the app name, e.g.
// "TUI Base" -> "tui-base.json".
func fragmentFileName(appName string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(appName)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "app"
	}
	return name + ".json"
}

// quoteCommand wraps a path in double quotes when it contains spaces, matching
// how Windows Terminal expects commandline values.
func quoteCommand(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `"` + path + `"`
	}
	return path
}

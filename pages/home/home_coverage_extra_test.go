// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package home

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jarvisfriends/snap/styles"
	"github.com/jarvisfriends/snap/uifx"

	tea "charm.land/bubbletea/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

// fakeOpener puts a no-op executable named like the platform's URL/path opener
// (xdg-open off Windows/macOS) first on PATH so openBrowserCmd/openPathCmd can
// run their success path without touching a real browser or file manager.
//
// Windows has no stubbable equivalent: openBrowserCmd shells out to rundll32
// and fileManager resolves explorer.exe by absolute path under %SystemRoot%,
// and CreateProcess will not launch a shell script standing in for either. The
// success-path tests skip there; the failure path still runs on every OS.
func fakeOpener(t *testing.T) {
	t.Helper()
	if runtime.GOOS == osWindows {
		t.Skip("no stubbable opener on Windows: rundll32/explorer.exe are real binaries")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, fileManager())
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil { //nolint:gosec // test helper must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// missingOpener points every lookup the openers use at an empty directory, so
// they resolve to nothing on any OS. PATH covers xdg-open/open/rundll32;
// SystemRoot and windir cover the absolute explorer.exe fileManager builds.
func missingOpener(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("SystemRoot", dir)
	t.Setenv("windir", dir)
}

func TestOpenBrowserCmdSuccess(t *testing.T) {
	fakeOpener(t)

	msg := openBrowserCmd("https://example.invalid/theme")()
	opened, ok := msg.(browserOpenedMsg)
	if !ok {
		t.Fatalf("got %T, want browserOpenedMsg", msg)
	}
	if opened.Err != nil {
		t.Fatalf("stubbed opener should succeed, got %v", opened.Err)
	}
}

func TestOpenBrowserCmdFailure(t *testing.T) {
	missingOpener(t)

	msg := openBrowserCmd("https://example.invalid/theme")()
	opened, ok := msg.(browserOpenedMsg)
	if !ok || opened.Err == nil {
		t.Fatalf("missing opener should report an error, got %#v", msg)
	}
}

func TestOpenPathCmdSuccess(t *testing.T) {
	fakeOpener(t)

	msg := openPathCmd("/")()
	opened, ok := msg.(browserOpenedMsg)
	if !ok {
		t.Fatalf("got %T, want browserOpenedMsg", msg)
	}
	if opened.Err != nil {
		t.Fatalf("stubbed file manager should succeed, got %v", opened.Err)
	}
}

func TestOpenPathCmdFailure(t *testing.T) {
	missingOpener(t)

	msg := openPathCmd("/")()
	opened, ok := msg.(browserOpenedMsg)
	if !ok || opened.Err == nil {
		t.Fatalf("missing file manager should report an error, got %#v", msg)
	}
}

// TestThemeSourceBranches drives themeSource/themeURLLine/openThemeSourceCmd
// through every attribution shape: no tint, an empty link, and a full credit.
func TestThemeSourceBranches(t *testing.T) {
	m := sized(t, 100, 40)

	// No original tint: nothing to attribute.
	noTint := *styles.Active()
	noTint.OrigTint = nil
	m.SetColors(&noTint)
	if _, _, ok := m.themeSource(); ok {
		t.Error("themeSource without OrigTint should report ok=false")
	}
	if m.themeURLLine() != "" {
		t.Error("themeURLLine without a source should be empty")
	}
	if m.openThemeSourceCmd() != nil {
		t.Error("openThemeSourceCmd without a source should be nil")
	}

	// A credit entry whose link is empty is treated as no attribution.
	emptyLink := *styles.Active()
	emptyLink.OrigTint = &tint.Tint{CreditSources: []*tint.CreditSource{{Name: "n", Link: ""}}}
	m.SetColors(&emptyLink)
	if _, _, ok := m.themeSource(); ok {
		t.Error("themeSource with an empty link should report ok=false")
	}

	// A real credit renders the hyperlink line and yields an open command.
	withLink := *styles.Active()
	withLink.OrigTint = &tint.Tint{
		ID:            "cover_tint",
		CreditSources: []*tint.CreditSource{{Name: "Source", Link: "https://example.invalid"}},
	}
	m.SetColors(&withLink)
	if line := m.themeURLLine(); line == "" {
		t.Error("themeURLLine with a source should render")
	}
	if m.openThemeSourceCmd() == nil {
		t.Error("openThemeSourceCmd with a source should return a command")
	}
}

func TestProgressViewEdgeStyles(t *testing.T) {
	m := sized(t, 100, 40)

	// Unknown style falls through to an empty bar.
	m.progStyle = progressStyle(99)
	if got := m.progressView(20); got != "" {
		t.Errorf("unknown style rendered %q", got)
	}
	if got := m.progStyle.String(); got != "Unknown" {
		t.Errorf("unknown style String() = %q", got)
	}

	// The zones style walks its color ramp (error/warn/success thresholds).
	m.progStyle = progressZones
	if got := m.progressView(20); got == "" {
		t.Error("zones style should render")
	}

	// Classic style at zero percent renders the empty gradient track.
	if got := gradientBar(0, 10, styles.Active().Accent, styles.Active().Success); got == "" {
		t.Error("empty gradient bar should still render track cells")
	}
	if got := gradientBar(50, 0, styles.Active().Accent, styles.Active().Success); got != "" {
		t.Errorf("zero-width gradient bar rendered %q", got)
	}

	// Unknown effects level offers no styles.
	if got := availableProgressStyles(uifx.Level(99)); got != nil {
		t.Errorf("unknown level styles = %v", got)
	}

	// step feeds the animated model only when the animated style is active.
	m.progStyle = progressAnimated
	if m.step() == nil {
		t.Error("animated step should return the spring command")
	}
}

func TestOffsetAtBranches(t *testing.T) {
	tests := []struct {
		name                         string
		y, barHeight, total, visible int
		want                         int
		wantAtMost                   int
		exact                        bool
	}{
		{"fits entirely", 3, 10, 5, 5, 0, 0, true},
		{"zero bar height", 3, 0, 100, 10, 0, 0, true},
		{"thumb fills track", 0, 1, 2, 1, 0, 0, true},
		{"mid track", 5, 10, 100, 10, 0, 90, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := offsetAt(tc.y, tc.barHeight, tc.total, tc.visible)
			if tc.exact && got != tc.want {
				t.Errorf("offsetAt = %d, want %d", got, tc.want)
			}
			if !tc.exact && (got < 0 || got > tc.wantAtMost) {
				t.Errorf("offsetAt = %d, want within [0,%d]", got, tc.wantAtMost)
			}
		})
	}
}

func TestUpdateForwardsUnknownMsgToViewport(t *testing.T) {
	m := sized(t, 100, 40)
	type unrelatedMsg struct{}
	if _, cmd := m.Update(unrelatedMsg{}); cmd != nil {
		t.Errorf("unknown message produced a command: %v", cmd)
	}
}

func TestTickCmdDeliversTickMsg(t *testing.T) {
	m := New()
	cmd := m.tickCmd()
	if cmd == nil {
		t.Fatal("tickCmd should return a command")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatal("tick command should deliver a tickMsg")
	}
}

func TestRightClickMisses(t *testing.T) {
	m := sized(t, 100, 40)

	// Right-click outside every zone is ignored.
	if cmd := m.onClick(tea.Mouse{X: 99, Y: 39, Button: tea.MouseRight}); cmd != nil {
		t.Error("right-click outside all zones should be a no-op")
	}
}

// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package tuibase

import (
	"testing"

	"github.com/jarvisfriends/snap/keys"

	"github.com/jarvisfriends/tui-base/config"
)

// TestEveryScalarOptionSetsItsField pins the one-line With* options that
// TestApplyOptionsLayersOnStruct doesn't exercise, so a renamed or re-wired
// field can't silently stop applying.
func TestEveryScalarOptionSetsItsField(t *testing.T) {
	t.Parallel()

	km := keys.DefaultKeyMap()
	sections := []config.Section[string]{{Title: "App"}}
	got := applyOptions(Options{}, []Option{
		WithAppVersion("1.2.3"),
		WithConfigDirName("my-app"),
		WithConfigDir(`C:\tmp\cfg`),
		WithDefaultPage("Reports"),
		WithInitialLogLevel("DEBUG"),
		WithSettingsSections(sections...),
		WithKeyMap(km),
	})

	if got.AppVersion != "1.2.3" {
		t.Errorf("AppVersion = %q", got.AppVersion)
	}
	if got.ConfigDirName != "my-app" {
		t.Errorf("ConfigDirName = %q", got.ConfigDirName)
	}
	if got.ConfigDir != `C:\tmp\cfg` {
		t.Errorf("ConfigDir = %q", got.ConfigDir)
	}
	if got.DefaultPage != "Reports" {
		t.Errorf("DefaultPage = %q", got.DefaultPage)
	}
	if got.InitialLogLevel != "DEBUG" {
		t.Errorf("InitialLogLevel = %q", got.InitialLogLevel)
	}
	if len(got.SettingsSections) != 1 || got.SettingsSections[0].Title != "App" {
		t.Errorf("SettingsSections = %+v", got.SettingsSections)
	}
	if got.KeyMap != km {
		t.Error("KeyMap not applied")
	}

	// WithSettingsSections appends rather than replaces.
	got = applyOptions(Options{SettingsSections: sections},
		[]Option{WithSettingsSections(config.Section[string]{Title: "More"})})
	if len(got.SettingsSections) != 2 {
		t.Errorf("SettingsSections should append; got %d", len(got.SettingsSections))
	}
}

// TestNewBuildsRouter: the New/NewWithOptions constructors produce a working
// router with the built-in pages plus the app's own.
func TestNewBuildsRouter(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(
		Options{ConfigDir: t.TempDir()},
		WithAppName("Cover App"),
		WithPages(RegisteredPage{Title: "Extra", Model: stubDebugModel{}}),
	)
	if m == nil {
		t.Fatal("NewWithOptions returned nil")
	}

	m2 := New(WithConfigDir(t.TempDir()))
	if m2 == nil {
		t.Fatal("New returned nil")
	}
}
